package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"bobb"
	bo "bobb/client"

	"github.com/valyala/fastjson"

	"sync"
	"time"
)

const batchSize = 1000
const indexSettingsFile = "index_settings.json"

// NOTE - As a safety measure the index bucket name must contain the word "index".
// This safety measure is enforced because the index bkt is deleted/created.
// The purpose is to prevent accidently deleting a data bkt.

// Loaded from index_settings.json
type IndexSetting struct {
	DataBkt   string   `json:"dataBkt"`   // bkt containing data records
	IndexBkt  string   `json:"indexBkt"`  // bkt containing index records, must have "index" in the name
	DataFlds  []string `json:"dataFlds"`  // data field name(s) containing value(s) to concatenate for index key
	DataIdFld string   `json:"dataIdFld"` // fld in data recs containing key (matches rec key)
}

var IndexSettings map[string]IndexSetting

var jsonParser fastjson.Parser

var httpClient *http.Client

func main() {
	name := flag.String("name", "", "use -name cmd line option to specify index name key in index_setting.json")
	flag.Parse()
	indexName := *name
	if indexName == "" {
		log.Fatalln("-name command line option required, ex: -name location_zip")
	}

	request := getRequestSettings(indexName) // request is instance of IndexSetting
	log.Println(request)

	test := false
	if test {
		os.Exit(1)
	}

	httpClient = new(http.Client)

	createIndexBkt(request.IndexBkt)

	dataRecs := getDataRecs(request.DataBkt)

	indexVals := make([]string, len(request.DataFlds)) // merged together to form indexData
	var indexData string                               // primary portion of index key

	indexes := make([]bobb.IndexKeyVal, 0, batchSize)

	wg := new(sync.WaitGroup)

	for i, rec := range dataRecs {
		v, err := jsonParser.ParseBytes(rec)
		if err != nil {
			log.Fatalf("cannot parse json: %s", err)
		}
		dataKey := string(v.GetStringBytes(request.DataIdFld))
		if len(request.DataFlds) > 1 {
			for i, fld := range request.DataFlds {
				fldVal := v.GetStringBytes(fld)
				indexVals[i] = string(fldVal)
			}
			indexData = strings.Join(indexVals, "|")
		} else {
			indexData = string(v.GetStringBytes(request.DataFlds[0]))
		}
		indexKey := indexData + "|" + strconv.Itoa(i) // add loop index to ensure uniqueness

		indexes = append(indexes, bobb.IndexKeyVal{Key: indexKey, Val: dataKey})

		if len(indexes) == batchSize {
			wg.Add(1)
			go run(request.IndexBkt, indexes, wg)
			time.Sleep(10 * time.Millisecond)
			indexes = make([]bobb.IndexKeyVal, 0, batchSize)
		}
	}
	if len(indexes) > 0 {
		wg.Add(1)
		go run(request.IndexBkt, indexes, wg)
	}

	wg.Wait() // wait for all runs to finish before ending program
}

func run(bkt string, indexes []bobb.IndexKeyVal, wg *sync.WaitGroup) {
	defer wg.Done()
	req := bobb.PutIndexRequest{
		BktName: bkt,
		Indexes: indexes,
	}
	resp, err := bo.Run(httpClient, "putindex", req)
	checkResp(resp, err)
}

func getDataRecs(dataBkt string) [][]byte {
	req := bobb.GetAllRequest{BktName: dataBkt}
	resp, err := bo.Run(httpClient, "getall", req)
	checkResp(resp, err)
	return resp.Recs
}

func getRequestSettings(indexName string) IndexSetting {
	jsonSettings, err := os.ReadFile(indexSettingsFile)
	if err != nil {
		log.Fatalln("error opening indexSettingsFile", err)
	}
	IndexSettings = make(map[string]IndexSetting)
	if err := json.Unmarshal(jsonSettings, &IndexSettings); err != nil {
		log.Fatalln("json.Unmarshal error on jsonSettings", err)
	}
	request, found := IndexSettings[indexName]
	if !found {
		log.Fatalln("indexName not found in indexSettingsFile", indexName)
	}
	if request.DataBkt == "" || request.DataFlds == nil || request.DataIdFld == "" || request.IndexBkt == "" {
		log.Fatalf("missing value in index_settings.json entry\n %+v \n", request)
	}
	return request
}

// delete and create index bkt
func createIndexBkt(indexBkt string) {
	if !strings.Contains(indexBkt, "index") {
		log.Fatalln("Index Bucket Name Must Contain Word - index")
	}
	req := bobb.BktRequest{BktName: indexBkt, Operation: "delete"}
	bo.Run(httpClient, "bkt", req)

	req.Operation = "create"
	resp, err := bo.Run(httpClient, "bkt", req)
	checkResp(resp, err)
}

func checkResp(resp *bobb.Response, err error) {
	if err != nil || resp.Status != bobb.StatusOk {
		log.Fatalln(err, resp.Status, resp.Msg)
	}
}
