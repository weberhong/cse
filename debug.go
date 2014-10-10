package main

import (
	"fmt"
)

type Debug struct {
	// 是否开启debug
	isdebug bool

	// 当次请求级别的debug日志
	querylog []string

	// doc相关的debug日志
	doclog map[uint32][]string
}

func NewDebug(isdebug bool) *Debug {
	d := Debug{}
	d.isdebug = isdebug

	if d.isdebug {
		d.querylog = make([]string, 0, 16)
		d.doclog = make(map[uint32][]string)
	}

	return &d
}

func (this *Debug) AddDebugInfo(format string, a ...interface{}) {
	if this.isdebug == false {
		return
	}
	this.querylog = append(this.querylog, fmt.Sprintf(format, a...))
}

func (this *Debug) AddDocDebugInfo(key uint32, format string, a ...interface{}) {
	if this.isdebug == false {
		return
	}

	strlog, ok := this.doclog[key]
	if !ok {
		strlog = make([]string, 0, 8)
	}
	strlog = append(strlog, fmt.Sprintf(format, a...))
	this.doclog[key] = strlog
}

func (this Debug) GetDebugInfo() []string {
	if this.isdebug == false {
		return nil
	}

	return this.querylog
}

func (this Debug) GetDocDebugInfo(key uint32) []string {
	if this.isdebug == false {
		return nil
	}
	strlog, ok := this.doclog[key]
	if !ok {
		return nil
	}
	return strlog
}
