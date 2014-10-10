package main

import (
	. "github.com/getwe/goose/utils"
	"math"
)

// query的命中情况
func (this *StySearcher) queryMatch(styData *strategyData, inId InIdType, termInQuery []TermInQuery,
	termInDoc []TermInDoc) float32 {

	radio := make([]float32, len(termInQuery), len(termInQuery))

	termInDocFeature := make([]termDocFeature, len(termInDoc), len(termInDoc))
	for i, t := range termInDoc {
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
	for i, q := range termInQuery {
		tm := (float32(q.Weight) / math.MaxUint16) * radio[i]

		styData.debug.AddDocDebugInfo(uint32(inId),
			"queryMatch t%d [%.3f] = wei[%.3f]*radio[%.3f]",
			i, tm, float32(q.Weight)/math.MaxUint16, radio[i])

		qm += tm

	}

	if qm > 1.0 {
		qm = 1.0
	}

	return qm
}

// doc的命中情况
func (this *StySearcher) docMatch(styData *strategyData, inId InIdType, termInQuery []TermInQuery,
	termInDoc []TermInDoc) float32 {

	termInDocFeature := make([]termDocFeature, len(termInDoc), len(termInDoc))
	for i, t := range termInDoc {
		termInDocFeature[i].decode(t.Weight)
	}

	dm := float32(0.0)
	for i, t := range termInDocFeature {
		// MainTitle和Title取大
		wei := t.MainTitleWeight
		if t.TitleWeight > wei {
			wei = t.TitleWeight
		}

		var titleBoost float32
		var keywordBoost float32

		switch termInQuery[i].Attr {
		// term属于专名的一部分,命中title很重要,命中keyword也应该适当加分
		case SECTION_ATTR_NAME:
			titleBoost = 1.0
			keywordBoost = 0.2
		// term属于keyword,应该更多考察keyword的命中程度
		case SECTION_ATTR_KEYWORD:
			fallthrough
		case SECTION_ATTR_KEYWORD_OMIT:
			titleBoost = 0.4
			keywordBoost = 1.0
		// 未知的词,更倾向于考察title的命中情况
		case SECTION_ATTR_UNKNOWN:
			fallthrough
		case SECTION_ATTR_OMIT:
			titleBoost = 0.9
			keywordBoost = 0.2
		}

		rwei := wei*titleBoost + t.KeyWordWeight*keywordBoost

		styData.debug.AddDocDebugInfo(uint32(inId),
			"docMatch t%d [%.3f] = title[%.3f]*%.2f+keyword[%.3f]*%.2f",
			i, rwei, wei, titleBoost, t.KeyWordWeight, keywordBoost)

		dm += rwei
	}

	if dm > 1.0 {
		dm = 1.0
	}

	return dm
}

// 可省词没命中,进行打压
func (this *StySearcher) omitTermPunish(styData *strategyData, inId InIdType, termInQuery []TermInQuery,
	termInDoc []TermInDoc) float32 {
	// TODO
	return 1.0
}
