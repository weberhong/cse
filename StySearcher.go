package main

import (
	"errors"
	"fmt"
	simplejson "github.com/bitly/go-simplejson"
	. "github.com/getwe/goose"
	"github.com/getwe/goose/config"
	. "github.com/getwe/goose/database"
	log "github.com/getwe/goose/log"
	. "github.com/getwe/goose/utils"
	"github.com/getwe/scws4go"
	"runtime"
	"sort"
)

// 策略的自定义临时数据
type strategyData struct {
	query string
	pn    int
	rn    int

	isdebug bool

	debug *Debug
}

// 检索的时候,goose框架收到一个完整的网络请求便认为是一次检索请求.
// 框架把收到的整个网络包都传给策略,不关心具体的检索协议.
type StySearcher struct {
	scws *scws4go.Scws

	trieDict *TrieDict

	// 调权字段个数
	adjustWeightFieldCount uint8

	valueBoost []float64
}

// 全局调用一次初始化策略
func (this *StySearcher) Init(conf config.Conf) (err error) {
	// scws初始化
	scwsDictPath := conf.String("Strategy.Searcher.Scws.xdbdict")
	scwsRulePath := conf.String("Strategy.Searcher.Scws.rules")
	scwsForkCnt := runtime.NumCPU()

	log.Debug("Searcher Strategy Init. scws dict[%s] rule[%s] cpu[%d]",
		scwsDictPath, scwsRulePath, scwsForkCnt)

	this.scws = scws4go.NewScws()
	err = this.scws.SetDict(scwsDictPath, scws4go.SCWS_XDICT_XDB|scws4go.SCWS_XDICT_MEM)
	if err != nil {
		return
	}
	err = this.scws.SetRule(scwsRulePath)
	if err != nil {
		return
	}
	this.scws.SetCharset("utf8")
	this.scws.SetIgnore(1)
	this.scws.SetMulti(scws4go.SCWS_MULTI_SHORT & scws4go.SCWS_MULTI_DUALITY & scws4go.SCWS_MULTI_ZMAIN)
	err = this.scws.Init(scwsForkCnt)
	if err != nil {
		return
	}

	// treedict
	trieDictDataPath := conf.String("Strategy.Searcher.TrieDict.DataFile")
	trieDictPath := conf.String("Strategy.Searcher.TrieDict.DictFile")
	this.trieDict, _ = NewTrieDict(trieDictDataPath, trieDictPath)

	// AdjustWeightFieldCount
	this.adjustWeightFieldCount = uint8(conf.Int64("Strategy.AdjustWeightFieldCount"))
	/*
		// 允许没有调权
		if this.adjustWeightFieldCount == 0 {
			return log.Error("AdjustWeightFieldCount[%d] illegal",
				this.adjustWeightFieldCount)
		}
	*/

	// valueBoost 调权参数权重
	this.valueBoost = conf.Float64Array("Strategy.ValueBoost")
	if len(this.valueBoost) != int(this.adjustWeightFieldCount) {
		return log.Error("ValueBoost %v length illegal,AdjustWeightFieldCount[%d]",
			this.valueBoost, this.adjustWeightFieldCount)
	}
	log.Debug("Strategy.ValueBoost : %v", this.valueBoost)

	return
}

// 解析请求
// 返回term列表,一个由策略决定的任意数据,后续接口都会透传
func (this *StySearcher) ParseQuery(request []byte,
	context *StyContext) ([]TermInQuery, interface{}, error) {

	// 策略在多个接口之间传递的数据
	styData := &strategyData{}

	// 解析命令
	searchReq, err := simplejson.NewJson(request)
	if err != nil {
		log.Warn(err)
		return nil, nil, err
	}

	termInQuery, err := this.parseQuery(searchReq, context, styData)
	if err != nil {
		return nil, nil, err
	}
	return termInQuery, styData, nil
}

// 对一个结果进行打分,确定相关性
// queryInfo    : ParseQuery策略返回的结构
// inId         : 需要打分的doc的内部id
// outId        : 需求打分的doc的外部id
// termInQuery  : 所有term在query中的打分
// termInDoc    : 所有term在doc中的打分
// termCnt      : term数量
// Weight       : 返回doc的相关性得分
// 返回错误当前结果则丢弃
// @NOTE query中的term不一定能命中doc,TermInDoc.Weight == 0表示这种情况
func (this *StySearcher) CalWeight(queryInfo interface{}, inId InIdType,
	outId OutIdType, termInQuery []TermInQuery, termInDoc []TermInDoc,
	termCnt uint32, context *StyContext) (TermWeight, error) {

	styData := queryInfo.(*strategyData)
	if styData == nil {
		return 0, errors.New("StrategyData nil")
	}

	queryMatch := this.queryMatch(styData, inId, termInQuery, termInDoc)
	docMatch := this.docMatch(styData, inId, termInQuery, termInDoc)
	omitPunish := this.omitTermPunish(styData, inId, termInQuery, termInDoc)

	weight := queryMatch * docMatch * omitPunish

	styData.debug.AddDocDebugInfo(uint32(inId),
		"bweight[%.3f] = queryMatch[%.3f] * docMatch[%.3f] * omitPunish[%.3f]",
		weight, queryMatch, docMatch, omitPunish)

	return TermWeight(weight * 100 * 100), nil
}

