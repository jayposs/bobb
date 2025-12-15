package bobb

import (
	"fmt"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// ReadLoop type provides functionality for reading bucket records sequentially.
// Optional Index, StartKey, EndKey
type ReadLoop struct {
	Bkt         *bolt.Bucket // data bkt
	Index       *bolt.Bucket // index bkt
	Csr         *bolt.Cursor // if index set, index csr, else data bkt csr
	StartKey    string       // begin loop with 1st key >= StartKey
	EndKey      string       // end loop with 1st key > EndKey
	MatchPrefix bool         // if StartKey == EndKey, rec key prefix must match StartKey
	UsingIndex  bool         // indicates if index is being used
	NextKey     []byte       // used for resp.NextKey when range-end or limit hit
	Limit       int          // results limit
	Count       int          // Count equal Limit triggers loop end, Count updated by caller
}

// Start method sets the cursor and returns 1st key/value pair.
// If UsingIndex, Csr will be set using index bkt.
// If startKey == "", loop starts with 1st key.
// If endKey == "", loop ends with last key.
func (loop *ReadLoop) Start(startKey, endKey string, limit int) (k, v []byte, bErr *BobbErr) {
	if loop.UsingIndex {
		loop.Csr = loop.Index.Cursor()
	} else {
		loop.Csr = loop.Bkt.Cursor()
	}
	if startKey == "" {
		k, v = loop.Csr.First()
	} else {
		k, v = loop.Csr.Seek([]byte(startKey))
	}
	if k == nil {
		return
	}
	loop.StartKey = startKey
	loop.EndKey = endKey
	loop.Limit = limit

	// if StartKey == EndKey, key must begin with StartKey
	if loop.StartKey != "" && loop.StartKey == loop.EndKey {
		loop.MatchPrefix = true
	}
	if loop.MatchPrefix {
		if !strings.HasPrefix(string(k), loop.StartKey) {
			k, v = nil, nil
			return
		}
	} else if loop.EndKey != "" && string(k) > loop.EndKey {
		k, v = nil, nil
		return
	}
	if loop.UsingIndex {
		dataVal := loop.Bkt.Get(v) // v is value of index which is key of data record
		if dataVal == nil {
			emsg := fmt.Sprintf("index val %s not key in data bkt", string(v))
			bErr = e(ErrIndexRef, emsg, k, v)
			return
		}
		v = dataVal
		return
	}
	return
}

// Next returns next key/value pair or nil/nil if loop ended.
// If UsingIndex, key is index key. Value is always from data bkt.
// If k is outside of range, loop.NextKey loaded with k.
func (loop *ReadLoop) Next() (k, v []byte, bErr *BobbErr) {
	k, v = loop.Csr.Next()
	if k == nil {
		return
	}
	if loop.Limit != 0 && loop.Count >= loop.Limit {
		loop.NextKey = k
		k, v = nil, nil
		return
	}
	if loop.MatchPrefix {
		if !strings.HasPrefix(string(k), loop.StartKey) {
			loop.NextKey = k
			k, v = nil, nil
			return
		}
	} else if loop.EndKey != "" && string(k) > loop.EndKey {
		loop.NextKey = k
		k, v = nil, nil
		return
	}
	if loop.UsingIndex {
		dataVal := loop.Bkt.Get(v) // v is value of index which is key of data record
		if dataVal == nil {
			emsg := fmt.Sprintf("index val %s not key in data bkt", string(v))
			bErr = e(ErrIndexRef, emsg, k, v)
			return
		}
		v = dataVal
	}
	return
}

// Create and return new instance of ReadLoop.
// Parm bkt is pointer to data bucket.
// Parm index is pointer to index bucket. If nil, no index.
func NewReadLoop(bkt, index *bolt.Bucket) *ReadLoop {
	readLoop := ReadLoop{
		Bkt: bkt,
	}
	if index != nil {
		readLoop.Index = index
		readLoop.UsingIndex = true
	}
	return &readLoop
}
