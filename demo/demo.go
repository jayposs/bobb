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

	bolt "go.etcd.io/bbolt"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"
)

const locationBkt = "location"                // name of bkt containing test data
const locationZipIndex = "location_zip_index" // index bkt
const locationLogBkt = "location_log"
const requestBkt = "request" // name of bkt used for join tests

var httpClient *http.Client

var locationData map[string]Location // loaded from location_data.json file, key is Id value
var requestData map[string]Request   // loaded from request_data.json file, key is Id value

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("demo program starting")

	bo.BaseURL = "http://localhost:50555/"
	bo.Debug = false

	httpClient = new(http.Client)

	loadLocationData("location_data.json") // load test data from json file into locationData map
	loadRequestData("request_data.json")   // load test data from json file into requestData map

	bo.DeleteBkt(httpClient, locationBkt) // using shortcut func in client pkg
	bo.CreateBkt(httpClient, locationBkt) // optional step, bkt would be auto created on 1st Put
	bo.DeleteBkt(httpClient, locationLogBkt)

	put() // load location and request bkts using data in maps

	copyDB() // copy open db and verify contents match

	get("100", "102", "999") // get specific recs from locationBkt and compare to vals in location data map

	getAll() // get all recs in locationBkt and compare to location data map

	putOne("102") // changes record in locationData map and updates record in db, using bo.PutOne func

	getOne("102") // verify prev step, putOne worked, use bo.GetOne func to get rec

	putOneLog() // use logput option which also adds timestamped record to location_log bkt

	getAllRange() // get recs in key range

	getAllKeys() // get all keys from locationBkt

	getAllLimit() // verify Limit and NextKey work

	qry1() // find str StartsWith, sort intDesc, strAsc

	qry2() // find str After, str contains, int equals, sort str desc, str asc

	qry3() // find str matches, int equals NOT, sort str asc

	qry4() // uses StrOption, "asis", "plain"

	qry5() // load and qry different set of data, use this qry to experiment

	qry6() // in list

	qry7() // fld exists, value is null

	putIndex() // load new index

	updateIndex() // change index just loaded

	getIndex() // use index go get all recs in index key range

	qryIndex() // use index to limit recs in qry range

	update("999") // update specific fld in record using Location type Update method

	delete("101", "999") // delete specific records by id

	getNextSeq() // get bkt next sequence numbers

	err_defaults() // test errs and UseDefault features

	export()
	putBkts()  // new feature added May 1, 2024
	qryJoin1() // new feature added Oct 17, 2024

	bktList := bo.GetBktList(httpClient) // get list of all db buckets
	log.Println("DB Buckets -> ", bktList)

	// ---- experimental requests, see experimental.go ------------------------
	getValues()
	searchKeys()

	log.Println("*** Demo Pgm Finished Successfully ***")
}

// -- put ------------------------------------------------------
// Loads all records in locationData map into location bkt.
// Data records must contain the key value. KeyField is the fld to be used for the key.
func put() {
	log.Println("-- put starting -----")

	// Put requires records be json.Marshalled
	putRecs := bo.MapToJson(locationData) // convert map of db recs to slice of json recs
	if putRecs == nil {
		log.Fatalln("put failed")
	}
	req := bobb.PutRequest{
		BktName:      locationBkt,
		KeyField:     "id", // json tag value of key field
		Recs:         putRecs,
		RequiredFlds: []string{"bad field name"}, // check fld verification
	}
	resp, err := bo.Run(httpClient, bobb.OpPut, req)
	log.Println("put request should fail, bad data sent: ", resp.Status, resp.Msg)
	if resp.Status == bobb.StatusOk {
		log.Fatalln("Put data verification failed, resp.Status should not be ok")
	}

	req.RequiredFlds = []string{"address", "city", "st"}
	resp, err = bo.Run(httpClient, bobb.OpPut, req)

	checkResp(resp, err, "put")

	// load records to request bkt
	putRecs = bo.MapToJson(requestData)
	req = bobb.PutRequest{
		BktName:  requestBkt,
		KeyField: "id", // json tag value of key field
		Recs:     putRecs,
	}
	bo.Run(httpClient, bobb.OpPut, req)

	log.Println("-- put done -----")
}

