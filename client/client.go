// The client package is used by client programs to send requests to bobb server.

package client

import (
	"bytes"
	"encoding/json"
	"errors"
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

	resp, doErr := httpClient.Do(req)
	defer func() {
		if doErr == nil {
			resp.Body.Close()
		}
	}()
	if resp.StatusCode != http.StatusOK || doErr != nil {
		log.Println("Request Failed, Status:", resp.Status)
		if resp.StatusCode == http.StatusNotFound {
			log.Println("verify op code specified in Run call is valid:", op, reqUrl)
		}
		return nil, doErr
	}
	result, err := io.ReadAll(resp.Body) // -> []byte
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

// format JSON in easy to view format
func fmtJSON(jsonContent []byte) string {
	var out bytes.Buffer
	json.Indent(&out, jsonContent, "", "  ")
	return out.String()
}

// GetOne is shortcut func. Gets rec from bkt where key=id.
// Unmarshals rec into result (pointer to type var).
func GetOne(httpClient *http.Client, bkt, id string, result any) error {
	req := bobb.GetOneRequest{BktName: bkt, Key: id}
	resp, err := Run(httpClient, bobb.OpGetOne, req)
	if resp.Status != bobb.StatusOk {
		return errors.New(resp.Msg)
	}
	err = json.Unmarshal(resp.Rec, result)
	return err
}

// PutOne is shortcut func. Puts rec into specified bkt.
// The rec key wil be the value of fld with name=idFld.
func PutOne(httpClient *http.Client, bkt, idFld string, rec any) error {
	jsonRec, err := json.Marshal(rec)
	if err != nil {
		log.Println("Client.PutOne json.Marshal failed", err)
		return err
	}
	req := bobb.PutOneRequest{BktName: bkt, KeyField: idFld, Rec: jsonRec}
	resp, err := Run(httpClient, bobb.OpPutOne, req)
	if resp.Status != bobb.StatusOk {
		log.Println(string(jsonRec))
		return errors.New(resp.Msg)
	}
	return err
}

func CreateBkt(httpClient *http.Client, bktName string) error {
	req := bobb.BktRequest{
		BktName:   bktName,
		Operation: "create",
	}
	resp, err := Run(httpClient, bobb.OpBkt, req)
	if resp.Status != bobb.StatusOk {
		return errors.New(resp.Msg)
	}
	return err
}

func DeleteBkt(httpClient *http.Client, bktName string) {
	req := bobb.BktRequest{
		BktName:   bktName,
		Operation: "delete",
	}
	Run(httpClient, bobb.OpBkt, req) // errors ignored
}
