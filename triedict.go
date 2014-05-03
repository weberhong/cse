package main

import (
    darts       "github.com/awsong/go-darts"
)

// trie词典查找结果
type TrieDictResult struct {
    Offset  int     // 在原串中的起始位置
    Length  int     // 长度
    Attr    int     // 属性
}

type TrieDict struct {
    dict    darts.Darts
}

// 在trie词典中搜索query,顺序标记query中每一段的成分
func (this TrieDict) matchDict(query string) []TrieDictResult {
    res := make([]TrieDictResult,0)

    key := []rune(query)

    length := len(key)

    lastMatchPos := 0
    pos := 0
    for pos < length {
        r := this.dict.CommonPrefixSearch(key[pos:], 0)
        if len(r) > 0 {
            if pos != lastMatchPos {
                offset := lastMatchPos
                matchlen := pos - lastMatchPos
                res = append(res,TrieDictResult{offset,matchlen,0})
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
            res = append(res,TrieDictResult{offset,matchlen,r[maxlenindex].Freq})
            pos = pos + maxlen
            lastMatchPos = pos
        } else {
            pos++
        }
    }
    if pos != lastMatchPos {
        offset := lastMatchPos
        matchlen := pos - lastMatchPos
        res = append(res,TrieDictResult{offset,matchlen,0})
    }

    return res
}