// -- copyDB ----------------------------
// Copy open db to another file.
func copyDB() {
	log.Println("-- copyDB starting -----")

	os.Remove("demo_copy.db")

	req := bobb.CopyDBRequest{
		FilePath: "demo_copy.db", // file created in server dir
	}
	resp, err := bo.Run(httpClient, bobb.OpCopyDB, req)
	checkResp(resp, err, "copyDB")

	// verify data in copy
	var dbCopy *bolt.DB
	dbCopy, err = bolt.Open("../bobb_server/demo_copy.db", 0600, nil)
	if err != nil {
		log.Fatalln("dbCopy opend failed", err)
	}
	dbRecs := make(map[string][]byte)
	dbCopy.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(locationBkt))
		if bkt == nil {
			log.Fatalln("bkt err not found in copy")
		}
		bkt.ForEach(func(k, v []byte) error {
			dbRecs[string(k)] = v
			return nil
		})
		return nil
	})
	var copyRec Location
	for origKey, origRec := range locationData {
		json.Unmarshal(dbRecs[origKey], &copyRec)
		compare(origRec, copyRec, "copyDB")
	}

	log.Println("-- copyDB done -----")
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

	results := bo.JsonToMap(resp.Recs, Location{}) // convert resp recs to map of Location recs

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

	results := bo.JsonToMap(resp.Recs, Location{}) // convert resp recs to map of Location recs

	// confirm results match desired results
	for id, original := range locationData {
		compare(original, results[id], "getAll")
	}
	log.Println("-- getAll done -----")
}

// -- putOne -------------------------------------
// Put a single record, using shortcut func, bo.PutOne
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
// Retrieve a single record by key, using shortcut bo.GetOne.
func getOne(id string) {
	log.Println("-- getOne starting -----")

	var result Location
	err := bo.GetOne(httpClient, locationBkt, id, &result) // use shortcut func in client pkg
	if err != nil {
		log.Fatalln(err)
	}
	// confirm results match desired results
	original := locationData[id]
	compare(original, result, "getOne")

	log.Println("-- getOne done -----")
}

// -- putOneLog -------------------------------------
// Puts a single record with logging.
func putOneLog() {
	log.Println("-- putOneLog starting -----")

	rec := Location{Id: "LogTest1", Address: "345 Doodle Way"}
	jsonRec, _ := json.Marshal(rec)
	req := bobb.PutOneRequest{
		BktName:  locationBkt,
		KeyField: "id",
		Rec:      jsonRec,
		LogPut:   true,
	}
	resp, err := bo.Run(httpClient, bobb.OpPutOne, req)
	checkResp(resp, err, "putOneLog")

	var logRec Location
	logKey := "LogTest1|" + time.Now().Format(time.DateTime) // requires timestamp to match to the second
	bo.GetOne(httpClient, locationLogBkt, logKey, &logRec)
	if logRec.Address != rec.Address {
		log.Println("putOneLog failed, may be caused by timestamp value not matching, due to secs incrementing")
	}
	req2 := bobb.DeleteRequest{BktName: locationBkt, Keys: []string{"LogTest1"}}
	resp, err = bo.Run(httpClient, bobb.OpDelete, req2)
	checkResp(resp, err, "putOneLog delete")

	log.Println("-- putOneLog done -----")
}

