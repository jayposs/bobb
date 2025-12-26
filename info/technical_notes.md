## Technical Notes

The underlying data store, Bolt/Bbolt, is a Go based key-value store that is screaming fast, but very minimalistic. It is an embedded db which means it runs as part of the main Go program accessing the db. Only 1 program can access the database file at a time. In Bobb's case, this is the http server, bobb_server.go. Multiple goroutines can read the db at the same time. Bolt allows 1 update request to process at a time, but manages simultaneous requests from multiple goroutines. The database is stored in a single file, which is memory-mapped by the underlying operating system.
  
For details see [bbolt documentation](https://github.com/etcd-io/bbolt?tab=readme-ov-file#caveats--limitations). I recommend reviewing the full readme file.  
Note - Bobb does not work with nested buckets.  

Also kudos to Go package [fastjson](https://pkg.go.dev/github.com/valyala/fastjson) which is a speed demon at parsing JSON.  

Records are accessed either directly by key or sequentially by key with a cursor. A cursor provides the means to quickly establish a starting point and then read sequentially in key order. Direct and cursor based sequential reads are fast. 

Records are stored as slice of bytes ( []byte ) typically created by json.Marshal of struct.

Fastjson allows values inside sub structs and slices to be accessed, but Bobb only uses top level fields for selection and sorting.

Put operations with a large number of records should be split into batches. 
See the bulkload/bulkload.go for example template program. 

### Information Files 
Folder "info" contains additional documentation files. See a-index.md for descriptions and links to each.  

**Warning On Memory Use and File Size**
```
Generally speaking, Bobb does not try to minimize memory use.
Results are stored in memory before being returned.
There are no paging or iterating features. By using StartKey / EndKey or Limit, a subset of full results can be returned.
 
The Results.NextKey value can be used as the StartKey for the next transaction.

If you have simultaneous requests with large results, a large amount of memory will be used.

Database file size can become quite large.
Bbolt has a compact function, but not currently implemented by Bobb.
```
**Process Flow**  
1. Client sends http request to running server using bobb/client Run() func
2. Based on url, the appropriate Request handler is called
3. Request handler creates response which is returned to client  

**Transactions**  
All requests are done inside a Bolt transaction. Updates will be rolled back if a database error occurs. Updates are committed when a transaction completes successfully.

### Buckets 
Records are stored in collections called buckets. Entries are simply a key and value, both of which are a slice of bytes ([]byte). Keys must be unique inside the containing bucket. On Put operations, if key does not exist, a new key/value entry is added to bucket. If key does exist, value is replaced with new value.  

Record values are the result of json.Marshalling so they can be complex structs with internal structs/slices/maps.  

### Data Record Keys 
**Bobb requires all keys be string values.** They are converted to []byte by Put request. The key value must exist as a field in the record. For example, the field "id" has the same value as the key. Requirement of Bobb, not Bolt.

Often the "primary" key of a database record is just a random unique value (ex. uuid). With bolt, that may not be the best choice. To assist with generating unique keys, there is an API "Bkt" request that returns auto incrementing next sequence number(s) for a bucket.  

One potential scenario would be parent and child record buckets. The parent record keys might be prefixed with a client key or transaction date and end with the bkt nextseq value. The child record keys might be prefixed with the parent key and suffixed with item number. In this example, child records for a particular parent can be accessed very quickly.

**Be Careful** - values used to compose keys should not change. This will cause complications.  

### Start / End Keys
Bolt can seek to a key really fast and then read sequentially in key order from that point. If the seek key is not found, the cursor is positioned at the next key. Reading continues until the record key is greater than the end key. If no start key is specified, reading begins with the first bucket record in key order. If no end key is specified, reading continues to last record in key order.    
  
**Using a key prefix >**  set StartKey and EndKey with prefix, all records where key prefix matches are included in range.

### Secondary Indexes  
These are simply buckets with keys and values. The index key is typically composed of 1 or more values from a data record made unique by appending a value to the end of it. The index value is the key of the data record. The developer is responsible for creating, loading, and maintaining index buckets.  
  
Index buckets can speed processing when the data keys don't provide useful start/end keys. If a large number of records must be scanned, it may be faster to not use an index but rather read the data bucket directly. Bobb can query thousands of records very quickly, so the key range doesn't need to be that small.   

The **PutIndex** request is used to load index buckets.

Pgm **indexloader/indexloader.go** can be used to create and bulk load an index bucket. Use it as an example, but feel free to load indexes however you want. Not all records in a bucket need be indexed for every index (think sparse index). 

See **indexrecs/indexrecs.go** for example pgm that loads index records for specific data records.  
  
Generally, index records should be created from existing records. The func MergeFlds uses a fastjson parsed record for input. It is a handy way to create index keys from multiple values in the record. See indexloader.go for example.

If the dataset is fairly static, for example once daily updates, a batch process to completely recreate the index
may be fine. The included indexloader program provides this capability. Periodically rebuilding any index may be a 
good idea.

If the index must be kept up to date at all times, then when an associated data record is added, a corresponding
index record must be added. The developer is responsible for implementing this logic. The PutIndex request 
simplifies adding index records.

**BEWARE**  
If the index key value changes (due to data record change) and a new record is Put into the index, the original index record must be deleted, else there will be multiple index records pointing to the same data record. Requests using this index may produce invalid results.  The PutIndex request has a feature to facilitate removal of the original index record when a replacement index record is added. The IndexKeyVal field, OldKey, identifies the previous index key which will be automatically deleted. See func updateIndex in demo.go for example.

Records are read sequentially from the index bucket beginning at the start key and directly from the data bucket using the primary data record keys. 

If start/end keys are not specified, all data records are read in index key order. Result records are also returned in this order unless sort order is specified in the Qry request.  

### Put Logic
Most higher function databases have separate logic for adding, updating, and replacing records. Bolt just uses Put, which either completely replaces or adds a record depending on the existence of the key or not. Bobb doesn't add additional logic to compensate for this loss of functionality.

See demo program "update" func for an example of how individual fields, of an existing record, could be updated.
