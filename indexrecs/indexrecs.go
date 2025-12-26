// indexrecs.go - program template to create index records in an index bucket for specific records in a data bucket
// This program is just an example. Different use cases may need a different approach.

package main

import (
	"log"
	"net/http"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"

	"github.com/valyala/fastjson"
)

// NOTE
// if an index record may already exist for a data record and the index key has changed,
// 		the existing index record must be deleted
// The IndexKeyVal.OldKey field can be used to specify the existing index key to be deleted.

// ------------------------------------------------------------
// parameters that would typically be passed to this program
var (
	dataBkt      = "location"                // name of bucket containing data records
	indexBkt     = "location_zip_type_index" // name of index bucket
	recIdFld     = "id"                      // field in data record used as record key
	fldSeparator = "|"                       // separator used in merged field values

	// Flds merged together to create index key
	mergeFlds = []bobb.FldFormat{
		{FldName: "zip", FldType: bobb.FldTypeStr, Length: 5},
		{FldName: "locationType", FldType: bobb.FldTypeInt, Length: 2},
	}
	recIds = []string{"100", "104"} // list of data record ids to index
)

// ------------------------------------------------------------

var jsonParser fastjson.Parser

var httpClient *http.Client = new(http.Client)

func main() {

	log.Println("indexrecs start")

	bo.BaseURL = "http://localhost:50555/" // change to use common client settings file
	bo.Debug = false

	req := bobb.GetRequest{
		BktName: dataBkt,
		Keys:    recIds,
	}
	resp, _ := bo.Run(httpClient, bobb.OpGet, req)
	if resp.Status != bobb.StatusOk {
		log.Fatalln("error in GetRequest", resp.Status, resp.Msg)
	}
	indexes := make([]bobb.IndexKeyVal, len(resp.Recs))

	for i, jsonRec := range resp.Recs {
		parsedRec, err := jsonParser.ParseBytes(jsonRec)
		if err != nil {
			log.Fatalf("cannot parse json: %s, %s", err, string(jsonRec))
		}
		recId := parsedRec.GetStringBytes(recIdFld)
		if recId == nil {
			log.Fatalln("error getting data key", string(jsonRec))
		}
		indexKey := bobb.MergeFlds(parsedRec, mergeFlds, fldSeparator) // merged plain string values
		indexKey = indexKey + fldSeparator + string(recId)             // append rec id to make index key unique

		log.Println(indexKey)
		// 11111|01|100
		// 54633|03|104

		indexes[i] = bobb.IndexKeyVal{Key: indexKey, Val: string(recId)}
	}
	// add index records
	req2 := bobb.PutIndexRequest{BktName: indexBkt, Indexes: indexes}
	resp2, err := bo.Run(httpClient, bobb.OpPutIndex, req2)

	if resp2.Status != bobb.StatusOk || err != nil {
		log.Fatalln("error in PutIndexRequest", resp2.Status, resp2.Msg, err)
	}

	log.Println("indexrecs complete")
}
