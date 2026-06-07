// demo.go exercises all bobb request types and verifies results are correct.
// The server must be running: cd bobb_server && go run .
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
	data "github.com/jayposs/bobb/datatypes" // contains test datatypes used by module programs such as demo
)

const (
	locationBkt      = "location"           // main test data bucket
	locationZipIndex = "location_zip_index" // manually managed zip index
	locationLogBkt   = "location_putlog"
	requestBkt       = "request" // join test data bucket
)

var httpClient *http.Client

var locationData map[string]data.Location // keyed by Location.Id, loaded from location_data.json
var requestData map[string]data.Request   // keyed by Request.Id, loaded from request_data.json

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("demo starting")

	bo.BaseURL = "http://localhost:50555/"
	bo.Debug = false

	httpClient = new(http.Client)

	loadLocationData("location_data.json")
	loadRequestData("request_data.json")

	// clean state and load test data
	bo.DeleteBkt(httpClient, locationBkt)
	bo.CreateBkt(httpClient, locationBkt)
	bo.DeleteBkt(httpClient, locationLogBkt)
	bo.DeleteBkt(httpClient, requestBkt)

	putData() // load data using PutRequest

	recCount := bo.GetRecCount(httpClient, locationBkt)
	if recCount != len(locationData) {
		log.Fatalf("put: expected %d recs in location bkt, got %d", len(locationData), recCount)
	}

	// ---- core CRUD -------------------------------------------------------
	copyDB()
	get("100", "102", "999", "badkey")
	getAll()
	putOne("102")
	getOne("102")
	putOneLog()

	// ---- GetAll variants -------------------------------------------------
	getAllRange()
	getAllKeys()
	getAllLimit()

	// ---- QryRequest ------------------------------------------------------
	qryFindOrAndSort() // FindOr conditions, multi-key int+str sort
	qryFindMultiple()  // multiple AND conditions, multi-key sort
	qryFindNot()       // Not flag on FindCondition
	qryStrOptions()    // StrAsIs, StrPlain
	qryCustomData()    // load a separate dataset, find + sort
	qryInList()        // FindInIntList, FindInStrList
	qryFindNull()      // FindIsNull
	qryContainsWord()  // FindContainsWord, FindEndsWith, FindStartsWith
	qryCountOnly()     // CountOnly, GetCnt

	// ---- Manual index management -----------------------------------------
	putIndex()
	updateIndex()
	getIndex()
	qryIndex()

	// ---- Automatic index via IndexSetting --------------------------------
	testIndexSetting()

	// ---- Other operations ------------------------------------------------
	update("999")
	deleteRecs("101", "999")
	getNextSeq()
	errDefaults()
	export()
	putBkts()
	qryJoin()
	putAddKeySuffix()

	// ---- Bucket list verification ----------------------------------------
	bktList := bo.GetBktList(httpClient)
	// verfify some of the expected buckets are present,
	for _, bkt := range []string{"location", "location_zip_index", "request"} {
		if !slices.Contains(bktList, bkt) {
			log.Fatalf("expected bucket %s not found in db", bkt)
		}
	}
	log.Println("db buckets:", bktList)

	// ---- Experimental requests -------------------------------------------
	getValues()
	searchKeys()

	log.Println("*** demo finished successfully ***")
}

// -----------------------------------------------------------------------

// putData loads location and request data into their buckets.
// Also verifies that required-field validation rejects bad data.
func putData() {
	log.Println("-- put starting -----")

	putRecs := bo.MapToJson(locationData)
	if putRecs == nil {
		log.Fatalln("put: MapToJson returned nil")
	}

	// send a bad required field name — Put should reject it
	resp, err := bo.Put(httpClient, locationBkt, putRecs, []string{"bad field name"})
	if resp.Status == bobb.StatusOk {
		log.Fatalln("put: expected validation failure for bad required field, got StatusOk")
	}
	log.Println("put correctly rejected bad required field:", resp.Msg)

	// load location bkt with correct required fields
	resp, err = bo.Put(httpClient, locationBkt, putRecs, []string{"address", "city", "st"})
	checkResp(resp, err, "put location bkt")

	// load request bkt
	putParm := bobb.PutParm{BktName: requestBkt, Recs: bo.MapToJson(requestData)}
	resp, err = bo.Run(httpClient, bobb.OpPut, bobb.PutRequest{PutParms: []bobb.PutParm{putParm}})
	checkResp(resp, err, "put request bkt")

	log.Println("-- put done -----")
}

// -----------------------------------------------------------------------

// copyDB copies the open db to a file and verifies the copy matches the source.
func copyDB() {
	log.Println("-- copyDB starting -----")

	os.Remove("demo_copy.db")

	resp, err := bo.Run(httpClient, bobb.OpCopyDB, bobb.CopyDBRequest{FilePath: "demo_copy.db"})
	checkResp(resp, err, "copyDB")

	// open the copy directly and verify every location record matches
	dbCopy, err := bolt.Open("../bobb_server/demo_copy.db", 0600, nil)
	if err != nil {
		log.Fatalln("copyDB: open failed", err)
	}
	dbRecs := make(map[string][]byte)
	dbCopy.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(locationBkt))
		if bkt == nil {
			log.Fatalln("copyDB: location bucket not found in copy")
		}
		bkt.ForEach(func(k, v []byte) error {
			dbRecs[string(k)] = v
			return nil
		})
		return nil
	})
	var copyRec data.Location
	for origKey, origRec := range locationData {
		json.Unmarshal(dbRecs[origKey], &copyRec)
		compare(origRec, copyRec, "copyDB")
	}

	log.Println("-- copyDB done -----")
}

// -----------------------------------------------------------------------

