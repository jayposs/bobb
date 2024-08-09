// Generic funcs for converting data between jsonRecs [][]byte to map[string]DBRec
// and jsonRecs to []DBRec,, []DBRec to jsonRecs, and map[string]DBRec to jsonRecs.

package client

import (
	"encoding/json"
	"log"
)

// Record Types using these funcs must implement DBRec interface.
type DBRec interface {
	RecId() string
}

// JsonToMap creates map of db recs from slice of json recs.
// Parm dbRec is only used to specify the record type.
func JsonToMap[T DBRec](jsonRecs [][]byte, dbRec T) map[string]T {
	response := make(map[string]T, len(jsonRecs))
	var rec T
	var emptyRec T // required to deal with slices/maps in recs, rec = T{} does not work.
	for _, jsonRec := range jsonRecs {
		if err := json.Unmarshal(jsonRec, &rec); err != nil {
			log.Println("JsonToMap - json.Unmarshal error", err)
			return nil
		}
		id := rec.RecId()
		response[id] = rec
		rec = emptyRec
	}
	return response
}

// JsonToSlice creates slice of db recs from slice of json recs.
// Parm dbRec is only used to specify the record type.
func JsonToSlice[T DBRec](jsonRecs [][]byte, dbRec T) []T {
	response := make([]T, len(jsonRecs))
	var rec T
	var emptyRec T // required to deal with slices/maps in recs, rec = T{} does not work.
	for i, jsonRec := range jsonRecs {
		if err := json.Unmarshal(jsonRec, &rec); err != nil {
			log.Println("JsonToSlice - json.Unmarshal error", err)
			return nil
		}
		response[i] = rec
		rec = emptyRec
	}
	return response
}

// SliceToJson creates slice of json recs from slice of db recs.
func SliceToJson[T DBRec](recs []T) [][]byte {
	response := make([][]byte, len(recs))
	for i, rec := range recs {
		jsonRec, err := json.Marshal(&rec)
		if err != nil {
			log.Println("SliceToJson - json.Marshal error", err)
			return nil
		}
		response[i] = jsonRec
	}
	return response
}

// MapToJson creates slice of json recs from map of db recs.
func MapToJson[T DBRec](recs map[string]T) [][]byte {
	response := make([][]byte, 0, len(recs))
	for _, rec := range recs {
		jsonRec, err := json.Marshal(&rec)
		if err != nil {
			log.Println("MapToJson - json.Marshal error", err)
			return nil
		}
		response = append(response, jsonRec)
	}
	return response
}