// -- getAllRange -----------------------------------------
// Gets all records in a range between start & end keys
func getAllRange() {
	log.Println("-- getAllRange starting -----")

	start := "100"
	end := "102"
	req := bobb.GetAllRequest{
		BktName:  locationBkt,
		StartKey: start,
		EndKey:   end,
	}
	resp, err := bo.Run(httpClient, bobb.OpGetAll, req)
	checkResp(resp, err, "getAllRange")

	results := bo.JsonToMap(resp.Recs, Location{}) // convert resp recs to map of Location recs

	for id := range results {
		if id < start || id > end {
			log.Fatalln("getAllRange failed, result key out of range", id)
		}
	}
	// confirm results match desired results
	for id, original := range locationData {
		if id < start || id > end {
			continue
		}
		compare(original, results[id], "getAll")
	}
	if resp.NextKey != "103" {
		log.Fatalln("getAllRange resp.NextKey incorrect", resp.NextKey)
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

// -- getAllLimit -----------------------------------------
func getAllLimit() {
	log.Println("-- getAllLimit starting -----")

	req := bobb.GetAllRequest{BktName: locationBkt, Limit: 5}
	resp, err := bo.Run(httpClient, bobb.OpGetAll, req)
	checkResp(resp, err, "getAllLimit")

	if len(resp.Recs) != 5 {
		log.Fatalln("getAllLimit wrong number of recs returned", len(resp.Recs))
	}
	if resp.NextKey != "999" {
		log.Fatalln("getAllLimit resp.NextKey incorrect", resp.NextKey)
	}
	log.Println("-- getAllLimit done -----")
}

// -- qry1 --------------------------------------
// Returns records meeting find conditions in sorted order
func qry1() {
	log.Println("-- qry1 starting -----")

	criteria := bo.Find(nil, "zip", bobb.FindStartsWith, "54")

	sortKeys := bo.Sort(nil, "locationType", bobb.SortDescInt)
	sortKeys = bo.Sort(sortKeys, "address", bobb.SortAscStr)

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
	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to map of Location recs

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

	criteria := bo.Find(nil, "st", bobb.FindAfter, "ok")
	criteria = bo.Find(criteria, "address", bobb.FindContains, "ave")
	criteria = bo.Find(criteria, "locationType", bobb.FindEquals, 3)

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
	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

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

	criteria := bo.Find(nil, "st", bobb.FindMatches, "TN")
	criteria = bo.Find(criteria, "locationType", bobb.FindEquals, 3, bobb.FindNot)

	sortKeys := bo.Sort(nil, "city", bobb.SortAscStr)

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
	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

	// confirm results match desired results
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "qry3")
	}
	log.Println("-- qry3 done -----")
}

// -- qry4 ----------------------------
// Uses StrOption
func qry4() {
	log.Println("-- qry4 starting -----")

	criteria := []bobb.FindCondition{
		{Fld: "st", Op: bobb.FindMatches, ValStr: "Ok", StrOption: "asis"},
	}
	req := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	if len(resp.Recs) != 0 {
		log.Fatalln("qry4 no resp.Recs should be returned")
	}
	criteria = []bobb.FindCondition{
		{Fld: "st", Op: bobb.FindMatches, ValStr: "OK", StrOption: "asis"},
		{Fld: "address", Op: bobb.FindMatches, ValStr: "101 green rd", StrOption: "plain"},
	}
	req.FindConditions = criteria
	resp, _ = bo.Run(httpClient, bobb.OpQry, req)
	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

	if !(len(results) == 1 && results[0].Id == "101") {
		log.Fatalln("qry4 results incorrect")
	}
	log.Println("-- qry4 done -----")
}

// -- qry5 ----------------------------
// Use criteria that pulls specific set of test records
func qry5() {
	log.Println("-- qry5 starting -----")

	data := []Location{
		{Id: "1", Address: "400 Hunter", LocationType: 0},
		{Id: "2", Address: "299 Milkyway", LocationType: 0},
		{Id: "3", Address: "76 Fireball", LocationType: 9},
	}
	jsonData := bo.SliceToJson(data)
	req := bobb.PutRequest{
		BktName:  "qry5",
		KeyField: "id",
		Recs:     jsonData,
	}
	bo.Run(httpClient, bobb.OpPut, req)

	criteria := bo.Find(nil, "locationType", bobb.FindEquals, 0)
	sortKeys := bo.Sort(nil, "address", bobb.SortAscStr)
	req2 := bobb.QryRequest{
		BktName:        "qry5",
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req2)
	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

	if !(results[0].Address == "299 Milkyway" && results[1].Address == "400 Hunter" && len(results) == 2) {
		log.Println("qry5 results \n", results)
		log.Fatalln("qry5 failed, results wrong")
	}
	log.Println("-- qry5 done -----")
}

