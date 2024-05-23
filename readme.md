## Bobb - Small, Simple, Fast Data Tool Built On [bbolt](https://github.com/etcd-io/bbolt)

### Motivation 
I wanted a database that was simple to setup and easy to understand. Most popular databases are pretty scary to manage yourself. Services are expensive and have other issues.  

### Change Log 
To see detailed information on changes, see changelog.md.  

### Overview
Bobb uses bbolt (fork of bolt db) for all the dirty work. Bbolt is a Go based key/value store that is screaming fast when it comes to reading data, but very minimalistic. It is an embedded db which means it runs as part of the main Go program accessing the db. Only 1 program can access the database file at a time.
  
Bobb is a thin layer on top of bbolt adding the following features:    
* Multiple programs can send requests to the bobb server 
* Simple but powerful API
* Ability to query for records meeting conditions and sorting results
* Secondary indexes to speed access  

Also kudos to Go package [fastjson](https://pkg.go.dev/github.com/valyala/fastjson) which greatly simplified and reduced coding for Bobb.

It is recommended to review the [bbolt documentation](https://pkg.go.dev/go.etcd.io/bbolt#section-readme).  
Note - Bobb does not work with nested buckets.  

Generally speaking, Bobb does not try to minimize memory use. Results are stored in memory before being returned. There are no paging or iterating features. By using start/end keys, a subset of full results can be returned.

**Process Flow**  
1. Client sends http request to running server using bobb/client Run() func
2. Based on url, the appropriate handler func is called
3. DB handler func creates response which is returned to client  

**Transactions**  
All requests are done inside a bbolt transaction. Updates will be rolled back if a database error occurs. Updates are committed when a transaction completes successfully.

## API

Every request is executed in the same manner. Example:  
```
import "bobb"
import	bo "bobb/client"
...
req := bobb.GetAllRequest{
    BktName:  "nameofbkt"
    StartKey: "A100-3000"
    EndKey:   "A100-4000"
}
resp, err := bo.Run(httpClient, bobb.OpGetAll, req)
```
Every api request has its own request struct type and handler function.   
All requests return the same response type. 
```
type Response struct {
	Status  string   `json:"status"`  // constants in codes.go ex. StatusOk
	Msg     string   `json:"msg"`     // warning/error message
	Recs    [][]byte `json:"recs"`    // result set
	Rec     []byte   `json:"rec"`     // result when only 1 rec can be returned
	PutCnt  int      `json:"putCnt"`  // for Put operations
	NextSeq []int    `json:"nextSeq"` // see Bkt request "nextseq"
}
```

The db server program receives http requests from client progams and calls the appropriate handler. Result records are returned as slice of bytes ([]byte). They must each be json.Unmarshalled into the appropriate record struct type. See demo program for examples.   

### API Requests
* Get - get multiple records using record keys
* GetOne - get a single record using record key
* GetAll - get all records in a bkt or all in key range
* GetAllKeys - like GetAll but returns key values
* GetIndex - works like GetAll but uses index bkt to speed processing
* Put - put multiple records
* PutBkts (added May 3, 2024) - put records to 2 buckets in a single transaction
* PutOne - put a single record
* PutIndex - put records into an index bucket
* Qry - return records meeting selection criteria in sorted order
* QryIndex - uses index bkt to speed processing
* Delete - delete 1 or more records
* Bkt - perform bucket requests create, delete, nextseq
* Export - works like GetAll, but results written to file in formatted json
* CopyDB - creates copy of db file
* Following use curl scripts (see scripts dir)
    * Down - shuts down server after 10 secs, blocking new requests
    * TraceOn - calls to Trace func (in util.go) will log msg
    * TraceOff - turns off Trace logging  

### Detail Documentation  
* See **types.go** for details on each request type.  
* See **codes.go** for request Op codes and Qry find & sort options.  

### Shell Scripts (see scripts dir)
* start.sh - start server
* down.sh - shutdown server (sleeps 10 secs to allow in-process requests to finish, blocking new requests)
* traceon.sh - turn tracing on
* traceoff.sh - turn tracing off


### Buckets 
Records are stored in collections called buckets. Entries are simply a key and value, both of which are a slice of bytes ([]byte). Keys must be unique inside the containing bucket. On Put operations, if key does not exist, a new key/value entry is added to bucket. If key does exist, value is replaced with new value.  

Record values are the result of json.Marshalling so they can be complex structs with internal structs/slices/maps.  

### Data Record Keys 
Bobb requires all keys be string values. They are converted to []byte by Put request.  

Often the "primary" key of a database record is just a random unique value (ex. uuid). With bbolt, that may not be the best choice. To assist with generating unique keys, there is an API "Bkt" request that returns auto incrementing next sequence number(s) for a bucket.  

One potential scenario would be parent and child record buckets. The parent record keys might be prefixed with a client key or transaction date and end with the bkt nextseq value. The child record keys might be prefixed with the parent key and suffixed with item number. In this example, child records for a particular parent can be accessed very quickly.

**Be Careful** - values used in keys should not change. This will cause complications.  

### Start / End Keys
Bbolt can seek to a key really fast and then read sequentially in key order from that point. If the seek key is not found, the cursor is positioned at the next key. Reading continues until the record key is greater than the end key. If no start key is specified, reading begins with the first bucket record in key order. If no end key is specified, reading continues to last record in key order.  

### Secondary Indexes  
These are simply buckets with keys and values. The key is typically composed of 1 or more values from a data record made unique by appending a value to the end of it. The record value is the key of the data record. The developer is responsible for creating, loading, and maintaining index buckets.  
  
Index buckets can speed processing when the data keys don't provide useful start/end keys. It is important to limit the range between start and end keys. If a large number of records must be scanned, it may be faster to not use an index (read the data bucket directly).   

The GetIndex, and QryIndex requests use index buckets to speed processing. The PutIndex request is used to load an index bucket with records.

The **Index Loader** (indexloader/indexloader.go) can be used to create and bulk load an index bucket. It reads the complete data bkt into memory, so for very large bkts, a custom index loader may need to be used.

**BEWARE**  
If the index key value changes (due to data record change) and a new record is Put into the index, the original index record must be deleted, else there will be multiple index records pointing to the same data record. Requests using this index may produce invalid results.  

Records are read sequentially from the index bucket beginning at the start key and directly from the data bucket using the primary data record keys. 

If start/end keys are not specified, all data records are read in index key order. Result records are also returned in this order unless sort order is specified in a QryIndex request.  

### Put Logic
Most higher function databases have separate logic for adding, updating, and replacing records. Bbolt just uses Put, which either completely replaces or adds a record depending on the existence of the key or not. Bobb doesn't add additional logic to compensate for this loss of functionality.

See demo program "update" func for an example solution to this problem.

### File Descriptions
* server/bobb_server.go - db server
    * uses std lib http.ListenAndServe and http.HandleFuncs
    * each request type has unique url which is tied to it's db handler function
    * all requests use POST method
    * Listening Port and DB file name are specified in bobb_settings.json
* client/client.go - only used by client programs
    * Run() function is used to build and send http requests to running bobb server and return response
    * Shortcut functions (GetOne, PutOne) reduces code to build/run request
    * Client program can set pkg vars Debug and BaseURL
    * If Debug=true, the request/response json is displayed
    * Requests are sent to BaseURL, default value is "http://localhost:8000/"
* types.go - all request type structs with documentation for each
    * used by both client and bobb server programs
* codes.go - various codes
    * used by both client and bobb server programs
    * Response status codes
    * Qry sorting codes
    * Qry find condition codes
* view_handlers.go - read only transactions
    * All use bolt db.View transaction
    * Get, GetOne, GetAll, GetIndex, Qry, ...
    * Also the openBkt func used by all request handlers
* updt_handlers.go - update transactions
    * All use bolt db.Update transaction
    * Put, PutOne, PutIndex, Delete, Bkt
    * Bkt func performs multiple operations (create, delete, nextseq)
* rec.go - support funcs used by various request handlers
    * Utilizing fastjson, values are pulled from record values ([]byte) 
    * recGetStr, recGetInt 
    * recFind determines if a record meets qry find conditions
* util.go - misc funcs, types    
* indexloader/indexloader.go - stand alone program
    * bulk loads secondary index bucket
    * uses specifications from index_settings.json
    * index bkt is deleted, created, loaded (for safety, index bkt name must include "index")
    * run command flag specifies which index to create  
    * see index_settings.json for example  
    * sends PutIndex requests to bobb server in batches (using goroutines/sync.WaitGroups)  
* demo/demo.go - demo + testing program 
* bulkloader/bulkloader.go - example record loader using batches 
* linux_notes.txt - for running server in background       

### Instructions  
I recommend treating Bobb like one of your personal projects. The code size is small and pretty
straight forward. Add features you want.  

Go version 1.21 or higher is required.  

Bobb is a Go module containing:
* executable programs (server.go, indexloader.go, demo.go, bulkloader.go)
* types, constants, and funcs used by client programs and the server program  
  
One install option is to clone the repo to your machine and run go mod tidy for the bobb module.  
Dependencies should be installed (bbolt, fastjson).

Your client program modules need the following lines in their module's go.mod file:
* replace bobb => /home/username/bobb  (location of bobb module)
* require bobb v0.0.0  
  
Example myapps go.mod:
```
module myapps

go 1.21.1

replace bobb => /home/myuser/bobb

require bobb v0.0.0

require (
	github.com/valyala/fastjson v1.6.4 // indirect
	go.etcd.io/bbolt v1.3.8 // indirect
	golang.org/x/sys v0.4.0 // indirect
)

```
To try out bobb:  
1. cd to bobb/server dir
2. edit bobb_settings.json to use desired db file location and port
3. go run bobb_server.go  (should display "waiting for requests ...")
4. open new terminal window or put server to background (see linux_notes.txt)
5. cd bobb/demo dir
6. go run demo.go  (last display line should indicate demo finished successfully)

Sorry Windows users, directions are geared for linux/osx. You could spinup a cheap cloud linux instance.  
   
NOTE  
If server not using same port as in client.BaseURL, client program must change client.BaseURL to use same port as server.

Client programs send http requests to the bobb db server using the Run() function located in the client package. The db server program must be running, listening for requests.  
  
File bobb_settings.json specifies various settings such as db file path and listening port. 
If the bbolt database file specified in settings does not exist, a new database file will be created.  

By default, the server pgm looks for bobb_settings.json in same dir as running server.
To change location, add -settings cmd line argument when starting server. 
``` 
Exampe: $HOME/bobb/bin/bobb_server -settings $HOME/myapps/  
```
See demo/demo.go for examples of how to use all request types.  
See types.go for documentation on each request type.  

For a production enviroment, compile bobb_server.go and place binary where desired.  
See linux_notes.txt for running server in background.

### Demo/Test Program

The demo program (demo/demo.go) serves 2 functions. It demonstrates how to use all the API requests and verifies all functions work correctly. Some requests, such as Qry have a large number of possible combinations. The testing does not cover all possibilities.  

Additional testing was done with a larger (~250,000 record) dataset.

### Backups

The CopyDB request copies the open db file to another filepath. This request should not interfere with other requests. See bbolt documentation for more information.

### Considerations for database selection/setup: 
* Cost - managed services are simple to use, but can be expensive
* Location - performance is much better if the db is on same local network as apps
* Freedom - managed service may limit what you can do with the db 
* Storage - use redundant block storage rather than local hd (most cloud providers offer)

An approach is to have multiple data tools. One geared for transactional and batch processing (Bobb). Another for reporting and data analysis (Snowflake).

  
