// bulkloader.go is an example of how data can be loaded in batches.
// Data source is a .csv file with ~85,000 records.

package main

import (
	"fmt"
	"strings"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"

	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"
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
	ManagerId    string   `json:"managerId"`
	ManagerName  string   `json:"manager_name,omitempty"`  // used for join testing in bigqry.go
	ManagerLevel string   `json:"manager_level,omitempty"` // used for join testing in bigqry.go
	Long1        string
	Long2        string
	Long3        string
	Int1         int
	Int2         int
}

var locationData []Location // loaded with data from csv file by loadCSVData func below

var httpClient *http.Client = new(http.Client)

func main() {

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	bo.BaseURL = "http://localhost:50555/"
	bo.Debug = false

	wg := new(sync.WaitGroup)

	loadCSVData() // loads data from .csv into locationData slice
	log.Println("csv rec count:", len(locationData))

	// DELETE BUCKET, NEW BUCKET CREATED AUTOMATICALLY ----------------
	bktReq := bobb.BktRequest{BktName: locationBkt, Operation: "delete"}
	bo.Run(httpClient, "bkt", bktReq)

	// upload records to db in batches of batchSize records, using goroutines
	batchSize := 1000
	var putReq *bobb.PutRequest
	for i := 0; i < 2; i++ { // loading the same records multiple times to increase bkt size
		putReq = newPutReq(batchSize)
		for _, rec := range locationData {
			rec.Id = fmt.Sprintf("%s-%d", rec.Id, i) // make record id unique since loading the same records i times
			jsonRec, err := json.Marshal(rec)        // convert each record to []byte
			if err != nil {
				log.Fatalln("json.Marshal failed", err)
			}
			putReq.Recs = append(putReq.Recs, jsonRec)
			if len(putReq.Recs) == batchSize {
				wg.Add(1)
				go run(putReq, wg)
				putReq = newPutReq(batchSize)
				time.Sleep(10 * time.Millisecond) // pause may be appropriate for large number of requests
			}
		}
	}
	if len(putReq.Recs) > 0 {
		wg.Add(1)
		go run(putReq, wg)
	}
	wg.Wait() // wait for all runs to finish before ending program

	showBktContents()

	qry1() // produce results simulating results for bigqry.go qry1()
}

func newPutReq(batchSize int) *bobb.PutRequest {
	return &bobb.PutRequest{
		BktName:      locationBkt,
		KeyField:     "id",
		Recs:         make([][]byte, 0, batchSize),
		RequiredFlds: []string{"address", "city", "st", "zip"},
	}
}

