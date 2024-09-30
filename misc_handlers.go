package bobb

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// openBkt returns pointer to bucket. Errors are logged and response is loaded with error info.
// Both View and Update handler funcs use openBkt.
func openBkt(tx *bolt.Tx, resp *Response, bktName string) *bolt.Bucket {
	bkt := tx.Bucket([]byte(bktName))
	if bkt == nil {
		log.Println("Bkt Not Found - ", bktName)
		resp.Status = StatusFail
		resp.Msg = "Bkt Not Found - " + bktName
	}
	return bkt
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

// Export writes bkt recs to file in formatted json.
func Export(tx *bolt.Tx, req *ExportRequest) *Response {
	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}

	exportFile, err := os.Create(req.FilePath)
	log.Println("created exportfile")
	if err != nil {
		log.Println("error creating export file", req.FilePath, err)
		resp.Status = StatusFail
		resp.Msg = "error creating export file:" + err.Error()
		return resp
	}
	csr := bkt.Cursor()
	var k, v []byte
	if req.StartKey == "" {
		k, v = csr.First()
	} else {
		k, v = csr.Seek([]byte(req.StartKey))
	}
	var counter int
	exportFile.WriteString("[\n")
	for k != nil {
		if req.EndKey != "" && string(k) > req.EndKey {
			break
		}
		if counter > 0 {
			exportFile.WriteString(",\n")
		}
		buffer := new(bytes.Buffer)
		json.Indent(buffer, v, "", "  ")
		exportFile.Write(buffer.Bytes())
		counter++
		if counter == req.Limit {
			break
		}
		k, v = csr.Next()
	}
	exportFile.WriteString("\n]")
	if err := exportFile.Close(); err != nil {
		log.Println("error closing export file", err)
		resp.Status = StatusFail
		resp.Msg = "error closing export file" + err.Error()
		return resp
	}
	resp.Status = StatusOk
	return resp
}

// CopyDB makes a copy of the open database to the specified file path.
func CopyDB(tx *bolt.Tx, req *CopyDBRequest) *Response {
	resp := new(Response)
	err := tx.CopyFile(req.FilePath, 0600)
	if err != nil {
		resp.Status = StatusFail
		resp.Msg = err.Error()
	} else {
		resp.Status = StatusOk
	}
	return resp
}
