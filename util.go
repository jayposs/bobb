package bobb

import (
	"log"
	"sync"
	//"time"
)

//const fmtTimeStamp = "20060102150405" // yyyymmddhhmmss

var MaxErrs int // from bobb_settings.json, set at startup by bobb_server.go

//func timeStamp() string {
//	return time.Now().Format(fmtTimeStamp)
//}

// Func e creates instance of BobbErr
func e(errCode, msg string, k, v []byte) *BobbErr {
	bErr := BobbErr{
		ErrCode: errCode,
		Msg:     msg,
		Key:     k,
		Val:     v,
	}
	return &bErr
}

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