// -- qry6 ----------------------------
// Uses InList condition.
func qry6() {
	log.Println("-- qry6 starting -----")

	req := bobb.QryRequest{
		BktName: locationBkt,
		FindConditions: []bobb.FindCondition{
			{Op: bobb.FindInIntList, Fld: "locationType", IntList: []int{1, 5}},
		},
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	checkResp(resp, err, "qry6")

	matchingIds := []string{"100", "101"} // resp recs should be in same order
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalln("qry6 wrong number of resp recs-", len(resp.Recs))
	}
	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

	// confirm results match desired results
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "qry6 inIntList")
	}

	criteria := bo.Find(nil, "st", bobb.FindInStrList, []string{"OK", "WA", "TX"})

	req2 := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
	}
	resp, err = bo.Run(httpClient, bobb.OpQry, req2)
	checkResp(resp, err, "qry6")

	matchingIds = []string{"101", "999"} // resp recs should be in same order
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalln("qry6 wrong number of resp recs-", len(resp.Recs))
	}
	results = bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

	// confirm results match desired results
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "qry6 inIntList")
	}
	log.Println("-- qry6 done -----")
}

// -- qry7 ----------------------------
// Uses FindExists, FindIsNull conditions.
func qry7() {
	log.Println("-- qry7 starting -----")

	// NOTE - FindExists needs additional data to be loaded
	/*
		req := bobb.QryRequest{
			BktName: locationBkt,
			FindConditions: []bobb.FindCondition{
				{Op: bobb.FindExists, Fld: "noval"}, // recs where fld "noval" exists in record
			},
		}
		resp, err := bo.Run(httpClient, bobb.OpQry, req)
		checkResp(resp, err, "qry7 findexists")

		matchingIds := []string{"100", "101"} // resp recs should be in same order
		if len(resp.Recs) != len(matchingIds) {
			log.Fatalln("qry7 FindExists wrong number of resp recs-", len(resp.Recs))
		}
		results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

		// confirm results match desired results
		for i, id := range matchingIds {
			original := locationData[id]
			result := results[i]
			compare(original, result, "qry7 FindExists")
		}
	*/
	criteria := bo.Find(nil, "nulltest", bobb.FindIsNull, nil) // recs where fld "noval" has null value

	req2 := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req2)
	checkResp(resp, err, "qry7 FindIsNull")

	matchingIds := []string{"100", "101"} // resp recs should be in same order
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalln("qry7 FindIsNull wrong number of resp recs-", len(resp.Recs))
	}
	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

	// confirm results match desired results
	for i, id := range matchingIds {
		original := locationData[id]
		result := results[i]
		compare(original, result, "qry7 IsNull")
	}
	log.Println("-- qry7 done -----")
}

