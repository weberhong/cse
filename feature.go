package main

import (
    . "github.com/getwe/goose/utils"
    "encoding/binary"
    "math"
)

// term在doc中的特征
type termDocFeature struct {
    // 命中mainTitle的权值
    MainTitleWeight     float32
    // 命中title的权值
    TitleWeight         float32
    // 命中keyword的权值
    KeyWordWeight       float32
}

// 压缩成TermWeight
func (t *termDocFeature) encode() TermWeight {
    order := binary.BigEndian

    buf := make([]byte,4)
    if t.MainTitleWeight > 1.0 {
        t.MainTitleWeight = 1.0
    }
    if t.TitleWeight > 1.0 {
        t.TitleWeight = 1.0
    }
    if t.KeyWordWeight > 1.0 {
        t.KeyWordWeight = 1.0
    }
    buf[0] = byte(t.MainTitleWeight * math.MaxUint8)
    buf[1] = byte(t.TitleWeight     * math.MaxUint8)
    buf[2] = byte(t.KeyWordWeight   * math.MaxUint8)
    buf[3] = 0 //暂时没使用

    return TermWeight(order.Uint32(buf))
}

// 从TermWeight解压
func (t *termDocFeature) decode(w TermWeight) {
    order := binary.BigEndian

    buf := make([]byte,4)
    order.PutUint32(buf,uint32(w))

    t.MainTitleWeight = float32(buf[0]) / math.MaxUint8
    t.TitleWeight     = float32(buf[1]) / math.MaxUint8
    t.KeyWordWeight   = float32(buf[2]) / math.MaxUint8
}

// 合并两个term,权重保留大的值
func (t *termDocFeature) merge(r *termDocFeature) {
    if t.MainTitleWeight < r.MainTitleWeight {
        t.MainTitleWeight = r.MainTitleWeight
    }
    if t.TitleWeight < r.TitleWeight {
        t.TitleWeight = r.TitleWeight
    }
    if t.KeyWordWeight < r.KeyWordWeight {
        t.KeyWordWeight = r.KeyWordWeight
    }
}

func newTermDocFeature(mainTitle,title,keyowrd float32) termDocFeature {
    t := termDocFeature{}
    t.MainTitleWeight = mainTitle
    t.TitleWeight = title
    t.KeyWordWeight = keyowrd
    return t
}


