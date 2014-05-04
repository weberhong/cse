package main

import (
    simplejson "github.com/bitly/go-simplejson"
    log "github.com/getwe/goose/log"

    . "github.com/getwe/goose"
    . "github.com/getwe/goose/utils"

    "math"
    "strings"
)

// 本地辅助计算用
type queryTerm struct {
    term    string
    idf     float64
    weight  float32
    attr    int
    omit    bool
}


func (this StySearcher) parseQuery(req *simplejson.Json,
    context *StyContext,styData *strategyData) ([]TermInQuery,error){

    var err error

    styData.query,err = req.Get("query").String()
    if err != nil {
        log.Warn(err)
        return nil,err
    }
    styData.pn = req.Get("pn").MustInt(0)
    styData.rn = req.Get("rn").MustInt(10)

    context.Log.Info("query",styData.query)

    termarr := make([]queryTerm,0)
    // 先对query进行分段
    // 分词上首先尊重策略自定义的词典
    dictRes := this.trieDict.matchDict(styData.query)
    // 对每一段进行切词
    for _,s := range dictRes {
        segResult,err := this.scws.Segment(
            styData.query[s.Offset: s.Offset+s.Length])
        if err != nil {
           log.Warn(err)
           continue
        }
        for _,t := range segResult {
            termarr = append(termarr,queryTerm{
                attr : s.Attr,// term的属性取的是trie的配置,而不是scws4go自带
                idf : t.Idf,
                term : t.Term})
        }

    }
    return this.calQueryTerm(context,styData.query,termarr)
}


// 根据Query,Query的切词结果,Query在trie词典的匹配情况以及查找到的属性
// 计算term重要性,是否可省
func (this StySearcher) calQueryTerm(context *StyContext,query string,
    termarr []queryTerm) ([]TermInQuery,error) {

    querylen := float32(len(query))

    weightsum := float32(0.0)

    for i,t := range termarr {
        // 根据term的长度算出重要性
        termarr[i].weight = float32(len(t.term)) / querylen

        // 利用scws4go的idf信息进行调整
        if t.idf > 1.0 {
            termarr[i].weight += float32(math.Log10(t.idf))
        }

        // 利用triedict配置的词属性调整权重
        switch t.attr {
        case SECTION_ATTR_NAME:
            // 专名,最重要的东西
            termarr[i].weight *= 1.5
            termarr[i].omit = false
        case SECTION_ATTR_KEYWORD:
            termarr[i].weight *= 1.1
            termarr[i].omit = false
        case SECTION_ATTR_OMIT:
            // 可省词降低权重
            termarr[i].weight *= 0.1
            termarr[i].omit = true
        case SECTION_ATTR_UNKNOWN:
            termarr[i].weight *= 0.3
            termarr[i].omit = true
        }

        weightsum += termarr[i].weight
    }

    termList := make([]TermInQuery,len(termarr),len(termarr))
    for i,t := range termarr {
        termList[i].Sign = TermSign(StringSignMd5(strings.ToLower(t.term)))
        termList[i].CanOmit = t.omit;
        termList[i].SkipOffset = true;
        //weight权值是[0,1]乘上MaxUint16保存,后续要用需要除于MaxUint16还原
        termList[i].Weight = TermWeight((t.weight/weightsum)*math.MaxUint16)

        {
            context.Log.Debug("term[%s] omit[%b] weight[%0.4f]",
                strings.ToLower(t.term),
                t.omit,
                termList[i].Weight)
        }
    }

    return termList,nil
}


