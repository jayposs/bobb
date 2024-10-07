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

func StrCompare(a, b string) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

func AssertInt(val any) int {
	if x, ok := val.(int); !ok {
		log.Println("bad int type assertion", val)
		return 0
	} else {
		return x
	}
}
func AssertStr(val any) string {
	if s, ok := val.(string); !ok {
		log.Println("bad string type assertion", val)
		return ""
	} else {
		return s
	}
}
