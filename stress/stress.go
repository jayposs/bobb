// used for stress testing

package main

import (
	"log"
	"net/http"
	"time"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"
)

const locationBkt = "location"

var httpClient = new(http.Client)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	bo.BaseURL = "http://localhost:50555/"
	bo.Debug = false

	//log.Println("starting")
	//for i := 0; i < 10000; i++ {
	for {
		log.Println("loop start")
		go qry1()
		go qry2()
		go qry3()
		time.Sleep(20 * time.Millisecond)
		go qryIndex()
		go qryIndex2()
		go qryIndex3()
		log.Println("loop done")
		time.Sleep(2 * time.Second)
	}
	//log.Println("ending")
}

// qry1 returns records meeting find conditions in sorted order
func qry1() {

	resp, _ := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
		BktName: locationBkt,
		Criteria: []bobb.FindGroup{
			bo.Find(nil, "zip", bobb.FindStartsWith, "5"),
		},
		SortKeys: []bobb.SortKey{
			{Fld: "locationType", Dir: bobb.SortDescInt},
			{Fld: "address", Dir: bobb.SortAscStr},
		},
	})
	log.Println("qry1", len(resp.Recs))
}

// qry2 returns records meeting find conditions in sorted order
func qry2() {

	criteria := bo.Find(nil, "st", bobb.FindAfter, "ok")
	criteria = bo.Find(criteria, "address", bobb.FindContainsWord, "ave")
	criteria = bo.Find(criteria, "locationType", bobb.FindEquals, 3)

	sortKeys := bo.Sort(nil, "locationType", bobb.SortAscInt)
	sortKeys = bo.Sort(sortKeys, "st", bobb.SortDescStr)
	sortKeys = bo.Sort(sortKeys, "city", bobb.SortAscStr)

	req := bobb.QryRequest{
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{criteria},
		SortKeys: sortKeys,
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
		BktName:  locationBkt,
		Criteria: []bobb.FindGroup{criteria},
		SortKeys: sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("qry3", len(resp.Recs))
}

// qryIndex uses start/end keys in index bkt to retrieve records from data bkt.
func qryIndex() {
	req := bobb.QryRequest{
		BktName:  locationBkt,
		IndexBkt: "location_zip_index",
		StartKey: "50000",
		EndKey:   "59999",
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("qryIndex", len(resp.Recs))
}

// qryIndex2 retrieves records using index bkt to control which records are read.
func qryIndex2() {
	criteria := bobb.FindGroup{
		{Fld: "address", Op: bobb.FindContains, ValStr: "ave"},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "city", Dir: bobb.SortDescStr},
	}
	req := bobb.QryRequest{
		BktName:  locationBkt,
		IndexBkt: "location_zip_index",
		StartKey: "40000",
		EndKey:   "69999",
		Criteria: []bobb.FindGroup{criteria},
		SortKeys: sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("qryIndex2", len(resp.Recs))
}

// qryIndex3 retrieves records using index bkt to control which records are read.
func qryIndex3() {
	criteria := bobb.FindGroup{
		{Fld: "address", Op: bobb.FindContains, ValStr: "ave"},
	}
	sortKeys := []bobb.SortKey{
		{Fld: "city", Dir: bobb.SortDescStr},
	}
	req := bobb.QryRequest{
		BktName:  locationBkt,
		IndexBkt: "location_zip_index",
		StartKey: "56000",
		EndKey:   "56999",
		Criteria: []bobb.FindGroup{criteria},
		SortKeys: sortKeys,
	}
	resp, _ := bo.Run(httpClient, bobb.OpQry, req)
	log.Println("qryIndex3", len(resp.Recs))
}