// get retrieves records by specific keys. "badkey" is intentionally included
// to verify ErrLimit and resp.Errs behavior.
func get(recIds ...string) {
	log.Println("-- get starting -----")

	req := bobb.GetRequest{
		BktName:  locationBkt,
		Keys:     recIds,
		ErrLimit: 1,
	}
	resp, _ := bo.Run(httpClient, bobb.OpGet, req)
	if resp.Status != bobb.StatusWarning {
		log.Fatalln("get: expected StatusWarning due to bad key, got", resp.Status)
	}
	log.Println("get error as expected:", resp.Errs[0].ErrCode, string(resp.Errs[0].Key))

	results := bo.JsonToMap(resp.Recs, data.Location{})
	for _, id := range recIds {
		if id == "badkey" {
			continue
		}
		compare(locationData[id], results[id], "get")
	}

	log.Println("-- get done -----")
}

// -----------------------------------------------------------------------

// getAll retrieves every record in the location bucket and compares to source data.
func getAll() {
	log.Println("-- getAll starting -----")

	resp, err := bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{BktName: locationBkt})
	checkResp(resp, err, "getAll")

	results := bo.JsonToMap(resp.Recs, data.Location{})
	log.Println("getAll - location records")
	for k, v := range results {
		log.Printf("key: %s, rec: %+v\n", k, v)
	}
	for id, original := range locationData {
		compare(original, results[id], "getAll")
	}

	log.Println("-- getAll done -----")
}

// -----------------------------------------------------------------------

// putOne updates a single record using bo.PutOne and reflects the change in locationData.
func putOne(id string) {
	log.Println("-- putOne starting -----")

	rec := locationData[id]
	rec.LastActionDt = time.Now().Format(time.DateOnly)
	locationData[id] = rec

	resp, err := bo.PutOne(httpClient, locationBkt, &rec, nil)
	checkResp(resp, err, "putOne")

	log.Println("-- putOne done -----")
}

// -----------------------------------------------------------------------

// getOne retrieves a single record using bo.GetOne and verifies it matches locationData.
func getOne(id string) {
	log.Println("-- getOne starting -----")

	var result data.Location
	if err := bo.GetOne(httpClient, locationBkt, id, &result); err != nil {
		log.Fatalln("getOne:", err)
	}
	compare(locationData[id], result, "getOne")

	log.Println("-- getOne done -----")
}

// -----------------------------------------------------------------------

// putOneLog puts a record with the LogPut and AddKeySuffix options, then verifies
// that both the data record (with suffix) and its log entry were created.
func putOneLog() {
	log.Println("-- putOneLog starting -----")

	tempBkt := "tempbkt"
	bo.DeleteBkt(httpClient, tempBkt)
	bo.DeleteBkt(httpClient, tempBkt+"_putlog")

	rec := data.Location{Id: "LogTest1", Address: "345 Doodle Way"}
	resp, err := bo.PutOne(httpClient, tempBkt, &rec, nil, bo.PutLogPut, bo.PutAddKeySuffix)
	checkResp(resp, err, "putOneLog")

	// verify data record exists with key suffix appended
	putId := rec.Id + "00000001"
	var result data.Location
	if err = bo.GetOne(httpClient, tempBkt, putId, &result); err != nil {
		log.Fatalf("putOneLog: GetOne failed for key %s: %v", putId, err)
	}
	if result.Id != putId {
		log.Fatalf("putOneLog: expected id %s, got %s", putId, result.Id)
	}

	// verify log record was created
	resp, err = bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{
		BktName:  tempBkt + "_putlog",
		StartKey: putId,
		EndKey:   putId,
	})
	checkResp(resp, err, "putOneLog - get log rec")
	if len(resp.Recs) != 1 {
		log.Fatalln("putOneLog: log record not found")
	}

	log.Println("-- putOneLog done -----")
}

// -----------------------------------------------------------------------

// getAllRange retrieves records within a key range and verifies NextKey.
func getAllRange() {
	log.Println("-- getAllRange starting -----")

	start, end := "100", "102"
	resp, err := bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{
		BktName:  locationBkt,
		StartKey: start,
		EndKey:   end,
	})
	checkResp(resp, err, "getAllRange")

	results := bo.JsonToMap(resp.Recs, data.Location{})
	for id := range results {
		if id < start || id > end {
			log.Fatalln("getAllRange: result key out of range", id)
		}
	}
	for id, original := range locationData {
		if id < start || id > end {
			continue
		}
		compare(original, results[id], "getAllRange")
	}
	if resp.NextKey != "103" {
		log.Fatalln("getAllRange: expected NextKey 103, got", resp.NextKey)
	}

	log.Println("-- getAllRange done -----")
}

// -----------------------------------------------------------------------

// getAllKeys returns all keys from the bucket and verifies they match source data.
func getAllKeys() {
	log.Println("-- getAllKeys starting -----")

	resp, err := bo.Run(httpClient, bobb.OpGetAllKeys, bobb.GetAllKeysRequest{BktName: locationBkt})
	checkResp(resp, err, "getAllKeys")

	results := make([]string, len(resp.Recs))
	for i, key := range resp.Recs {
		results[i] = string(key)
	}

	original := make([]string, 0, len(locationData))
	for k := range locationData {
		original = append(original, k)
	}
	slices.Sort(original)

	if slices.Compare(original, results) != 0 {
		log.Fatalln("getAllKeys: returned keys do not match source data")
	}

	log.Println("-- getAllKeys done -----")
}

// -----------------------------------------------------------------------

// getAllLimit verifies that Limit caps the number of records returned and NextKey is set.
func getAllLimit() {
	log.Println("-- getAllLimit starting -----")

	resp, err := bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{BktName: locationBkt, Limit: 5})
	checkResp(resp, err, "getAllLimit")

	if len(resp.Recs) != 5 {
		log.Fatalf("getAllLimit: expected 5 recs, got %d", len(resp.Recs))
	}
	if resp.NextKey != "999" {
		log.Fatalln("getAllLimit: expected NextKey 999, got", resp.NextKey)
	}

	log.Println("-- getAllLimit done -----")
}

// -----------------------------------------------------------------------
// QryRequest tests
// -----------------------------------------------------------------------

