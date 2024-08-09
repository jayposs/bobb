// The client package is used by client programs to send requests to bobb server.

package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"bobb"
)

var BaseURL string = "http://localhost:8000/" // must match port used by server.go from bobb-settings.json
var Debug bool

// Run sends bobb server http request using the provided payload.
// Used by client Go programs to interact with db.
func Run(httpClient *http.Client, op string, payload interface{}) (*bobb.Response, error) {
	reqUrl := BaseURL + op
	jsonContent, err := json.Marshal(&payload) // -> []byte

	if Debug {
		log.Println("request url > ", reqUrl)
		log.Println("--- client sending ---")
		log.Println(fmtJSON(jsonContent))
	}

	reqBody := bytes.NewReader(jsonContent) // -> io.Reader

	req, err := http.NewRequest("POST", reqUrl, reqBody)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept-Encoding", "gzip")

	httpResp, doErr := httpClient.Do(req)
	defer func() {
		//if doErr == nil {
		httpResp.Body.Close()
		//}
	}()
	if httpResp.StatusCode != http.StatusOK || doErr != nil {
		log.Println("Request Failed, Status:", httpResp.Status)
		if httpResp.StatusCode == http.StatusNotFound {
			log.Println("verify op code specified in Run call is valid:", op, reqUrl)
		}
		return nil, doErr
	}

	var result []byte
	encoding := httpResp.Header.Get("Content-Encoding")
	if encoding == "gzip" {
		gzipContent, _ := gzip.NewReader(httpResp.Body)
		result, err = io.ReadAll(gzipContent)
		gzipContent.Close()
	} else {
		result, err = io.ReadAll(httpResp.Body)
	}
	if err != nil {
		log.Println("Read Http Response.Body Failed:", err)
		return nil, err
	}

	if Debug {
		log.Println("--- client receiving ---")
		log.Println(fmtJSON(result))
	}

	bobbResp := new(bobb.Response)
	err = json.Unmarshal(result, bobbResp)

	return bobbResp, err
}
