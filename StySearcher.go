package main

import (
    . "github.com/getwe/goose"
    . "github.com/getwe/goose/utils"
    . "github.com/getwe/goose/database"
    "github.com/getwe/goose/config"
    "github.com/getwe/scws4go"
    "errors"
    "runtime"
    log         "github.com/getwe/goose/log"
    simplejson  "github.com/bitly/go-simplejson"
    "sort"
    "fmt"
)

// 策略的自定义临时数据
type strategyData struct {
    query   string
    pn      int
    rn      int
}

// 检索的时候,goose框架收到一个完整的网络请求便认为是一次检索请求.
// 框架把收到的整个网络包都传给策略,不关心具体的检索协议.
type StySearcher struct {
    scws        *scws4go.Scws

    trieDict    *TrieDict
}

// 全局调用一次初始化策略
func (this *StySearcher) Init(conf config.Conf) (err error) {
    // scws初始化
    scwsDictPath := conf.String("Strategy.Searcher.Scws.xdbdict")
    scwsRulePath := conf.String("Strategy.Searcher.Scws.rules")
    scwsForkCnt  := runtime.NumCPU()

    log.Debug("Searcher Strategy Init. scws dict[%s] rule[%s] cpu[%d]",
        scwsDictPath,scwsRulePath,scwsForkCnt)

    this.scws = scws4go.NewScws()
    err = this.scws.SetDict(scwsDictPath, scws4go.SCWS_XDICT_XDB|scws4go.SCWS_XDICT_MEM)
    if err != nil { return }
    err = this.scws.SetRule(scwsRulePath)
    if err != nil { return }
    this.scws.SetCharset("utf8")
    this.scws.SetIgnore(1)
    this.scws.SetMulti(scws4go.SCWS_MULTI_SHORT & scws4go.SCWS_MULTI_DUALITY & scws4go.SCWS_MULTI_ZMAIN)
    err = this.scws.Init(scwsForkCnt)
    if err != nil { return }

    // treedict
    trieDictDataPath := conf.String("Strategy.Searcher.TrieDict.DataFile")
    trieDictPath := conf.String("Strategy.Searcher.TrieDict.DictFile")
    this.trieDict,_ = NewTrieDict(trieDictDataPath,trieDictPath)

    return
}

// 解析请求
// 返回term列表,一个由策略决定的任意数据,后续接口都会透传
func (this *StySearcher) ParseQuery(request []byte,
    context *StyContext)([]TermInQuery,interface{},error) {

    // 策略在多个接口之间传递的数据
    styData:= &strategyData{}

    // 解析命令
    searchReq,err := simplejson.NewJson(request)
    if err != nil {
        log.Warn(err)
        return nil,nil,err
    }

    termInQuery,err := this.parseQuery(searchReq,context,styData)
    if err != nil {
        return nil,nil,err
    }
    return termInQuery,styData,nil
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
func (this *StySearcher) CalWeight(queryInfo interface{},inId InIdType,
    outId OutIdType,termInQuery []TermInQuery,termInDoc []TermInDoc,
    termCnt uint32,context *StyContext) (TermWeight,error) {

    queryMatch := this.queryMatch(inId,termInQuery,termInDoc)
    docMatch := this.docMatch(inId,termInQuery,termInDoc)
    omitPunish := this.omitTermPunish(inId,termInQuery,termInDoc)

    weight := queryMatch * docMatch * omitPunish

    return TermWeight( weight * 100 * 100 ),nil
}
// 对结果拉链进行过滤

func (this *StySearcher) Filt(queryInfo interface{},list SearchResultList,
    context *StyContext) (error) {
    log.Debug("in Filt Strategy")
    return nil
}

// 结果调权
// 确认最终结果列表排序
func (this *StySearcher) Adjust(queryInfo interface{},list SearchResultList,
    db ValueReader,context *StyContext) (error) {
    /*
    type SearchResult struct {
        InId    InIdType
        OutId   OutIdType
        Weight  TermWeight
    }
    */


    log.Debug("in Adjust Strategy")
    // 不调权,直接排序返回
    sort.Sort(list)
    return nil
}

// 构建返回包
func (this *StySearcher) Response(queryInfo interface{},list SearchResultList,
    db DataBaseReader,response []byte,context *StyContext) (reslen int,err error) {
    log.Debug("in Response Strategy")

    styData := queryInfo.(*strategyData)
    if styData == nil {
        return 0,errors.New("StrategyData nil")
    }

    // 分页
    begin := styData.pn * styData.rn
    end := begin + styData.rn
    if end > len(list) {
        end = len(list)
    }
    relist := list[begin:end]
    context.Log.Debug("result list len[%d] range [%d:%d]",len(list),begin,end)

    searchRes,err := simplejson.NewJson([]byte(`{}`))
    if err != nil {
        return 0,errors.New("open json buf fail")
    }

    tmpData := NewData()

    for i,e := range relist {
        // 建库把整个doc当成二进制作为Data,这里取出来后需要重新解析
        err := db.ReadData(e.InId,&tmpData)
        if err != nil {
            context.Log.Warn("ReadData fail[%s] InId[%d] OutId[%d]",err,e.InId,e.OutId)
            continue
        }

        context.Log.Debug("inId[%d] weight[%d]",e.InId,e.Weight)

        doc,err := simplejson.NewJson(tmpData)
        if err != nil {
            context.Log.Warn(err)
        } else {
            searchRes.Set(fmt.Sprintf("result%d",i),doc)
        }
    }

    searchRes.Set("retNum",len(relist))
    searchRes.Set("dispNum",len(list))

    context.Log.Info("retNum",len(relist))
    context.Log.Info("dispNum",len(list))

    // 进行序列化
    tmpbuf,err := searchRes.Encode()
    if err != nil {
        return 0,err
    }

    if len(tmpbuf) > cap(response) {
        return 0,errors.New("respone buf too small")
    }

    // 重复了一次内存拷贝!
    copy(response,tmpbuf)

    return len(tmpbuf),nil
}



