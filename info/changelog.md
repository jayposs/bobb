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
