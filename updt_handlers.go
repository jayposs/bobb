// File updt_handlers.go contains a func to process each db.Update request.
// These funcs are called by the dbHandler func in the server.go program.

package bobb

import (
	"log"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// Put adds or replaces records, based on existence of key.
// The KeyField specified in the request is used as the key and this field must exist in all request.Recs.
func Put(tx *bolt.Tx, req *PutRequest) (*Response, error) {

	resp := new(Response)
	if req.KeyField == "" {
		resp.Status = StatusFail
		resp.Msg = "request KeyField cannot be blank"
		return resp, nil
	}
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	for _, rec := range req.Recs { // req.Recs is [][]byte
		key := recGetStr(rec, req.KeyField)
		if key == "" {
			log.Println("key value not found in record for specified KeyField - ", req.KeyField)
			log.Println(string(rec))
			resp.Status = StatusFail
			resp.Msg = "key value not found in record for specified KeyField - " + req.KeyField
			return resp, nil
		}
		err := bkt.Put([]byte(key), rec)
		if err != nil {
			log.Println("db error - put failed", err)
			return nil, err // trans will be rolled back and err returned to client
		}
		resp.PutCnt++
	}
	resp.Status = StatusOk
	return resp, nil
}

// PutOne adds or replaces a single record. Works same as Put.
func PutOne(tx *bolt.Tx, req *PutOneRequest) (*Response, error) {

	resp := new(Response)
	if req.KeyField == "" {
		resp.Status = StatusFail
		resp.Msg = "request KeyField cannot be blank"
		return resp, nil
	}
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	key := recGetStr(req.Rec, req.KeyField)
	if key == "" {
		log.Println("key value not found in record", req.KeyField)
		resp.Status = StatusFail
		resp.Msg = "key value not found in record - " + req.KeyField
		return resp, nil
	}
	err := bkt.Put([]byte(key), req.Rec)
	if err != nil {
		log.Println("db error - put failed", err)
		return nil, err // trans will be rolled back and err returned to client
	}
	resp.PutCnt = 1
	resp.Status = StatusOk
	return resp, nil
}

// PutIndex is used to add index records.
// Bolt Put rules apply: if key does not exist, rec is added, else rec is replaced.
// Key is field value(s) from primary bkt (made unique).
// Val is key of record in primary bkt.
// WARNING - if data rec already has index rec, changing index key will cause multiple records for same data rec.
// To change index key, previous index record must be deleted.
func PutIndex(tx *bolt.Tx, req *PutIndexRequest) (*Response, error) {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	for _, index := range req.Indexes { // []IndexKeyVal
		err := bkt.Put([]byte(index.Key), []byte(index.Val))
		if err != nil {
			log.Println("db error - put index failed", err)
			return nil, err // trans will be rolled back and err returned to client
		}
		resp.PutCnt++
	}
	resp.Status = StatusOk
	return resp, nil
}

// Delete deletes recs with keys matching specified keys.
func Delete(tx *bolt.Tx, req *DeleteRequest) (*Response, error) {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	for _, key := range req.Keys {
		err := bkt.Delete([]byte(key))
		if err != nil { // key not found does not return error
			log.Println("db error - delete failed", err)
			return nil, err // trans will be rolled back and err returned to client
		}
	}
	resp.Status = StatusOk
	return resp, nil
}

// Bkt performs bucket requests: "create", "delete", "nextseq"
func Bkt(tx *bolt.Tx, req *BktRequest) (*Response, error) {

	resp := new(Response)

	var err error
	op := strings.ToLower(req.Operation)
	switch op {
	case "create":
		bkt := tx.Bucket([]byte(req.BktName))
		if bkt != nil {
			resp.Status = StatusFail
			resp.Msg = "bucket already exists -" + req.BktName
			return resp, nil
		}
		_, err = tx.CreateBucket([]byte(req.BktName))
	case "delete":
		tx.DeleteBucket([]byte(req.BktName)) // NOTE - delete error is ignored
	case "nextseq":
		bkt := openBkt(tx, resp, req.BktName)
		if bkt == nil {
			return resp, nil
		}
		resp.NextSeq, err = bktNextSeq(bkt, req.NextSeqCount)
	}
	if err != nil {
		log.Println("db error - bkt operation failed", req.Operation, req.BktName, err)
		return nil, err
	}
	resp.Status = StatusOk
	return resp, nil
}

func bktNextSeq(bkt *bolt.Bucket, count int) ([]int, error) {
	if count > 100 {
		count = 100
	}
	if count == 0 {
		count = 1
	}
	seqNumbers := make([]int, count)
	for i := 0; i < count; i++ {
		seqNo, err := bkt.NextSequence()
		if err != nil {
			return nil, err
		}
		seqNumbers[i] = int(seqNo)
	}
	return seqNumbers, nil
}
