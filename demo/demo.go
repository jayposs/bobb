// demo.go provides examples of how to use all bobb requests and validates results are correct.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"time"

	"bobb"
	bo "bobb/client"
	data "bobb/demodata" // data types(datatypes.go) & conversion funcs(datafuncs.go)
)

const locationBkt = "location"                // name of bkt used for all tests
const locationZipIndex = "location_zip_index" // index bkt

var httpClient *http.Client

var locationData map[string]data.Location // loaded from location_data.json file, key is Id value

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("demo program starting")

	//bo.BaseURL = "http://localhost:8000/" // must be where server is listening

	bo.Debug = false

	httpClient = new(http.Client)

	loadLocationData("location_data.json") // load test data from json file into locationData map

	bo.DeleteBkt(httpClient, locationBkt) // using shortcut func in client pkg
	bo.CreateBkt(httpClient, locationBkt)

	put()
	get("100", "102", "999")
	getAll()
	putOne("102") // changes record in locationData and updates record in db
	getOne("102") // gets updated record and verifies it matches record in locationMap
	getAllRange("100", "102")
	getAllKeys()
	qry1() // find str startsWith, sort intDesc, strAsc
	qry2() // find str greaterthan, str contains, int equals, sort st desc, city asc
	qry3() // uses Not condition
	putIndex()
	updateIndex()
	getIndex()
	qryIndex()
	update("999")
	delete("101", "999")
	getNextSeq()
	export()
	copyDB()
	putBkts() // new feature added May 1, 2024
	getValues()

	log.Println("*** Demo Pgm Finished Successfully ***")
}

// -- put ------------------------------------------------------
// Loads all records in locationData map into location bkt.
// Data records must contain the key value. KeyField is the fld to be used for the key.
func put() {
	log.Println("-- put starting -----")

	// Put requires records be json.Marshalled
	putRecs := data.MapToJson(locationData) // convert map of db recs to slice of json recs
	if putRecs == nil {
		log.Fatalln("put failed")
	}
	req := bobb.PutRequest{
		BktName:  locationBkt,
		KeyField: "id", // json tag value of key field
		Recs:     putRecs,
	}
	resp, err := bo.Run(httpClient, bobb.OpPut, req)

	checkResp(resp, err, "put")

	log.Println("-- put done -----")
}

// -- get ------------------------------------
// Retrieves records with specific keys.
func get(recIds ...string) {
	log.Println("-- get starting -----")

	req := bobb.GetRequest{
		BktName: locationBkt,
		Keys:    recIds,
	}
	resp, err := bo.Run(httpClient, bobb.OpGet, req)
	checkResp(resp, err, "get")

	results := data.JsonToMap(resp.Recs, data.Location{}) // convert resp recs to map of Location recs

	// confirm results match desired results
	for _, id := range recIds {
		original := locationData[id]
		result := results[id]
		compare(original, result, "get")
	}
	log.Println("-- get done -----")
}

// -- getAll ------------------------------------------------
// Retrieves all records in a bkt.
func getAll() {
	log.Println("-- getAll starting -----")

	req := bobb.GetAllRequest{
		BktName: locationBkt,
	}
	resp, err := bo.Run(httpClient, bobb.OpGetAll, req)
	checkResp(resp, err, "getAll")

	results := data.JsonToMap(resp.Recs, data.Location{}) // convert resp recs to map of Location recs

	// confirm results match desired results
	for id, original := range locationData {
		log.Println(results[id])
		compare(original, results[id], "getAll")
	}
	log.Println("-- getAll done -----")
}

// -- putOne -------------------------------------
// Puts a single record.
func putOne(id string) {
	log.Println("-- putOne starting -----")

	// update entry in locationData map, it will also be used for comparison in getOne
	rec := locationData[id]
	rec.LastActionDt = time.Now().Format(time.DateOnly)
	locationData[id] = rec

	err := bo.PutOne(httpClient, locationBkt, "id", &rec) // using shortcut func in client pkg
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("-- putOne done -----")
}

// -- getOne ---------------------------------------
// Retrieves a single record by key.
func getOne(id string) {
	log.Println("-- getOne starting -----")

	var result data.Location
	err := bo.GetOne(httpClient, locationBkt, id, &result) // use shortcut func in client pkg
	if err != nil {
		log.Fatalln(err)
	}
	// confirm results match desired results
	original := locationData[id]
	compare(original, result, "getOne")

	log.Println("-- getOne done -----")
}

