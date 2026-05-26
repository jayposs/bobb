// exportcsv.go program creates a csv file from results of qry or getall requests.
// exportcsv_settings.yaml contains parameters used when executing the pgm.
// The YamlReq file contains the QryRequest or GetAllRequest attributes.
// See example yaml files in qry_scripts and getall_scripts sub folders.

// The datatype associated with the request BktName must implement the CsvExport interface defined in bobb/types.go.
// See datatypes/location.go for example.
// An entry must be added to CsvDataRec func in datatypes/csvexport.go for each datatype implementing CsvExport interface.
// The switch stmt case values match BktName values in requests.

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"
	data "github.com/jayposs/bobb/datatypes"

	"encoding/csv"

	"gopkg.in/yaml.v3"
)

var httpClient = new(http.Client)

var settings struct {
	RequestType string //`yaml:"RequestType"` // qry or getall
	YamlReq     string //`yaml:"YamlReq"`     // name of yaml request file
	ExportFile  string //`yaml:"ExportFile"`  // name of csv export file
}

func main() {

	yamlSettings, err := os.ReadFile("exportcsv_settings.yaml")
	if err != nil {
		fmt.Println("error reading exportcsv_settings.yaml file:", err)
		os.Exit(1)
	}
	if err := yaml.Unmarshal(yamlSettings, &settings); err != nil {
		fmt.Println("error parsing yamlSettings", err)
		os.Exit(1)
	}
	fmt.Println("-- settings ----- \n", settings)

	yamlReq, err := os.ReadFile(settings.YamlReq)
	if err != nil {
		fmt.Println("error reading yaml file: ", settings.YamlReq, err)
		os.Exit(1)
	}
	fmt.Println("-- yamlReq --------- \n", string(yamlReq))

	bo.BaseURL = "http://localhost:50555/"

	switch settings.RequestType {
	case "qry":
		exportFromQry(yamlReq) // recs is [][]byte
	case "getall":
		exportFromGetAll(yamlReq)
	default:
		fmt.Println("invalid request type: ", settings.RequestType)
		os.Exit(1)
	}
}

func exportFromGetAll(yamlReq []byte) {
	fmt.Println("-- exportFromGetAll -----------------")

	var req bobb.GetAllRequest
	if err := yaml.Unmarshal(yamlReq, &req); err != nil {
		fmt.Println("error parsing getall yaml:", err)
		os.Exit(1)
	}
	fmt.Println(req)
	resp, err := bo.Run(httpClient, bobb.OpGetAll, req)
	if err != nil || resp.Status != bobb.StatusOk {
		fmt.Println("error:", resp.Msg, err)
		os.Exit(1)
	}
	fmt.Println("getall resp.Recs count", len(resp.Recs))
	writeCsvFile(req.BktName, resp.Recs, settings.ExportFile, false)
}

func exportFromQry(yamlReq []byte) {
	fmt.Println("-- exportFromQry -----------------")

	var req bobb.QryRequest
	if err := yaml.Unmarshal(yamlReq, &req); err != nil {
		fmt.Println("error parsing qry yaml:", err)
		os.Exit(1)
	}
	fmt.Println("-- bktName, indexBkt > ", req.BktName, req.IndexBkt)
	fmt.Println("-- criteria ----------")
	for i, grp := range req.Criteria {
		for _, cond := range grp {
			fmt.Println(i, cond.Fld, cond.Op, cond.ValStr)
		}
	}
	fmt.Println("-- sortkeys ----------")
	for i, sortKey := range req.SortKeys {
		fmt.Println(i, sortKey.Fld, sortKey.Dir, sortKey.UseDefault)
	}
	resp, err := bo.Run(httpClient, bobb.OpQry, req)
	if err != nil || resp.Status != bobb.StatusOk {
		fmt.Println(resp.Errs)
		fmt.Println("error:", resp.Msg, err)
		os.Exit(1)
	}
	var includeJoins bool
	if len(req.JoinsAfterFind) > 0 || len(req.JoinsBeforeFind) > 0 {
		includeJoins = true
	}
	fmt.Println("qry resp.Recs count", len(resp.Recs))
	writeCsvFile(req.BktName, resp.Recs, settings.ExportFile, includeJoins)
}

func writeCsvFile(bktName string, recs [][]byte, exportFile string, includeJoins bool) {
	// open export file for writing
	expFile, err := os.Create(exportFile)
	if err != nil {
		fmt.Println("error creating export file:", exportFile, err)
		os.Exit(1)
	}
	defer expFile.Close()

	writer := csv.NewWriter(expFile)
	defer writer.Flush() // executes before expFile.Close() above (defers are LIFO)

	var dataRecord bobb.CsvExport // CsvExport is interface type (see types.go)
	for i, rec := range recs {
		dataRecord = data.CsvDataRec(rec, bktName) // get instance of bkt dataType populated from rec jsonBytes
		if i == 0 {
			if err := writer.Write(dataRecord.CsvHeader(includeJoins)); err != nil {
				fmt.Println("error writing csv header:", err)
			}
		}
		if err := writer.Write(dataRecord.CsvData(includeJoins)); err != nil {
			fmt.Println("error writing csv data line:", err)
		}
	}
}
