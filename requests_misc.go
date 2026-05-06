package bobb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	bolt "go.etcd.io/bbolt"
)

const CreateIfNotExists = true

// openBkt returns pointer to bucket. Errors are logged and response is loaded with error info.
// Both View and Update handler funcs use openBkt.
// If tx is update transaction, createIfNotExists option can be used.
func openBkt(tx *bolt.Tx, resp *Response, bktName string, createIfNotExists ...bool) *bolt.Bucket {
	if bktName == "" {
		resp.Status = StatusFail
		resp.Msg = "BktName not specfied in request"
		return nil
	}
	var bkt *bolt.Bucket
	var err error
	if len(createIfNotExists) > 0 && createIfNotExists[0] {
		bkt, err = tx.CreateBucketIfNotExists([]byte(bktName))
		if err != nil {
			log.Println("Open Bkt Failed - ", bktName, err)
			resp.Status = StatusFail
			resp.Msg = fmt.Sprintf("Open Bkt Failed - %s - %s", bktName, err.Error())
			return nil
		}
	} else {
		bkt = tx.Bucket([]byte(bktName))
		if bkt == nil {
			log.Println("Open Bkt Failed - ", bktName)
			resp.Status = StatusFail
			resp.Msg = fmt.Sprintf("Open Bkt Failed - %s", bktName)
		}
	}
	return bkt
}

// BktRequest performs bucket requests: "create", "delete", "nextseq", "list", "count".
// For "nextseq" operations: if NextSeqCount = 0, 1 value is returned in NextSeq.
// Note - a maximum of 100 seq #'s are returned per request.
// List operation returns names of all bkts in resp.Recs. BktName fld not used.
// See shortcut func GetBktList() in client/util.go.
// Count operation returns number of keys in specified bkt.
// See shortcut func GetRecCount() in client/util.go.
type BktRequest struct {
	BktName      string
	Operation    string // "create", "delete", "nextseq", "list", "count"
	NextSeqCount int    // used with nextseq op to specify how many (max 100)
}

func (req BktRequest) IsUpdtReq() bool {
	return true
}

func (req *BktRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	var err error
	op := strings.ToLower(req.Operation)
	switch op {
	case BktCreate:
		bkt := tx.Bucket([]byte(req.BktName))
		if bkt != nil {
			resp.Status = StatusFail
			resp.Msg = "bucket already exists -" + req.BktName
			return resp, nil
		}
		_, err = tx.CreateBucket([]byte(req.BktName))
	case BktDelete:
		tx.DeleteBucket([]byte(req.BktName)) // NOTE - delete error is ignored
	case BktNextSeq:
		bkt := openBkt(tx, resp, req.BktName, CreateIfNotExists)
		if bkt == nil {
			return resp, nil
		}
		resp.NextSeq, err = bktNextSeq(bkt, req.NextSeqCount)
	case BktList:
		resp.Recs = make([][]byte, 0, 100)
		tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			resp.Recs = append(resp.Recs, name)
			return nil
		})
	case BktCount:
		bkt := tx.Bucket([]byte(req.BktName))
		if bkt == nil {
			resp.Status = StatusFail
			resp.Msg = fmt.Sprintf("bucket %s not found", req.BktName)
			return resp, nil
		}
		resp.GetCnt = bkt.Stats().KeyN
	default:
		resp.Status = StatusFail
		resp.Msg = "Invalid Bkt Operation-" + op
		return resp, nil
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

// DeleteRequest is used to delete specific records by Key.
// Keys not found are ignored.
type DeleteRequest struct {
	BktName string
	Keys    []string // keys of records to be deleted
}

func (req DeleteRequest) IsUpdtReq() bool {
	return true
}
func (req *DeleteRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	indexBkts, indexInvertedBkts, err := getIndexBkts(tx, req.BktName)
	if err != nil {
		log.Println("error getting index bkts for data bkt", req.BktName, err)
		resp.Status = StatusFail
		resp.Msg = "error getting index bkts for data bkt, see log for details"
		return resp, err
	}
	for _, key := range req.Keys {
		err := bkt.Delete([]byte(key))
		if err != nil { // key not found does not return error
			log.Println("db error - Delete failed", err)
			resp.Status = StatusFail
			resp.Msg = "Delete failed, see log for details"
			return resp, err // trans will be rolled back
		}
		// delete index entries for this data key
		for i, indexBkt := range indexBkts {
			indexInvertedBkt := indexInvertedBkts[i]
			indexKey := indexInvertedBkt.Get([]byte(key))
			if indexKey != nil {
				indexBkt.Delete(indexKey)
				indexInvertedBkt.Delete([]byte(key))
			}
		}
	}
	resp.Status = StatusOk
	return resp, nil
}

