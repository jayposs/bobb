## Change Log

**April 22, 2024 - Add Compress Response option**   

Provides option to have response gzipped.  
May be valuable if client not on same network as server.  
   
* Add field compressResponse to bobb_settings.json
* In bobb_server.go
    * Add field CompressResponse to settings type
    * Add var gzipWriterPool
    * Add if settings.CompressResponse to dbHandler func
    * Add func compressResponse
  
**May 7, 2024 - Add PutBkts feature**  
  
Provides ability to put records to 2 bkts in a single transaction.  
  
* Add const OpPutBkts to codes.go
* Add func PutBkts to updt_handlers.go
* Add func putRec to updt_handlers.go
* Move common code that was in Put, PutOne to putRec
* Add type PutBktsRequest to types.go
* Add handler logic for "putbkts" request to bobb_server.go  

**May 23, 2024 - Add demodata pkg**
  
Separate db types and funcs from demo.go main pgm.

* demodata/datatypes.go - db record types
* demodata/datafuncs.go - convert db recs from/to json recs using generic funcs

**June 4, 2024 - Rewrite Qry Sort**  

Sorting for query requests is now much faster.  
Previous version did not use fastjson parsing in an efficient way.  

* view_handlers.go - qrySort func rewrite, with minor changes to Qry, QryIndex funcs.

**June 12, 2024 - Modify PutIndex**
  
When replacing existing index record due to data change that affects index key,  
old index record can be deleted on same request.  

* types.go - add OldKey field to IndexKeyVal type
* updt_handlers.go - add logic to PutIndex func to delete rec where key = OldKey
* demo/demo.go - add updateIndex func

**June 14, 2024 - Add Experimental Request, GetValues**

Provides ability to request values only for specific fields rather than entire records.  
This code is more of an example that may need to be modified for specific needs.  

* exp_requests.go - GetValuesRequest and RecValues types, GetValues func
* server/bobb_server.go - add routing code
* demo/demo.go - add getValues func