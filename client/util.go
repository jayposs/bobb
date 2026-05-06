package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"

	"github.com/jayposs/bobb"
)

/*
Shortcut funcs for common operations. These are not required but can be convenient.
They use the more general Run func to execute requests, and are intended to simplify common operations
by reducing the amount of code needed in the calling program.
*/

// format JSON in easy to view format
func fmtJSON(jsonContent []byte) string {
	var out bytes.Buffer
	json.Indent(&out, jsonContent, "", "  ")
	return out.String()
}

// Options when creating PutParm. See PutParm func for details.
const (
	PutAddKeySuffix     = "addkeysuffix"
	PutLogPut           = "logput"
	PutIndexingOff      = bobb.IndexingOff
	PutIndexingNoUpdate = bobb.IndexingNoUpdate
)

// PutParm is convenience func to create instance of bobb.PutParm used by Put requests.
// Options can be added by including option code(s) in options parm. See Put* consts above for option codes.
func PutParm(bktName string, recs [][]byte, requiredFlds []string, options ...string) (bobb.PutParm, error) {
	var putParm bobb.PutParm
	for _, option := range options {
		switch option {
		case PutAddKeySuffix:
			putParm.AddKeySuffix = true
		case PutLogPut:
			putParm.LogPut = true
		case PutIndexingOff:
			putParm.IndexingOption = bobb.IndexingOff
		case PutIndexingNoUpdate:
			putParm.IndexingOption = bobb.IndexingNoUpdate
		default:
			return putParm, fmt.Errorf("PutParm error - invalid option: %s", option)
		}
	}
	putParm.BktName = bktName
	putParm.Recs = recs
	putParm.RequiredFlds = requiredFlds

	return putParm, nil
}

// Put is shortcut func. Uses a single PutParm to put recs into specified bkt. See PutParm func for options.
// Options can be added by including option code in options parm. See Put* consts above for option codes.
func Put(httpClient *http.Client, bkt string, recs [][]byte, requiredFlds []string, options ...string) (*bobb.Response, error) {
	resp := new(bobb.Response)
	var err error

	var putParm bobb.PutParm
	putParm, err = PutParm(bkt, recs, requiredFlds, options...)
	if err != nil {
		resp.Status = bobb.StatusFail
		resp.Msg = "Put Failed: error creating PutParm: " + err.Error()
		return resp, err
	}
	req := bobb.PutRequest{PutParms: []bobb.PutParm{putParm}}
	resp, err = Run(httpClient, bobb.OpPut, req)

	return resp, err
}

// PutOne is shortcut func. Puts single rec into specified bkt.
// Options can be added by including option code in options parm. See Put* consts above for option codes.
func PutOne(httpClient *http.Client, bkt string, rec any, requiredFlds []string, options ...string) (*bobb.Response, error) {
	resp := new(bobb.Response)
	var err error

	var jsonRec []byte
	jsonRec, err = json.Marshal(rec)
	if err != nil {
		resp.Status = bobb.StatusFail
		resp.Msg = "PutOne Failed: error marshaling record: " + err.Error()
		return resp, err
	}
	var putParm bobb.PutParm
	putParm, err = PutParm(bkt, [][]byte{jsonRec}, requiredFlds, options...)
	if err != nil {
		resp.Status = bobb.StatusFail
		resp.Msg = "PutOne Failed: error creating PutParm: " + err.Error()
		return resp, err
	}
	req := bobb.PutRequest{PutParms: []bobb.PutParm{putParm}}
	resp, err = Run(httpClient, bobb.OpPut, req)

	return resp, err
}

// ----------------------------------------------------------------------------------

// GetOne is shortcut func. Gets rec from bkt where key=id.
// Unmarshals rec into result (pointer to type var).
func GetOne(httpClient *http.Client, bkt, id string, result any) error {
	req := bobb.GetOneRequest{BktName: bkt, Key: id}
	resp, err := Run(httpClient, bobb.OpGetOne, req)
	if resp.Status != bobb.StatusOk {
		return fmt.Errorf(resp.Msg)
	}
	if err = json.Unmarshal(resp.Rec, result); err != nil {
		return fmt.Errorf("GetOne Failed: error unmarshaling response record: %v", err)
	}
	return nil
}

