// indexrecs.go - program example to create index records in an index bucket for specific records in a data bucket

package main

import (
	"log"
	"net/http"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"
)

const dataBkt = "indexrecs_data"
const indexBkt = "indexrecs_data_locationtype_zip_index"

type IndexRecs_Rec struct {
	Id           string `json:"id"`
	Zip          string `json:"zip"`
	LocationType int    `json:"locationType"`
}

func (rec IndexRecs_Rec) RecId() string { // to satisfy DBRec interface (see client/data_conversion.go)
	return rec.Id
}

// ------------------------------------------------------------

var httpClient *http.Client = new(http.Client)

func main() {

	log.Println("indexrecs start")

	bo.BaseURL = "http://localhost:50555/" // change to use common client settings file
	bo.Debug = false

	bo.DeleteBkt(httpClient, dataBkt)
	bo.DeleteBkt(httpClient, indexBkt)

	loadDataBkt()

	resp, err := bo.Run(httpClient, bobb.OpIndexRequest, bobb.IndexRequest{
		DataBkt:  dataBkt,
		IndexBkt: indexBkt,
		DataKeys: []string{"100", "102", "103", "104"},
		MergeFlds: []bobb.FldFormat{
			{FldName: "locationType", FldType: bobb.FldTypeInt, Length: 2, UseDefault: bobb.DefaultNever},
			{FldName: "zip", FldType: bobb.FldTypeStr, Length: 5, UseDefault: bobb.DefaultNever},
		},
		FldSeparator:   "|", // ex. 02|54633,
		KeySuffixWidth: 3,   // adds indexBkt next seq# as suffix to index key (ex. 02|54633|001) to allow for multiple records with same zip and locationType values.
	})
	if resp.Status != bobb.StatusOk {
		log.Fatalln("IndexRequest failed", resp.Status, resp.Msg, err)
	}

	resp, err = bo.Run(httpClient, bobb.OpGetAllKeys, bobb.GetAllKeysRequest{BktName: indexBkt})
	if resp.Status != bobb.StatusOk {
		log.Fatalln("GetAllKeysRequest failed", resp.Status, resp.Msg, err)
	}
	log.Println("indexkeys")
	for _, key := range resp.Recs {
		log.Println(string(key))
	}
	resp, err = bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{BktName: indexBkt})
	log.Println("indexvalues")
	for _, r := range resp.Recs {
		log.Println(string(r))
	}
	resp, err = bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{BktName: dataBkt, IndexBkt: indexBkt})
	log.Println("data recs in index order, rec id 101 was not indexed so not in this list")
	recs := bo.JsonToSlice(resp.Recs, IndexRecs_Rec{})
	for _, rec := range recs {
		log.Println(rec)
	}
}

func loadDataBkt() {
	// load data bucket with records to be indexed
	recs := []IndexRecs_Rec{
		{Id: "100", Zip: "11111", LocationType: 1},
		{Id: "101", Zip: "11111", LocationType: 2},
		{Id: "102", Zip: "11111", LocationType: 3},
		{Id: "103", Zip: "54633", LocationType: 1},
		{Id: "104", Zip: "54633", LocationType: 3},
	}
	resp, err := bo.Put(httpClient, dataBkt, bo.SliceToJson(recs), nil)
	if resp.Status != bobb.StatusOk || err != nil {
		log.Fatalln("loadDataBkt failed, error in PutRequest", resp.Status, resp.Msg, err)
	}
}
