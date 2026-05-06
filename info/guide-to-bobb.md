## Guide To Bobb

### General Description

Bobb Server is a http server that receives Post requests from client apps. All requests return the same
response type (see Response type in types.go). Records can be added and updated using PutRequest, queried using QryRequest, gotten by key or key range using GetRequest/GetAllRequest. Entry values are JSON bytes that can be parsed to extract values used by bobb operations.  

Generally speaking bobb is fast. Get operations that use a key range are super fast.  
    
### Key Points
* Bobb is a thin layer on top of bolt/bbolt key-value data store
* Entries are simply keys and values, both of which are []byte
* Entries are organized into buckets (like sql table)
* Every key is unique inside a bucket 
* Data values are typically the result of json marshalling a struct value
* Data keys are strings converted to []byte

**Review [Bolt/Bbolt (etcd-io/bbolt)](https://github.com/etcd-io/bbolt#caveats--limitations) documentation**

**See [demo/demo.go](../demo/demo.go) for usage examples**

Detail documentation is contained in source code files. 
* Adding/Updating records using PutRequest, PutIndexRequest - see requests_put.go
* Query - see requests_qry.go
* Getting specific records or records in key range - see requests_get.go 
* Index requests - see requests_index.go
* Other operations (ex. BktRequest) - see requests_misc.go
* Types, not specific to a request, such as Response - see types.go
* Codes, constants such as Op, Sort, Find codes - see codes.go
* Misc funcs, constants, global vals - see util.go

**To get an overview of functionality review [codes.go](../codes.go).**  
  
### Indexing  

Indexes are typically used to speed operations for a range of data. For example, records in a date range. When a request specifies an index, only data records with an index entry are read. 
  
There are several options for creating indexes.  
1. Use IndexSettings for automatic indexing - see requests_index.go  
2. Use PutIndexRequest to manually load index entries into an index bucket - see requests_put.go
3. Use IndexRequest - to add index entries for specific data records - see requests_index.go  
   
An index is a bucket with keys and values. Keys are typically composed from 1 or more data values in an 
associated data record. The index value is the key of the data record. The index key is unique, so if it is possible for mutiple data records to have the same index key, a unique suffix can be added to the index key.  

Index keys are a single string value. When merging together multiple values, a set of rules is needed to determine how the key is created. See type **FldFormat** in types.go for details on how data field values are merged together to form index keys.  
  
With the flexible indexing options, you can be creative with how indexing is used. For example, indexing a subset of records that are used for a set of operations, to speed processing.

### Start End Keys

Records are read either directly by key or sequentially in key order.  
If using an index, the order of index keys is used.  
Requests can use Start and End keys to specify a range of records.  
Seeking to a Start key and reading from that point is very fast.
  
Range starts with first rec where key is >= Start key.  
Range ends with last rec where key is <= End key.  

**Using a key prefix**   
To use a key prefix, set StartKey and EndKey to the prefix value. All records where key prefix matches are in range.

### Client Pkg

* client/client.go - contains Run func which sends Requests to and receives Responses from bobb_server
* client/util.go - shortcut funcs (ex. GetOne)
* client/data_conversion.go - data conversion funcs (ex. JsonToMap)

### Batch Loading  

When loading a large number of data or index entries, it may be faster to load in batches. See bulkload and indexloader programs for examples.  

### Missing and Null Values 

Some operations provide a UseDefault option which controls what happens when a field value is missing or has a null value. Consider:  
* query criteria or sort field 
* value used as part of an index key 
