package test

import (
	"encoding/json"
	"log"
	"net/http"
	"testing"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"
	data "github.com/jayposs/bobb/datatypes" // contains test datatypes used by module programs such as demo
)

// -- testIndexSetting -------------------------------------------
// Tests automatic index maintenance via IndexSettingRequest + PutRequest + VerifyIndexRequest.
// Uses a dedicated bkt (index_test) to avoid interfering with other tests.
// Covers: index creation on Put, index update when record changes, VerifyIndexRequest,
// and GetAllRequest using an index bkt to return records in index key order.
func TestIndexing1(t *testing.T) {
	log.Println("-- testIndexing1 starting -----")

	bo.BaseURL = "http://localhost:50555/"
	bo.Debug = true

	httpClient := &http.Client{}

	const testBkt = "index_test"
	const testCityIndex = "index_test_city_index"

	// clean up any bkts from prior runs
	bo.DeleteBkt(httpClient, testBkt)
	bo.DeleteBkt(httpClient, testCityIndex)
	bo.DeleteBkt(httpClient, testCityIndex+"_inverted")

	// register a city+st composite index for testBkt
	// index key format: city (20 chars, lowercase) | st (2 chars, lowercase)
	setting := bobb.IndexSetting{
		DataBkt:  testBkt,
		IndexBkt: testCityIndex,
		KeyFlds: []bobb.FldFormat{
			{FldName: "city", FldType: bobb.FldTypeStr, Length: 20, StrOption: bobb.StrLowerCase, UseDefault: bobb.DefaultAlways},
			{FldName: "st", FldType: bobb.FldTypeStr, Length: 2, StrOption: bobb.StrLowerCase, UseDefault: bobb.DefaultAlways},
		},
	}
	settingReq := bobb.IndexSettingRequest{IndexSettings: []bobb.IndexSetting{setting}}
	resp, err := bo.Run(httpClient, bobb.OpIndexSetting, settingReq)
	checkResp(resp, err, "testIndexSetting - IndexSettingRequest")

	// put initial test records - PutRequest should auto-create index entries
	testRecs := []data.Location{
		{Id: "t1", City: "Memphis", St: "TN", Address: "1 Elm St", Zip: "38101"},
		{Id: "t2", City: "Austin", St: "TX", Address: "2 Oak Ave", Zip: "78701"},
		{Id: "t3", City: "Portland", St: "OR", Address: "3 Pine Rd", Zip: "97201"},
		{Id: "t4", City: "Memphis", St: "TN", Address: "4 Maple Dr", Zip: "38103"}, // same city as t1
	}
	jsonRecs := bo.SliceToJson(testRecs)
	resp, err = bo.Put(httpClient, testBkt, jsonRecs, nil)
	checkResp(resp, err, "testIndexSetting - Put")

	// index count should match data count
	indexCount := bo.GetRecCount(httpClient, testCityIndex)
	if indexCount != len(testRecs) {
		log.Fatalf("testIndexSetting - index count %d, expected %d", indexCount, len(testRecs))
	}

	// verify index integrity: all index values point to valid data keys, all data keys are indexed
	verifyReq := bobb.VerifyIndexRequest{
		DataBkt:        testBkt,
		IndexBkt:       testCityIndex,
		AllDataIndexed: true,
		ErrLimit:       10,
	}
	resp, err = bo.Run(httpClient, bobb.OpVerifyIndex, verifyReq)
	if len(resp.Errs) > 0 {
		log.Println("testIndexSetting - initial verify errors:")
		for _, e := range resp.Errs {
			log.Printf("  index key: %s, index val: %s, msg: %s\n", e.Key, e.Val, e.Msg)
		}
	}
	checkResp(resp, err, "testIndexSetting - VerifyIndex initial")

	// update t1: Memphis,TN -> Seattle,WA — index entry should move to new city key
	updated := testRecs[0]
	updated.City = "Seattle"
	updated.St = "WA"
	jsonRec, _ := json.Marshal(updated)
	resp, err = bo.Put(httpClient, testBkt, [][]byte{jsonRec}, nil)
	checkResp(resp, err, "testIndexSetting - Put updated rec")

	// index should still be valid after city change
	resp, err = bo.Run(httpClient, bobb.OpVerifyIndex, verifyReq)
	checkResp(resp, err, "testIndexSetting - VerifyIndex after update")
	if len(resp.Errs) > 0 {
		log.Fatalln("testIndexSetting - post-update verify errors:", resp.Errs)
	}

	// get all via index — should return records in ascending city order
	getAllReq := bobb.GetAllRequest{
		BktName:  testBkt,
		IndexBkt: testCityIndex,
	}
	resp, err = bo.Run(httpClient, bobb.OpGetAll, getAllReq)
	checkResp(resp, err, "testIndexSetting - GetAll via index")

	results := bo.JsonToSlice(resp.Recs, data.Location{})
	if len(results) != len(testRecs) {
		log.Fatalf("testIndexSetting - GetAll returned %d recs, expected %d", len(results), len(testRecs))
	}
	// after update: Austin(t2), Memphis(t4), Portland(t3), Seattle(t1)
	expectedOrder := []string{"t2", "t4", "t3", "t1"}
	for i, expectedId := range expectedOrder {
		if results[i].Id != expectedId {
			log.Fatalf("testIndexSetting - GetAll order wrong at pos %d: got id=%s city=%s, expected id=%s",
				i, results[i].Id, results[i].City, expectedId)
		}
	}

	// clean up
	bo.DeleteBkt(httpClient, testBkt)
	bo.DeleteBkt(httpClient, testCityIndex)
	bo.DeleteBkt(httpClient, testCityIndex+"_inverted")

	log.Println("-- testIndexSetting done -----")
}
