// bulkload.go is a template of how a large number of records can be loaded quickly.
// Upload records to db bucket in batches using goroutines.
// Better than loading a large number of records in 1 transaction.

package main

import (
	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"

	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

const targetBkt = "target"

type TargetRec struct {
	Id     string `json:"id"`
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

var importRecs []TargetRec

var httpClient *http.Client = new(http.Client)

func main() {

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("bulk load start")

	wg := new(sync.WaitGroup)

	loadImportRecs() // load records to be imported into importRecs slice

	batchSize := 1000

	putReq := newPutReq(targetBkt, batchSize)

	for _, rec := range importRecs {
		jsonRec, err := json.Marshal(rec) // convert each record to []byte
		if err != nil {
			log.Fatalln("json.Marshal failed", err)
		}
		putReq.Recs = append(putReq.Recs, jsonRec)
		if len(putReq.Recs) == batchSize {
			wg.Add(1)
			go run(putReq, wg)
			putReq = newPutReq(targetBkt, batchSize)
			time.Sleep(10 * time.Millisecond) // pause may be appropriate for large number of requests
		}
	}
	if len(putReq.Recs) > 0 {
		wg.Add(1)
		go run(putReq, wg)
	}
	wg.Wait() // wait for all runs to finish before ending program

	log.Println("bulk load complete")
}

func newPutReq(bkt string, batchSize int) *bobb.PutRequest {
	return &bobb.PutRequest{
		BktName:      bkt,
		KeyField:     "id",
		Recs:         make([][]byte, 0, batchSize),
		RequiredFlds: []string{"field1", "field2"}, // optional
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

func loadImportRecs() {
	// code to import records into importRecs

	// NOTE - may want to add logic to make id value useful, it will be used as key

	// consider implementing JSON schema to validate data
}
