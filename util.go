package bobb

import (
	"log"
	"sync"

	"github.com/valyala/fastjson"
)

var InitialRespRecsSize int // from bobb_settings.json, response.Recs slice initial allocation for this size

var MaxErrs int // from bobb_settings.json, set at startup by bobb_server.go

var parserPool = new(fastjson.ParserPool)

//const fmtTimeStamp = "20060102150405" // yyyymmddhhmmss

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