// getIndexBkts used by DeleteRequest.
// Inverted bkt key is data key, val is index key. This allows us to find index entry for a data key.
// getIndexBkts retrieves the index buckets and their corresponding inverted index buckets
// for a given data bucket name within a BoltDB transaction.
//
// It looks up index settings in the IndexSettingsBkt bucket, filters them by the provided
// dataBkt name, and collects the associated index and inverted index buckets.
//
// Parameters:
//
//	tx      - The BoltDB transaction to use for bucket access.
//	dataBkt - The name of the data bucket for which to find index buckets.
//
// Returns:
//
//	indexBkts         - A slice of pointers to the found index buckets.
//	indexInvertedBkts - A slice of pointers to the corresponding inverted index buckets.
//	err               - An error if any occurs during processing (e.g., unmarshalling settings).
func getIndexBkts(tx *bolt.Tx, dataBkt string) (indexBkts []*bolt.Bucket, indexInvertedBkts []*bolt.Bucket, err error) {

	settingsBkt := tx.Bucket([]byte(IndexSettingsBkt))
	if settingsBkt == nil {
		return nil, nil, nil // no index settings, so no index bkts
	}
	csr := settingsBkt.Cursor()
	prefix := []byte(dataBkt)
	var setting IndexSetting
	indexBkts = make([]*bolt.Bucket, 0, 5)
	indexInvertedBkts = make([]*bolt.Bucket, 0, 5)
	for k, v := csr.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = csr.Next() {
		err = json.Unmarshal(v, &setting)
		if err != nil {
			return nil, nil, fmt.Errorf("error unmarshalling index setting for index bkt %s - %s", string(k), err.Error())
		}
		if setting.DataBkt != dataBkt {
			continue // possible for prefix to match multiple data bkts, ex. "order" prefix matches "order", "order_item"
		}
		indexBkt := tx.Bucket([]byte(setting.IndexBkt))
		if indexBkt == nil {
			continue
		}
		indexBkts = append(indexBkts, indexBkt)

		indexInvertedBktName := setting.IndexBkt + "_inverted"
		indexInvertedBkt := tx.Bucket([]byte(indexInvertedBktName))
		if indexInvertedBkt == nil {
			continue
		}
		indexInvertedBkts = append(indexInvertedBkts, indexInvertedBkt)
	}
	return indexBkts, indexInvertedBkts, nil
}

// Export writes bkt records to a file as formatted json.
type ExportRequest struct {
	BktName  string
	StartKey string // if not "", keys >= this value
	EndKey   string // if not "", keys <= this value
	Limit    int    // max # recs to write
	FilePath string // where export file is written
}

func (req ExportRequest) IsUpdtReq() bool {
	return false
}
func (req *ExportRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	exportFile, err := os.Create(req.FilePath)
	if err != nil {
		log.Println("error creating export file", req.FilePath, err)
		resp.Status = StatusFail
		resp.Msg = "error creating export file:" + err.Error()
		return resp, nil
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
		return resp, nil
	}
	resp.Status = StatusOk
	return resp, nil
}

// CopyDB copies the open db to another file. Does not block other operations.
type CopyDBRequest struct {
	FilePath string `json:"filePath"`
}

func (req CopyDBRequest) IsUpdtReq() bool {
	return false
}
func (req *CopyDBRequest) Run(tx *bolt.Tx) (*Response, error) {
	resp := new(Response)
	err := tx.CopyFile(req.FilePath, 0600)
	if err != nil {
		resp.Status = StatusFail
		resp.Msg = err.Error()
	} else {
		resp.Status = StatusOk
	}
	return resp, nil
}
