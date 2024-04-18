// bulkloader.go is an example of how data can be loaded in batches.
// Data source is a .csv file with ~85,000 records.
// Load time was ~2 seconds.
// After loading, all records are retrieved from db using 1 GetAll request (runtime ~300ms).
// Rec count and first 100 records in response are displayed to verify results.

package main

import (
	"fmt"

	"bobb"
	bo "bobb/client"

	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"os"
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
}

var locationData []Location // loaded with data from csv file by loadCSVData func below

var httpClient *http.Client

func main() {

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	wg := new(sync.WaitGroup)

	loadCSVData() // loads data from .csv into locationData slice

	var err error
	var resp *bobb.Response

	httpClient = new(http.Client)

	// DELETE / CREATE BUCKET ---------------------------------------
	bktReq := bobb.BktRequest{BktName: "location", Operation: "delete"}
	resp, _ = bo.Run(httpClient, "bkt", bktReq)

	bktReq.Operation = "create"
	resp, err = bo.Run(httpClient, "bkt", bktReq)
	if err != nil {
		log.Fatalln("bkt create failed", err, resp.Msg)
	}

	// upload records to db in batches of batchSize records, using goroutines
	batchSize := 1000
	putReq := newPutReq(batchSize)
	var z int
	for x := 0; x < 3; x++ {
		for _, rec := range locationData {
			z++
			rec.Id = fmt.Sprintf("%d-%s", z, rec.Id)
			jsonRec, err := json.Marshal(rec) // convert each record to []byte
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
		if len(putReq.Recs) > 0 {
			wg.Add(1)
			go run(putReq, wg)
		}
	}
	wg.Wait() // wait for all runs to finish before ending program

	showBktContents()
}

func newPutReq(batchSize int) *bobb.PutRequest {
	return &bobb.PutRequest{
		BktName:  locationBkt,
		KeyField: "id",
		Recs:     make([][]byte, 0, batchSize),
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

	var x int // used to provide random values
	for i, csvRec := range csvRecs {
		if i == 0 { // skip header
			continue
		}
		if csvRec[0] == "" {
			continue
		}
		locRec := Location{
			Id:      csvRec[0],
			Address: csvRec[1],
			City:    csvRec[2],
			St:      csvRec[3],
			Zip:     csvRec[4],
			Notes: []string{
				"Note #1",
				"Note #2",
			},
		}
		if x < 100 {
			locRec.LocationType = 1
			locRec.LastActionDt = "2021-03-22"
		} else if x < 200 {
			locRec.LocationType = 2
			locRec.LastActionDt = "2022-06-10"
		} else if x < 300 {
			locRec.LocationType = 3
			locRec.LastActionDt = "2023-09-01"
		} else {
			x = 0
		}
		x++
		locationData = append(locationData, locRec)
	}
}

func run(req *bobb.PutRequest, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		log.Println("done")
	}()
	//log.Println("start")
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
	req := bobb.GetAllRequest{BktName: "location"}
	resp, err := bo.Run(httpClient, "getall", req)
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
