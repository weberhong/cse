package main

import (
    simplejson "github.com/bitly/go-simplejson"
    log "github.com/getwe/goose/log"
    "github.com/getwe/scws4go"

    . "github.com/getwe/goose"
    . "github.com/getwe/goose/utils"
)

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

    dictRes := this.matchDict(styData.query)
    segResult,err := this.scws.Segment(styData.query)
    if err != nil {
        err = log.Warn(err)
        return nil,err
    }

    return this.calQueryTerm(styData.query,segResult,dictRes)
}

// 根据Query,Query的切词结果,Query在trie词典的匹配情况以及查找到的属性
// 计算term重要性,是否可省
func (this StySearcher) calQueryTerm(query string,segResult []scws4go.ScwsRes,
    dictRes []dictResult) ([]TermInQuery,error) {

    return nil,nil
}

type dictResult struct {
    offset  int
    length  int
    value   int
}

// 在trie词典中搜索query,顺序标记query中每一段的成分
func (this StySearcher) matchDict(query string) []dictResult {
    res := make([]dictResult,0)

    key := []rune(query)

    length := len(key)

    lastMatchPos := 0
    pos := 0
    for pos < length {
        r := this.trieDict.CommonPrefixSearch(key[pos:], 0)
        if len(r) > 0 {
            if pos != lastMatchPos {
                offset := lastMatchPos
                matchlen := pos - lastMatchPos
                res = append(res,dictResult{offset,matchlen,0})
            }

            maxlen := 0
            maxlenindex := 0
            for i := 0; i < len(r); i++ {
                if r[i].PrefixLen > maxlen {
                    maxlen = r[i].PrefixLen
                    maxlenindex = i
                }
            }
            offset := pos
            matchlen := r[maxlenindex].PrefixLen
            res = append(res,dictResult{offset,matchlen,r[maxlenindex].Freq})
            pos = pos + maxlen
            lastMatchPos = pos
        } else {
            pos++
        }
    }
    if pos != lastMatchPos {
        offset := lastMatchPos
        matchlen := pos - lastMatchPos
        res = append(res,dictResult{offset,matchlen,0})
    }

    return res
}