// qryFindOrAndSort tests FindOrConditions and multi-key int+str sort.
// zip starts with "54" OR zip matches "11111", sorted by locationType desc then address asc.
func qryFindOrAndSort() {
	log.Println("-- qryFindOrAndSort starting -----")

	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: locationBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "zip", bobb.FindStartsWith, "54"), // startwith 54
			bo.Find(nil, "zip", bobb.FindMatches, "11111"), // or matches 11111
		},
		SortKeys: bo.Sort(bo.Sort(nil, "locationType", bobb.SortDescInt), "address", bobb.SortAscStr),
	})
	checkResp(resp, err, "qryFindOrAndSort")

	matchingIds := []string{"104", "102", "103", "100"} // expected order after sort
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalf("qryFindOrAndSort: expected %d recs, got %d", len(matchingIds), len(resp.Recs))
	}
	results := bo.JsonToSlice(resp.Recs, data.Location{})
	for i, id := range matchingIds {
		compare(locationData[id], results[i], "qryFindOrAndSort")
	}

	log.Println("-- qryFindOrAndSort done -----")
}

// -----------------------------------------------------------------------

// qryFindMultiple tests multiple AND FindConditions with a multi-key sort.
// st after "ok" AND address contains "ave" AND locationType equals 3, sorted st desc then city asc.
func qryFindMultiple() {
	log.Println("-- qryFindMultiple starting -----")

	findGroup1 := bo.Find(nil, "st", bobb.FindAfter, "ok")
	findGroup1 = bo.Find(findGroup1, "address", bobb.FindContains, "ave") // and condition added to same FindGroup
	findGroup1 = bo.Find(findGroup1, "locationType", bobb.FindEquals, 3)  // and condition added to same FindGroup

	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{findGroup1},
		SortKeys: []bobb.SortKey{
			{Fld: "st", Dir: bobb.SortDescStr},
			{Fld: "city", Dir: bobb.SortAscStr},
		},
	})
	checkResp(resp, err, "qryFindMultiple")

	matchingIds := []string{"999", "104"} // expected order after sort
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalf("qryFindMultiple: expected %d recs, got %d", len(matchingIds), len(resp.Recs))
	}
	results := bo.JsonToSlice(resp.Recs, data.Location{})
	for i, id := range matchingIds {
		compare(locationData[id], results[i], "qryFindMultiple")
	}

	log.Println("-- qryFindMultiple done -----")
}

// -----------------------------------------------------------------------

// qryFindNot tests the Not flag on a FindCondition.
// st matches "TN" AND locationType NOT equals 3, sorted by city asc.
func qryFindNot() {
	log.Println("-- qryFindNot starting -----")

	findGroup1 := bobb.FindGroup{
		{Fld: "st", Op: bobb.FindMatches, ValStr: "TN"},
		{Fld: "locationType", Op: bobb.FindEquals, ValInt: 3, Not: true, UseDefault: bobb.DefaultAlways},
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{findGroup1},
		SortKeys: bo.Sort(nil, "city", bobb.SortAscStr),
	})
	checkResp(resp, err, "qryFindNot")

	matchingIds := []string{"102", "103"} // expected order after sort
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalf("qryFindNot: expected %d recs, got %d", len(matchingIds), len(resp.Recs))
	}
	results := bo.JsonToSlice(resp.Recs, data.Location{})
	for i, id := range matchingIds {
		compare(locationData[id], results[i], "qryFindNot")
	}

	log.Println("-- qryFindNot done -----")
}

// -----------------------------------------------------------------------

// qryStrOptions tests StrOption codes: StrAsIs requires exact case, StrPlain strips punctuation.
func qryStrOptions() {
	log.Println("-- qryStrOptions starting -----")

	findGroup1 := bobb.FindGroup{
		bobb.FindCondition{Fld: "st", Op: bobb.FindMatches, ValStr: "Ok", StrOption: bobb.StrAsIs},
	}
	// StrAsIs: "Ok" does not match "OK" as stored in db → should have 0 results
	resp, _ := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{findGroup1},
	})
	if resp.Status != bobb.StatusOk || resp.GetCnt != 0 {
		log.Fatalln("qryStrOptions: StrAsIs wrong case should return 0 recs", resp.Status)
	}

	// StrAsIs exact match + StrPlain punctuation-insensitive match → record 101
	findGroup1[0].ValStr = "OK"
	findGroup1 = append(findGroup1, bobb.FindCondition{Fld: "address", Op: bobb.FindMatches, ValStr: "101 green rd", StrOption: bobb.StrPlain})

	resp, _ = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{findGroup1},
	})
	results := bo.JsonToSlice(resp.Recs, data.Location{})
	if len(results) != 1 || results[0].Id != "101" {
		log.Fatalln("qryStrOptions: expected single result with id 101")
	}

	log.Println("-- qryStrOptions done -----")
}

// -----------------------------------------------------------------------

// qryCustomData loads a small purpose-built dataset into a separate bucket
// and verifies find + sort returns records in the correct order.
func qryCustomData() {
	log.Println("-- qryCustomData starting -----")

	bo.DeleteBkt(httpClient, "qryCustomData")

	locData := []data.Location{
		{Id: "1", Address: "400 Hunter", LocationType: 0},
		{Id: "2", Address: "299 Milkyway", LocationType: 0},
		{Id: "3", Address: "76 Fireball", LocationType: 9},
	}
	resp, err := bo.Put(httpClient, "qryCustomData", bo.SliceToJson(locData), nil)
	checkResp(resp, err, "qryCustomData put")

	resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: "qryCustomData",
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "locationType", bobb.FindEquals, 0),
		},
		SortKeys: bo.Sort(nil, "address", bobb.SortAscStr),
	})
	checkResp(resp, err, "qryCustomData qry")

	results := bo.JsonToSlice(resp.Recs, data.Location{})
	if len(results) != 2 || results[0].Address != "299 Milkyway" || results[1].Address != "400 Hunter" {
		log.Fatalln("qryCustomData: results incorrect", results)
	}
	log.Println("-- qryCustomData done -----")
}

// -----------------------------------------------------------------------

