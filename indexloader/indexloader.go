package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"

	"github.com/valyala/fastjson"

	"sync"
	"time"
)

//************************
// >> THE KEY OF EACH DATA RECORD MUST MATCH THE VALUE STORED IN THE DataIdFld <<
//************************

const batchSize = 1000
const indexSettingsFile = "index_settings.json"

// NOTE - As a safety measure the index bucket name must contain the word "index".
// This safety measure is enforced because the index bkt is deleted/created.
// The purpose is to prevent accidently deleting a data bkt.

// Loaded from index_settings.json
type IndexSetting struct {
	DataBkt   string           `json:"dataBkt"`   // bkt containing data records
	IndexBkt  string           `json:"indexBkt"`  // bkt containing index records, must have "index" in the name
	DataFlds  []bobb.FldFormat `json:"dataFlds"`  // data field name(s) containing value(s) to concatenate for index key
	DataIdFld string           `json:"dataIdFld"` // fld in data recs containing key (matches rec key)
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
	log.Println("indexloader start")

	request := getRequestSettings(indexName) // request is instance of IndexSetting
	log.Println(request)

	test := false
	if test {
		os.Exit(1)
	}

	httpClient = new(http.Client)

	createIndexBkt(request.IndexBkt)

	indexes := make([]bobb.IndexKeyVal, 0, batchSize)

	wg := new(sync.WaitGroup)

	// *** WARNING ***
	// ALL RECORDS ARE RETURNED IN 1 REQUEST
	// FOR VERY LARGE BKTS, A MULTI REQUEST PROCESS USING REQUEST.LIMIT AND RESPONSE.NEXTKEY MAY BE NEEDED

	req := bobb.QryRequest{BktName: request.DataBkt}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	checkResp(resp, err)
	log.Println("dataBkt count", len(resp.Recs))

	for i, rec := range resp.Recs {
		parsedRec, err := jsonParser.ParseBytes(rec)
		if err != nil {
			log.Fatalf("cannot parse json: %s", err)
		}
		v := parsedRec.GetStringBytes(request.DataIdFld)
		if v == nil {
			log.Fatalln("error getting data key from fld-", request.DataIdFld)
			break
		}
		dataKey := string(v)

		indexKey := bobb.MergeFlds(parsedRec, request.DataFlds, "|") // merged plain string values
		indexKey += "|" + strconv.Itoa(i)                            // make unique
		indexes = append(indexes, bobb.IndexKeyVal{Key: indexKey, Val: dataKey})

		if i < 100 { // used to visually verify index keys look as expected
			log.Println(indexKey)
		}

		if len(indexes) == batchSize {
			wg.Add(1)
			go run(request.IndexBkt, indexes, wg)
			time.Sleep(20 * time.Millisecond)
			indexes = make([]bobb.IndexKeyVal, 0, batchSize)
		}
	}
	if len(indexes) > 0 {
		wg.Add(1)
		go run(request.IndexBkt, indexes, wg)
	}

	wg.Wait() // wait for all runs to finish before ending program

	log.Println("indexloader complete")
}

// run executes PutIndex request
func run(bkt string, indexes []bobb.IndexKeyVal, wg *sync.WaitGroup) {
	defer wg.Done()
	req := bobb.PutIndexRequest{
		BktName: bkt,
		Indexes: indexes,
	}
	resp, err := bo.Run(httpClient, bobb.OpPutIndex, req)
	checkResp(resp, err)
	log.Println("batch complete")
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
