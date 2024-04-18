// File view_handlers.go contains funcs to process db.View (readonly) requests.
// These funcs are called by the dbHandler func in the server.go program.

package bobb

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"slices"

	bolt "go.etcd.io/bbolt"
)

var DefaultQryRespSize = 400 // response slice initial allocation for this size

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

// Get returns recs with keys matching requested keys.
func Get(tx *bolt.Tx, req *GetRequest) *Response {

	resp := new(Response)
	resp.Status = StatusOk // may be changed to Warning below if key not found

	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	resp.Recs = make([][]byte, 0, len(req.Keys))

	for _, key := range req.Keys {
		v := bkt.Get([]byte(key))
		if v == nil {
			log.Println("key not found", key)
			resp.Status = StatusWarning
			resp.Msg = "not found"
			continue // NOTE - THIS BEHAVIOUR MAY NOT BE APPROPRIATE FOR ALL SITUATIONS
		}
		resp.Recs = append(resp.Recs, v)
	}
	return resp
}

// GetOne returns a rec where key matches requested key.
func GetOne(tx *bolt.Tx, req *GetOneRequest) *Response {
	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	v := bkt.Get([]byte(req.Key))
	if v == nil {
		log.Println("GetOne key not found", req.Key)
		resp.Status = StatusWarning
		resp.Msg = "not found"
		return resp
	}
	resp.Rec = v
	resp.Status = StatusOk
	return resp
}

// GetAll returns all records in specified bucket.
// Optionally, Start and End keys can be included in the request.
// If StartKey != "", then result begins at 1st key >= Start key.
// If EndKey != "", then result ends at last key <= End key.
func GetAll(tx *bolt.Tx, req *GetAllRequest) *Response {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	resp.Recs = make([][]byte, 0, DefaultQryRespSize)

	csr := bkt.Cursor()

	var k, v []byte
	if req.StartKey == "" {
		k, v = csr.First()
	} else {
		k, v = csr.Seek([]byte(req.StartKey))
	}
	for k != nil {
		if req.EndKey != "" && string(k) > req.EndKey {
			break
		}
		resp.Recs = append(resp.Recs, v)
		if len(resp.Recs) == req.Limit { // note - if limit == 0, len(resp.Recs) cannot be zero when compare is made
			break
		}
		k, v = csr.Next()
	}
	resp.Status = StatusOk
	return resp
}

// GetAllKeys returns all keys in specified bucket.
// Keys are returned in the Response.Recs.
// All keys are json.Marshaled string.
func GetAllKeys(tx *bolt.Tx, req *GetAllKeysRequest) *Response {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	resp.Recs = make([][]byte, 0, 1000)

	csr := bkt.Cursor()

	var k []byte
	if req.StartKey == "" {
		k, _ = csr.First()
	} else {
		k, _ = csr.Seek([]byte(req.StartKey))
	}
	for k != nil {
		if req.EndKey != "" && string(k) > req.EndKey {
			break
		}
		resp.Recs = append(resp.Recs, k)
		if len(resp.Recs) == req.Limit { // note - if limit == 0, len(resp.Recs) cannot be zero when compare is made
			break
		}
		k, _ = csr.Next()
	}
	resp.Status = StatusOk
	return resp
}

// GetIndex uses secondary index specified in req.IndexBtk.
// Optionally, Start and End keys can be included in the request.
// If StartKey != "", then result begins at 1st key >= Start key.
// If EndKey != "", then result ends at last key <= End key.
func GetIndex(tx *bolt.Tx, req *GetIndexRequest) *Response {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	indexBkt := openBkt(tx, resp, req.IndexBkt)
	if indexBkt == nil {
		return resp
	}
	resp.Recs = make([][]byte, 0, DefaultQryRespSize)

	csr := indexBkt.Cursor()

	var indexKey, dataKey []byte
	if req.StartKey == "" {
		indexKey, dataKey = csr.First()
	} else {
		indexKey, dataKey = csr.Seek([]byte(req.StartKey))
	}
	for indexKey != nil {
		if req.EndKey != "" && string(indexKey) > req.EndKey {
			break
		}
		// get value from primary bkt using key stored in index
		val := bkt.Get(dataKey)
		if val == nil {
			log.Println("using index value, key not found in primary bkt", req.BktName, req.IndexBkt, indexKey, dataKey)
			resp.Status = StatusFail
			resp.Msg = "index value not found in data bkt, see server log"
			return resp
		}
		resp.Recs = append(resp.Recs, val)
		if len(resp.Recs) == req.Limit { // note - if limit == 0, len(resp.Recs) cannot be zero when compare is made
			break
		}
		indexKey, dataKey = csr.Next()
	}
	resp.Status = StatusOk
	return resp
}