// -- getAllRange -----------------------------------------
// Gets all records in a range between start & end keys
func getAllRange(start, end string) {
	log.Println("-- getAllRange starting -----")

	req := bobb.GetAllRequest{
		BktName:  locationBkt,
		StartKey: start,
		EndKey:   end,
	}
	resp, err := bo.Run(httpClient, bobb.OpGetAll, req)
	checkResp(resp, err, "getAllRange")

	results := data.JsonToMap(resp.Recs, data.Location{}) // convert resp recs to map of Location recs

	// confirm results match desired results
	for id, original := range locationData {
		if id < start || id > end {
			continue
		}
		compare(original, results[id], "getAll")
	}
	log.Println("-- getAllRange done -----")
}

// -- getAllKeys ------------------------------------------------
// Returns all keys from bkt into resp.Recs.
// Keys are converted from []byte to strings.
func getAllKeys() {
	log.Println("-- getAllKeys starting -----")

	req := bobb.GetAllKeysRequest{
		BktName: locationBkt,
	}
	resp, err := bo.Run(httpClient, bobb.OpGetAllKeys, req)
	checkResp(resp, err, "getAllKeys")

	// convert keys in resp.Recs to strings
	results := make([]string, len(resp.Recs))
	for i, key := range resp.Recs {
		results[i] = string(key)
	}
	// build original keys slice in key order
	original := make([]string, 0, len(locationData))
	for k := range locationData {
		original = append(original, k)
	}
	slices.Sort(original)

	// confirm results match desired results
	n := slices.Compare(original, results)
	if n != 0 {
		log.Fatalln("compare keys failed", "getAllKeys")
	}
	log.Println("-- getAllKeys done -----")
}

// -- qry1 --------------------------------------
// Returns records meeting find conditions in sorted order
func qry1() {
	log.Println("-- qry1 starting -----")

	criteria := []bobb.FindCondition{
		{Fld: "zip", Op: bobb.FindStartsWith, ValStr: "54"},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "locationType", Dir: bobb.SortDescInt},
		{Fld: "address", Dir: bobb.SortAscStr},
	}
	req := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	checkResp(resp, err, "qry1")

	matchingIds := []string{"104", "102", "103"} // resp recs should be in same order
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalln("qry1 wrong number of resp recs-", len(resp.Recs))
	}
	results := data.JsonToSlice(resp.Recs, data.Location{}) // convert resp recs to map of Location recs

	// confirm results match desired results
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "qry1")
	}
	log.Println("-- qry1 done -----")
}

// -- qry2 -------------------------------------------------
// Returns records meeting find conditions in sorted order.
func qry2() {
	log.Println("-- qry2 starting -----")

	criteria := []bobb.FindCondition{
		{Fld: "st", Op: bobb.FindAfter, ValStr: "ok"},
		{Fld: "address", Op: bobb.FindContains, ValStr: "ave"},
		{Fld: "locationType", Op: bobb.FindEquals, ValInt: 3},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "st", Dir: bobb.SortDescStr},
		{Fld: "city", Dir: bobb.SortAscStr},
	}
	req := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	checkResp(resp, err, "qry2")

	matchingIds := []string{"999", "104"} // resp recs should be in same order
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalln("qry2 wrong number of resp recs-", len(resp.Recs))
	}
	results := data.JsonToSlice(resp.Recs, data.Location{}) // convert resp recs to slice of Location recs

	// confirm results match desired results
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "qry2")
	}
	log.Println("-- qry2 done -----")
}

// -- qry3 ----------------------------
// Uses Not find condition.
func qry3() {
	log.Println("-- qry3 starting -----")

	criteria := []bobb.FindCondition{
		{Fld: "st", Op: bobb.FindMatches, ValStr: "TN"},
		{Fld: "locationType", Op: bobb.FindEquals, ValInt: 3, Not: true},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "city", Dir: bobb.SortAscStr},
	}
	req := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	checkResp(resp, err, "qry3")

	matchingIds := []string{"102", "103"} // resp recs should be in same order
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalln("qry3 wrong number of resp recs-", len(resp.Recs))
	}
	results := data.JsonToSlice(resp.Recs, data.Location{}) // convert resp recs to slice of Location recs

	// confirm results match desired results
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "qry3")
	}
	log.Println("-- qry3 done -----")
}