// qryInList tests FindInIntList and FindInStrList.
func qryInList() {
	log.Println("-- qryInList starting -----")

	// FindInIntList: locationType in [1, 5] → records 100, 101
	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: locationBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "locationType", bobb.FindInIntList, []int{1, 5}),
		},
	})
	checkResp(resp, err, "qryInList FindInIntList")

	matchingIds := []string{"100", "101"}
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalf("qryInList FindInIntList: expected %d recs, got %d", len(matchingIds), len(resp.Recs))
	}
	results := bo.JsonToSlice(resp.Recs, data.Location{})
	for i, id := range matchingIds {
		compare(locationData[id], results[i], "qryInList FindInIntList")
	}

	// FindInStrList: st in [OK, WA, TX] → records 101, 999
	resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: locationBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "st", bobb.FindInStrList, []string{"Ok", "WA", "tx"}),
		},
	})
	checkResp(resp, err, "qryInList FindInStrList")

	matchingIds = []string{"101", "999"}
	if resp.GetCnt != len(matchingIds) {
		log.Fatalf("qryInList FindInStrList: expected %d recs, got %d", len(matchingIds), len(resp.Recs))
	}
	results = bo.JsonToSlice(resp.Recs, data.Location{})
	for i, id := range matchingIds {
		compare(locationData[id], results[i], "qryInList FindInStrList")
	}

	log.Println("-- qryInList done -----")
}

// -----------------------------------------------------------------------

// qryFindNull tests FindIsNull — records where the nulltest field has a JSON null value.
func qryFindNull() {
	log.Println("-- qryFindNull starting -----")

	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: locationBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "nulltest", bobb.FindIsNull, nil),
		},
	})
	checkResp(resp, err, "qryFindNull")

	matchingIds := []string{"100", "101"}
	if len(resp.Recs) != len(matchingIds) {
		log.Fatalf("qryFindNull: expected %d recs, got %d", len(matchingIds), len(resp.Recs))
	}
	results := bo.JsonToSlice(resp.Recs, data.Location{})
	for i, id := range matchingIds {
		compare(locationData[id], results[i], "qryFindNull")
	}
	log.Println("-- qryFindNull done -----")
}

// -----------------------------------------------------------------------

// qryContainsWord tests FindContainsWord (whole word), FindEndsWith, and FindStartsWith.
// All three target record 102 with address "102 Nomad Lane".
func qryContainsWord() {
	log.Println("-- qryContainsWord starting -----")

	// FindContainsWord: "Nomad" is a whole word in the address
	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: locationBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "address", bobb.FindContainsWord, "Nomad"),
		},
	})
	checkResp(resp, err, "qryContainsWord FindContainsWord")
	results := bo.JsonToMap(resp.Recs, data.Location{})
	if len(results) != 1 {
		log.Fatalf("qryContainsWord FindContainsWord: expected 1, got %d", len(results))
	}
	if _, found := results["102"]; !found {
		log.Fatalln("qryContainsWord FindContainsWord: expected record 102")
	}

	fg := []bobb.FindCondition{
		{Fld: "address", Op: bobb.FindEndsWith, ValStr: "lane", UseDefault: bobb.DefaultNever}, // if address is missing or null, generate err
	}
	// FindEndsWith: address ends with "Lane"
	resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{fg},
		ErrLimit: 2,
	})
	checkResp(resp, err, "qryContainsWord FindEndsWith")
	results = bo.JsonToMap(resp.Recs, data.Location{})
	if len(results) != 1 {
		log.Fatalf("qryContainsWord FindEndsWith: expected 1, got %d", len(results))
	}
	if _, found := results["102"]; !found {
		log.Fatalln("qryContainsWord FindEndsWith: expected record 102")
	}

	// FindStartsWith: address starts with "102"
	resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: locationBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "address", bobb.FindStartsWith, "102"),
		},
	})
	checkResp(resp, err, "qryContainsWord FindStartsWith")

	results = bo.JsonToMap(resp.Recs, data.Location{})
	if len(results) != 1 {
		log.Fatalf("qryContainsWord FindStartsWith: expected 1, got %d", len(results))
	}
	if _, found := results["102"]; !found {
		log.Fatalln("qryContainsWord FindStartsWith: expected record 102")
	}

	log.Println("-- qryContainsWord done -----")
}

// -----------------------------------------------------------------------

// qryCountOnly verifies that CountOnly returns a count and clears resp.Recs.
func qryCountOnly() {
	log.Println("-- qryCountOnly starting -----")

	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: locationBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "zip", bobb.FindStartsWith, "54"),
		},
		CountOnly: true,
	})
	checkResp(resp, err, "qryCountOnly")

	if resp.GetCnt != 3 {
		log.Fatalf("qryCountOnly: expected GetCnt 3, got %d", resp.GetCnt)
	}
	if len(resp.Recs) != 0 {
		log.Fatalf("qryCountOnly: expected 0 recs in response, got %d", len(resp.Recs))
	}
	log.Println("-- qryCountOnly done -----")
}

// -----------------------------------------------------------------------
// Manual index management
// -----------------------------------------------------------------------

// putIndex builds the location zip index manually using PutIndexRequest.
// Index key format: "zip|locationId"
func putIndex() {
	log.Println("-- putIndex starting -----")

	bo.DeleteBkt(httpClient, locationZipIndex)

	indexes := make([]bobb.IndexKeyVal, 0, len(locationData))
	for _, rec := range locationData {
		indexes = append(indexes, bobb.IndexKeyVal{
			Key: fmt.Sprintf("%s|%s", rec.Zip, rec.Id),
			Val: rec.Id,
		})
	}
	resp, err := bo.Run(httpClient, bobb.OpPutIndex, bobb.PutIndexRequest{
		BktName: locationZipIndex,
		Indexes: indexes,
	})
	checkResp(resp, err, "putIndex")

	log.Println("-- putIndex done -----")
}

// -----------------------------------------------------------------------