// Export writes bkt recs to file in formatted json.
func Export(tx *bolt.Tx, req *ExportRequest) *Response {
	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	csr := bkt.Cursor()

	var k, v []byte
	if req.StartKey == "" {
		k, v = csr.First()
	} else {
		k, v = csr.Seek([]byte(req.StartKey))
	}
	var out bytes.Buffer
	var counter int
	out.WriteString("[\n")
	for k != nil {
		if req.EndKey != "" && string(k) > req.EndKey {
			break
		}
		if counter > 0 {
			out.WriteString(",\n")
		}
		json.Indent(&out, v, "", "  ")
		counter++
		if counter == req.Limit {
			break
		}
		k, v = csr.Next()
	}
	out.WriteString("\n]")

	exportFile, err := os.Create(req.FilePath)
	if err == nil {
		_, err = exportFile.Write(out.Bytes())
	}
	if err != nil {
		resp.Status = StatusFail
		resp.Msg = "error creating export file:" + err.Error()
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

// Qry returns records that meet request FindConditions and in specified sort order.
func Qry(tx *bolt.Tx, req *QryRequest) *Response {
	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	resultRecs := make(map[string][]byte, DefaultQryRespSize) // recs meeting criteria, map key is db Key, map value is db Value
	resultKeys := make([]string, 0, DefaultQryRespSize)       // loaded with keys of records meeting selection criteria

	var k, v []byte
	csr := bkt.Cursor()
	if req.StartKey == "" {
		k, v = csr.First()
	} else {
		k, v = csr.Seek([]byte(req.StartKey))
	}
	Trace("__ qry find start __")
	for k != nil {
		key := string(k)
		if req.EndKey != "" && key > req.EndKey {
			break
		}
		keep := true
		if len(req.FindConditions) > 0 {
			keep = recFind(v, req.FindConditions)
		}
		if keep {
			resultRecs[key] = v
			resultKeys = append(resultKeys, key)
			if len(resultKeys) == req.Limit { // note - if limit == 0, len(keys) is never zero when compare is made
				break
			}
		}
		k, v = csr.Next()
	}
	Trace("__ qry find done __")

	if len(req.SortKeys) > 0 {
		resultKeys = qrySort(req.SortKeys, resultKeys, resultRecs)
		if resultKeys == nil {
			resp.Status = StatusFail
			resp.Msg = "qry sort failed"
			return resp
		}
	}
	// load response.Recs slice in order based on sorted order of keys
	resp.Recs = make([][]byte, len(resultKeys))
	for i, key := range resultKeys {
		resp.Recs[i] = resultRecs[key]
	}
	resp.Status = StatusOk
	return resp
}

func QryIndex(tx *bolt.Tx, req *QryIndexRequest) *Response {
	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	indexBkt := openBkt(tx, resp, req.IndexBkt)
	if indexBkt == nil {
		return resp
	}
	resultRecs := make(map[string][]byte, DefaultQryRespSize) // recs meeting criteria, map key is db Key, map value is db Value
	resultKeys := make([]string, 0, DefaultQryRespSize)       // loaded with keys of records meeting selection criteria

	csr := indexBkt.Cursor()

	var indexKey, dataKey []byte // dataKey is value of index record
	if req.StartKey == "" {
		indexKey, dataKey = csr.First()
	} else {
		indexKey, dataKey = csr.Seek([]byte(req.StartKey))
	}
	for indexKey != nil {
		if req.EndKey != "" && string(indexKey) > req.EndKey {
			break
		}
		// get value from data bkt using key stored in index
		dataVal := bkt.Get(dataKey)
		if dataVal == nil {
			log.Println("using index value, key not found in data bkt", req.BktName, req.IndexBkt, indexKey, dataKey)
			resp.Status = StatusFail
			resp.Msg = "index value not found in data bkt"
			return resp
		}
		keep := true
		if len(req.FindConditions) > 0 {
			keep = recFind(dataVal, req.FindConditions)
		}
		if keep {
			key := string(dataKey)
			resultRecs[key] = dataVal
			resultKeys = append(resultKeys, key)
			if len(resultKeys) == req.Limit { // note - if limit == 0, len(keys) is never zero when compare is made
				break
			}
		}
		indexKey, dataKey = csr.Next()
	}
	if len(req.SortKeys) > 0 {
		resultKeys = qrySort(req.SortKeys, resultKeys, resultRecs)
		if resultKeys == nil {
			resp.Status = StatusFail
			resp.Msg = "qry sort failed"
			return resp
		}
	}
	// load response.Recs slice in order based on sorted order of keys
	resp.Recs = make([][]byte, len(resultKeys))
	for i, key := range resultKeys {
		resp.Recs[i] = resultRecs[key]
	}
	resp.Status = StatusOk
	return resp
}

// qrySort is used by Qry requests.
// It returns a copy of the resultKeys (record keys meeting selection criteria) in sorted order.
func qrySort(sortKeys []SortKey, resultKeys []string, resultData map[string][]byte) []string {
	if !sortKeysOk(sortKeys) {
		return nil
	}
	Trace("~ qry sort start ~")
	keys := make([]string, len(resultKeys))
	copy(keys, resultKeys)
	var n int
	slices.SortFunc(keys, func(a, b string) int { // slices pkg added in Go 1.21
		reca := resultData[a]
		recb := resultData[b]
		for _, sortKey := range sortKeys {
			if slices.Contains(StrSortCodes, sortKey.Dir) { // compare str values
				vala := recGetStr(reca, sortKey.Fld, StrToLower)
				valb := recGetStr(recb, sortKey.Fld, StrToLower)
				n = strCompare(vala, valb)
			} else if slices.Contains(IntSortCodes, sortKey.Dir) { // compare int values
				vala := recGetInt(reca, sortKey.Fld)
				valb := recGetInt(recb, sortKey.Fld)
				n = intCompare(vala, valb)
			}
			if n == 0 { // sort key values are equal
				continue
			}
			if slices.Contains(DescSortCodes, sortKey.Dir) { // if sort direction is descending, negate return value
				n = n * -1
			}
			return n
		}
		return 0 // all sort key values are equal
	})
	Trace("~ qry sort done ~")
	return keys
}
func sortKeysOk(sortKeys []SortKey) bool {
	for _, sortKey := range sortKeys {
		if slices.Contains(StrSortCodes, sortKey.Dir) || slices.Contains(IntSortCodes, sortKey.Dir) {
			continue
		}
		log.Println("ERROR - Invalid SortKey Dir Attribute", sortKey)
		return false
	}
	return true
}