func loadCSVData() {
	var filePath = "/home/jay/data/properties.csv"
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalln("open csv file failed", err)
	}
	reader := csv.NewReader(file)
	csvRecs, err := reader.ReadAll()

	locationData = make([]Location, 0, len(csvRecs))

	// load records that will be 1st in sorted order, used to verify biqqry tests
	for i := 0; i < 5; i++ {
		firstRec := Location{
			Id:           fmt.Sprintf("00000-%d", i),
			Address:      fmt.Sprintf("000 address-%d", i),
			City:         fmt.Sprintf("aaa city-%d", i),
			St:           fmt.Sprintf("AA-%d", i),
			Zip:          fmt.Sprintf("00000-%d", i),
			LocationType: 0,
			LastActionDt: "2025-06-01",
		}
		locationData = append(locationData, firstRec)
	}
	// load records that will be last in sorted order, used to verify biqqry tests
	for i := 0; i < 5; i++ {
		lastRec := Location{
			Id:           fmt.Sprintf("99999-%d", i),
			Address:      fmt.Sprintf("999 address-%d", i),
			City:         fmt.Sprintf("zzz city-%d", i),
			St:           fmt.Sprintf("ZZ-%d", i),
			Zip:          fmt.Sprintf("99999-%d", i),
			LocationType: 0,
			LastActionDt: "2025-06-30",
		}
		locationData = append(locationData, lastRec)
	}

	var x int // used to provide random values
	for i, csvRec := range csvRecs {
		if i == 0 { // skip header
			continue
		}
		if csvRec[0] == "" {
			continue
		}
		locRec := Location{
			Id: fmt.Sprintf("%s-%d", csvRec[2], i),
			//Id:      csvRec[0],
			Address: csvRec[1],
			City:    csvRec[2],
			St:      csvRec[3],
			Zip:     csvRec[4],
			Notes: []string{
				"Note #1",
				"Note #2",
			},
			Long1: "qwerryrtyrtyrtyrtrwdsvsd",
			Long2: "fjglgj98ifkldkkgll9uyrbvcvxgs",
			Long3: "zxchhgmoiury78765gfhisn",
			Int1:  123456789,
			Int2:  98876549987,
		}
		if x < 100 {
			locRec.LocationType = 1
			locRec.LastActionDt = "2021-03-22"
			locRec.A = "red"
			locRec.B = "green"
			locRec.C = "blue"
			locRec.D = "yellow"
			locRec.ManagerId = "100"
		} else if x < 200 {
			locRec.LocationType = 2
			locRec.LastActionDt = "2022-06-10"
			locRec.A = "yellow"
			locRec.B = "blue"
			locRec.C = "green"
			locRec.D = "red"
			locRec.ManagerId = "200"
		} else if x < 300 {
			locRec.LocationType = 3
			locRec.LastActionDt = "2023-09-01"
			locRec.A = "one"
			locRec.B = "two"
			locRec.C = "three"
			locRec.D = "four"
			locRec.ManagerId = "300"
		} else {
			locRec.LocationType = 4
			locRec.LastActionDt = "2023-09-01"
			locRec.A = "ace"
			locRec.B = "king"
			locRec.C = "queen"
			locRec.D = "jack"
			locRec.ManagerId = "xxx" // indicates no manager
			x = 0
		}
		x++
		locationData = append(locationData, locRec)
	}
}

func run(req *bobb.PutRequest, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		log.Println("-- put request complete")
	}()
	log.Println("++ start put request")
	resp, err := bo.Run(httpClient, "put", req)
	if err != nil {
		log.Fatalln("put req failed", err)
	}
	if resp.Status != bobb.StatusOk {
		log.Fatalln("ERROR", resp.Status, resp.Msg)
	}
}

func showBktContents() {
	log.Println("getall start")
	req := bobb.QryRequest{BktName: "location"}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	if err != nil {
		log.Fatalln("getall req failed", err)
	}
	if resp.Status != bobb.StatusOk {
		log.Fatalln("ERROR", resp.Status, resp.Msg)
	}
	log.Println("getall done")
	log.Println("cnt", len(resp.Recs))
	for i, rec := range resp.Recs {
		locRec := new(Location)
		json.Unmarshal(rec, locRec)
		log.Println(i, locRec.Id, locRec.Address, locRec.LocationType, locRec.City, locRec.Notes)
		if i > 100 {
			break
		}
	}
	log.Println("complete")
}

func qry1() {
	matching := make([]int, 0, len(locationData))
	for i, rec := range locationData {
		if strings.HasPrefix(rec.Zip, "5") {
			matching = append(matching, i)
		}
	}
	slices.SortFunc(matching, func(a, b int) int {
		n := locationData[a].LocationType - locationData[b].LocationType // desc locationType
		if n != 0 {
			n = n * -1
			return n
		}
		return strings.Compare(locationData[a].Address, locationData[b].Address) // asc address
	})
	log.Println("qry1 count", len(matching))
	// show 1st 20 records
	for i := 0; i < 20; i++ {
		x := matching[i]
		rec := locationData[x]
		fmt.Println(rec.LocationType, rec.Address, rec.Zip)
	}
	// show last 20 records
	for i := 20; i > 0; i-- {
		z := len(matching) - i
		x := matching[z]
		rec := locationData[x]
		fmt.Println(rec.LocationType, rec.Address, rec.Zip)
	}
}