// updateIndex changes an existing index entry by supplying OldKey so the
// server deletes it before writing the new one.
func updateIndex() {
	log.Println("-- updateIndex starting -----")

	oldKey := "54633|104"
	newKey := "54633|104-b"

	resp, err := bo.Run(httpClient, bobb.OpPutIndex, bobb.PutIndexRequest{
		BktName: locationZipIndex,
		Indexes: []bobb.IndexKeyVal{{Key: newKey, Val: "104", OldKey: oldKey}},
	})
	checkResp(resp, err, "updateIndex")

	// verify old key was removed and new key was added
	resp, _ = bo.Run(httpClient, bobb.OpGetAllKeys, bobb.GetAllKeysRequest{BktName: locationZipIndex})
	var newKeyFound bool
	for _, r := range resp.Recs {
		if string(r) == oldKey {
			log.Fatalln("updateIndex: old key was not removed")
		}
		if string(r) == newKey {
			newKeyFound = true
		}
	}
	if !newKeyFound {
		log.Fatalln("updateIndex: new key not found")
	}

	log.Println("-- updateIndex done -----")
}

// -----------------------------------------------------------------------

// getIndex uses an index to read data records in zip-code order via GetAllRequest.
func getIndex() {
	log.Println("-- getIndex starting -----")

	resp, err := bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{
		BktName:  locationBkt,
		IndexBkt: locationZipIndex,
		StartKey: "30000",
		EndKey:   "60000",
	})
	checkResp(resp, err, "getIndex")

	// returned in zip-code order: 35422(101), 54321(102), 54633(104-b→104), 54711(103)
	matchingIds := []string{"101", "102", "104", "103"}
	results := bo.JsonToSlice(resp.Recs, data.Location{})
	for i, id := range matchingIds {
		compare(locationData[id], results[i], "getIndex")
	}

	log.Println("-- getIndex done -----")
}

// -----------------------------------------------------------------------

// qryIndex uses an index to restrict which records are scanned before applying
// FindConditions. Only records with zip >= 54700 are visited.
func qryIndex() {
	log.Println("-- qryIndex starting -----")

	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  locationBkt,
		IndexBkt: locationZipIndex,
		StartKey: "54700",
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "address", bobb.FindContains, "ave"),
		},
		SortKeys: bo.Sort(nil, "city", bobb.SortDescStr),
	})
	checkResp(resp, err, "qryIndex")

	matchingIds := []string{"999", "103"}
	results := bo.JsonToSlice(resp.Recs, data.Location{})
	for i, id := range matchingIds {
		compare(locationData[id], results[i], "qryIndex")
	}
	log.Println("-- qryIndex done -----")
}

// -----------------------------------------------------------------------
// Automatic index via IndexSetting
// -----------------------------------------------------------------------

// testIndexSetting verifies automatic index maintenance:
// index entries are created on Put, updated when a record changes, and
// VerifyIndexRequest confirms integrity throughout.
func testIndexSetting() {
	log.Println("-- testIndexSetting starting -----")

	const testBkt = "index_test"
	const testCityIndex = "index_test_city_index"

	bo.DeleteBkt(httpClient, testBkt)
	bo.DeleteBkt(httpClient, testCityIndex)
	bo.DeleteBkt(httpClient, testCityIndex+"_inverted")

	// register a city+st composite index — key: city (20 chars) | st (2 chars), both lowercase
	setting := bobb.IndexSetting{
		DataBkt:  testBkt,
		IndexBkt: testCityIndex,
		KeyFlds: []bobb.FldFormat{
			{FldName: "city", FldType: bobb.FldTypeStr, Length: 20, StrOption: bobb.StrLowerCase, UseDefault: bobb.DefaultAlways},
			{FldName: "st", FldType: bobb.FldTypeStr, Length: 2, StrOption: bobb.StrLowerCase, UseDefault: bobb.DefaultAlways},
		},
	}
	resp, err := bo.Run(httpClient, bobb.OpIndexSetting, bobb.IndexSettingRequest{IndexSettings: []bobb.IndexSetting{setting}})
	checkResp(resp, err, "testIndexSetting - IndexSettingRequest")

	// Put records — PutRequest auto-creates index entries
	testRecs := []data.Location{
		{Id: "t1", City: "Memphis", St: "TN", Address: "1 Elm St", Zip: "38101"},
		{Id: "t2", City: "Austin", St: "TX", Address: "2 Oak Ave", Zip: "78701"},
		{Id: "t3", City: "Portland", St: "OR", Address: "3 Pine Rd", Zip: "97201"},
		{Id: "t4", City: "Memphis", St: "TN", Address: "4 Maple Dr", Zip: "38103"},
	}
	resp, err = bo.Put(httpClient, testBkt, bo.SliceToJson(testRecs), nil)
	checkResp(resp, err, "testIndexSetting - Put")

	indexCount := bo.GetRecCount(httpClient, testCityIndex)
	if indexCount != len(testRecs) {
		log.Fatalf("testIndexSetting: index count %d, expected %d", indexCount, len(testRecs))
	}

	// verify initial index integrity
	verifyReq := bobb.VerifyIndexRequest{DataBkt: testBkt, IndexBkt: testCityIndex, AllDataIndexed: true, ErrLimit: 10}
	resp, err = bo.Run(httpClient, bobb.OpVerifyIndex, verifyReq)
	if len(resp.Errs) > 0 {
		for _, e := range resp.Errs {
			log.Printf("testIndexSetting verify error: key=%s val=%s msg=%s", e.Key, e.Val, e.Msg)
		}
	}
	checkResp(resp, err, "testIndexSetting - VerifyIndex initial")

	// update t1: Memphis,TN → Seattle,WA — index entry should move to new city key
	updated := testRecs[0]
	updated.City = "Seattle"
	updated.St = "WA"
	jsonRec, _ := json.Marshal(updated)
	resp, err = bo.Put(httpClient, testBkt, [][]byte{jsonRec}, nil)
	checkResp(resp, err, "testIndexSetting - Put updated rec")

	// index must still be valid after the city change
	resp, err = bo.Run(httpClient, bobb.OpVerifyIndex, verifyReq)
	checkResp(resp, err, "testIndexSetting - VerifyIndex after update")
	if len(resp.Errs) > 0 {
		log.Fatalln("testIndexSetting: post-update verify errors:", resp.Errs)
	}

	// GetAll via index returns records in ascending city order
	resp, err = bo.Run(httpClient, bobb.OpGetAll, bobb.GetAllRequest{BktName: testBkt, IndexBkt: testCityIndex})
	checkResp(resp, err, "testIndexSetting - GetAll via index")

	results := bo.JsonToSlice(resp.Recs, data.Location{})
	if len(results) != len(testRecs) {
		log.Fatalf("testIndexSetting: GetAll returned %d recs, expected %d", len(results), len(testRecs))
	}
	// after update: Austin(t2), Memphis(t4), Portland(t3), Seattle(t1)
	for i, expectedId := range []string{"t2", "t4", "t3", "t1"} {
		if results[i].Id != expectedId {
			log.Fatalf("testIndexSetting: order wrong at pos %d: got id=%s city=%s, expected id=%s",
				i, results[i].Id, results[i].City, expectedId)
		}
	}

	bo.DeleteBkt(httpClient, testBkt)
	bo.DeleteBkt(httpClient, testCityIndex)
	bo.DeleteBkt(httpClient, testCityIndex+"_inverted")

	log.Println("-- testIndexSetting done -----")
}

