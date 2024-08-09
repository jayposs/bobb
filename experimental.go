// Experimental request handlers.

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
	Key     string            `json:"key"`     // record key
	FldVals map[string]string `json:"fldVals"` // fldName:value
}

// GetValues returns specific values rather than entire record.
// Default field type is string. To get other types add "|type" to field name.
// Example "count|int". All return values converted to string.
// If value cannot be extracted from record, return value is empty string.
// Response.Recs loaded with slice of json marshalled RecValues (see type above)
// Valid type values: string, int, float64, bool (defaults to string)
type GetValuesRequest struct {
	BktName string   `json:"bktName"`
	Keys    []string `json:"keys"`   // keys of records to be returned
	Fields  []string `json:"fields"` // field values to return
}

// GetValues returns specific values rather than entire record.
func GetValues(tx *bolt.Tx, req *GetValuesRequest) *Response {

	resp := new(Response)
	resp.Status = StatusOk // may be changed below

	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
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
			switch fldType {
			case "string":
				fldVal = parsedRecGetStr(parsedRec, fldName)
			case "int":
				intVal := parsedRecGetInt(parsedRec, fldName)
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
	return resp
}
