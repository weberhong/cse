package main

import (
    . "github.com/getwe/goose/utils"
    "math"
)

// query的命中情况
func (this *StySearcher) queryMatch(inId InIdType,termInQuery []TermInQuery,
    termInDoc []TermInDoc) float32 {

    radio := make([]float32,len(termInQuery),len(termInQuery))

    termInDocFeature := make([]termDocFeature,len(termInDoc),len(termInDoc))
    for i,t := range termInDoc {
        termInDocFeature[i].decode(t.Weight)

        f := &termInDocFeature[i]
        // 基于title的检索系统,有没有命中maintitle是关键点
        if f.MainTitleWeight > 0 {
            radio[i] = 1.0
        } else {
            if f.KeyWordWeight > 0 {
                radio[i] = 0.9
            } else {
                if f.TitleWeight > 0 {
                    radio[i] = 0.8
                } else {
                    radio[i] = 0.0
                }
            }
        }
    }

    qm := float32(0)
    for i,q := range termInQuery {
        qm += ( float32(q.Weight) / math.MaxUint16 ) * radio[i]
    }

    return qm
}

// doc的命中情况
func (this *StySearcher) docMatch(inId InIdType,termInQuery []TermInQuery,
    termInDoc []TermInDoc) float32 {

    termInDocFeature := make([]termDocFeature,len(termInDoc),len(termInDoc))
    for i,t := range termInDoc {
        termInDocFeature[i].decode(t.Weight)
    }

    dm := float32(0.0)
    for _,t := range termInDocFeature {
        // MainTitle和Title取大
        wei := t.MainTitleWeight
        if t.KeyWordWeight > wei {
            wei = t.KeyWordWeight
        }

        wei = wei * 0.8 + t.KeyWordWeight * 0.2
        dm += wei
    }

    return dm
}

// 可省词没命中,进行打压
func (this *StySearcher) omitTermPunish(inId InIdType,termInQuery []TermInQuery,
    termInDoc []TermInDoc) float32 {
    // TODO
    return 1.0
}
