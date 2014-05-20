package main

import (
    . "github.com/getwe/goose/utils"
    . "github.com/getwe/goose/database"
    log         "github.com/getwe/goose/log"

    "encoding/binary"
)


type csedoc struct {

    // 内部id
    InId    InIdType

    // 外部id
    OutId   OutIdType

    // 文本相关性得分
    Bweight     int

    // 最终调权后相关性得分
    Weight      int

    // 聚类id
    ClusterId   uint32

    // 调权字段原始值
    AdjustValue     []float64
}

type csedocarray []csedoc

// 支持sort包排序
func (v csedocarray) Len() int {
    return len(v)
}

func (v csedocarray) Swap(i,j int) {
    v[i],v[j] = v[j],v[i]
}

// 根据Weight排序,用于决定最终排序
type WeightSort struct {
    csedocarray
}
func (v WeightSort) Less(i,j int) bool {
    if v.csedocarray[i].Weight > v.csedocarray[j].Weight {
        return true
    }

    if v.csedocarray[i].Weight < v.csedocarray[j].Weight {
        return false
    }

    return v.csedocarray[i].InId < v.csedocarray[j].InId
}

// 把相同的clusterid的元素排在一起
// 排序后每一块会保留第一个作为聚类结果
type GroupByClusterId struct {
    csedocarray
}
func (v GroupByClusterId) Less(i,j int) bool {
    if v.csedocarray[i].ClusterId > v.csedocarray[j].ClusterId {
        return true
    }
    if v.csedocarray[i].ClusterId < v.csedocarray[j].ClusterId {
        return false
    }
    // OutId一样,说明是相同两个doc多次插入,保留后更新的,即InId比较大的
    if v.csedocarray[i].OutId == v.csedocarray[j].OutId {
        return v.csedocarray[i].InId > v.csedocarray[j].InId
    }

    // OutId不一样,保留Weight大的
    if v.csedocarray[i].Weight > v.csedocarray[j].Weight {
        return true
    }
    if v.csedocarray[i].Weight < v.csedocarray[j].Weight {
        return false
    }
    // Weight 也一样,保留先入库的
    return v.csedocarray[i].InId > v.csedocarray[j].InId
}

// 读取Value,根据策略的设计解析数据
// 解析得到ClusterId和AdjustValue数组,同时完成调权计算得到Weight
func (this *csedoc) ParseValue(reader ValueReader,valueBoost []float64) error {
    valueArrLen := len(valueBoost)

    value,err := reader.ReadValue(this.InId)
    if err != nil {
        log.Warn(value)
    }

    // valueBoost指的是调权字段的个数
    // 4个字节是ClusterId的空间
    // 建库写入的value应当是等于len(valueBoost)+4才对
    if len(value) < valueArrLen + 4 {
        return log.Warn("Value length [%d] != value boost length[%d] + 4",
            len(value),valueArrLen)
    }
    // 解析前4个字节,得到ClusterId
    order := binary.BigEndian
    this.ClusterId = order.Uint32(value[0:4])

    // 解析拿到调权字段
    this.AdjustValue = make([]float64,valueArrLen)
    for i:=0;i<valueArrLen;i++ {
        this.AdjustValue[i] = float64(value[i+4])
    }

    // 进行调权
    this.Weight = this.Bweight
    for i:=0;i<valueArrLen;i++ {
        w := float64(this.Bweight) * this.AdjustValue[i] * valueBoost[i]
        this.Weight += int(w)
        log.Debug("adjustWei[%d] = bweight[%d]*adjustvalue[%.3f]*boost[%.3f]",
            int(w),this.Bweight,this.AdjustValue[i],valueBoost[i])
    }

    return nil
}