// -- putIndex -----------------------------------------------
// Put records into an index bkt.
// Index-Key composed from data value(s). Index-Val is data rec key.
func putIndex() {
	log.Println("-- putIndex starting -----")

	bo.DeleteBkt(httpClient, locationZipIndex)
	bo.CreateBkt(httpClient, locationZipIndex)

	indexes := make([]bobb.IndexKeyVal, 0, len(locationData))
	for _, rec := range locationData {
		key := fmt.Sprintf("%s|%s", rec.Zip, rec.Id) // add suffix that makes key unique
		val := rec.Id
		indexEntry := bobb.IndexKeyVal{Key: key, Val: val}
		indexes = append(indexes, indexEntry)
	}
	req := bobb.PutIndexRequest{
		BktName: locationZipIndex,
		Indexes: indexes,
	}
	resp, err := bo.Run(httpClient, bobb.OpPutIndex, req)
	checkResp(resp, err, "putIndex")

	log.Println("-- putIndex done -----")
}

// -- updateIndex -----------------------------------------------
// Use the OldKey in IndexKeyVal, so that existing index rec is deleted.
func updateIndex() {
	log.Println("-- updateIndex starting -----")

	oldKey := "54633|104"
	newKey := "54633|104-b"
	newIndex := bobb.IndexKeyVal{
		Key:    newKey,
		Val:    "104",
		OldKey: oldKey,
	}
	req := bobb.PutIndexRequest{
		BktName: locationZipIndex,
		Indexes: []bobb.IndexKeyVal{newIndex},
	}
	resp, err := bo.Run(httpClient, bobb.OpPutIndex, req)
	checkResp(resp, err, "updateIndex")

	// verify old index removed and new one added
	req2 := bobb.GetAllKeysRequest{
		BktName: locationZipIndex,
	}
	resp, _ = bo.Run(httpClient, bobb.OpGetAllKeys, req2)

	var newKeyFound bool
	for _, r := range resp.Recs {
		if string(r) == oldKey {
			log.Fatalln("updateIndex failed, old key not removed")
		}
		if string(r) == newKey {
			newKeyFound = true
		}
	}
	if !newKeyFound {
		log.Fatalln("updateIndex failed, new key not found")
	}
	log.Println("-- updateIndex done -----")
}

// -- getIndex --
// Uses start/end keys in index bkt to retrieve records from data bkt.
func getIndex() {
	log.Println("-- getIndex starting -----")

	req := bobb.GetIndexRequest{
		BktName:  locationBkt,
		IndexBkt: locationZipIndex,
		StartKey: "30000", // zip >= 30000
		EndKey:   "60000", // zip <= 60000
	}
	resp, err := bo.Run(httpClient, bobb.OpGetIndex, req)
	checkResp(resp, err, "getIndex")

	results := data.JsonToSlice(resp.Recs, data.Location{}) // convert resp recs to slice of Location recs

	// confirm results match desired results
	matchingIds := []string{"101", "102", "104", "103"} // returned in zip code order
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "getIndex")
	}
	log.Println("-- getIndex done -----")
}

// -- qryIndex -------------------------------------------
// Retrieves records using index bkt to control which records are read.
// In this example, only location records where zip >= 54700 are scanned.
func qryIndex() {
	log.Println("-- qryIndex starting -----")
	criteria := []bobb.FindCondition{
		{Fld: "address", Op: bobb.FindContains, ValStr: "ave"},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "city", Dir: bobb.SortDescStr},
	}
	req := bobb.QryIndexRequest{
		BktName:        locationBkt,
		IndexBkt:       locationZipIndex,
		StartKey:       "54700",
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, err := bo.Run(httpClient, bobb.OpQryIndex, req)
	checkResp(resp, err, "qryIndex")

	results := data.JsonToSlice(resp.Recs, data.Location{}) // convert resp recs to slice of Location recs

	// confirm results match desired results
	matchingIds := []string{"999", "103"}
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "qryIndex")
	}
	log.Println("-- qryIndex done -----")
}