func CreateBkt(httpClient *http.Client, bktName string) error {
	req := bobb.BktRequest{
		BktName:   bktName,
		Operation: "create",
	}
	resp, err := Run(httpClient, bobb.OpBkt, req)
	if resp.Status != bobb.StatusOk {
		return fmt.Errorf("CreateBkt Failed: %s", resp.Msg)
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

// GetBktList returns []string with all bucket names in db
func GetBktList(httpClient *http.Client) []string {
	req := bobb.BktRequest{
		Operation: "list",
	}
	resp, _ := Run(httpClient, bobb.OpBkt, req)
	if resp.Status != bobb.StatusOk {
		return []string{"GetBktList Failed", resp.Msg}
	}
	bktList := make([]string, len(resp.Recs))
	for i, rec := range resp.Recs {
		bktList[i] = string(rec)
	}
	return bktList
}

// GetRecCount returns number of keys in bucket.
func GetRecCount(httpClient *http.Client, bktName string) int {
	req := bobb.BktRequest{
		BktName:   bktName,
		Operation: "count",
	}
	resp, _ := Run(httpClient, bobb.OpBkt, req)
	if resp.Status != bobb.StatusOk {
		log.Println("GetRecCount Failed for bkt", bktName, ":", resp.Msg)
		return -1
	}
	return resp.GetCnt
}

// Find is convenience func used to create/load bobb.FindGroup ([]bobb.FindCondition) used by qry requests.
// First parm is the slice of conditions to which entry will be appended.
// If nil, a new slice will be created.
// To set parm notCondition, use bobb.FindNot constant in call.
// Depending on find op code, either ValInt or ValStr will be loaded with val.
func Find(conditions bobb.FindGroup, fld, op string, val any, notCondition ...bool) bobb.FindGroup {
	if conditions == nil {
		conditions = make(bobb.FindGroup, 0, 9)
	}
	condition := bobb.FindCondition{
		Fld: fld,
		Op:  op,
	}
	var ok bool
	switch {
	case op == bobb.FindInIntList:
		if condition.IntList, ok = val.([]int); !ok {
			log.Println("error - find value not of type []int", val, op)
			return nil
		}
	case op == bobb.FindInStrList:
		if condition.StrList, ok = val.([]string); !ok {
			log.Println("error - find value not of type []string", val, op)
			return nil
		}
	case slices.Contains(bobb.IntFindOps, op):
		if condition.ValInt, ok = val.(int); !ok {
			log.Println("error - find value not of type int", val, op)
			return nil
		}
	case slices.Contains(bobb.StrFindOps, op):
		if condition.ValStr, ok = val.(string); !ok {
			log.Println("error - find value not of type string", val, op)
			return nil
		}
	case !slices.Contains(bobb.AllFindOps, op): // covers FindIsNull, FindExists where no validation is needed
		log.Println("error - invalid Find op", op)
		return nil
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

// GetSeqNos returns next sequence number(s) for specified bkt as slice of strings.
// Count parm specifies how many sequence numbers to return.
// Width parm specifies width of sequence number string with leading zeros. Zero returns number in default width with no leading zeros.
func GetSeqNos(httpClient *http.Client, bktName string, count int, width int) ([]string, error) {
	req := bobb.BktRequest{
		BktName:      bktName,
		Operation:    bobb.BktNextSeq,
		NextSeqCount: count,
	}
	resp, err := Run(httpClient, bobb.OpBkt, req)
	if resp.Status != bobb.StatusOk || err != nil {
		return nil, fmt.Errorf("GetSeqNos Failed: %s - %s", resp.Msg, err.Error())
	}
	seqNos := make([]string, len(resp.NextSeq))
	for i, seqNo := range resp.NextSeq {
		seqNos[i] = fmt.Sprintf("%0*d", width, seqNo)
	}
	return seqNos, nil
}
