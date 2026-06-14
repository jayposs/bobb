// The client package is used by client programs to send requests to bobb server.

package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jayposs/bobb"
)

// vars set by client programs
var (
	BaseURL string // port must match value in settings file used by bobb_server
	Debug   bool
)

// A minimal, valid gzip string (empty content compressed)
// used to safely initialize new readers in the pool.
var emptyGzipBytes = []byte{
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0xff, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
}

var gzipReaderPool = sync.Pool{
	New: func() any {
		// Initialize the reader with a dummy valid gzip header
		r, err := gzip.NewReader(bytes.NewReader(emptyGzipBytes))
		if err != nil {
			panic(err)
		}
		return r
	},
}

// Run sends bobb server http request using the provided payload.
// Used by client Go programs to interact with db.
func Run(httpClient *http.Client, op string, payload any) (*bobb.Response, error) {

	var requestFailed bool

	reqUrl := BaseURL + op

	jsonContent, err := json.Marshal(&payload) // -> []byte
	if err != nil {
		log.Println("client.Run, json.Marshal of payload failed", err)
		return nil, err
	}
	if Debug {
		log.Println("request url > ", reqUrl)
		log.Println("--- client sending ---")
		log.Println(fmtJSON(jsonContent))
	}

	reqBody := bytes.NewReader(jsonContent) // -> io.Reader

	req, err := http.NewRequest("POST", reqUrl, reqBody)
	if err != nil {
		log.Println("client.Run, http.NewRequest failed", err)
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept-Encoding", "gzip")

	httpClient.Timeout = 30 * time.Second
	httpResp, err := httpClient.Do(req)
	if err != nil {
		log.Println("client.Run, http.Do request failed", err)
		return nil, err
	}

	defer func() {
		if requestFailed {
			io.Copy(io.Discard, httpResp.Body) // ensure body is drained to allow connection reuse
		}
		httpResp.Body.Close()
	}()

	if httpResp.StatusCode != http.StatusOK {
		requestFailed = true
		log.Println("Request Failed, Status:", httpResp.Status)
		if httpResp.StatusCode == http.StatusNotFound {
			log.Println("verify op code specified in Run call is valid:", op, reqUrl)
		}
		return nil, errors.New("bad http response Status-" + httpResp.Status)
	}

	var respReader io.Reader

	encoding := httpResp.Header.Get("Content-Encoding")
	if encoding == "gzip" {
		gzipReader := gzipReaderPool.Get().(*gzip.Reader)

		defer func() {
			gzipReader.Reset(bytes.NewReader(emptyGzipBytes)) // for safety sake, not required
			gzipReaderPool.Put(gzipReader)
		}()

		err = gzipReader.Reset(httpResp.Body)
		if err != nil {
			requestFailed = true
			log.Println("gzipReader.Reset(httpResp.Body) failed", err)
			return nil, err
		}
		respReader = gzipReader
	} else {
		respReader = httpResp.Body
	}

	var bobbResp bobb.Response

	if Debug {
		log.Println("--- DEBUG MODE ON > client receiving ---")
		var resultBuffer bytes.Buffer
		_, err = io.Copy(&resultBuffer, respReader)
		if err != nil {
			requestFailed = true
			log.Println("Debug Mode, io.Copy from http respReader failed", err)
			return nil, err
		}
		result := resultBuffer.Bytes()
		log.Println(fmtJSON(result))
		err = json.Unmarshal(result, &bobbResp)
	} else {
		err = json.NewDecoder(respReader).Decode(&bobbResp)
	}

	if err != nil {
		requestFailed = true
		log.Println("json.Decode result into bobb.Response Failed:", err)
		return nil, err
	}
	return &bobbResp, err
}
