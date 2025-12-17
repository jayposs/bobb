// File updt_handlers.go contains funcs to process db.Update requests.
// These funcs are called by the dbHandler func in the server.go program.

package bobb

import (
	"log"
	"time"

	"github.com/valyala/fastjson"
	bolt "go.etcd.io/bbolt"
)

// putRec used by Put, PutBkts, PutOne funcs.
// It adds or replaces a record in the bkt based on existence of key.
// The value of keyField is used as the record key and this field must exist in rec.
func putRec(bkt *bolt.Bucket, rec []byte, keyField string, parser *fastjson.Parser, requiredFlds []string) error {
	parsedRec, err := parser.ParseBytes(rec)
	if err != nil {
		log.Println("putRec error - rec is not valid json", err)
		log.Println(string(rec))
		return ErrBadInputData
	}
	key := parsedRec.GetStringBytes(keyField) // key is []byte
	if key == nil {
		log.Println("putRec error - key value not found or not string - ", err, keyField)
		log.Println(string(rec))
		return ErrBadInputData
	}
	for _, fld := range requiredFlds {
		if !parsedRec.Exists(fld) {
			log.Println("putRec error - required fld not in rec:", fld)
			return ErrBadInputData
		}
	}
	err = bkt.Put(key, rec)
	if err != nil {
		log.Println("db error - put failed", err)
	}
	return err
}

// putLogRec used by PutOne func to write put requests to log bkt.
// Key appended with timestamp so point in time value can be retrieved.
func putLogRec(bkt *bolt.Bucket, key string, rec []byte) error {
	fullKey := key + "|" + time.Now().Format(time.DateTime)
	err := bkt.Put([]byte(fullKey), rec)
	if err != nil {
		log.Println("db error - putLogRec failed", err)
	}
	return err
}

// PutRequest is used to add or replace records.
// If key exists, existing record is replaced otherwise record is added.
// Recs must include the KeyField to be used as the key (unique id).
// Recs are the json marshaled value of the record type.
// RequiredFlds (optional), fld names that must be included in recs.
// Only top level fld names allowed.
type PutRequest struct {
	BktName      string
	KeyField     string   // field in Rec containing value to be used as key
	Recs         [][]byte // records to be added or replaced in db
	RequiredFlds []string // recs must include these fields (optional)
}

func (req PutRequest) IsUpdtReq() bool {
	return true
}
func (req *PutRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	if req.KeyField == "" {
		resp.Status = StatusFail
		resp.Msg = "PutRequest.KeyField cannot be blank"
		return resp, nil
	}
	bkt := openBkt(tx, resp, req.BktName, CreateIfNotExists)
	if bkt == nil {
		return resp, nil
	}
	parser := parserPool.Get()
	defer parserPool.Put(parser)

	for _, rec := range req.Recs { // req.Recs is [][]byte
		err := putRec(bkt, rec, req.KeyField, parser, req.RequiredFlds)
		if err != nil {
			resp.Status = StatusFail
			resp.Msg = "Put request failed, see log for details"
			return resp, err // trans will be rolled back
		}
		resp.PutCnt++
	}
	resp.Status = StatusOk
	return resp, nil
}

// PutBktsRequest is used to add or replace records in 2 bkts with 1 transaction.
// For example: adding new order and order items.
// If either bkt update fails, complete transaction is rolled back.
// RequiredFlds (optional), fld names that must be included in recs.
type PutBktsRequest struct {
	BktName       string
	KeyField      string   // field in Rec containing value to be used as key
	Recs          [][]byte // records to be added or replaced in bkt 1
	RequiredFlds  []string // recs must include these fields (optional)
	Bkt2Name      string
	Recs2         [][]byte // records to be added or replaced in bkt 2
	RequiredFlds2 []string // recs must include these fields (optional)
}

