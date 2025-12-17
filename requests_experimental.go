// Experimental requests.

package bobb

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	bolt "go.etcd.io/bbolt"
)

const OpGetValues = "getvalues"

// RecValues is record type returned by GetValues
type RecValues struct {
	Key     string            // record key
	FldVals map[string]string // fldName:value
}

// GetValues returns specific values rather than entire record.
// Default field type is string. To get other types add "|type" to field name.
// Example "count|int". All return values converted to string.
// If value cannot be extracted from record, return value is empty string.
// Response.Recs loaded with slice of json marshalled RecValues (see type above)
// Valid type values: string, int, float64, bool (defaults to string)
type GetValuesRequest struct {
	BktName string
	Keys    []string // keys of records to be returned
	Fields  []string // field values to return
}

func (req GetValuesRequest) IsUpdtReq() bool {
	return false
}

func (req *GetValuesRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	resp.Status = StatusOk // may be changed below

	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	parser := parserPool.Get()
	defer parserPool.Put(parser)

	results := make([]RecValues, 0, len(req.Keys))

	for _, key := range req.Keys {
		v := bkt.Get([]byte(key))
		if v == nil {
			log.Println("GetValues request - key not found", req.BktName, key)
			resp.Status = StatusWarning
			resp.Msg = "not found"
			continue // NOTE - THIS BEHAVIOUR MAY NOT BE APPROPRIATE FOR ALL SITUATIONS
		}
		parsedRec, _ := parser.ParseBytes(v)

		fldVals := make(map[string]string)
		for _, fld := range req.Fields {
			fldName, fldType, _ := strings.Cut(fld, "|")

			if !parsedRec.Exists(fldName) {
				fldVals[fldName] = "fld not in rec-" + fldName
				continue
			}
			if fldType == "" {
				fldType = "string"
			}
			fldVal := ""

			// ** ERROR HANDLING NOT INCLUDED *****

			switch fldType {
			case "string":
				fldVal, _ = parsedRecGetStr(parsedRec, fldName, DefaultAlways)
			case "int":
				intVal, _ := parsedRecGetInt(parsedRec, fldName, DefaultAlways)
				fldVal = strconv.Itoa(intVal)
			case "float64":
				floatVal := parsedRec.GetFloat64(fldName)
				fldVal = strconv.FormatFloat(floatVal, 'f', -1, 64)
			case "bool":
				boolVal := parsedRec.GetBool(fldName)
				fldVal = strconv.FormatBool(boolVal)
			default:
				log.Println("GetValues Request - invalid field type - ", fldType)
			}
			fldVals[fldName] = fldVal
		}
		recVals := RecValues{
			Key:     key,
			FldVals: fldVals,
		}
		results = append(results, recVals)
	}
	resp.Recs = make([][]byte, len(results))

	var err error
	for i, result := range results {
		resp.Recs[i], err = json.Marshal(&result)
		if err != nil {
			log.Println("GetVals json.Marshal result entry failed-", err)
			resp.Status = StatusFail
			resp.Msg = "json.Marshal result failed-see server log"
			break
		}
	}
	return resp, nil
}

const OpSearchKeys = "searchkeys"

// SearchKeys returns records where key contains search value.
// If bkt is an index bkt, the returned value is the key of the indexed data record.
type SearchKeysRequest struct {
	BktName     string
	SearchValue string
	StartKey    string
	EndKey      string
	Limit       int
}

func (req SearchKeysRequest) IsUpdtReq() bool {
	return false
}
func (req *SearchKeysRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)
	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	resp.Recs = make([][]byte, 0, DefaultQryRespSize)

	csr := bkt.Cursor()
	var k, v []byte
	if req.StartKey == "" {
		k, v = csr.First()
	} else {
		k, v = csr.Seek([]byte(req.StartKey))
	}
	// if startKey == endKey, use matchPrefix logic
	var matchPrefix bool
	if req.StartKey != "" && req.StartKey == req.EndKey {
		matchPrefix = true
	}
	for k != nil {
		if matchPrefix { // all rec keys must begin with StartKey
			if !strings.HasPrefix(string(k), req.StartKey) {
				break
			}
		} else if req.EndKey != "" && string(k) > req.EndKey {
			break
		}
		if strings.Contains(string(k), req.SearchValue) {
			resp.Recs = append(resp.Recs, v)
			if len(resp.Recs) == req.Limit {
				break
			}
		}
		k, v = csr.Next()
	}
	resp.Status = StatusOk
	return resp, nil
}
