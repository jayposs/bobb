package bobb

import (
	"log"
	"sync"
)

type GlobalVal struct {
	Lock  sync.RWMutex
	Value string
}

func (gv *GlobalVal) Get() string {
	gv.Lock.RLock()
	v := gv.Value
	gv.Lock.RUnlock()
	return v
}
func (gv *GlobalVal) Set(newVal string) {
	gv.Lock.Lock()
	gv.Value = newVal
	gv.Lock.Unlock()
}

var ServerStatus GlobalVal
var TraceStatus GlobalVal

func Trace(msg string) {
	if TraceStatus.Get() == "on" {
		log.Println(msg)
	}
}

func strCompare(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func intCompare(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
