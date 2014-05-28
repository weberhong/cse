package main

import (
    simplejson "github.com/bitly/go-simplejson"
    log "github.com/getwe/goose/log"
    . "github.com/getwe/goose/utils"

    "github.com/getwe/scws4go"

    "strings"
    "encoding/binary"
    "math"
)

func (this StyIndexer) parseValue(document *simplejson.Json) (Value,error) {
    // NewValue(len,cap)
    value := NewValue(int(this.valueSize),int(this.valueSize))

    valueObj := document.Get("cse_value")

    order := binary.BigEndian
    // 第一个数字是聚类id,占用4个字节
    clusterid,err := valueObj.GetIndex(0).Int()
    if err != nil {
        return nil,log.Warn("cse_value[0] not a int32 num")
    } else {
        order.PutUint32(value[0:4],uint32(clusterid))
    }

    // 剩下空间用于写入调权字段
    advalue := value[4:]

    // 最多可以写入的value
    cse_value_length := uint8(1)
    for i:=0;i<len(value);i++ {
        num,err := valueObj.GetIndex(i+1).Int()
        if err != nil {
            // 拿出来不是数字
            // 数组遍历完
            // 都走这里的逻辑
            break
        }

        if num > math.MaxUint8 {
            // 不是数字,或者数字太大,抛失败吧,让这条记录建库失败以尽早发现问题
            return nil,log.Warn("cse_value[%d] error",i+1)
        }

       advalue[i] = byte(num)
       cse_value_length++
    }

    // value截掉多余元素
    advalue = advalue[:cse_value_length-1]
    value = value[:len(advalue)+4]

    if cse_value_length != this.adjustWeightFieldCount + 1 {
        return nil,log.Warn("cse_value : clusterid[%d] %v length illegal," +
            "AdjustWeightFieldCount is [%d]",
            clusterid,advalue,this.adjustWeightFieldCount)
    }

    return value,nil
}

func (this StyIndexer) parseTerm(document *simplejson.Json) ([]TermInDoc,error) {

    termHash := make(map[string]termDocFeature)

    this.parseMainTitle(document,termHash)
    this.parseTitle(document,termHash)
    this.parseKeyword(document,termHash)

    termList := make([]TermInDoc,0,len(termHash))
    for k,v := range termHash {
        termList = append(termList,TermInDoc{
            Sign : TermSign(StringSignMd5(k)),
            Weight : v.encode()})
    }

    return termList,nil
}

func (this StyIndexer) parseMainTitle(document *simplejson.Json,termHash map[string]termDocFeature) {
    maintitlearr,err := document.Get("cse_maintitle").StringArray()
    if err != nil {
        log.Warn("get cse_maintitle fail : %s",err)
        return
    }

    for _,title := range maintitlearr {
        segResult,err := this.scws.Segment(title)
        if err != nil {
            log.Warn("segment[%s] fail : %s",title,err)
            continue
        }

        for _,term := range segResult {
            termStr := strings.ToLower(term.Term)
            termWei := this.calTitleTermWei(title,term,this.mainTitleBoost)

            oldwei,ok := termHash[termStr]
            newwei := newTermDocFeature(termWei,0,0)
            if ok {
                newwei.merge(&oldwei)
            }
            termHash[termStr] = newwei
        }
    }
}

func (this StyIndexer) parseTitle(document *simplejson.Json,termHash map[string]termDocFeature) {
    titlearr,err := document.Get("cse_title").StringArray()
    if err != nil {
        log.Warn("get cse_title fail : %s",err)
        return
    }

    for _,title := range titlearr {
        segResult,err := this.scws.Segment(title)
        if err != nil {
            log.Warn("segment[%s] fail : %s",title,err)
            continue
        }

        for _,term := range segResult {
            termStr := strings.ToLower(term.Term)
            termWei := this.calTitleTermWei(title,term,this.titleBoost)

            oldwei,ok := termHash[termStr]
            newwei := newTermDocFeature(0,termWei,0)
            if ok {
                newwei.merge(&oldwei)
            }
            termHash[termStr] = newwei
        }
    }
}

func (this StyIndexer) parseKeyword(document *simplejson.Json,termHash map[string]termDocFeature) {
    keywordarr := document.Get("cse_keyword")

    var i int = 0
    for {
        j := keywordarr.GetIndex(i)
        keyword,err := j.Get("kw").String()
        if err != nil {
            break
        }
        boost,err := j.Get("boost").Float64()
        if err != nil {
            break
        }

        segResult,err := this.scws.Segment(keyword)
        if err != nil {
            log.Warn("segment[%s] fail : %s",keyword,err)
            continue
        }

        for _,term := range segResult {
            termStr := strings.ToLower(term.Term)
            termWei := this.calKeywordTermWei(keyword,term,float32(boost))

            oldwei,ok := termHash[termStr]
            newwei := newTermDocFeature(0,0,termWei)
            if ok {
                newwei.merge(&oldwei)
            }
            termHash[termStr] = newwei
        }
        i++
    }
}

// 根据title,切词得到的term,权重因子计算term在doc中的重要性
func (this StyIndexer) calTitleTermWei(title string,term scws4go.ScwsRes,boost float32) float32 {
    wei := float32(len(term.Term)) / float32(len(title))
    if term.Idf > 0 {
        wei += float32(math.Log10(term.Idf))
    }

    return wei * boost * 0.5
}

func (this StyIndexer) calKeywordTermWei(keyword string,term scws4go.ScwsRes,boost float32) float32 {
    wei := float32(len(term.Term)) / float32(len(keyword))
    if term.Idf > 0 {
        wei += float32(math.Log10(term.Idf))
    }

    return wei * boost * 0.5
}