func (req PutBktsRequest) IsUpdtReq() bool {
	return true
}
func (req *PutBktsRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	if req.KeyField == "" {
		resp.Status = StatusFail
		resp.Msg = "PutBktsRequest.KeyField cannot be blank"
		return resp, nil
	}
	bkt1 := openBkt(tx, resp, req.BktName, CreateIfNotExists)
	if bkt1 == nil {
		return resp, nil
	}
	bkt2 := openBkt(tx, resp, req.Bkt2Name, CreateIfNotExists)
	if bkt2 == nil {
		return resp, nil
	}
	parser := parserPool.Get()
	defer parserPool.Put(parser)

	// -- process puts for bkt 1 -----------------------------------
	for _, rec := range req.Recs { // req.Recs is [][]byte
		err := putRec(bkt1, rec, req.KeyField, parser, req.RequiredFlds)
		if err != nil {
			resp.Status = StatusFail
			resp.Msg = "PutBkts request failed, see log for details"
			return resp, err // trans will be rolled back
		}
		resp.PutCnt++
	}
	// -- process puts for bkt 2 -----------------------------------
	for _, rec := range req.Recs2 { // req.Recs2 is [][]byte
		err := putRec(bkt2, rec, req.KeyField, parser, req.RequiredFlds2)
		if err != nil {
			resp.Status = StatusFail
			resp.Msg = "PutBkts request failed, see log for details"
			return resp, err // trans will be rolled back
		}
		resp.PutCnt++
	}
	resp.Status = StatusOk
	return resp, nil
}

// PutOneRequest is used to add or replace a single record.
// Rec must include the KeyField to be used as the key (unique id).
// Rec is the json marshaled value of the record type.
// RequiredFlds (optional), fld names that must be included in recs.
// LogPut indicates to put record to bktname_putlog bkt.
// Key is dataKey|timestamp. Value is Rec. Provides point in time values.
type PutOneRequest struct {
	BktName      string
	KeyField     string   // field in Rec containing value to be used as key
	Rec          []byte   // record to be added or replaced in db
	RequiredFlds []string // recs must include these fields (optional)
	LogPut       bool     // if true, write record to bktname_putlog bkt
}

func (req PutOneRequest) IsUpdtReq() bool {
	return true
}

func (req *PutOneRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	if req.KeyField == "" {
		resp.Status = StatusFail
		resp.Msg = "PutOneRequest.KeyField cannot be blank"
		return resp, nil
	}
	bkt := openBkt(tx, resp, req.BktName, CreateIfNotExists)
	if bkt == nil {
		return resp, nil
	}
	parser := parserPool.Get()
	defer parserPool.Put(parser)

	err := putRec(bkt, req.Rec, req.KeyField, parser, req.RequiredFlds)
	if err != nil {
		resp.Status = StatusFail
		resp.Msg = "PutOne request failed, see log for details"
		return resp, err // trans will be rolled back
	}
	if req.LogPut { // write record to log bkt
		logBkt := openBkt(tx, resp, req.BktName+"_log", CreateIfNotExists)
		if logBkt == nil {
			return resp, nil
		}
		key := fastjson.GetString(req.Rec, req.KeyField)
		err := putLogRec(logBkt, key, req.Rec)
		if err != nil {
			resp.Status = StatusFail
			resp.Msg = "PutOne-LogPut request failed, see log for details"
			return resp, err // trans will be rolled back
		}
	}
	resp.PutCnt = 1
	resp.Status = StatusOk
	return resp, nil
}

// IndexKeyVal type is used by PutIndexRequest.
// Key is typically created from value(s) in data record (must be made unique).
// Val is key of record in data bkt.
// If OldKey not empty, it will be deleted. No problem if it does not exist.
// MergeFlds func in rec.go can be used to merge multiple flds together to form key.
type IndexKeyVal struct {
	Key    string
	Val    string
	OldKey string // used when index rec already exists for data key
}

// PutIndex is used to add or replace index records.
// Bolt Put rules apply: if key does not exist, rec is added, else rec is replaced.
// Key is field value(s) from primary bkt (made unique).
// Val is key of record in primary bkt.
// WARNING - if data rec already has index rec, changing index key will cause multiple records for same data rec.
// Use OldKey to delete existing index rec.
type PutIndexRequest struct {
	BktName string
	Indexes []IndexKeyVal // slice of index key/val/oldkey structs
}

func (req PutIndexRequest) IsUpdtReq() bool {
	return true
}

func (req *PutIndexRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName, CreateIfNotExists)
	if bkt == nil {
		return resp, nil
	}
	for _, index := range req.Indexes { // []IndexKeyVal
		if index.OldKey != "" {
			bkt.Delete([]byte(index.OldKey))
		}
		err := bkt.Put([]byte(index.Key), []byte(index.Val))
		if err != nil {
			log.Println("db error - PutIndex Failed", err)
			resp.Status = StatusFail
			resp.Msg = "PutIndex failed-" + err.Error()
			return resp, err // trans will be rolled back
		}
		resp.PutCnt++
	}
	resp.Status = StatusOk
	return resp, nil
}
