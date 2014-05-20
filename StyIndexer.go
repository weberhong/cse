package main

import (
    "github.com/getwe/goose"
    . "github.com/getwe/goose/utils"
    "github.com/getwe/goose/config"
    log "github.com/getwe/goose/log"

    "reflect"
    "runtime"

    "github.com/getwe/scws4go"
    simplejson "github.com/bitly/go-simplejson"
)

type StyIndexer struct {
    // 共用切词工具
    scws    *scws4go.Scws

    // 共用的只读配置信息

    // value内存块大小
    valueSize   uint8

    // 调权字段个数
    adjustWeightFieldCount uint8

    // 各个检索字段权重
    mainTitleBoost float32
    titleBoost     float32
    keywordBoost   float32
}

// 分析一个doc,返回其中的term列表,Value,Data.(必须保证框架可并发调用ParseDoc)
func (this *StyIndexer) ParseDoc(doc interface{},context *goose.StyContext) (
    outId OutIdType,termList []TermInDoc,value Value,data Data,err error) {
    // ParseDoc的功能实现需要注意的是,这个函数是可并发的,使用StyIndexer.*需要注意安全
    defer func() {
        if r := recover();r != nil {
            err = log.Warn(r)
        }
    }()

    // 策略假设每一个doc就是一个[]buf
    realValue := reflect.ValueOf(doc)
    docbuf := realValue.Bytes()

    document,err := simplejson.NewJson(docbuf)
    if err != nil {
        return 0,nil,nil,nil,log.Warn(err)
    }

    cse_docid,err := document.Get("cse_docid").Int()
    if err != nil {
        return 0,nil,nil,nil,log.Warn("get cse_docid fail : %s",err)
    }
    // outid
    outId = OutIdType(cse_docid)

    // write termInDoc
    termList,err = this.parseTerm(document)

    // write value
    value,err = this.parseValue(document)
    if err != nil {
        return 0,nil,nil,nil,log.Warn("parseValue fail : %s",err)
    }

    // write data
    data,err = document.Get("cse_data").Encode()
    if err != nil {
        return 0,nil,nil,nil,log.Warn("encode cse_data fail : %s",err)
    }

    return
}

// 调用一次初始化
func (this *StyIndexer) Init(conf config.Conf) (err error) {

    // scws初始化
    scwsDictPath := conf.String("Strategy.Indexer.Scws.xdbdict")
    scwsRulePath := conf.String("Strategy.Indexer.Scws.rules")
    scwsForkCnt  := runtime.NumCPU()
    this.scws = scws4go.NewScws()
    err = this.scws.SetDict(scwsDictPath, scws4go.SCWS_XDICT_XDB|scws4go.SCWS_XDICT_MEM)
    if err != nil {
        return log.Error(err)
    }
    err = this.scws.SetRule(scwsRulePath)
    if err != nil {
        return log.Error(err)
    }
    this.scws.SetCharset("utf8")
    this.scws.SetIgnore(1)
    this.scws.SetMulti(scws4go.SCWS_MULTI_SHORT & scws4go.SCWS_MULTI_DUALITY & scws4go.SCWS_MULTI_ZMAIN)
    err = this.scws.Init(scwsForkCnt)
    if err != nil {
        return log.Error(err)
    }

    // ValueSize
    this.valueSize = uint8(conf.Int64("GooseBuild.DataBase.ValueSize"))
    if this.valueSize < 4 {
        return log.Error("read conf GooseBuild.DataBase.ValueSize[%d] fail" +
            "at least 4",this.valueSize)
    }

    // AdjustWeightFieldCount
    this.adjustWeightFieldCount = uint8(conf.Int64("Strategy.AdjustWeightFieldCount"))
    if this.adjustWeightFieldCount == 0 ||
        this.adjustWeightFieldCount + 4 > this.valueSize {
        return log.Error("AdjustWeightFieldCount[%d] out of limit. ValueSize[%d]",
            this.adjustWeightFieldCount,this.valueSize)
    }

    // Weight boost
    this.mainTitleBoost = float32(conf.Float64("Strategy.Indexer.Weight.MainTitleBoost"))
    this.titleBoost = float32(conf.Float64("Strategy.Indexer.Weight.TitleBoost"))
    this.keywordBoost = float32(conf.Float64("Strategy.Indexer.Weight.KeyWordBoost"))
    if this.mainTitleBoost == 0.0 || this.titleBoost == 0.0 || 
        this.keywordBoost == 0.0 {
        log.Warn("index weight conf mainTitleBoost[%f] titleBoost[%f] keywordBoost[%f]",
            this.mainTitleBoost,this.titleBoost,this.keywordBoost)
    }

    return nil
}

