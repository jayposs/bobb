// used for testing random queries with larger size db

package main

import (
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
	A            string   `json:"a"`
	B            string   `json:"b"`
	C            string   `json:"c"`
	D            string   `json:"d"`
}

func (rec Location) RecId() string {
	return rec.Id
}

var httpClient *http.Client

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("--- bigqry program starting ---")

	bo.BaseURL = "http://localhost:8000/" // must be where server is listening

	httpClient = new(http.Client)

	getAllKeys("location_city_address_index")
	qry1()
	qry2()
	qry3()
	getIndex()
	getIndex2()
	qryIndex()
	qryIndex2()
	getAllMatchPrefix()

	log.Println("--- bigqry pgm finished ---")
}

func getAllKeys(bkt string) {
	log.Println("-- getAllKeys starting -----")

	req := bobb.GetAllKeysRequest{BktName: bkt}
	resp, _ := bo.Run(httpClient, bobb.OpGetAllKeys, req)
	log.Println("count", len(resp.Recs))

	for i := 0; i < 100; i++ {
		log.Println(string(resp.Recs[i]))
	}
	log.Println("-- getAllKeys done -----")
}

func qry1() {
	log.Println("-- qry1 starting -----")

	criteria := bo.Find(nil, "zip", bobb.FindStartsWith, "5")

	sortKeys := bo.Sort(nil, "locationType", bobb.SortDescInt)
	sortKeys = bo.Sort(sortKeys, "address", bobb.SortAscStr)

	req := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("count", len(resp.Recs))

	results := bo.JsonToSlice(resp.Recs, Location{})
	for i := 0; i < 20; i++ {
		log.Println(results[i].LocationType, results[i].Address, results[i].Zip)
	}
	for i := 20; i > 0; i-- {
		x := len(results) - i
		log.Println(results[x].LocationType, results[x].Address, results[x].Zip)
	}
	log.Println("-- qry1 done -----")
}

func qry2() {
	log.Println("-- qry2 starting -----")

	criteria := bo.Find(nil, "st", bobb.FindAfter, "ok")
	criteria = bo.Find(criteria, "address", bobb.FindContains, "ave")
	criteria = bo.Find(criteria, "locationType", bobb.FindEquals, 3)
	criteria = bo.Find(criteria, "a", bobb.FindMatches, "red", bobb.FindNot)

	sortKeys := bo.Sort(nil, "locationType", bobb.SortAscInt)
	sortKeys = bo.Sort(sortKeys, "st", bobb.SortDescStr)
	sortKeys = bo.Sort(sortKeys, "city", bobb.SortAscStr)
	sortKeys = bo.Sort(sortKeys, "a", bobb.SortAscStr)

	req := bobb.QryRequest{
		BktName:        locationBkt,
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("count", len(resp.Recs))

	results := bo.JsonToSlice(resp.Recs, Location{})
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- qry2 done -----")
}

func qry3() {
	log.Println("-- qry3 starting, uses NOT condition -----")

	criteria := bo.Find(nil, "st", bobb.FindMatches, "TN")
	criteria = bo.Find(criteria, "locationType", bobb.FindEquals, 3, bobb.FindNot) // val != 3

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

	results := bo.JsonToSlice(resp.Recs, Location{})
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- qry3 done -----")
}

// getIndex uses start/end keys in index bkt to retrieve records from data bkt.
func getIndex() {
	log.Println("-- getIndex starting -----")
	req := bobb.GetIndexRequest{
		BktName:  locationBkt,
		IndexBkt: "location_zip_index",
		StartKey: "50000",
		EndKey:   "62000",
	}
	resp, _ := bo.Run(httpClient, bobb.OpGetIndex, req)
	log.Println("count", len(resp.Recs))

	results := bo.JsonToSlice(resp.Recs, Location{})
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- getIndex done -----")
}

// getIndex2 uses start/end keys in index bkt to retrieve records from data bkt.
// StartKey = EndKey, indicates key prefix must match StartKey.
func getIndex2() {
	log.Println("-- getIndex2 starting -----")
	req := bobb.GetIndexRequest{
		BktName:  locationBkt,
		IndexBkt: "location_city_address_index",
		StartKey: "abilene        |150pil",
		EndKey:   "adams          |134commercialst",
	}
	resp, _ := bo.Run(httpClient, bobb.OpGetIndex, req)
	log.Println("count", len(resp.Recs))

	results := bo.JsonToSlice(resp.Recs, Location{})
	for i := 0; i < len(results); i++ {
		log.Println(results[i])
	}
	log.Println("-- getIndex2 done -----")
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
		IndexBkt:       "location_zip_index",
		StartKey:       "40000",
		EndKey:         "69999",
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQryIndex, req)
	log.Println("count", len(resp.Recs))

	results := bo.JsonToSlice(resp.Recs, Location{})
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
		IndexBkt:       "location_zip_index",
		StartKey:       "56000",
		EndKey:         "56999",
		FindConditions: criteria,
		SortKeys:       sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQryIndex, req)
	log.Println("count", len(resp.Recs))

	results := bo.JsonToSlice(resp.Recs, Location{})
	for i := 0; i < 10; i++ {
		log.Println(results[i])
	}
	log.Println("-- qryIndex2 done -----")
}

func getAllMatchPrefix() {
	log.Println("-- getAllMatchPrefix starting -----")

	req := bobb.GetAllRequest{
		BktName:  locationBkt,
		StartKey: "ABI",
		EndKey:   "ABI",
	}
	resp, _ := bo.Run(httpClient, bobb.OpGetAll, req)
	log.Println("count", len(resp.Recs))

	results := bo.JsonToSlice(resp.Recs, Location{})
	for i := 0; i < len(results); i++ {
		log.Println(results[i])
	}
	log.Println("-- getAllMatchPrefix done -----")
}
