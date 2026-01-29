package bobb

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

type IndexSetting struct {
	DataBkt     string      // name of data bkt, ex. "inquiry"
	IndexBkt    string      // name of index bkt, must begin with value of DataBkt and end with "_index", ex. "inquiry_timestamp_index"
	MergeFlds   []FldFormat // defines how index key, is constructed
	NoKeySuffix bool        // if true, IndexBkt NextSeq# will not be added to end of MergeFlds value, to ensure uniqueness
	DataIdFld   string      // name of field in data bkt recs containing key value (ex. "id")
}

//PutRequest IndexOption Codes (IndexingNormal default) used in PutRequest

//IndexingNormal - adds and updates to index bkts (most processing)
//IndexingOff - no adds or updates to index bkts
//IndexingNoUpdate - no updates to index bkts, only adds (no checks for index already existing for data key)

// IndexSettingRequest loads IndexSettings into the index_settings bkt.
// Key is value of IndexSetting.IndexBkt.
// Val is json.Marshalled instance of IndexSetting.
type IndexSettingRequest struct {
	IndexSettings []IndexSetting
}

func (req IndexSettingRequest) IsUpdtReq() bool {
	return true
}

func (req *IndexSettingRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)

	settingsBkt := openBkt(tx, resp, "index_settings", CreateIfNotExists)
	if settingsBkt == nil {
		return resp, nil
	}

	var indexBkt *bolt.Bucket
	var prevIndexName string
	for _, setting := range req.IndexSettings {
		if setting.IndexBkt != prevIndexName {
			indexBkt = openBkt(tx, resp, setting.IndexBkt, CreateIfNotExists)
			if indexBkt == nil {
				return resp, ErrBadInputData // trans will rollback
			}
			prevIndexName = setting.IndexBkt
		}
		if setting.DataIdFld == "" {
			resp.Status = StatusFail
			resp.Msg = "IndexSetting missing DataIdFld"
			return resp, ErrBadInputData // trans will rollback
		}
		key := []byte(setting.IndexBkt)
		val, err := json.Marshal(&setting)
		if err != nil {
			resp.Status = StatusFail
			resp.Msg = "IndexSetting json marshal error - " + err.Error()
			return resp, err // trans will be rolled back
		}
		err = settingsBkt.Put(key, val)
		if err != nil {
			resp.Status = StatusFail
			resp.Msg = "IndexSetting put error - " + err.Error()
			return resp, err // trans will be rolled back
		}
		resp.PutCnt++
	}
	resp.Status = StatusOk
	return resp, nil
}

// IndexRequest is used to add index records for specific data keys.
/*
type IndexRequest struct {
	DataBkt      string      // data bkt, DataKeys refer to this bkt
	IndexBkt     string      // where index entries will be written
	DataKeys     []string    // used for IndexSpecific option
	FldSeparator string      // separator used in merged field values
	MergeFlds    []FldFormat // defines index composition
}

func (req IndexRequest) IsUpdtReq() bool {
	return true
}

func (req *IndexRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	index := openBkt(tx, resp, req.IndexBkt, CreateIfNotExists)
	if index == nil {
		return resp, nil
	}
	for _, key := range req.DataKeys {
		v := bkt.Get([]byte(key))
		if v == nil {
			bErr := e(ErrNotFound, "Key Not Found", []byte(key), nil)
			resp.Errs = append(resp.Errs, *bErr)
			if len(resp.Errs) > req.ErrLimit {
				resp.Status = StatusFail
				resp.Msg = "too many errors, see resp.Errs for details"
				return resp, nil
			}
			continue
		}
		resp.PutCnt++
	}
	resp.Status = StatusOk
	return resp, nil
}

// IndexAllRequest is used to add index records for all data keys in range.
type IndexAllRequest struct {
	DataBkt      string      // data bkt, DataKeys refer to this bkt
	IndexBkt     string      // where index entries will be written
	StartKey     string      // index all records where data key >=
	EndKey       string      // index all records where data key <=
	FldSeparator string      // separator used in merged field values
	MergeFlds    []FldFormat // defines fields used to form index key
}
*/

// VerifyIndexRequest verifies index records are valid.
// Check index values are unique and refer to an existing data key.
// If AllDataIndexed true, verify all data keys have entry in index.
type VerifyIndexRequest struct {
	DataBkt        string // data bkt
	IndexBkt       string //
	AllDataIndexed bool   //if true, verify all data keys have entry in index
	ErrLimit       int    // limit of BobbErr's returned in Response.Errs
}

func (req VerifyIndexRequest) IsUpdtReq() bool {
	return false
}

func (req *VerifyIndexRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	dataBkt := openBkt(tx, resp, req.DataBkt)
	if dataBkt == nil {
		return resp, nil
	}
	indexBkt := openBkt(tx, resp, req.IndexBkt)
	if indexBkt == nil {
		return resp, nil
	}
	var k, v []byte

	// -- load data keys into map ---------------------

	dataKeys := make(map[string]bool)

	csr := dataBkt.Cursor()
	k, _ = csr.First()

	for k != nil {
		dataKeys[string(k)] = false
		k, _ = csr.Next()
	}

	// -- verify each index value exists once and only once in dataKeys -----------------

	csr = indexBkt.Cursor()
	k, v = csr.First()

	for k != nil {
		if len(resp.Errs) > req.ErrLimit {
			break
		}
		alreadyChecked, keyExists := dataKeys[string(v)]
		if !keyExists { // index value does not exist in dataKeys
			bErr := e(ErrInvalidIndexValue, "index value is not a valid data key", k, v)
			resp.Errs = append(resp.Errs, *bErr)
			k, v = csr.Next()
			continue
		}
		if alreadyChecked { // duplicate index for same data key detected
			bErr := e(ErrDuplicateIndexValue, "index value is not unique", k, v)
			resp.Errs = append(resp.Errs, *bErr)
			k, v = csr.Next()
			continue
		}
		dataKeys[string(v)] = true
		k, v = csr.Next()
	}

	if req.AllDataIndexed {
		for dataKey, indexFound := range dataKeys {
			if !indexFound {
				bErr := e(ErrDataKeyNotIndexed, "data rec has no entry in index", []byte(dataKey), nil)
				resp.Errs = append(resp.Errs, *bErr)
				if len(resp.Errs) > req.ErrLimit {
					break
				}
			}
		}
	}
	if len(resp.Errs) > 0 {
		resp.Status = StatusFail
		resp.Msg = "problem(s) with index detected, see resp.Errs for details"
	} else {
		resp.Status = StatusOk
	}
	return resp, nil
}