// -----------------------------------------------------------------------
// Other operations
// -----------------------------------------------------------------------

// update changes a single field on a record using the Location.Update method.
func update(id string) {
	log.Println("-- update starting -----")

	dateStamp := time.Now().Format(time.DateOnly)

	// reflect change in source map so subsequent compare calls pass
	original := locationData[id]
	original.LastActionDt = dateStamp
	locationData[id] = original

	var currRec data.Location
	if err := bo.GetOne(httpClient, locationBkt, id, &currRec); err != nil {
		log.Fatalln("update: GetOne failed:", err)
	}
	if err := currRec.Update(map[string]any{"lastActionDt": dateStamp}); err != nil {
		log.Fatalln("update: Location.Update failed:", err)
	}
	resp, err := bo.PutOne(httpClient, locationBkt, &currRec, nil)
	checkResp(resp, err, "update - PutOne")

	var newRec data.Location
	if err := bo.GetOne(httpClient, locationBkt, id, &newRec); err != nil {
		log.Fatalln("update: post-update GetOne failed:", err)
	}
	compare(locationData[id], newRec, "update")

	log.Println("-- update done -----")
}

// -----------------------------------------------------------------------

// deleteRecs deletes records by key and verifies they are gone.
func deleteRecs(ids ...string) {
	log.Println("-- deleteRecs starting -----")

	resp, err := bo.Run(httpClient, bobb.OpDelete, bobb.DeleteRequest{BktName: locationBkt, Keys: ids})
	checkResp(resp, err, "deleteRecs")

	var rec data.Location
	if err = bo.GetOne(httpClient, locationBkt, ids[0], &rec); err.Error() != bobb.ErrNotFound {
		log.Fatalln("deleteRecs: record should not exist after delete:", ids[0])
	}

	log.Println("-- deleteRecs done -----")
}

// -----------------------------------------------------------------------

// getNextSeq verifies BktRequest nextseq returns consecutive sequence numbers.
// getNextSeq uses a dedicated temporary bucket so the sequence counter is
// isolated from locationBkt, which is used by putAddKeySuffix.
func getNextSeq() {
	log.Println("-- getNextSeq starting -----")

	const seqBkt = "nextseq_test"
	bo.DeleteBkt(httpClient, seqBkt)
	defer bo.DeleteBkt(httpClient, seqBkt)

	resp, err := bo.Run(httpClient, bobb.OpBkt, bobb.BktRequest{
		BktName:      seqBkt,
		Operation:    "nextseq",
		NextSeqCount: 5,
	})
	checkResp(resp, err, "getNextSeq")

	if slices.Compare(resp.NextSeq, []int{1, 2, 3, 4, 5}) != 0 {
		log.Fatalln("getNextSeq: unexpected sequence numbers:", resp.NextSeq)
	}

	log.Println("-- getNextSeq done -----")
}

// -----------------------------------------------------------------------

// errDefaults tests ErrLimit behavior and UseDefault codes.
// DefaultNever triggers errors; DefaultNotFound silently returns zero values.
func errDefaults() {
	log.Println("-- errDefaults starting -----")

	// use DefaultNever on missing/null field: each record produces an error until ErrLimit+1 is hit
	fg1 := []bobb.FindCondition{
		{Fld: "MissingFld", Op: bobb.FindMatches, ValStr: "abc", UseDefault: bobb.DefaultNever},
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{fg1},
		ErrLimit: 2,
	})
	if len(resp.Errs) != 3 {
		log.Fatalf("errDefaults: expected 3 errors, got %d", len(resp.Errs))
	}
	for i, bobbErr := range resp.Errs {
		var val data.Location
		json.Unmarshal(bobbErr.Val, &val)
		log.Printf("errDefaults error %d: %s %s %s", i, bobbErr.ErrCode, bobbErr.Msg, string(bobbErr.Key))
	}

	// use DefaultNotFound on missing field: no error generated due to Fld not being in a record, default/zero value of "" is used
	fgb := []bobb.FindCondition{
		{Fld: "MissingFld", Op: bobb.FindMatches, ValStr: "abc", UseDefault: bobb.DefaultNotFound},
	}
	resp, _ = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{fgb},
		ErrLimit: 2,
	})
	if len(resp.Errs) != 0 {
		log.Fatalf("errDefaults: expected 0 errors with DefaultNotFound, got %d", len(resp.Errs))
	}
	log.Println("-- errDefaults done -----")
}

// -----------------------------------------------------------------------

