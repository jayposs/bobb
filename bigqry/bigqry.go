// used for testing random queries with larger size db

package main

import (
	"encoding/json"
	"log"
	"net/http"

	"bobb"
	bo "bobb/client"
)

const locationBkt = "location"

type Agent struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}
type Location struct {
	Id           string   `json:"id"`
	Address      string   `json:"address"`
	City         string   `json:"city"`
	St           string   `json:"st"`
	Zip          string   `json:"zip"`
	LocationType int      `json:"locationType"`
	LastActionDt string   `json:"lastActionDt"` // "yyyy-mm-dd"
	Notes        []string `json:"notes"`
	LocAgent     Agent    `json:"agent"`
}

var httpClient *http.Client

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("--- bigqry program starting ---")

	bo.BaseURL = "http://localhost:8000/" // must be where server is listening

	httpClient = new(http.Client)

	qry1()
	qry2()
	qry3()
	getIndex()
	qryIndex()
	qryIndex2()

	log.Println("--- bigqry pgm finished ---")
}

// qry1 returns records meeting find conditions in sorted order
func qry1() {
	log.Println("-- qry1 starting -----")
	criteria := []bobb.FindCondition{
		{Fld: "zip", Op: bobb.FindStartsWith, ValStr: "5"},
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
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("count", len(resp.Recs))

	results := make([]Location, len(resp.Recs))
	for i, rec := range resp.Recs {
		json.Unmarshal(rec, &results[i])
	}
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- qry1 done -----")
}

// qry2 returns records meeting find conditions in sorted order
func qry2() {
	log.Println("-- qry2 starting -----")
	criteria := []bobb.FindCondition{
		{Fld: "st", Op: bobb.FindAfter, ValStr: "ok"},
		{Fld: "address", Op: bobb.FindContains, ValStr: "ave"},
		{Fld: "locationType", Op: bobb.FindEquals, ValInt: 3},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "locationType", Dir: bobb.SortAscInt},
		{Fld: "st", Dir: bobb.SortDescStr},
		{Fld: "city", Dir: bobb.SortAscStr},
	}
	req := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("count", len(resp.Recs))

	results := make([]Location, len(resp.Recs))
	for i, rec := range resp.Recs {
		json.Unmarshal(rec, &results[i])
	}
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- qry2 done -----")
}

// qry3 uses Not find condition
func qry3() {
	log.Println("-- qry3 starting -----")
	criteria := []bobb.FindCondition{
		{Fld: "st", Op: bobb.FindMatches, ValStr: "TN"},
		{Fld: "locationType", Op: bobb.FindEquals, ValInt: 3, Not: true},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "locationType", Dir: bobb.SortDescInt},
		{Fld: "city", Dir: bobb.SortAscStr},
	}
	req := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("count", len(resp.Recs))

	results := make([]Location, len(resp.Recs))
	for i, rec := range resp.Recs {
		json.Unmarshal(rec, &results[i])
	}
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- qry3 done -----")
}

// getIndex uses start/end keys in index bkt to retrieve records from data bkt.
// runs in 9ms -returns 3800 recs out of 250,000 total
func getIndex() {
	log.Println("-- getIndex starting -----")
	req := bobb.GetIndexRequest{
		BktName:  locationBkt,
		IndexBkt: "location_zipbig_index",
		StartKey: "5", // zip >= 50000
		EndKey:   "6", // zip <= 60000
	}
	resp, _ := bo.Run(httpClient, bobb.OpGetIndex, req)
	log.Println("count", len(resp.Recs))

	results := make([]Location, len(resp.Recs))
	for i, rec := range resp.Recs {
		json.Unmarshal(rec, &results[i])
	}
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- getIndex done -----")
}

// qryIndex retrieves records using index bkt to control which records are read.
// In this example, only location records where zip between 54000-59999 are scanned.
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
		IndexBkt:       "location_zipbig_index",
		StartKey:       "40000",
		EndKey:         "69999",
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQryIndex, req)
	log.Println("count", len(resp.Recs))

	results := make([]Location, len(resp.Recs))
	for i, rec := range resp.Recs {
		json.Unmarshal(rec, &results[i])
	}
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- qryIndex done -----")
}

// qryIndex2 retrieves records using index bkt to control which records are read.
// In this example, only location records where zip between 56000-56999 are scanned.
func qryIndex2() {
	log.Println("-- qryIndex2 starting -----")
	criteria := []bobb.FindCondition{
		{Fld: "address", Op: bobb.FindContains, ValStr: "ave"},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "city", Dir: bobb.SortDescStr},
	}
	req := bobb.QryIndexRequest{
		BktName:        locationBkt,
		IndexBkt:       "location_zipbig_index",
		StartKey:       "56000",
		EndKey:         "56999",
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQryIndex, req)
	log.Println("count", len(resp.Recs))

	results := make([]Location, len(resp.Recs))
	for i, rec := range resp.Recs {
		json.Unmarshal(rec, &results[i])
	}
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- qryIndex2 done -----")
}
