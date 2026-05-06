package bobb

/*
Request types for managing indexes in Bobb.
IndexSettingRequest - load index settings into index_settings bkt
IndexRequest - add index entries for specific data keys, keys in a range, or all keys in a data bkt
VerifyIndexRequest - verify index entries are valid
*/

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	bolt "go.etcd.io/bbolt"
)

/* NOTE ----------------------------------------------
Type Indexr is located in types.go, it contains the logic for creating/updating index entries during PutRequest.
IndexSetting values, which are stored in the database, are used by the Indexr.
*/

// IndexSetting defines how index entries are constructed for a data bkt.
// They are loaded into the "index_settings" bkt.
// PutRequests use these settings to create/update index entries automatically when data records are added/updated in the data bkt.
//
// KeySuffixWidth is used to pad the index key suffix to fixed width with leading zeros.
// This ensures proper sorting of index keys.
// Example - if KeySuffixWidth is 6, index keys will end with suffixes like "000001", "000002", ..., "000010", etc.
type IndexSetting struct {
	DataBkt        string      // name of data bkt, ex. "inquiry"
	IndexBkt       string      // name of index bkt, must begin with value of DataBkt and end with "_index", ex. "inquiry_timestamp_index"
	KeyFlds        []FldFormat // defines how index key is constructed, if empty, index key is just IndexBkt NextSequence#
	FldSeparator   string      // optional separator used in merged field values, ex. "|" > "critical   |00033|temp high     "
	KeySuffixWidth int         // using IndexBkt nextSeq# add numeric suffix to index key, 0 means use KeySuffixWidth from bobb_setting.json, -1 no suffix
	SkipOnErr      bool        // if true, if error creating/updating index entry for a data rec, skip and do not fail entire PutRequest
}