// export writes a key range of the location bucket to a JSON file.
func export() {
	log.Println("-- export starting -----")

	resp, err := bo.Run(httpClient, bobb.OpExport, bobb.ExportRequest{
		BktName:  locationBkt,
		FilePath: "demo_export.json",
		StartKey: "103",
		EndKey:   "104",
	})
	checkResp(resp, err, "export")

	log.Println("-- export done -----")
}

// -----------------------------------------------------------------------

// putBkts puts records into two buckets in a single PutRequest and verifies
// the results using prefix-match key range via QryRequest.
func putBkts() {
	log.Println("-- putBkts starting -----")

	bo.DeleteBkt(httpClient, "order")
	bo.DeleteBkt(httpClient, "order_item")

	order1 := data.Order{Id: "00377_00005244", OrderDate: "2024-05-23", CustomerId: "00377"}
	items := []data.OrderItem{
		{"00377_00005244_001", "00377_00005244", 1, "A4576", 2},
		{"00377_00005244_002", "00377_00005244", 2, "A1721", 1},
		{"00377_00005244_003", "00377_00005244", 3, "C1600", 5},
		{"99999_xxxxxxxx_001", "00377_00005244", 3, "C1600", 5}, // extra rec to test prefix-match exclusion
	}

	jsonOrder, _ := json.Marshal(&order1)

	putParm1, err := bo.PutParm("order", [][]byte{jsonOrder}, nil)
	if err != nil {
		log.Fatal(err)
	}
	putParm2, err := bo.PutParm("order_item", bo.SliceToJson(items), nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, runErr := bo.Run(httpClient, bobb.OpPut, bobb.PutRequest{PutParms: []bobb.PutParm{putParm1, putParm2}})
	checkResp(resp, runErr, "putBkts")

	// verify order
	var savedOrder data.Order
	bo.GetOne(httpClient, "order", order1.Id, &savedOrder)
	if order1 != savedOrder {
		log.Fatalln("putBkts: saved order does not match sent order")
	}

	// verify order items using prefix-match (StartKey == EndKey) — only the 3 matching items
	resp2, _ := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName:  "order_item",
		StartKey: "00377_00005244",
		EndKey:   "00377_00005244",
	})
	results := bo.JsonToSlice(resp2.Recs, data.OrderItem{})
	if len(results) != 3 {
		log.Fatalf("putBkts: expected 3 order items, got %d", len(results))
	}
	for i, resultRec := range results {
		if items[i] != resultRec {
			log.Fatalln("putBkts: order item mismatch at index", i)
		}
	}

	log.Println("-- putBkts done -----")
}

// -----------------------------------------------------------------------

// qryJoin tests JoinsBeforeFind and JoinsAfterFind.
// Joins pull location fields into request records; the before-find join
// enables filtering on a joined value (location_st).
func qryJoin() {
	log.Println("-- qryJoin starting -----")

	// location_st is used in the find condition, so it must be a JoinBeforeFind
	joinsBeforeFind := []bobb.Join{
		{JoinBkt: locationBkt, JoinFld: "locationId", FromFld: "st", ToFld: "location_st"},
	}
	// location_city and location_address are only needed in the result
	joinsAfterFind := []bobb.Join{
		{JoinBkt: locationBkt, JoinFld: "locationId", FromFld: "city", ToFld: "location_city"},
		{JoinBkt: locationBkt, JoinFld: "locationId", FromFld: "address", ToFld: "location_address"},
	}

	resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: requestBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "location_st", bobb.FindMatches, "TN"),
		},
		SortKeys:        bo.Sort(nil, "location_address", bobb.SortDescStr),
		JoinsBeforeFind: joinsBeforeFind,
		JoinsAfterFind:  joinsAfterFind,
	})
	checkResp(resp, err, "qryJoin")

	results := bo.JsonToSlice(resp.Recs, data.Request{})
	expectedResults := []data.Request{
		{Id: "2024-10-16 002", LocationId: "103", Description: "replace front camera", LocationSt: "TN", LocationCity: "Tuggville", LocationAddress: "103 Big Way Ave"},
		{Id: "2024-10-16 001", LocationId: "102", Description: "check on landscaping", LocationSt: "TN", LocationCity: "Hangover", LocationAddress: "102 Nomad Lane"},
	}
	for i, expected := range expectedResults {
		if results[i] != expected {
			log.Printf("qryJoin: result[%d] = %+v", i, results[i])
			log.Printf("qryJoin: expected[%d] = %+v", i, expected)
			log.Fatalln("qryJoin: result did not match expected")
		}
	}

	log.Println("-- qryJoin done -----")
}

// -----------------------------------------------------------------------

// putAddKeySuffix puts records with the AddKeySuffix option and verifies
// the server appended the expected sequence-number suffixes.
func putAddKeySuffix() {
	log.Println("-- putAddKeySuffix starting -----")

	newRecs := []data.Location{
		{Id: "beaver", St: "UT", City: "Beaver", Address: "1 Main St"},
		{Id: "fox", St: "UT", City: "RedTail", Address: "28 Trout Rd"},
	}

	resp, err := bo.Run(httpClient, bobb.OpPut, bobb.PutRequest{
		PutParms: []bobb.PutParm{{
			BktName:      locationBkt,
			Recs:         bo.SliceToJson(newRecs),
			AddKeySuffix: true,
		}},
	})
	checkResp(resp, err, "putAddKeySuffix")

	log.Println("putAddKeySuffix keys used:")
	for _, key := range resp.PutKeys[0] {
		log.Println(string(key))
	}

	// verify records exist in location bkt with suffix-appended keys
	// keySuffixWidth=8 in bobb_settings.json → suffix is "00000001", "00000002"
	req2 := bobb.GetRequest{
		BktName: locationBkt,
		Keys:    []string{"beaver00000001", "fox00000002"},
	}
	resp2, _ := bo.Run(httpClient, bobb.OpGet, req2)
	results := bo.JsonToMap(resp2.Recs, data.Location{})

	if results["beaver00000001"].City != "Beaver" || results["fox00000002"].City != "RedTail" {
		log.Fatalln("putAddKeySuffix: records not found with expected keys")
	}

	log.Println("-- putAddKeySuffix done -----")
}

