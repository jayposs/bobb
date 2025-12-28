## Bobb API 

### Contents
* [Client Programs](#client-programs)
* [Codes](#codes)
* [Requests](#requests)
* [Keys](#keys)
* [Start/End Keys](#start-end-keys)
* [ShortCut Funcs](#shortcut-funcs)
* [Data Conversion Funcs](#data-conversion-funcs)
* [Qry FindCondition and SortKey](#qry-findcondition-and-sortkey)
* [Qry Errs and ErrLimit](#qry-errs-and-errlimit)
* [Joins](#joins) 
* [Indexes](#indexes)
* [Bucket Operations](#bucket-operations)
* [Misc Requests](#misc-requests)
* [Experimental Requests](#experimental-requests)
* [Data Type Structs](#data-type-structs)

-------------------------------------------------------------------------------------------------------
See demo/demo.go for examples of all request types.

See bolt_direct for example pgm that interacts directly with db (not through server).  

**To view standard Go documentation:**
* repo must be cloned to local
* go to bobb directory
* enter: godoc -http :6060
* using browser: open localhost:6060
* under Packages, select Third party

### Client Programs

See install.txt for example client program setup.  

Client pgms must set the BaseURL var defined in client/client.go.
A good practice would be to set this value using a common external client settings file.

Program demo/demo.go demonstrates how to send/receive requests to bobb_server.

Include the following imports:
```
import "github.com/jayposs/bobb"
import bo "github.com/jayposs/bobb/client"
```
### Codes

For a complete list of code constants, see codes.go. Generally all codes of a particular type begin with the same prefix. For example, all FindCondition Op codes begin with "Find". With modern IDE's getting a list of options is very convenient.

### Requests

Request types are located in requests_get.go, requests_qry.go, requests_put.go, ... with full documentation for each request.
* Read Requests: Get, GetOne, GetAll, GetAllKeys, Qry
* Update Requests: Put, PutOne, PutIndex, PutBkts, Delete
* Other Requests: Bkt, Export, CopyDB

All requests follow this pattern:
```
req := bobb.GetAll{
    BktName:  "stuff",
    IndexBkt: "stuff_date_index",
    StartKey: "2024-01-01 0000000",
    EndKey:   "2024-12-31 9999999",
}
resp, err := bo.Run(httpClient, bobb.OpGetAll, req)  
```
Common Request attributes. 
* BktName  - data bkt, required for most requests
* IndexBkt - optional, StartKey/EndKey will refer to index keys
* StartKey - reads begin 
* EndKey - reads end
* Limit - max recs returned (0 for no limit)

QryRequest adds these attributes:
* FindConditions - selection criteria
* SortKeys - sort order
* JoinsBeforeFind - related values added before find step
* JoinsAfterFind - related values added only to result recs

The Run func is located in client/client.go.  
It forms the URL, loads request body, sends http request, and receives response from server.  
All requests use POST method and have the same response type, located in types.go:    
```
type Response struct {
	Status  string    `json:"status"`  // constants in codes.go (StatusOk, StatusWarning, StatusFail)
	Msg     string    `json:"msg"`     // if status is not Ok, Msg will indicate reason
	Recs    [][]byte  `json:"recs"`    // for request responses with potentially more than 1 record
	Rec     []byte    `json:"rec"`     // for requests that only return 1 record
	PutCnt  int       `json:"putCnt"`  // number of records either added or replaced by Put operation
	NextSeq []int     `json:"nextSeq"` // returned by Bkt request with Operation = "nextseq"
	NextKey string    `json:"nextKey"` // next key in bkt after last one returned in Recs
	Errs    []BobbErr `json:"errs"`    // errs occuring until req.ErrLimit hit
}
```
**Request Types**
* Get - Keys attribute identifies specific recs to be returned in Response.Recs
* GetOne - Key attribute identifies specific rec to be returned, shortcut func - bo.GetOne()
* GetAll - returns all recs in bkt or all in key range in key order
* GetAllKeys - returns keys rather than values in Response.Recs
* Qry - returns recs in range matching FindConditions in SortKeys order
* Put - adds or replaces records
* PutOne - adds or replaces a single record, shortcut func - bo.PutOne()
    * includes option to auto write entry to log bkt("bktname_log"), set LogPut parm = true
    * timestamp is appended to key, providing ability to see rec values at point in time    
* PutIndex - adds or replaces index records
* PutBkts - puts records to 2 bkts in single transaction
* Delete - deletes 1 or more records by key
* Bkt - bucket operations: "create", "delete", "nextseq", "list", "count", see shortcut funcs
* Export - export range of records to file in formatted json
* CopyDB - copies open database file to another location  

**Bulk Loads**  
When loading a large number of records, group records into batches. Load each batch
with a separate Go routine. See bulkload/bulkload.go for template pgm.
  
**NOTE** - Put operations will create the bkt if it does not already exist  

-------------------------------------------------------------------------------------------------------
### Keys

Keys are unique inside a bucket. Bobb requires keys be string values.  
Put transactions either add or replace a record depending on existence of the key.  
  
The record key value must also be contained in a record field.  
Normal convention is to use field name "id".  

If creating a key from data values:  
1. if data value, used in key, may change, consider how to deal with complications   
2. see MergeFlds func in rec.go for merging multiple values together from existing record  

**WARNING**  
When using MergeFlds to create a key, negative integers may not produce the desired value.  
Larger absolute values will sort after smaller absolute values. For example -00010 will sort before -00100.

-------------------------------------------------------------------------------------------------------
### Start End Keys

Records are read either directly by key or sequentially in key order.  
If using an index, the order of index keys is used.  
Requests can use Start and End keys to specify a range of records.  
Seeking to a Start key and reading from that point is very fast.
  
Range starts with first rec where key is >= Start key.  
Range ends with last rec where key is <= End key.  

NOTE on string comparison rules (all keys are strings):  

> The comparison proceeds until it finds the first byte where the two strings differ. The string with the smaller byte value at that position is considered the smaller string overall.  
> If one string is a prefix of the other (meaning all its bytes match the beginning of the other string), the shorter, prefix string is considered smaller than the longer string. The longer string is considered the larger one. 

**Using a key prefix**   
To use a key prefix, set StartKey and EndKey to the prefix value. All records where key prefix matches are in range.
  
-------------------------------------------------------------------------------------------------------
### Shortcut Funcs

The following funcs are located in client/util.go. They create the request and call Run func (if needed) in a single step.  
  
* GetOne(httpClient, bktName, recId, target) - retrieves a record by id and unmarshals into target  
* PutOne(httpClient, bktName, idFld, rec any) - marshals the db record and executes PutOne request  
* CreateBkt - creates a new bucket  
* DeleteBkt - deletes existing bucket  
* GetBktList(httpClient) - returns all bucket names in open db as []string
* GetRecCount(httpClient, bktName) - returns number of keys in bucket 
* Find(conditions, fld, op, val, ) - adds condition to []bobb.FindRequests or creates new slice, used by Qry requests  
* Sort(sortKeys, fld, dir) - adds sortkey to []bobb.SortKeys or creates new slice, used by Qry requests   

-------------------------------------------------------------------------------------------------------
### Data Conversion Funcs

The following funcs are located in client/data_conversion.go.  
They use generics to convert between json recs ( [][]byte ) and maps/slices of db record struct types.  
  
JsonToMap - creates map of db recs from slice of json recs, key is db record key  
JsonToSlice - creates slice of db recs from slice of json recs  
SliceToJson - creates slice of json recs from slice of db recs  
MapToJson - creates slice of json recs from map of db recs  
  
To use these funcs the record type must implement DBRec interface. See demo/datatypes.go for example.  
```
type DBRec interface {
	RecId() string   // returns field(typically id fld) value containing record key
}  
```
-------------------------------------------------------------------------------------------------------
### Qry FindCondition and SortKey

**Note -** Find and Sort operations only support string and int values.  

FindConditions specify the criteria for selecting records. The Op(eration) defines how values are compared and the value type, either string or int. For example, FindEquals uses ValInt, whereas FindMatches uses ValStr. All Op codes are defined in codes.go. IDE's like VSCode display all options when bobb.Find is entered. The StrOption determines how strings are compared. Default is lowercase, "plain" also removes non alpha-numeric, "asis", uses values as they are. The StrOption is applied to both the find value and record value. Records must meet all find conditions.  
  
Qry requests also include FindOrConditions. Records meeting either all Find or all FindOr Conditions will be selected.

The UseDefault attribute determines what happens when Fld either does not exist in a record or has a value of null.  
The default value is DefaultAlways. From codes.go:
```
const (
	DefaultAlways   = "always"   // on not found or null use zero value
	DefaultNever    = "never"    // return error if notfound or null
	DefaultIsNull   = "isnull"   // if null, use zero value, not found is error
	DefaultNotFound = "notfound" // if not found, use zero value, null is error
)
```
```
type FindCondition struct {
	Fld        string   // field containing compare value
	Op         string   // defines match operation and value type
	ValStr     string   // for string ops, this value also converted based on StrOption
	ValInt     int      // for int Ops
	StrList    []string // used by op FindInStrList
	IntList    []int    // used by op FindInIntList
	Not        bool     // exclude records that meet condition
	UseDefault string   // controls what default value is used, see Default* codes in codes.go
	StrOption  string   // controls string conversion, see Str* codes in codes.go, default StrLowerCase
}
```
SortKeys specify the sort order of results. The Dir attribute defines both direction and value type. For example, SortAscInt and SortDescStr. See codes.go for all sort codes. If not specified, results are returned in key/index order.
```
type SortKey struct {
	Fld string  // name of field
	Dir string  // direction (asc/desc) and field type (str/int)
    UseDefault string	// controls what default value is used, see Default* codes in codes.go
}
```  
-------------------------------------------------------------------------------------------------------
### Joins

Used in qry requests to pull values from other bucket(s) into response records.  
If there is a problem with the join, no value will be added to result record.
When rec is unmarshalled on client, fields will auto load with zero value.

A join set is a slice of Join instances:
```
type Join struct {
	JoinBkt    string // name of related bkt where value is pulled from
	JoinFld    string // fld in primary rec containing key value of join rec
	FromFld    string // fld in join rec where value comes from
	ToFld      string // fld in primary rec where value is loaded
	UseDefault bool   // if true, on join problem, don't error
}
```
Join entries for the same JoinBkt should be loaded sequentially in slice.
The first join operation for a JoinBkt performs most of the work.
If the next join is for the same bkt, the join value will be extracted from the already parsed join record.

The record type definition of the primary bkt in the request should include fields for the joined values.  
By using the json tag, "omitempty" option, these fields are not stored in the database.  
Example:
```
type Product struct {
    Id            string `json:"id"`  
    ProductName   string `json:"productName"`
}
type OrderItem struct {
    Id          string `json:"id"`
    OrderId     string `json:"orderId"`
    ProductId   string `json:"productId"`
    Qty         int    `json:"qty"`    
    ProductName string `json:"product_productName,omitempty"`  // loaded using join
}
req.JoinsAfterFind = []bobb.Join {
    { JoinBkt: "product",
      JoinFld: "productId",
      FromFld: "productName",
      ToFld:   "product_productName"
    },
}    
```
Qry requests have 2 fields for join parameters, JoinsBeforeFind and JoinsAfterFind.
If the joined value(s) are needed for the find step, include in JoinsBeforeFind. 
Otherwise place joins in JoinsAfterFind. If used in the find step, all records in range will be joined, taking more processing time.  
  
All joined values can be used in the sort step and are loaded to response recs.  

If the join record or fld does not exist, a BobbError will be returned, unless UseDefault is true in which case nothing is done and no error returned. The join fld will simply not exist in the result record.

-------------------------------------------------------------------------------------------------------
### Qry Errs and ErrLimit

The Response type includes Errs which is a slice of bobb.BobbErr. This feature provides detail information about each processing error encountered. Qry ErrLimit sets a limit for how many errs it takes to trigger an end of the request. FindCondition and SortKey "UseDefault" setting impacts when errs are created. For example: FindCondition.UseDefault == DefaultNever and a records find Fld value is null, an entry would be added to Response.Errs. If ErrLimit is 0, request would end on 1st err. If ErrLimit is 100, the request would continue to process until 101 errs were encountered. An ErrLimit of -1 indicates to use the MaxErrs value set in bobb_settings.json.

```
type BobbErr struct {
	ErrCode string // see Error code constants in codes.go
	Msg     string // error msg
	Key     []byte // bkt or index key depending on ErrCode
	Val     []byte // bkt or index val depending on ErrCode
}
```

-------------------------------------------------------------------------------------------------------
### Indexes

Use when primary key does not provide efficient way to access subset of records. Developer is responsible for maintaining index buckets.  
  
Key - string, typically data value(s) merged together from data rec, made unique  
Value - string, key of data rec in data bucket  
  
See indexloader/indexloader.go, index_settings.json for example pgm to bulk load all index recs for bkt.  

See indexrecs/indexrecs.go for example pgm to load index recs for specific data recs.
  
Func MergeFlds in rec.go can be used to create keys composed of multiple fld values.  
  
Example:
```
    data bkt - "inquiry"
    data key - "b176-83"
    data val - {id:"b176-83", timestamp: "2021-03-23 08:17:44", msg: "what up", ...}

    index bkt - "inquiry_timestamp_index"
    index key - "2021-03-23 08:17:44 b176-83" (suffix - any value to make unique)
    index val - "b176-83"

    PutIndexRequest{
        BktName: "inquiry_timestamp_index",
		Indexes: []bobb.IndexKeyVal{
            {Key: "2021-03-23 08:17:44 b176-83", Val:"b176-83"},
        },
	}
    // return all inquiry records with timestamp in 1st quarter of 2021
    GetAllRequest{
		BktName:  "inquiry",
		IndexBkt: "inquiry_timestamp_index,
		StartKey: "2021-01-01 00:00:00",
		EndKey:   "2021-03-31 99:99:99",
	}
```
Records returned in index order.    

BEWARE OF CHANGING INDEX KEY, OLD INDEX RECORD MUST BE DELETED 

Use IndexKeyVal.OldKey if a key may already exist in the index for data record.
For example, a data value changes that is part of the index key.
The index record with key = OldKey will be deleted. Not an error, if OldKey does not exist.

In some cases rebuilding the complete index bucket may be appropriate.
See indexloader/indexloader.go for an example program.

NOTE - if the Start/End keys include a large percentage of all the keys,
it may be faster to scan the complete data bkt, rather than using an index.

-------------------------------------------------------------------------------------------------------
### Bucket Operations

BktRequest - create, delete, get next sequence #, list, count  
- Operation field specifies the action
	- see codes.go for Bkt* constants
    - BktCreate to create bkt, BktDelete to delete bkt, BktNextSeq for seq numbers, BktList to get all bkt names,
    BktCount to get number of keys in bkt
- Bolt provides a NextSequence feature which returns an auto incrementing integer for each bkt
- NextSeqCount field specifies how many sequence numbers to return (max 100) 
    - A NextSeqCount of 20 will return the next 20 numbers in order  
    - If not specified (NextSeqCount = 0), 1 value will be returned
    - These numbers will not be reused for this bkt  
- To get list of bucket names in []string, use GetBktList() func in client/util.go
    - ex. bktList := bo.GetBktList(httpClient) 
- To get count of keys in bucket, use GetRecCount() func in client/util.go
    - ex recCount := bo.GetRecCount(httpClient, "bktName")         

-------------------------------------------------------------------------------------------------------
### Misc Requests

ExportRequest - writes bkt recs to a file as formatted json  
CopyDBRequest - copies open db to designated filepath  

Operations requested via direct http request (using curl or browser)  
See scripts folder.  
curl http://localhost:8000/down - shuts server down, pausing 10 secs to allow any running requests to complete (new requests are blocked)  
curl http://localhost:8000/traceon - turns trace feature on   
curl http://localhost:8000/traceoff - turns trace feature off  

Trace feature  
- Calls to Trace func are placed in strategic points in the server program  
- For example: begin and end of qry sort process  
- Log entries are prefixed with timestamp in microseconds, providing info on performance  
- If traceon, calls to Trace func will generate output to the server log file  
- If traceoff, calls to Trace func will not generate output  
- Tracing can be set(on/off) via bobb_settings.json at startup or with http request (see above)  

-------------------------------------------------------------------------------------------------------
### Experimental Requests  

See requests_experimental.go for these types.
They are included in bobb_server.go and demo.go.

* GetValues - returns specific values rather than whole records
* SearchKeys - searches the key values rather than data values (works with data and index bkts)

-------------------------------------------------------------------------------------------------------
### Data Type Structs  

Structs that contain a db record structure must include RecId() method which returns the value of the field containing record key, typically from the field named "id".
