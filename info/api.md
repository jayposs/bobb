# BOBB API 

### Contents
* [Requests](#requests)
* [Keys](#keys)
* [Start/End Keys](#start-end-keys)
* [ShortCut Funcs](#shortcut-funcs)
* [Data Conversion Funcs](#data-conversion-funcs)
* [Get Requests](#get-requests)
* [Put Requests](#put-requests)
* [Qry Requests](#qry-requests)
* [Indexes](#indexes)
* [Bucket Operations](#bucket-operations)
* [Misc Operations](#misc-operations)
* [Experimental Requests](#experimental-requests)


-------------------------------------------------------------------------------------------------------
### REQUESTS

See demo/demo.go for examples of all request types.

Request types are located in types.go with full documentation for each request.
* Get, GetOne, GetAll, GetIndex, GetAllKeys
* Put, PutOne, PutIndex, PutBkts, Delete
* Qry, QryIndex
* Bkt, Export, CopyDB

All requests follow this pattern:
```
    import "bobb"
    import bo "bobb/client"
    ....
    req := bobb.GetAllRequest{
            BktName:  "stuff",
            StartKey: "2024-01-01 0000000",
            EndKey:   "2024-12-31 9999999",
        }
    resp, err := bo.Run(httpClient, bobb.OpGetAll, req)  
```
The Run func is located in client/client.go.  
It forms the URL, loads request body, sends http request, and receives response from server.  
All requests use POST method.  
  
All requests have the same response type, also located in types.go.    
```
    type Response struct {
        Status  string   `json:"status"`  // constants in codes.go (StatusOk, StatusWarning, StatusFail)
        Msg     string   `json:"msg"`     // if status is not Ok, Msg will indicate reason
        Recs    [][]byte `json:"recs"`    // for request responses with potentially more than 1 record
        Rec     []byte   `json:"rec"`     // for requests that only return 1 record
        PutCnt  int      `json:"putCnt"`  // number of records either added or replaced by Put operation
        NextSeq []int    `json:"nextSeq"` // returned by bkt request with Operation = "nextseq"
        NextKey string   `json:"nextKey"` // next key in bkt after last one returned in Recs
    }
```
-------------------------------------------------------------------------------------------------------
### KEYS

Keys are unique inside a bucket. Bobb requires keys be string values.  
Put transactions either add or replace a record depending on existence of the key.  
  
The record key value must also be contained in a field.  
I normally have a field called "id" that is the record key.  

If creating a key from data values:  
1. if data value, used in key, may change, consider how to deal with complications   
2. see MergeFlds func in rec.go for merging multiple values together  

-------------------------------------------------------------------------------------------------------
### START END KEYS

Records are read sequentially in key order.  
Many requests use Start and End keys to specify a range of records.  
Seeking to a Start key and reading from that point is very fast.
  
Result starts with first rec where key is >= Start key.  
Result ends with last rec where key is <= End key.  

**Using a key prefix**   
To use a key prefix, set EndKey = StartKey. All records where key prefix matches StartKey are returned.
  
-------------------------------------------------------------------------------------------------------
### SHORTCUT FUNCS 

The following funcs are located in client/util.go. They create the request and call Run func in a single step.  
  
GetOne - retrieves a record by id and unmarshals into target  
PutOne - marshals the db record and executes PutOne request  
CreateBkt - creates a new bucket  
DeleteBkt - deletes existing bucket  
  
Find - used to build []bobb.FindRequests used by Qry requests  
Sort - used to build []bobb.SortKeys used by Qry requests  

-------------------------------------------------------------------------------------------------------
### DATA CONVERSION FUNCS

The following funcs are located in client/data_conversion.go.  
They use generics to convert between json recs ( [][]byte ) and maps/slices of db record struct types.  
  
JsonToMap - creates map of db recs from slice of json recs, key is db record key  
JsonToSlice - creates slice of db recs from slice of json recs  
SliceToJson - creates slice of json recs from slice of db recs  
MapToJson - creates slice of json recs from map of db recs  
  
To use these funcs the record type must implement DBRec interface. See demodata/datatypes.go for example.  
```
type DBRec interface {
	RecId() string   // returns field(typically id fld) value containing record key
}  
```
-------------------------------------------------------------------------------------------------------
### GET REQUESTS

Results are returned in Response.Recs or Response.Rec.  
Response.Recs can be converted from json recs to db recs using bo.JsonToMap and bo.JsonToSlice.  
See types.go for full documentation on each request type.  

GetRequest - returns recs for specific keys in Response.Recs  
GetOneRequest - returns a single record by key in Response.Rec  
GetAllRequest - returns all records or records between Start and End keys  
_ If only Start key is set, returns all records from Start key to end of bkt  
_ If only End key is set, returns all records from 1st key to End key  
GetIndexRequest - uses Start/End keys in an index bkt to specify the range of records to read  
GetAllKeysRequest - works like GetAllRequest except the key values are returned in Response.Recs  

-------------------------------------------------------------------------------------------------------
### PUT REQUESTS

Records stored in struct types can be converted to json recs using bo.SliceToJson and bo.MapToJson. 
See types.go for full documentation on each request type.  

Put requests either add or replace records depending on existence of key in bucket.  
Updates are done inside a bolt transaction. On error, updates are rolled back.  

Bobb (not bolt) requires key values be strings.  
PutRequest KeyField parm specifies the field in the record to be used as the key.  
The value of the key must be included as a field in the record.  

RequiredFlds is optional list of fields that must be included in each rec.  
Including a subset of all the record fields is fine.  
Only top level fields are allowed. Fields in embedded objects are not currently supported.  
Note - fastjson does support accessing embedded fields.  
The KeyField is always required and need not be in RequiredFlds.  
  
PutRequest - puts records into a bkt  
PutOneRequest - puts 1 record into a bkt    
* includes option to auto write entry to log bkt("bktname_log"), set LogPut parm = true
* timestamp is appended to key, providing ability to see rec values at point in time    

PutBkts - puts records into 2 bkts in single transaction, on error all updates are rolled back  
PutIndex - puts records into an index bucket  
  
DeleteRequest - deletes specific records by id   
  
NOTE - fastjson does support changing individual fields in a json record.  
 
**Bulk Loads**  
When loading a large number of records, group records into batches. Load each batch
with a separate Go routine. See bulkloader for example code.
  
**NOTE** - Put operations will create the bkt if it does not already exist  

-------------------------------------------------------------------------------------------------------
### QRY REQUESTS

**Note -** Filter and Sort operations only support string and int values.  
  
Results are filtered using []bobb.FindCondition.  
Records must meet all find conditions (no "or" functionality).  
Convenience func Find (see client/util.go) can be used to create filter.  
See types.go for full documentation on FindCondition type.  
See codes.go for find Op codes.  
```
type FindCondition struct {
	Fld       string // field containing compare value
	Op        string // defines match criteria
	ValStr    string // for string ops, also converted based on StrOption
	ValInt    int    // for int Ops
	Not       bool   // only include records that do not meet condition
	StrOption string // "", "plain", "asis"
}
```
Results are sorted using []bobb.SortKey.  
Convenience func Sort (see client/util.go) can be used to create sort keys.  
See types.go for full documentation on SortKey type.  
See codes.go for SortKey Dir (direction) codes.  
```
type SortKey struct {
	Fld string `json:"fld"` // name of field
	Dir string `json:"dir"` // direction (asc/desc) and field type (str/int)
}
```

FindConditions and SortKeys use different op codes for different value types.  
See codes.go for details.   
In addition, find condition values use different parm fields depending on type (StrVal, IntVal).  
  
QryRequest - returns records meeting find conditions in sort keys order  
_ start/end keys can be used to specify a range of records to be queried  
QryIndexRequest - same as Qry except range of keys in index is used  
_ for example: index keys are by zip code, qry only data recs in range of zip codes  
  
-------------------------------------------------------------------------------------------------------
### INDEXES

Use when primary key does not provide efficient way to access subset of records.  

Developer is responsible for maintaining index buckets.  
  
Key - data value(s) from data rec, made unique  
Value - key of data rec in data bucket  
  
See indexloader/indexloader.go, index_settings.json for examples.  
  
Func MergeFlds in rec.go can be used to create keys composed of multiple fld values.  
  
Example:
```
    data bkt - "inquiry"
    data key - "b176-83"
    data val - {id:"b176-83", timestamp: "2021-03-23 08:17:44", ...}
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
    GetIndexRequest{
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
### BUCKET OPERATIONS

BktRequest - create, delete, get next sequence #  
- Operation field specifies the action
    - "create" to create bkt, "delete" to delete bkt, "nextseq" for seq numbers
- Bolt provides a NextSequence feature which returns an auto incrementing integer
- NextSeqCount field specifies how many sequence numbers to return (max 100) 
    - A NextSeqCount of 20 will return the next 20 numbers in order  
    - If not specified (NextSeqCount = 0), 1 value will be returned
    - These numbers will not be reused for this bkt

-------------------------------------------------------------------------------------------------------
### MISC OPERATIONS

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

See experimental.go for these requests.
They are included in bobb_server.go and demo.go.

* GetValues - returns specific values rather than whole records
* SearchKeys - searches the key values rather than data values (works with data and index bkts)