// -----------------------------------------------------------------------
// Experimental requests
// -----------------------------------------------------------------------

// getValues retrieves specific field values from records by key.
func getValues() {
	log.Println("-- getValues starting -----")

	resp, err := bo.Run(httpClient, bobb.OpGetValues, bobb.GetValuesRequest{
		BktName: locationBkt,
		Keys:    []string{"102", "103", "104"},
		Fields:  []string{"address", "city", "locationType|int"},
	})
	checkResp(resp, err, "getValues")

	results := make([]bobb.RecValues, len(resp.Recs))
	for i, jsonRec := range resp.Recs {
		json.Unmarshal(jsonRec, &results[i])
	}

	expected := []bobb.RecValues{
		{Key: "102", FldVals: map[string]string{"address": "102 Nomad Lane", "city": "Hangover", "locationType": "2"}},
		{Key: "103", FldVals: map[string]string{"address": "103 Big Way Ave", "city": "Tuggville", "locationType": "2"}},
		{Key: "104", FldVals: map[string]string{"address": "900 Hammer Hill Ave", "city": "Anville", "locationType": "3"}},
	}
	for i, exp := range expected {
		if results[i].Key != exp.Key {
			log.Fatalf("getValues: key mismatch at %d: expected %s, got %s", i, exp.Key, results[i].Key)
		}
		for k, v := range exp.FldVals {
			if results[i].FldVals[k] != v {
				log.Fatalf("getValues: field %s mismatch: expected %s, got %s", k, v, results[i].FldVals[k])
			}
		}
	}

	log.Println("-- getValues done -----")
}

// -----------------------------------------------------------------------

// searchKeys finds records where the key falls within a prefix range and
// the key itself contains a search string.
func searchKeys() {
	log.Println("-- searchKeys starting -----")

	testBkt := "test1"
	bo.CreateBkt(httpClient, testBkt)
	defer bo.DeleteBkt(httpClient, testBkt)

	recs := []data.Location{
		{Id: "ca|angelrock  |4800billst", St: "CA", City: "Angel Rock", Address: "4800 Bill St"},
		{Id: "fl|watertown  |120phillips", St: "FL", City: "Watertown", Address: "120 Phillips"},
		{Id: "ca|angelflow  |1008linkwood", St: "CA", City: "Angelflow", Address: "1008 Linkwood"},
		{Id: "tx|deeks      |600rewdave", St: "TX", City: "Deeks", Address: "600 Rewd Ave"},
		{Id: "ca|beeton     |976langlyway", St: "CA", City: "Beeton", Address: "976 Langly Way"},
	}
	resp, err := bo.Put(httpClient, testBkt, bo.SliceToJson(recs), nil)
	checkResp(resp, err, "searchKeys - Put")

	// find records where key prefix is "ca" and key contains "angel"
	resp, err = bo.Run(httpClient, bobb.OpSearchKeys, bobb.SearchKeysRequest{
		BktName:     testBkt,
		SearchValue: "angel",
		StartKey:    "ca",
		EndKey:      "ca",
	})
	checkResp(resp, err, "searchKeys")

	if len(resp.Recs) != 2 {
		log.Fatalf("searchKeys: expected 2 recs, got %d", len(resp.Recs))
	}
	respRecs := bo.JsonToSlice(resp.Recs, data.Location{})
	if respRecs[0].City != "Angelflow" && respRecs[1].City != "Angel Rock" {
		log.Fatalln("searchKeys: unexpected cities:", respRecs[0].City, respRecs[1].City)
	}

	log.Println("-- searchKeys done -----")
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

func loadLocationData(fileName string) {
	jsonData, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatalln("loadLocationData: error reading file:", err)
	}
	var inputRecs []data.Location
	if err := json.Unmarshal(jsonData, &inputRecs); err != nil {
		log.Fatalln("loadLocationData: json.Unmarshal failed:", err)
	}
	locationData = make(map[string]data.Location, len(inputRecs))
	for _, rec := range inputRecs {
		locationData[rec.Id] = rec
		log.Println(rec)
	}
}

func loadRequestData(fileName string) {
	jsonData, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatalln("loadRequestData: error reading file:", err)
	}
	var inputRecs []data.Request
	if err := json.Unmarshal(jsonData, &inputRecs); err != nil {
		log.Fatalln("loadRequestData: json.Unmarshal failed:", err)
	}
	requestData = make(map[string]data.Request, len(inputRecs))
	for _, rec := range inputRecs {
		log.Printf("%+v", rec)
		requestData[rec.Id] = rec
	}
}

// compare asserts that two Location values have identical field values.
func compare(original, result data.Location, funcName string) {
	originalStr := original.Id + original.Address + original.City + original.St + original.Zip + original.LastActionDt
	resultStr := result.Id + result.Address + result.City + result.St + result.Zip + result.LastActionDt
	if originalStr != resultStr {
		log.Println("compare original:", originalStr)
		log.Println("compare result:  ", resultStr)
		log.Fatalln("compare string fields failed:", funcName)
	}
	for i, v := range original.Notes {
		if result.Notes[i] != v {
			log.Println("compare original notes:", original.Notes)
			log.Println("compare result notes:  ", result.Notes)
			log.Fatalln("compare notes failed:", funcName)
		}
	}
	if original.LocationType != result.LocationType {
		log.Printf("compare locationType: original=%d result=%d", original.LocationType, result.LocationType)
		log.Fatalln("compare locationType failed:", funcName)
	}
}

// checkResp calls log.Fatalln if the response status is not Ok or err is non-nil.
func checkResp(resp *bobb.Response, err error, funcName string) {
	if resp == nil {
		log.Fatalln(funcName, "nil response")
	}
	if !(resp.Status == bobb.StatusOk && err == nil) {
		log.Fatalln(funcName, "failed:", resp.Msg, err)
	}
}
