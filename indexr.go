package bobb

import (
	"fmt"
	"strconv"

	"github.com/valyala/fastjson"
	bolt "go.etcd.io/bbolt"
)

// Indexr is used to perform indexing (add/change index rec) for a data bkt based on an IndexSetting (see requests_index.go).
// Indexrs are created and run by PutRequest, see requests_put.go.
type Indexr struct {
	IndexBkt         *bolt.Bucket // pointer to index bkt where index entries are stored
	IndexBktName     string       // name of index bkt, used for error msgs
	IndexInvertedBkt *bolt.Bucket // maps data key to index key, so easy to find old index entry for data key
	KeyFlds          []FldFormat  // defines how index key, is constructed
	FldSeparator     string       // separator used in merged field values
	KeySuffixFormat  string       // using IndexBkt nextSeq#, formatted with leading zeros
	SkipOnErr        bool         // if true, on error skip writing index entry and don't fail PutRequest
}

// Run performs indexing for a data key and record by adding/updating index entry in IndexBkt and IndexInvertedBkt based on Indexr settings.
func (indexr *Indexr) Run(tx *bolt.Tx, dataKey []byte, parsedRec *fastjson.Value, indexingOption string) error {

	// note - for IndexingOption of IndexingNoUpdate, we do not check for existing index entry for this data key
	if indexingOption == IndexingNormal { // delete old index entry if exists for this data key
		oldIndexKey := indexr.IndexInvertedBkt.Get(dataKey)
		if oldIndexKey != nil {
			indexr.IndexBkt.Delete(oldIndexKey)
		}
	}
	// add new index entry to IndexBkt and IndexInvertedBkt
	indexKey, err := MergeFlds(parsedRec, indexr.KeyFlds, indexr.FldSeparator) // ex. if KeyFlds are Fld1 and Fld2, indexKey will be "val1|val2"
	if err != nil {
		if indexr.SkipOnErr {
			// consider logging this error to log bkt
			return nil
		}
		return fmt.Errorf("error merging field values for %s index key for data key %s: %v", indexr.IndexBktName, string(dataKey), err)
	}
	if indexr.KeySuffixFormat != "" { // add suffix from IndexBkt NextSequence#
		seqNo, err := indexr.IndexBkt.NextSequence()
		if err != nil {
			return fmt.Errorf("error getting NextSequence for index bkt %s: %s", indexr.IndexBktName, err.Error())
		}
		suffix := fmt.Sprintf(indexr.KeySuffixFormat, seqNo)
		if indexKey == "" { // indexKey will just be the seqNo, so no divider added
			indexKey = suffix
		} else {
			indexKey = indexKey + indexr.FldSeparator + suffix
		}
	}
	if indexKey == "" {
		return fmt.Errorf("index_setting KeyFlds result in empty index key for data key %s", string(dataKey))
	}
	err = indexr.IndexBkt.Put([]byte(indexKey), dataKey)
	if err != nil {
		return fmt.Errorf("IndexBkt Put failed, index key %s, data key %s, %s", string(indexKey), string(dataKey), err.Error())
	}
	err = indexr.IndexInvertedBkt.Put(dataKey, []byte(indexKey))
	if err != nil {
		return fmt.Errorf("IndexInvertedBkt Put failed, data key %s, index key %s, %s", string(dataKey), string(indexKey), err.Error())
	}
	return nil
}

func NewIndxr(tx *bolt.Tx, setting *IndexSetting) (*Indexr, error) {
	indexBkt, err := tx.CreateBucketIfNotExists([]byte(setting.IndexBkt))
	if err != nil {
		return nil, fmt.Errorf("open/create index bkt %s failed: %v", setting.IndexBkt, err)
	}
	indexInvertedBktName := setting.IndexBkt + "_inverted"
	indexInvertedBkt, err := tx.CreateBucketIfNotExists([]byte(indexInvertedBktName))
	if err != nil {
		return nil, fmt.Errorf("open/create inverted index bkt %s failed: %v", indexInvertedBktName, err)
	}
	var suffixFormat string
	if setting.KeySuffixWidth > 0 {
		suffixFormat = "%0" + strconv.Itoa(setting.KeySuffixWidth) + "d"
	}
	return &Indexr{
		IndexBkt:         indexBkt,             // pointer to index bkt where index entries are stored
		IndexBktName:     setting.IndexBkt,     // used for error msgs
		IndexInvertedBkt: indexInvertedBkt,     // maps data key to index key, so easy to find old index entry for data key
		KeyFlds:          setting.KeyFlds,      // defines how index key is constructed
		FldSeparator:     setting.FldSeparator, // separator used in merged field values
		KeySuffixFormat:  suffixFormat,         // using IndexBkt nextSeq#, formatted with leading zeros
		SkipOnErr:        setting.SkipOnErr,    // if true, on error skip writing index entry and don't fail PutRequest
	}, nil
}
