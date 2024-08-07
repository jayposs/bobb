// File updt_handlers.go contains funcs to process db.Update requests.
// These funcs are called by the dbHandler func in the server.go program.

package bobb

import (
	"log"
	"strings"

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
		return DataError
	}
	key := parsedRec.GetStringBytes(keyField) // key is []byte
	if key == nil {
		log.Println("putRec error - key value not found or not string - ", err, keyField)
		log.Println(string(rec))
		return DataError
	}
	for _, fld := range requiredFlds {
		if !parsedRec.Exists(fld) {
			log.Println("putRec error - required fld not in rec:", fld)
			return DataError
		}
	}
	err = bkt.Put(key, rec)
	if err != nil {
		log.Println("db error - put failed", err)
	}
	return err
}

// Put adds or replaces records in specified bkt.
func Put(tx *bolt.Tx, req *PutRequest) (*Response, error) {
	resp := new(Response)
	if req.KeyField == "" {
		resp.Status = StatusFail
		resp.Msg = "PutRequest.KeyField cannot be blank"
		return resp, nil
	}
	bkt := openBkt(tx, resp, req.BktName)
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

// PutBkts adds or replaces records in 2 bkts with a single transaction (tx).
func PutBkts(tx *bolt.Tx, req *PutBktsRequest) (*Response, error) {
	resp := new(Response)
	if req.KeyField == "" {
		resp.Status = StatusFail
		resp.Msg = "PutBktsRequest.KeyField cannot be blank"
		return resp, nil
	}
	bkt1 := openBkt(tx, resp, req.BktName)
	if bkt1 == nil {
		return resp, nil
	}
	bkt2 := openBkt(tx, resp, req.Bkt2Name)
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

// PutOne adds or replaces a single record.
func PutOne(tx *bolt.Tx, req *PutOneRequest) (*Response, error) {
	resp := new(Response)
	if req.KeyField == "" {
		resp.Status = StatusFail
		resp.Msg = "PutOneRequest.KeyField cannot be blank"
		return resp, nil
	}
	bkt := openBkt(tx, resp, req.BktName)
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
	resp.PutCnt = 1
	resp.Status = StatusOk
	return resp, nil
}

// PutIndex is used to add index records.
// Bolt Put rules apply: if key does not exist, rec is added, else rec is replaced.
// Key is field value(s) from primary bkt (made unique).
// Val is key of record in primary bkt.
// WARNING - if data rec already has index rec, changing index key will cause multiple records for same data rec.
// Use OldKey to delete existing index rec.
func PutIndex(tx *bolt.Tx, req *PutIndexRequest) (*Response, error) {
	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
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
			log.Println("db error - Delete failed", err)
			resp.Status = StatusFail
			resp.Msg = "Delete failed, see log for details"
			return resp, err // trans will be rolled back
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
		resp.Status = StatusFail
		resp.Msg = "Bkt request failed, see log for details"
		return resp, err
	}
	resp.Status = StatusOk
	return resp, nil
}

func bktNextSeq(bkt *bolt.Bucket, count int) ([]int, error) {
	if count > 100 {
		log.Println("bktNexSeq - too many return values requested, max of 100 returned, ", count)
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
