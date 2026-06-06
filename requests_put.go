package bobb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strconv"
	"time"

	"github.com/valyala/fastjson"
	bolt "go.etcd.io/bbolt"
)

/*
Put requests add or replace entries in a bucket. If the key already exists, the entry value is replaced,
else a new entry is added. The key value must be contained in the KeyField in each record, defaults to defaultKeyFld
in bobb_settings.json.

Multiple buckets can be loaded in a single PutRequest. If there is an error, all updates will be rolled back.

See IndexSetting type in requests_index.go for information on indexing.

If a suffix is auto added to the key (see AddKeySuffix), the full key will be returned in resp.PutKeys,
so caller can see keys used.
*/

// PutParm(s) used by PutRequest to specify parameters for each put operation.
type PutParm struct {
	BktName        string   // data bkt where recs will be put, created if not exists
	KeyField       string   // fld in recs containing key value, default is defaultKeyFld from bobb_settings.json
	Recs           [][]byte // typically json marshaled value of records
	RequiredFlds   []string // optional, fld names that must be included in recs
	AddKeySuffix   bool     // if true, add bkt NextSeq# to end of key
	IndexingOption string   // see Indexing* codes in codes.go, IndexingNormal is default
	LogPut         bool     // if true, write record to bktname_putlog bkt. Key is dataKey|timestamp. Value is Rec. Provides point in time values.
}

// PutRequest is used to add or replace records.
// Multiple PutParms can be included in a single request, allows for multiple bkts to be updated in a single transaction.
type PutRequest struct {
	PutParms []PutParm
}

func (req PutRequest) IsUpdtReq() bool {
	return true
}

func (req *PutRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)

	parser := parserPool.Get()
	defer parserPool.Put(parser)

	var bkt, logBkt *bolt.Bucket
	var indexrs []Indexr
	var parsedRec *fastjson.Value
	var recKey, keyBytes []byte

	var putKeys []string // used to hold keys for all recs in a PutParm, added to resp.PutKeys at end of loop for recs in PutParm
	var putKeysNdx int   // index used for resp.PutKeys map

	suffixFormat := "%0" + strconv.Itoa(KeySuffixWidth) + "d" // ex. convert 123 to 00000123, see util.go for global KeySuffixWidth

	var totalRecs int
	for _, p := range req.PutParms {
		totalRecs += len(p.Recs)
	}
	resp.PutKeys = make(map[int][]string, len(req.PutParms)) // initialize map to hold keys used for each PutParm

	// -- Outer Loop, Executed once for each PutParm -----------------------------
	for parmNo, parms := range req.PutParms {
		err := validatePutParms(&parms) // validatePutParms also sets default values for certain parms if not included in request
		if err != nil {
			resp.Status = StatusFail
			resp.Msg = "PutRequest validation failed for PutParms index: " + strconv.Itoa(parmNo) + "-" + err.Error()
			return resp, err // trans will be rolled back
		}
		bkt = openBkt(tx, resp, parms.BktName, CreateIfNotExists)
		if bkt == nil {
			return resp, fmt.Errorf("invalid BktName - %s", parms.BktName) // trans will be rolled back
		}
		if parms.LogPut {
			logBkt = openBkt(tx, resp, parms.BktName+"_putlog", CreateIfNotExists)
			if logBkt == nil {
				log.Println("error opening put log bkt -", parms.BktName+"_putlog")
				return resp, fmt.Errorf("invalid log BktName - %s_putlog", parms.BktName) // trans will be rolled back
			}
		}
		if parms.IndexingOption != IndexingOff {
			indexrs, err = loadIndexrs(tx, parms.BktName)
			if err != nil {
				resp.Status = StatusFail
				resp.Msg = "PutRequest failed, error in loadIndexrs-" + err.Error()
				return resp, err
			}
		}

		// add keys used for prev PutParm to resp.PutKeys map
		if parmNo > 0 {
			resp.PutKeys[putKeysNdx] = putKeys
			putKeysNdx++
		}
		putKeys = make([]string, 0, len(parms.Recs)) // initialize slice to hold keys used for this PutParm

		// -- Inner Loop, Executed Once For Each Record -------------------------------------------------
		for recNo, rec := range parms.Recs { // parms.Recs is [][]byte

			// parse input rec ([]byte) to *fastjson.Value
			parsedRec, err = parser.ParseBytes(rec)
			if err != nil {
				resp.Status = StatusFail
				resp.Msg = "PutRequest failed, error in parsing rec-" + err.Error()
				return resp, ErrBadInputData // trans will be rolled back
			}
			// extract key value from parsedRec
			keyBytes = parsedRec.GetStringBytes(parms.KeyField)
			if keyBytes == nil {
				resp.Status = StatusFail
				resp.Msg = fmt.Sprintf("PutRequest failed, key field '%s' not found in ParmNo %d rec# %d", parms.KeyField, parmNo, recNo)
				return resp, ErrBadInputData // trans will be rolled back
			}
			recKey = make([]byte, len(keyBytes))
			copy(recKey, keyBytes) // make copy of keyBytes since recKey may be modified if AddKeySuffix is true, may not be safe to modify keyBytes

			// verify required fields are present in parsedRec
			for _, fld := range parms.RequiredFlds {
				if !parsedRec.Exists(fld) {
					resp.Status = StatusFail
					resp.Msg = fmt.Sprintf("PutRequest failed, required field '%s' not found in ParmNo %d rec# %d", fld, parmNo, recNo)
					return resp, ErrBadInputData // trans will be rolled back
				}
			}
			// if parms.AddKeySuffix, add bkt NextSeq# to end of key and update key field value in input rec
			if parms.AddKeySuffix {
				seqNo, err := bkt.NextSequence()
				if err != nil {
					log.Println("bkt.NextSequence failed -", parms.BktName, err)
					resp.Status = StatusFail
					resp.Msg = "PutRequest failed, error in bkt.NextSequence-" + parms.BktName + "-" + err.Error()
					return resp, err // trans
				}
				suffix := fmt.Sprintf(suffixFormat, seqNo)
				recKey = append(recKey, []byte(suffix)...)

				// fastjson.Value (fastKey) is required to update parsedRec
				fastKey, err := fastjson.Parse(`"` + string(recKey) + `"`) // keys with special characters may cause Parse to fail
				if err != nil {
					log.Println("fastjson.Parse failed for key with suffix -", string(recKey), err)
					resp.Status = StatusFail
					resp.Msg = "PutRequest failed, error in fastjson.Parse for key with suffix-" + string(recKey) + "-" + err.Error()
					return resp, err // trans will be rolled back
				}
				parsedRec.Set(parms.KeyField, fastKey) // update parsedRec with new key that includes suffix
				rec = parsedRec.MarshalTo(nil)         // set rec to updated marshaled value, []byte
			}

			err = bkt.Put(recKey, rec)
			if err != nil {
				log.Println("bkt.Put failed -", parms.BktName, err)
				resp.Status = StatusFail
				resp.Msg = "PutRequest failed, error in bkt.Put-" + parms.BktName + "-" + err.Error()
				return resp, err // trans will be rolled back
			}
			// add key used in bkt.Put to resp.PutKeys[parmNo], may be needed by caller if suffix was added
			putKeys = append(putKeys, string(recKey))

			resp.PutCnt++

			// if parms.LogPut, write record to log bkt with key format dataKey|timestamp for point in time values
			//   WARNING - if AddKeySuffix is true, key value in log will include suffix, so may not be ideal for use as point in time value since it will be different on each put, but it will work if you want to keep track of what was actually put in data bkt
			if parms.LogPut {
				logKey := string(recKey) + "|" + time.Now().Format(time.DateTime)
				err = logBkt.Put([]byte(logKey), rec)
				if err != nil {
					resp.Status = StatusFail
					resp.Msg = fmt.Sprintf("LogPut request failed for bkt %s - %s", parms.BktName+"_putlog", err.Error())
					return resp, err // trans will be rolled back
				}
			}
			// perform indexing if specified and if indexrs exist for this bkt
			if len(indexrs) > 0 {
				for _, indexr := range indexrs {
					err = indexr.Run(tx, recKey, parsedRec, parms.IndexingOption)
					if err != nil {
						resp.Status = StatusFail
						resp.Msg = "Put request indexing failed-" + err.Error()
						return resp, err
					}
				}
			}
		} // end of inner loop for recs in each PutParm
	} // end of outer loop for PutParms

	resp.PutKeys[putKeysNdx] = putKeys // add keys used for last PutParm to resp.PutKeys map

	resp.Status = StatusOk
	return resp, nil
}