// -- update ---------------------------------------------
// Uses update method of Location type to change a single field.
func update(id string) {
	log.Println("-- update starting -----")

	dateStamp := time.Now().Format(time.DateOnly)

	// modify record in locationData map
	original := locationData[id]
	original.LastActionDt = dateStamp
	locationData[id] = original

	// get current rec from db using GetOne func in client pkg
	var currRec data.Location
	if err := bo.GetOne(httpClient, locationBkt, id, &currRec); err != nil {
		log.Fatalln(err)
	}

	// update lastActionDt in retrieved record
	updates := map[string]any{
		"lastActionDt": dateStamp,
	}
	if err := currRec.Update(updates); err != nil {
		log.Fatalln(err)
	}
	if err := bo.PutOne(httpClient, locationBkt, "id", &currRec); err != nil {
		log.Fatalln(err)
	}
	// get record to make sure it updated correctly
	var newRec data.Location
	if err := bo.GetOne(httpClient, locationBkt, id, &newRec); err != nil {
		log.Fatalln(err)
	}
	compare(locationData[id], newRec, "update")

	log.Println("-- update done -----")
}

// -- delete ---------------------------------------
// Deletes rec(s) matching ids.
func delete(ids ...string) {
	log.Println("-- delete starting -----")

	req := bobb.DeleteRequest{
		BktName: locationBkt,
		Keys:    ids,
	}
	resp, err := bo.Run(httpClient, bobb.OpDelete, req)
	checkResp(resp, err, "delete")

	// verify rec is deleted
	var rec data.Location
	if err = bo.GetOne(httpClient, locationBkt, ids[0], &rec); err.Error() != "not found" {
		log.Fatalln("delete failed-", err)
	}

	log.Println("-- delete done -----")
}

// -- getNextSeq ---------------------------------------------------------
// Uses bkt request with "nextseq" operation, to get next sequence numbers
func getNextSeq() {
	log.Println("-- getNextSeq starting -----")

	req := bobb.BktRequest{
		BktName:      locationBkt,
		Operation:    "nextseq",
		NextSeqCount: 5, // returns the next 5 sequence numbers
	}

	resp, err := bo.Run(httpClient, bobb.OpBkt, req)
	checkResp(resp, err, "getNextSeq")

	if n := slices.Compare(resp.NextSeq, []int{1, 2, 3, 4, 5}); n != 0 {
		log.Fatalln("getNextSeq failed -", resp.NextSeq)
	}
	log.Println("-- getNextSeq done -----")
}

// -- export ------------------------------
// Write contents of bkt as formatted json.
func export() {
	log.Println("-- export starting -----")

	req := bobb.ExportRequest{
		BktName:  locationBkt,
		FilePath: "demo_export.json",
	}
	resp, err := bo.Run(httpClient, bobb.OpExport, req)
	checkResp(resp, err, "export")

	log.Println("-- export done -----")
}

// -- copyDB ----------------------------
// Copy open db to another file.
func copyDB() {
	log.Println("-- copyDB starting -----")

	req := bobb.CopyDBRequest{
		FilePath: "demo_copy.db",
	}
	resp, err := bo.Run(httpClient, bobb.OpCopyDB, req)
	checkResp(resp, err, "copyDB")

	log.Println("-- copyDB done -----")
}

// -- putBkts ---------------------------------------------------
// new feature added 2024-5-1
func putBkts() {
	log.Println("-- putBkts starting -----")

	bo.DeleteBkt(httpClient, "order")
	bo.CreateBkt(httpClient, "order")
	bo.DeleteBkt(httpClient, "order_item")
	bo.CreateBkt(httpClient, "order_item")

	var order1 = data.Order{
		Id:         "00377_00005244",
		OrderDate:  "2024-05-23",
		CustomerId: "00377",
	}
	var items = []data.OrderItem{
		{"00377_00005244_001", "00377_00005244", 1, "A4576", 2},
		{"00377_00005244_002", "00377_00005244", 2, "A1721", 1},
		{"00377_00005244_003", "00377_00005244", 3, "C1600", 5},
	}

	jsonOrder, _ := json.Marshal(&order1)
	jsonItems := make([][]byte, len(items))
	for i, item := range items {
		jsonItem, _ := json.Marshal(&item)
		jsonItems[i] = jsonItem
	}
	req := bobb.PutBktsRequest{
		BktName:  "order",
		KeyField: "id",
		Recs:     [][]byte{jsonOrder},
		Bkt2Name: "order_item",
		Recs2:    jsonItems,
	}
	resp, err := bo.Run(httpClient, bobb.OpPutBkts, req)
	checkResp(resp, err, "putBkts")

	// verify order in db matches order sent
	var savedOrder data.Order // order saved to db
	bo.GetOne(httpClient, "order", order1.Id, &savedOrder)
	if order1 != savedOrder {
		log.Fatalln("putBkts db order does not match sent order")
	}

	// verify order items in db match items sent
	req2 := bobb.GetAllRequest{BktName: "order_item"}
	resp2, _ := bo.Run(httpClient, bobb.OpGetAll, req2)

	results2 := data.JsonToSlice(resp2.Recs, data.OrderItem{})
	for i, jsonRec := range results2 {
		if items[i] != jsonRec {
			log.Fatalln("putBkts db item does not match sent item")
		}
	}

	//var savedItem orderItem
	//for i, jsonRec := range resp2.Recs {
	//		json.Unmarshal(jsonRec, &savedItem)
	//	if items[i] != savedItem {
	//		log.Fatalln("putBkts db item does not match sent item")
	//	}
	//}

	log.Println("-- putBkts done -----")
}

