// used for stress testing

package main

import (
	"log"
	"net/http"
	"time"

	"bobb"
	bo "bobb/client"
)

const locationBkt = "location"

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

var httpClient = new(http.Client)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	bo.BaseURL = "http://localhost:8000/" // must be where server is listening

	log.Println("starting")
	for i := 0; i < 10000; i++ {
		go qry1()
		go qry2()
		go qry3()
		time.Sleep(20 * time.Millisecond)
		go getIndex()
		go qryIndex()
		go qryIndex2()
		time.Sleep(1 * time.Second)
		log.Println("loop", i, "done")
	}
	log.Println("ending")
}

// qry1 returns records meeting find conditions in sorted order
func qry1() {

	criteria := bo.Find(nil, "zip", bobb.FindStartsWith, "5")

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
	log.Println("qry1", len(resp.Recs))
}

// qry2 returns records meeting find conditions in sorted order
func qry2() {

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
	log.Println("qry2", len(resp.Recs))
}

// qry3 uses Not find condition
func qry3() {

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
	log.Println("qry3", len(resp.Recs))
}

// getIndex uses start/end keys in index bkt to retrieve records from data bkt.
func getIndex() {
	req := bobb.GetIndexRequest{
		BktName:  locationBkt,
		IndexBkt: "location_zip_index",
		StartKey: "50000",
		EndKey:   "59999",
	}
	resp, _ := bo.Run(httpClient, bobb.OpGetIndex, req)
	log.Println("getIndex", len(resp.Recs))
}

// qryIndex retrieves records using index bkt to control which records are read.
func qryIndex() {
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
	log.Println("qryIndex", len(resp.Recs))
}

// qryIndex2 retrieves records using index bkt to control which records are read.
// In this example, only location records where zip between 56000-56999 are scanned.
func qryIndex2() {
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
	log.Println("qryIndex2", len(resp.Recs))
}
