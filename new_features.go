//go:build exclude

package bobb

import (
	"log"

	bolt "go.etcd.io/bbolt"
)

type Join struct {
	JoinBkt string            // where join value comes from
	JoinFld string            // field containing foreign key
	FromTo  map[string]string // key is source fld in join bkt, val is dest fld in primary bkt
}

type JoinRequest struct {
	BktName string
	Keys    []string
	Joins   []Join
}

// Join loads values from join bkts into bkt fields.
// *** NOT CODE COMPLETE ******************
func Join(tx *bolt.Tx, req *JoinRequest) *Response {

	resp := new(Response)
	resp.Status = StatusOk // may be changed to Warning below if key not found

	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp
	}
	joinBkts := make([]*bolt.Bucket, len(req.Joins))
	for i, join := range req.Joins {
		if jBkt := tx.Bucket([]byte(join.BktName)); jBkt == nil {
			resp.Status = StatusFail
			resp.Msg = "invalid join bkt name"
			return resp
		} else {
			joinBkts[i] = jBkt
		}
	}
	resp.Recs = make([][]byte, 0, len(req.Keys))

	for _, key := range req.Keys {
		v := bkt.Get([]byte(key))
		if v == nil {
			log.Println("key not found", key)
			resp.Status = StatusWarning
			resp.Msg = "not found"
			continue // NOTE - THIS BEHAVIOUR MAY NOT BE APPROPRIATE FOR ALL SITUATIONS
		}
		for i, join := range req.Joins {
			for 
			joinRec := joinBkts[i].Get
		}
		resp.Recs = append(resp.Recs, v)
	}
	return resp
}

type JoinAllRequest struct {
	BktName  string
	StartKey string
	EndKey   string
	Joins    []Join
}