// --------------------------------------------------------------
// experimental requests
// --------------------------------------------------------------

// -- getValues ------------------------------------
// Retrieves specific field values from records with requested keys.

func getValues() {
	log.Println("-- getValues starting -----")

	req := bobb.GetValuesRequest{
		BktName: locationBkt,
		Keys:    []string{"102", "103", "104"},
		Fields:  []string{"address", "city", "locationType|int"},
	}
	resp, err := bo.Run(httpClient, bobb.OpGetValues, req)
	checkResp(resp, err, "getValues")

	results := make([]bobb.RecValues, len(resp.Recs))

	// load results from resp
	for i, jsonRec := range resp.Recs {
		json.Unmarshal(jsonRec, &results[i])
	}

	expectedResults := []bobb.RecValues{
		{Key: "102", FldVals: map[string]string{"address": "102 Nomad Lane", "city": "Hangover", "locationType": "2"}},
		{Key: "103", FldVals: map[string]string{"address": "103 Big Way Ave", "city": "Tuggville", "locationType": "2"}},
		{Key: "104", FldVals: map[string]string{"address": "900 Hammer Hill Ave", "city": "Anville", "locationType": "3"}},
	}
	// confirm results match expected results
	for i, expected := range expectedResults {
		r := results[i]
		if r.Key != expected.Key {
			log.Println("expected key - result key", expected.Key, r.Key)
			log.Fatalln("getValues failed")
		}
		for k, v := range expected.FldVals {
			if v != r.FldVals[k] {
				log.Println("expected - result ", v, r.FldVals[k])
				log.Fatalln("getValues failed, values don't match")
			}
		}
	}

	log.Println("-- getValues done -----")
}

// --------------------------------------------------------------
// load test data from json file into map locationData
func loadLocationData(fileName string) {
	jsonData, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatalln("error opening json data file", err)
	}
	inputRecs := make([]data.Location, 0, 100)
	if err := json.Unmarshal(jsonData, &inputRecs); err != nil {
		log.Fatalln("json.Unmarshal error on jsonData", err)
	}
	locationData = make(map[string]data.Location)
	for _, rec := range inputRecs {
		locationData[rec.Id] = rec
	}
}

func compare(original, result data.Location, funcName string) {
	originalStrVals := original.Id + original.Address + original.City + original.St + original.Zip + original.LastActionDt
	resultStrVals := result.Id + result.Address + result.City + result.St + result.Zip + result.LastActionDt
	if originalStrVals != resultStrVals {
		log.Println("original:", originalStrVals)
		log.Println("result:", resultStrVals)
		log.Fatalln("compare strvals failed", funcName)
	}
	for i, v := range original.Notes {
		if result.Notes[i] != v {
			log.Println("original:", original.Notes)
			log.Println("result:", result.Notes)
			log.Fatalln("compare notes failed", funcName)
		}
	}
	if original.LocationType != result.LocationType {
		log.Println("original:", original.LocationType)
		log.Println("result:", result.LocationType)
		log.Fatalln("compare int failed", funcName)
	}
}

func checkResp(resp *bobb.Response, err error, funcName string) {
	if resp == nil {
		log.Fatalln("nil response, check logs for info")
	}
	if !(resp.Status == bobb.StatusOk && err == nil) {
		log.Fatalln(funcName, "failed", resp.Msg, err)
	}
}