// -- putIndex -----------------------------------------------
// Put records into an index bkt.
// Index-Key composed from data value(s). Index-Val is data rec key.
func putIndex() {
	log.Println("-- putIndex starting -----")

	bo.DeleteBkt(httpClient, locationZipIndex)
	// bo.CreateBkt(httpClient, locationZipIndex)

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

	req := bobb.GetAllRequest{
		BktName:  locationBkt,
		IndexBkt: locationZipIndex,
		StartKey: "30000", // zip >= 30000
		EndKey:   "60000", // zip <= 60000
	}
	resp, err := bo.Run(httpClient, bobb.OpGetAll, req)
	checkResp(resp, err, "getIndex")

	for _, r := range resp.Recs {
		log.Println(string(r))
	}

	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

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

	criteria := bo.Find(nil, "address", bobb.FindContains, "ave")
	sortKeys := bo.Sort(nil, "city", bobb.SortDescStr)

	req := bobb.QryRequest{
		BktName:        locationBkt,
		IndexBkt:       locationZipIndex,
		StartKey:       "54700",
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	checkResp(resp, err, "qryIndex")

	results := bo.JsonToSlice(resp.Recs, Location{}) // convert resp recs to slice of Location recs

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
	var currRec Location
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
	var newRec Location
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
	var rec Location
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

// test err and UseDefault features
func err_defaults() {
	log.Println("-- err_defaults starting -----")

	req := bobb.QryRequest{
		BktName: locationBkt,
		FindConditions: []bobb.FindCondition{
			{Fld: "MissingFld", Op: bobb.FindMatches, ValStr: "abc", UseDefault: bobb.DefaultNever},
		},
		ErrLimit: 2,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	if len(resp.Errs) != 3 {
		log.Fatalln("err_defauls failed, wrong # errs")
	}
	var val Location
	for i, err := range resp.Errs {
		log.Println(i, err.ErrCode, err.Msg, string(err.Key))
		json.Unmarshal(err.Val, &val)
		log.Println(val)
	}
	req = bobb.QryRequest{
		BktName: locationBkt,
		FindConditions: []bobb.FindCondition{
			{Fld: "MissingFld", Op: bobb.FindMatches, ValStr: "abc", UseDefault: bobb.DefaultNotFound},
		},
		ErrLimit: 2,
	}
	resp, _ = bo.Run(httpClient, bobb.OpQry, req)
	if len(resp.Errs) != 0 {
		log.Fatalln("err_defauls failed, wrong # errs")
	}
}

// -- export ------------------------------
// Write contents of bkt as formatted json.
func export() {
	log.Println("-- export starting -----")

	req := bobb.ExportRequest{
		BktName:  locationBkt,
		FilePath: "demo_export.json",
		StartKey: "103",
		EndKey:   "104",
	}
	resp, err := bo.Run(httpClient, bobb.OpExport, req)
	checkResp(resp, err, "export")

	log.Println("-- export done -----")
}

// -- putBkts ---------------------------------------------------
// new feature added 2024-5-1
func putBkts() {
	log.Println("-- putBkts starting -----")

	bo.DeleteBkt(httpClient, "order")
	//bo.CreateBkt(httpClient, "order")  bkt auto created
	bo.DeleteBkt(httpClient, "order_item")
	//bo.CreateBkt(httpClient, "order_item")  bkt auto created

	var order1 = Order{
		Id:         "00377_00005244",
		OrderDate:  "2024-05-23",
		CustomerId: "00377",
	}
	var items = []OrderItem{
		{"00377_00005244_001", "00377_00005244", 1, "A4576", 2},
		{"00377_00005244_002", "00377_00005244", 2, "A1721", 1},
		{"00377_00005244_003", "00377_00005244", 3, "C1600", 5},
		{"99999_xxxxxxxx_001", "00377_00005244", 3, "C1600", 5}, // bogus record added to test GetAll using match prefix logic
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
	var savedOrder Order // order saved to db
	bo.GetOne(httpClient, "order", order1.Id, &savedOrder)
	if order1 != savedOrder {
		log.Fatalln("putBkts db order does not match sent order")
	}

	// verify order items in db match items sent using match prefix logic in GetAll request
	req2 := bobb.QryRequest{
		BktName:  "order_item",
		StartKey: "00377_00005244",
		EndKey:   "00377_00005244",
	}
	resp2, _ := bo.Run(httpClient, bobb.OpQry, req2)

	results2 := bo.JsonToSlice(resp2.Recs, OrderItem{})
	if len(results2) != 3 {
		log.Fatalln("putBkts db item result count should be 3, but is", len(results2))
	}
	for i, resultRec := range results2 {
		log.Println(resultRec)
		if items[i] != resultRec {
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

// -- qryJoin1 ----------------------------
func qryJoin1() {
	log.Println("-- qryJoin1 starting -----")

	criteria := bo.Find(nil, "location_st", bobb.FindMatches, "TN")
	sortKeys := bo.Sort(nil, "location_address", bobb.SortDescStr)

	// pulling values (st,city,address) from location bkt into request records

	// location state is used in find criteria, so add to joinsBeforeFind
	joinsBeforeFind := []bobb.Join{
		{JoinBkt: locationBkt, JoinFld: "locationId", FromFld: "st", ToFld: "location_st"},
	}
	joinsAfterFind := []bobb.Join{
		{JoinBkt: locationBkt, JoinFld: "locationId", FromFld: "city", ToFld: "location_city"},
		{JoinBkt: locationBkt, JoinFld: "locationId", FromFld: "address", ToFld: "location_address"},
	}
	req := bobb.QryRequest{
		BktName:         requestBkt,
		FindConditions:  criteria,
		SortKeys:        sortKeys,
		JoinsBeforeFind: joinsBeforeFind,
		JoinsAfterFind:  joinsAfterFind,
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	checkResp(resp, err, "qryJoin1")

	log.Println(string(resp.Recs[0]))

	results := bo.JsonToSlice(resp.Recs, Request{}) // convert resp recs to slice of Request recs

	log.Println(results)

	expectedResults := []Request{
		{Id: "2024-10-16 002", LocationId: "103", Description: "replace front camera", LocationSt: "TN", LocationCity: "Tuggville", LocationAddress: "103 Big Way Ave"},
		{Id: "2024-10-16 001", LocationId: "102", Description: "check on landscaping", LocationSt: "TN", LocationCity: "Hangover", LocationAddress: "102 Nomad Lane"},
	}
	for i, expected := range expectedResults {
		//log.Println("result", results[i])
		//log.Println("expected", expected)
		if results[i] != expected {
			log.Fatalln("qryJoin1 failed results did not match expected")
		}
	}
	log.Println("-- qryJoin1 done -----")
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

func searchKeys() {
	log.Println("-- searchKeys starting -----")

	testBkt := "test1"
	bo.CreateBkt(httpClient, testBkt)
	defer bo.DeleteBkt(httpClient, testBkt)

	recs := []Location{
		{Id: "ca|angelrock  |4800billst", St: "CA", City: "Angel Rock", Address: "4800 Bill St"},
		{Id: "fl|watertown  |120phillips", St: "FL", City: "Watertown", Address: "120 Phillips"},
		{Id: "ca|angelflow  |1008linkwood", St: "CA", City: "Angelflow", Address: "1008 Linkwood"},
		{Id: "tx|deeks      |600rewdave", St: "TX", City: "Deeks", Address: "600 Rewd Ave"},
		{Id: "ca|beeton     |976langlyway", St: "CA", City: "Beeton", Address: "976 Langly Way"},
	}
	jsonRecs := bo.SliceToJson(recs)
	req := bobb.PutRequest{
		BktName:  testBkt,
		KeyField: "id",
		Recs:     jsonRecs,
	}
	bo.Run(httpClient, bobb.OpPut, req)

	// find recs where key starts with "ca" and contains "angel"
	req2 := bobb.SearchKeysRequest{
		BktName:     testBkt,
		SearchValue: "angel",
		StartKey:    "ca",
		EndKey:      "ca",
	}
	resp, _ := bo.Run(httpClient, bobb.OpSearchKeys, req2)
	if len(resp.Recs) != 2 {
		log.Fatalln("SearchKeys failed\n", resp)
	}
	respRecs := bo.JsonToSlice(resp.Recs, Location{})
	if respRecs[0].City != "Angelflow" && respRecs[1].City != "Angel Rock" {
		log.Fatalln("SearchKeys failed", respRecs)
	}
	log.Println("-- searchKeys done -----")
}

// --------------------------------------------------------------
// load test data from json file into map locationData
func loadLocationData(fileName string) {
	log.Println("loadLocationData")
	jsonData, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatalln("error opening json data file", err)
	}
	inputRecs := make([]Location, 0, 100)
	if err := json.Unmarshal(jsonData, &inputRecs); err != nil {
		log.Fatalln("json.Unmarshal error on jsonData", err)
	}
	locationData = make(map[string]Location)
	for _, rec := range inputRecs {
		locationData[rec.Id] = rec
		log.Println(rec)
	}
}

// load test data from json file into map requestData
func loadRequestData(fileName string) {
	log.Println("loadRequestData")
	jsonData, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatalln("error opening json data file", err)
	}
	inputRecs := make([]Request, 0, 100)
	if err := json.Unmarshal(jsonData, &inputRecs); err != nil {
		log.Fatalln("json.Unmarshal error on jsonData", err)
	}
	requestData = make(map[string]Request)
	for _, rec := range inputRecs {
		log.Printf("%+v\n", rec)
		requestData[rec.Id] = rec
	}
}

func compare(original, result Location, funcName string) {
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