// 构建返回包
func (this *StySearcher) Response(queryInfo interface{},
	list SearchResultList,
	valueReader ValueReader,
	dataReader DataReader,
	response []byte, context *StyContext) (reslen int, err error) {
	/*
	   // from goose
	   type SearchResult struct {
	       InId    InIdType
	       OutId   OutIdType
	       Weight  TermWeight
	   }
	*/

	styData := queryInfo.(*strategyData)
	if styData == nil {
		return 0, errors.New("StrategyData nil")
	}

	// 策略自己定义的拉链
	stylist := make([]csedoc, 0, len(list))
	for _, e := range list {
		stylist = append(stylist, csedoc{
			InId:    e.InId,
			OutId:   e.OutId,
			Bweight: int(e.Weight),
		})
	}

	// 对拉链加载解析Value ( 应该是耗时操作 )
	for i := 0; i < len(stylist); i++ {
		stylist[i].ParseValue(styData, valueReader, this.valueBoost)
	}

	// 类聚去重
	// 先排序整理,按clusterid聚成块
	sort.Sort(GroupByClusterId{stylist})
	// TODO

	// 根据Weight做最终排序
	sort.Sort(WeightSort{stylist})

	return this.buildRes(styData, stylist, dataReader, response, context)
}

// 构建返回包
func (this *StySearcher) buildRes(styData *strategyData, list csedocarray,
	db DataReader, response []byte, context *StyContext) (reslen int, err error) {
	log.Debug("in Response Strategy")

	// 分页
	begin := styData.pn * styData.rn
	if begin > len(list) {
		begin = len(list)
	}
	end := begin + styData.rn
	if end > len(list) {
		end = len(list)
	}
	relist := list[begin:end]
	context.Log.Debug("result list len[%d] range [%d:%d]", len(list), begin, end)

	searchRes, err := simplejson.NewJson([]byte(`{}`))
	if err != nil {
		return 0, errors.New("open json buf fail")
	}

	tmpData := NewData()

	for i, e := range relist {
		// 建库把整个doc当成二进制作为Data,这里取出来后需要重新解析
		err := db.ReadData(e.InId, &tmpData)
		if err != nil {
			context.Log.Warn("ReadData fail[%s] InId[%d] OutId[%d]", err, e.InId, e.OutId)
			continue
		}

		context.Log.Debug("inId[%d] weight[%d]", e.InId, e.Weight)

		doc, _ := simplejson.NewJson([]byte(`{}`))

		// ------------------------------------------------
		data, err := simplejson.NewJson(tmpData)
		if err != nil {
			context.Log.Warn(err)
			continue
		}
		doc.Set("cse_data", data)

		// ------------------------------------------------
		weightInfo, _ := simplejson.NewJson([]byte(`{}`))
		weightInfo.Set("bweight", e.Bweight)
		weightInfo.Set("weight", e.Weight)

		doc.Set("weightInfo", weightInfo)
		// ------------------------------------------------
		if styData.isdebug {
			debugInfo, _ := simplejson.NewJson([]byte(`{}`))
			debugInfo.Set("docDebugLog", styData.debug.GetDocDebugInfo(uint32(e.InId)))

			doc.Set("debugInfo", debugInfo)
		}

		// ------------------------------------------------
		searchRes.Set(fmt.Sprintf("result%d", i), doc)
	}

	searchRes.Set("retNum", len(relist))
	searchRes.Set("dispNum", len(list))

	context.Log.Info("retNum", len(relist))
	context.Log.Info("dispNum", len(list))

	if styData.isdebug {
		debug, _ := simplejson.NewJson([]byte(`{}`))
		debug.Set("queryDebugLog", styData.debug.GetDebugInfo())

		searchRes.Set("debugInfo", debug)
	}

	// 进行序列化
	tmpbuf, err := searchRes.Encode()
	if err != nil {
		return 0, err
	}

	if len(tmpbuf) > cap(response) {
		return 0, errors.New("respone buf too small")
	}

	// 重复了一次内存拷贝!
	copy(response, tmpbuf)

	return len(tmpbuf), nil
}
