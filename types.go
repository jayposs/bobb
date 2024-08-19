package bobb

// FldFormat is used by MergeFlds in rec.go, typically for creating index keys.
type FldFormat struct {
	FldName string `json:"fldName"`
	FldType string `json:"fldType"` // "string, int"
	Length  int    `json:"length"`
}

// Response type is returned by all db requests.
// Individual recs must be json.Unmarshaled into appropriate type by receiver.
type Response struct {
	Status  string   `json:"status"`  // constants in codes.go (StatusOk, StatusWarning, StatusFail)
	Msg     string   `json:"msg"`     // if status is not Ok, Msg will indicate reason
	Recs    [][]byte `json:"recs"`    // for request responses with potentially more than 1 record
	Rec     []byte   `json:"rec"`     // for requests that only return 1 record
	PutCnt  int      `json:"putCnt"`  // number of records either added or replaced by Put operation
	NextSeq []int    `json:"nextSeq"` // returned by bkt request with Operation = "nextseq"
}

// BktRequest is used to create / delete bkt and get the auto incremented NextSequence number.
// For "nextseq" operations: if NextSeqCount = 0, 1 value is returned in NextSeq.
// Note - a maximum of 100 seq #'s are returned per request.
type BktRequest struct {
	BktName      string `json:"bktName"`
	Operation    string `json:"operation"`    // "create", "delete", "nextseq"
	NextSeqCount int    `json:"nextSeqCount"` // used with nextseq op to specify how many (max 100)
}

// GetRequest is used to get specific records by key.
// Keys must be string values. They will be converted to []byte by Get request.
type GetRequest struct {
	BktName string   `json:"bktName"`
	Keys    []string `json:"keys"` // keys of records to be returned
}

// GetAllRequest returns all records in bucket or records in range between Start/End keys.
// If start/end keys are specified, a cursor is used to establish a starting point and then reads sequentially.
// A subset of bkt records can be retrieved very quickly.
// Records are returned in key order.
// If StartKey == EndKey, rec key prefix must match StartKey.
type GetAllRequest struct {
	BktName  string `json:"bktName"`
	StartKey string `json:"startKey"` // if not "", keys >= this value
	EndKey   string `json:"endKey"`   // if not "", keys <= this value
	Limit    int    `json:"limit"`    // max # recs to return
}

// GetAllKeys works same as GetAll except only the key values are returned.
// Keys are returned in the Response.Recs.
// Receiving app may need to convert each key in Resp.Recs from []byte to string.
// If StartKey == EndKey, rec key prefix must match StartKey.
type GetAllKeysRequest struct {
	BktName  string `json:"bktName"`
	StartKey string `json:"startKey"` // if not "", keys >= this value
	EndKey   string `json:"endKey"`   // if not "", keys <= this value
	Limit    int    `json:"limit"`    // max # recs to return
}

// GetOneRequest is used to get a specific record by Key.
// Key must be string value. It will be converted to []byte by GetOne request.
type GetOneRequest struct {
	BktName string `json:"bktName"`
	Key     string `json:"key"` // key of record to be returned
}

// GetIndexRequest uses IndexBkt to access records in data Bkt using a secondary index.
// StartKey and EndKey are optional. These refer to keys in the index bkt.
// Uses index records where key >= StartKey and <= EndKey.
// If StartKey = "", reads from beginning. If EndKey = "" reads to end.
// Value of index record (key of data record) is used to Get record from data bkt.
// Records are returned in index key order.
// If StartKey == EndKey, rec key prefix must match StartKey.
type GetIndexRequest struct {
	BktName  string `json:"bktName"`  // where data records are located
	IndexBkt string `json:"indexBkt"` // name of bkt used as index
	StartKey string `json:"startKey"` // key in index bkt to start with
	EndKey   string `json:"endKey"`   // key in index bkt to end with
	Limit    int    `json:"limit"`    // max # recs to return
}

// PutRequest is used to add or replace records.
// If key exists, existing record is replaced otherwise record is added.
// Recs must include the field to be used as the key (unique id).
// Recs are the json marshaled value of the record type.
// RequiredFlds (optional), fld names that must be included in recs.
// Only top level fld names allowed.
type PutRequest struct {
	BktName      string   `json:"bktName"`
	KeyField     string   `json:"keyField"`     // field in Rec containing value to be used as key
	Recs         [][]byte `json:"recs"`         // records to be added or replaced in db
	RequiredFlds []string `json:"requiredFlds"` // recs must include these fields (optional)
}

// PutBktsRequest is used to add or replace records in 2 bkts with 1 transaction.
// For example: adding new order and order items.
// If either bkt update fails, complete transaction is rolled back.
// RequiredFlds (optional), fld names that must be included in recs.
type PutBktsRequest struct {
	BktName       string   `json:"bktName"`
	KeyField      string   `json:"keyField"`     // field in Rec containing value to be used as key
	Recs          [][]byte `json:"recs"`         // records to be added or replaced in bkt 1
	RequiredFlds  []string `json:"requiredFlds"` // recs must include these fields (optional)
	Bkt2Name      string   `json:"bkt2Name"`
	Recs2         [][]byte `json:"recs2"`         // records to be added or replaced in bkt 2
	RequiredFlds2 []string `json:"requiredFlds2"` // recs must include these fields (optional)
}