// IndexSettingRequest loads IndexSettings into the "index_settings" bkt.
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

	settingsBkt := openBkt(tx, resp, IndexSettingsBkt, CreateIfNotExists)
	if settingsBkt == nil {
		return resp, nil
	}
	for _, setting := range req.IndexSettings {
		if setting.DataBkt == "" || setting.IndexBkt == "" {
			resp.Status = StatusFail
			resp.Msg = "IndexSetting missing DataBkt or IndexBkt"
			return resp, ErrBadInputData // trans will rollback
		}
		// index bkt name must begin with data bkt name and end with _index to ensure clear relationship and avoid confusion with regular bkts
		if !strings.HasPrefix(setting.IndexBkt, setting.DataBkt) || !strings.HasSuffix(setting.IndexBkt, "_index") {
			resp.Status = StatusFail
			resp.Msg = "IndexSetting IndexBkt must begin with DataBkt and end with _index"
			return resp, ErrBadInputData // trans will rollback
		}
		if setting.KeySuffixWidth == 0 {
			setting.KeySuffixWidth = KeySuffixWidth // use global KeySuffixWidth, set at startup by bobb_server.go from bobb_settings.json
		}
		if setting.KeySuffixWidth == -1 && len(setting.KeyFlds) == 0 {
			resp.Status = StatusFail
			resp.Msg = "IndexSetting must have KeyFlds and/or KeySuffixWidth > -1"
			return resp, ErrBadInputData // trans will rollback
		}
		if setting.KeySuffixWidth < 0 { // negative value means no suffix
			setting.KeySuffixWidth = 0
		}
		for _, fld := range setting.KeyFlds {
			if fld.FldName == "" || fld.FldType == "" || fld.Length <= 0 {
				resp.Status = StatusFail
				resp.Msg = "IndexSetting KeyFlds must have FldName, FldType and Length > 0"
				return resp, ErrBadInputData // trans will rollback
			}
			if fld.FldType != FldTypeStr && fld.FldType != FldTypeInt {
				resp.Status = StatusFail
				resp.Msg = fmt.Sprintf("IndexSetting KeyFlds have invalid FldType: %s, must be bobb.FldTypeStr('string') or bobb.FldTypeInt('int'), fld %s", fld.FldType, fld.FldName)
				return resp, ErrBadInputData // trans will rollback
			}
			if fld.FldType == FldTypeStr && !slices.Contains(AllStrOptions, fld.StrOption) {
				resp.Status = StatusFail
				resp.Msg = fmt.Sprintf("IndexSetting KeyFlds with FldTypeStr have invalid StrOption: %s, must be one of bobb.StrPlain, bobb.StrLowerCase, bobb.StrAsIs, fld %s", fld.StrOption, fld.FldName)
				return resp, ErrBadInputData // trans will rollback
			}
			if !slices.Contains(AllDefaultCodes, fld.UseDefault) {
				resp.Status = StatusFail
				resp.Msg = fmt.Sprintf("IndexSetting KeyFlds have invalid UseDefault: %s, must be one of bobb.DefaultAlways, bobb.DefaultNever, bobb.DefaultIsNull, bobb.DefaultNotFound, fld %s", fld.UseDefault, fld.FldName)
				return resp, ErrBadInputData // trans will rollback
			}
		}
		key := []byte(setting.IndexBkt)    // key for index_settings bkt is index bkt name
		val, err := json.Marshal(&setting) // val is json marshalled IndexSetting
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

// IndexRequest is used to add index records for specific data keys, keys in a range, or all keys in a data bkt.
// Only one indexing mode can be used per request:
//   - IndexAll: true — index all records in DataBkt
//   - StartKey/EndKey — index records within key range
//   - DataKeys — index specific records by key
//
// Errors are collected in resp.Errs until SkipOnErrLimit is exceeded.
// Warning - if there is potential for the result of MergeFlds to not be unique, a KeySuffix is required
type IndexRequest struct {
	DataBkt        string      // name of data bkt, DataKeys refer to this bkt
	IndexBkt       string      // name of index bkt, where index entries will be written
	MergeFlds      []FldFormat // defines index composition
	FldSeparator   string      // separator used in merged field values
	KeySuffixWidth int         // using IndexBkt nextSeq# add numeric suffix to index key, 0 means use KeySuffixWidth from bobb_setting.json, -1 no suffix
	DataKeys       []string    // index specific data records
	StartKey       string      // index records in range from StartKey
	EndKey         string      // index records in range to EndKey
	IndexAll       bool        // index all records in DataBkt
	SkipOnErrLimit int         // if errors exceed this limit, fail request with rollback
}

func (req IndexRequest) IsUpdtReq() bool {
	return true
}

func (req *IndexRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)

	dataBkt := openBkt(tx, resp, req.DataBkt)
	if dataBkt == nil {
		return resp, nil
	}
	indexBkt := openBkt(tx, resp, req.IndexBkt, CreateIfNotExists)
	if indexBkt == nil {
		return resp, nil
	}

	// resolve KeySuffixWidth: 0 = use global setting, -1 = no suffix
	keySuffixWidth := req.KeySuffixWidth
	if keySuffixWidth == 0 {
		keySuffixWidth = KeySuffixWidth // global KeySuffixWidth, set at startup by bobb_server.go from bobb_settings.json
	} else if keySuffixWidth < 0 {
		keySuffixWidth = 0
	}
	var suffixFormat string
	if keySuffixWidth > 0 {
		suffixFormat = "%0" + strconv.Itoa(keySuffixWidth) + "d"
	}

	parser := parserPool.Get()
	defer parserPool.Put(parser)

	// addEntry parses one data record and writes its index entry.
	addEntry := func(k, v []byte) *BobbErr {
		parsedRec, err := parser.ParseBytes(v)
		if err != nil {
			return e(ErrParseRec, err.Error(), k, v)
		}
		indexKey, err := MergeFlds(parsedRec, req.MergeFlds, req.FldSeparator)
		if err != nil {
			return e("Index MergeFlds Error", err.Error(), k, v)
		}
		if suffixFormat != "" {
			seqNo, err := indexBkt.NextSequence()
			if err != nil {
				return e("Index Suffix Error", "NextSequence failed: "+err.Error(), k, v)
			}
			suffix := fmt.Sprintf(suffixFormat, seqNo)
			if indexKey == "" {
				indexKey = suffix
			} else {
				indexKey = indexKey + req.FldSeparator + suffix
			}
		}
		if indexKey == "" {
			return e("Index Error", "MergeFlds produced empty index key", k, v)
		}
		if err = indexBkt.Put([]byte(indexKey), k); err != nil {
			return e("Index Error", "index Put failed: "+err.Error(), k, v)
		}
		resp.PutCnt++
		return nil
	}

	// errCheck appends bErr and returns true if SkipOnErrLimit is exceeded.
	errCheck := func(bErr *BobbErr) bool {
		if bErr == nil {
			return false
		}
		resp.Errs = append(resp.Errs, *bErr)
		if len(resp.Errs) > req.SkipOnErrLimit {
			resp.Status = StatusFail
			resp.Msg = "too many errors, see resp.Errs for details"
			return true
		}
		return false
	}

	switch {
	case req.IndexAll:
		// index every record in DataBkt
		csr := dataBkt.Cursor()
		for k, v := csr.First(); k != nil; k, v = csr.Next() {
			if errCheck(addEntry(k, v)) {
				return resp, nil
			}
		}

	case req.StartKey != "" || req.EndKey != "":
		// index records within key range
		readLoop := NewReadLoop(dataBkt, nil)
		k, v, bErr := readLoop.Start(req.StartKey, req.EndKey, 0)
		for k != nil {
			if bErr != nil {
				if errCheck(bErr) {
					return resp, nil
				}
				k, v, bErr = readLoop.Next()
				continue
			}
			if errCheck(addEntry(k, v)) {
				return resp, nil
			}
			k, v, bErr = readLoop.Next()
		}

	default:
		// index specific data keys
		for _, key := range req.DataKeys {
			k := []byte(key)
			v := dataBkt.Get(k)
			if v == nil {
				if errCheck(e(ErrNotFound, "key not found in data bkt", k, nil)) {
					return resp, nil
				}
				continue
			}
			if errCheck(addEntry(k, v)) {
				return resp, nil
			}
		}
	}

	if len(resp.Errs) > 0 {
		resp.Status = StatusWarning
		resp.Msg = "completed with errors, see resp.Errs for details"
	} else {
		resp.Status = StatusOk
	}
	return resp, nil
}

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
