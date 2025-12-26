package bobb

import (
	"log"

	"github.com/valyala/fastjson"

	bolt "go.etcd.io/bbolt"
)

var DefaultQryRespSize = 400 // response slice initial allocation for this size

var parserPool = new(fastjson.ParserPool)

// GetRequest is used to get specific records by key.
// Keys must be string values. They will be converted to []byte by Get request.
type GetRequest struct {
	BktName string
	Keys    []string // keys of records to be returned
}

func (req GetRequest) IsUpdtReq() bool {
	return false
}

func (req *GetRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	resp.Status = StatusOk // may be changed to Warning below if key not found

	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	resp.Recs = make([][]byte, 0, len(req.Keys))

	for _, key := range req.Keys {
		v := bkt.Get([]byte(key))
		if v == nil {
			resp.Status = StatusFail
			resp.Msg = "not found, key: " + key
			return resp, nil
		}
		resp.Recs = append(resp.Recs, v)
	}
	return resp, nil
}

// GetAllRequest returns all records in bucket or records in range between Start/End keys.
// If start/end keys are specified, a cursor is used to establish a starting point and then reads sequentially.
// Records are returned in key order.
// If StartKey == EndKey, rec key prefix must match StartKey.
// If StartKey = "", reads from beginning. If EndKey = "" reads to end.
// If end of bkt not reached, response.NextKey will be next key in order.
type GetAllRequest struct {
	BktName  string
	IndexBkt string // name of bkt used as index
	StartKey string // if not "", keys >= this value
	EndKey   string // if not "", keys <= this value
	Limit    int    // max # recs to return
	ErrLimit int    // run stops when ErrLimit exceeded, default 0, settings.MaxErrs limit if -1
}

func (req GetAllRequest) IsUpdtReq() bool {
	return false
}

func (req *GetAllRequest) Run(tx *bolt.Tx) (*Response, error) {
	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	var index *bolt.Bucket
	if req.IndexBkt != "" {
		index = openBkt(tx, resp, req.IndexBkt)
		if index == nil {
			return resp, nil
		}
	}
	resp.Recs = make([][]byte, 0, DefaultQryRespSize)

	var k, v []byte
	var bErr *BobbErr

	readLoop := NewReadLoop(bkt, index)
	k, v, bErr = readLoop.Start(req.StartKey, req.EndKey, req.Limit)
	if bErr != nil {
		resp.Errs = append(resp.Errs, *bErr)
		k, v, bErr = readLoop.Next()
	}

	for k != nil {
		if len(resp.Errs) > req.ErrLimit {
			resp.Status = StatusFail
			resp.Msg = "too many errors, see resp.Errs for details"
			return resp, nil
		}
		if bErr != nil { // triggered when readLoop returns errCode
			resp.Errs = append(resp.Errs, *bErr)
			k, v, bErr = readLoop.Next()
			continue
		}
		resp.Recs = append(resp.Recs, v)
		readLoop.Count++
		k, v, bErr = readLoop.Next()
	}
	if readLoop.NextKey != nil { // ReadLoop.NextKey is loaded by Next() at end of range.
		resp.NextKey = string(readLoop.NextKey)
	}
	resp.Status = StatusOk
	return resp, nil
}

// GetKeys returns keys, not values, from specified bucket.
// Keys are returned in the Response.Recs as json.Marshaled string.
// Use Start/End keys to specify range to be included.
// If StartKey == EndKey, key prefix must match StartKey.
type GetAllKeysRequest struct {
	BktName  string
	StartKey string // if not "", keys >= this value
	EndKey   string // if not "", keys <= this value
	Limit    int    // max # recs to return
}

func (req GetAllKeysRequest) IsUpdtReq() bool {
	return false
}

func (req *GetAllKeysRequest) Run(tx *bolt.Tx) (*Response, error) {
	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	resp.Recs = make([][]byte, 0, 1000)

	readLoop := NewReadLoop(bkt, nil)
	k, _, _ := readLoop.Start(req.StartKey, req.EndKey, req.Limit)
	for k != nil {
		resp.Recs = append(resp.Recs, k)
		readLoop.Count++
		k, _, _ = readLoop.Next()
	}
	if readLoop.NextKey != nil { // ReadLoop.NextKey is loaded by Next() at end of range.
		resp.NextKey = string(readLoop.NextKey)
	}
	resp.Status = StatusOk
	return resp, nil
}

// GetOneRequest is used to get a specific record by Key.
// Key must be string value. It will be converted to []byte by GetOne request.
type GetOneRequest struct {
	BktName string
	Key     string // key of record to be returned
}

func (req GetOneRequest) IsUpdtReq() bool {
	return false
}
func (req *GetOneRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	v := bkt.Get([]byte(req.Key))
	if v == nil {
		log.Println("GetOne key not found", req.Key)
		resp.Status = StatusWarning
		resp.Msg = "not found"
		return resp, nil
	}
	resp.Rec = v
	resp.Status = StatusOk
	return resp, nil
}
