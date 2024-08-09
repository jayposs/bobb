package client

import (
	"bobb"
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
)

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

// Find is convenience func used to create/load []bobb.FindCondition used by qry requests.
// First parm is the slice of conditions to which entry will be appended.
// If nil, a new slice will be created.
// To set parm notCondition, use bobb.FindNot constant in call.
func Find(conditions []bobb.FindCondition, fld, op string, val any, notCondition ...bool) []bobb.FindCondition {
	if conditions == nil {
		conditions = make([]bobb.FindCondition, 0, 9)
	}
	condition := bobb.FindCondition{
		Fld: fld,
		Op:  op,
	}
	var ok bool
	switch {
	case slices.Contains(bobb.IntFindOps, op):
		if condition.ValInt, ok = val.(int); !ok {
			log.Println("error - find value not of type int", val, op)
		}
	case slices.Contains(bobb.StrFindOps, op):
		if condition.ValStr, ok = val.(string); !ok {
			log.Println("error - find value not of type string", val, op)
		}
	default:
		log.Println("error - invalid Find op", op)
	}
	if len(notCondition) > 0 {
		condition.Not = notCondition[0]
	}
	conditions = append(conditions, condition)
	return conditions
}

// Sort is convenience funcs used to create/load []bobb.SortKey used by qry requests.
// First parm is the slice of sortkeys to which entry will be appended.
// If nil, a new slice will be created.
func Sort(sortKeys []bobb.SortKey, fld, dir string) []bobb.SortKey {
	if sortKeys == nil {
		sortKeys = make([]bobb.SortKey, 0, 9)
	}
	sortKey := bobb.SortKey{Fld: fld, Dir: dir}
	sortKeys = append(sortKeys, sortKey)
	return sortKeys
}