// PutOneRequest is used to add or replace a single record.
// Rec must include the field to be used as the key (unique id).
// Rec is the json marshaled value of the record type.
// RequiredFlds (optional), fld names that must be included in recs.
type PutOneRequest struct {
	BktName      string   `json:"bktName"`
	KeyField     string   `json:"keyField"`     // field in Rec containing value to be used as key
	Rec          []byte   `json:"rec"`          // record to be added or replaced in db
	RequiredFlds []string `json:"requiredFlds"` // recs must include these fields (optional)
}

// IndexKeyVal type is used by PutIndexRequest.
// Key is typically created from value(s) in data record (must be made unique).
// Val is key of record in data bkt.
// If OldKey not empty, it will be deleted. No problem if it does not exist.
type IndexKeyVal struct {
	Key    string `json:"key"`
	Val    string `json:"val"`
	OldKey string `json:"oldKey"` // used when index rec already exists for data key
}

// PutIndexRequest is used to add or replace records in an index bkt.
// Typically the index key value should not already exist in the index bkt.
// If 2 keys are associated with the same value, that is likely a problem.
// The developer is responsible for maintaining index bkts.
// NOTE - bbolt db is very fast at reading records.
// A secondary index may not be needed, even if requests require scanning the entire bkt.
// Also consider if the primary data key can be designed to increase efficiency.
// For large semi-static bkts, rebuild the entire index at scheduled times.
// An example would be a history bkt that is only updated daily.
// Use the Indexloader program for this purpose.
type PutIndexRequest struct {
	BktName string        `json:"bktName"`
	Indexes []IndexKeyVal `json:"indexes"`
}

// DeleteRequest is used to delete specific records by Key.
// Keys not found are ignored.
type DeleteRequest struct {
	BktName string   `json:"bktName"`
	Keys    []string `json:"keys"` // keys of records to be deleted
}

// SortKey is used by QryRequest to sort results.
// Value of the record Fld is extracted using fastjson.
// The Dir(ection) specifies both direction (asc/desc) and value type (str/int).
// For example: DescInt is descending direction and fld value is of int type.
// Constants are defined in codes.go: AscStr, DescStr, AscInt, DescInt.
// Only fields of type string or int are currently supported.
type SortKey struct {
	Fld string `json:"fld"` // name of field
	Dir string `json:"dir"` // direction (asc/desc) and field type (str/int)
}

// FindCondition is used by QryRequest to define select criteria.
// Also used by recFind() in rec.go.
// Value of the record Fld is extracted using fastjson.
// The Op code specifies both operation and value type (str/int).
// Op code constants are defined in codes.go. For example,
// FindStartsWith: includes recs where rec value starts with ValStr.
// Only fields of type string or int are currently supported.
// If Not is true, recs that meet find condition are excluded.
type FindCondition struct {
	Fld    string // field containing compare value
	Op     string // defines match criteria
	ValStr string // for string ops
	ValInt int    // for int Ops
	Not    bool   // only include records that do not meet condition
}

// QryRequest is used to filter and sort records.
// Included recs must meet all FindConditions.
// If no FindConditions, all recs included.
// Results are returned in sorted order, if specified.
// If Start/End keys are specified, only recs inside that range are compared.
type QryRequest struct {
	BktName        string          `json:"bktName"`
	FindConditions []FindCondition `json:"findConditions"`
	SortKeys       []SortKey       `json:"sortKeys"`
	StartKey       string          `json:"startKey"`
	EndKey         string          `json:"endKey"`
	Limit          int             `json:"limit"`
}

// QryIndexRequest works same as QryRequest but uses an index to speed processing.
// The Start/End keys refer to keys in the IndexBkt.
// Results are in index key order unless sorted.
// For example, using an index with date based keys, the qry needs only scan records in a date range.
type QryIndexRequest struct {
	BktName        string          `json:"bktName"`
	IndexBkt       string          `json:"indexBkt"`
	FindConditions []FindCondition `json:"findConditions"`
	SortKeys       []SortKey       `json:"sortKeys"`
	StartKey       string          `json:"startKey"`
	EndKey         string          `json:"endKey"`
	Limit          int             `json:"limit"`
}

// Export writes bkt records to a file as formatted json.
type ExportRequest struct {
	BktName  string `json:"bktName"`
	StartKey string `json:"startKey"` // if not "", keys >= this value
	EndKey   string `json:"endKey"`   // if not "", keys <= this value
	Limit    int    `json:"limit"`    // max # recs to write
	FilePath string `json:"filePath"` // where export file is written
}

// CopyDB copies the open db to another file. Does not block other operations.
type CopyDBRequest struct {
	FilePath string `json:"filePath"`
}