// validatePutParms checks for required parms and sets default values for certain optional parms if not included in request.
func validatePutParms(parms *PutParm) error {
	if parms.BktName == "" {
		return fmt.Errorf("BktName cannot be blank")
	}
	if parms.KeyField == "" {
		parms.KeyField = DefaultKeyFld // see bobb_settings.json for global DefaultKeyFld value, typically "id"
	}
	if parms.IndexingOption == "" {
		parms.IndexingOption = IndexingNormal
	}
	if !slices.Contains(AllIndexingOptions, parms.IndexingOption) {
		return fmt.Errorf("invalid IndexingOption - %s", parms.IndexingOption)
	}
	if len(parms.Recs) == 0 {
		return fmt.Errorf("at least one record must be included in Recs")
	}
	return nil
}

// loadIndexrs loads indexrs for a data bkt using index settings from index_settings bkt
// The Indexr type which performs the indexing operations, is defined in indexr.go.
func loadIndexrs(tx *bolt.Tx, dataBkt string) (indexrs []Indexr, err error) {

	settingsBkt := tx.Bucket([]byte(IndexSettingsBkt))
	if settingsBkt == nil {
		return nil, nil // no index settings, so return empty slice, not an error
	}
	// load indexrs using IndexSettings for this dataBkt
	indexrs = make([]Indexr, 0, 5) // start with capacity of 5, will grow as needed, number of indexrs for a data bkt is typically small
	csr := settingsBkt.Cursor()
	prefix := []byte(dataBkt)
	var setting IndexSetting
	var indexr *Indexr
	for k, v := csr.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = csr.Next() {
		err = json.Unmarshal(v, &setting)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling index setting for index bkt %s - %s", string(k), err.Error())
		}
		if setting.DataBkt != dataBkt {
			continue // possible for prefix to match multiple data bkts, ex. "order" prefix matches "order", "order_item"
		}
		indexr, err = NewIndxr(tx, &setting)
		if err != nil {
			return nil, err
		}
		indexrs = append(indexrs, *indexr)
	}
	return indexrs, nil
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
