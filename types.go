package bobb

import (
	bolt "go.etcd.io/bbolt"
)

type Request interface {
	IsUpdtReq() bool                 // true if request performs update
	Run(*bolt.Tx) (*Response, error) // executes the request
}

// FldFormat is used by MergeFlds in rec.go, typically for creating index keys.
// Strings - padded to right with spaces or truncated as needed.
// Ints - leading zeros added as needed.
type FldFormat struct {
	FldName string `json:"fldName"` // name of fld in record
	FldType string `json:"fldType"` // "string" or "int"
	Length  int    `json:"length"`  // output length of value
}

// Response type is returned by all db requests.
// Individual recs must be json.Unmarshaled into appropriate type by receiver.
type Response struct {
	Status  string    // constants in codes.go (StatusOk, StatusWarning, StatusFail)
	Msg     string    // if status is not Ok, Msg will indicate reason
	Recs    [][]byte  // for request responses with potentially more than 1 record
	Rec     []byte    // for requests that only return 1 record
	PutCnt  int       // number of records either added or replaced by Put operation
	GetCnt  int       // used for other non Put counts
	NextSeq []int     // returned by Bkt request with Operation = "nextseq"
	NextKey string    // next key in bkt after last one returned in Recs
	Errs    []BobbErr // errs occuring until req.ErrLimit hit
}

type BobbErr struct {
	ErrCode string // see Error code constants in codes.go
	Msg     string // error msg
	Key     []byte // bkt or index key depending on ErrCode
	Val     []byte // bkt or index val depending on ErrCode
}
