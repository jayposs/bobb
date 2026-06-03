## Bobb — JSON Database with HTTP Access

Bobb is a lightweight JSON database server for Go applications. It wraps bbolt, a proven embedded key-value store and adds an HTTP layer, a query engine, and automatic secondary index management on top of it.  

### What it gives you

* Simple data model. Records are JSON objects stored in named buckets.
* No schema definition required.                    
* HTTP access. The server runs as a standalone process.
* Included Go client pkg enables easy communication with server.
* Secondary indexing with manual or automated loading/maintenance.
* Queryies with wide range of filter conditions and sorting.
* Simple lookup style joins.
* Quick and easy CSV exports from query results.
 
### Good Fit For 
* Go applications that need a persistent, queryable store using a small, fast, and simple database.
* Systems where multiple processes need simultaneous access.
* Moderate data volumes where simplicity of JSON is better match than relational options.

### Simplicity of Design

Example http routes, they all work the same way:
```
    http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetRequest
		process(bobb.OpGet, &req, w, r)
	})
	http.HandleFunc("/put", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.PutRequest
		process(bobb.OpPut, &req, w, r)
	})
	http.HandleFunc("/qry", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.QryRequest
		process(bobb.OpQry, &req, w, r)
	})
```
All request types have Run method.
The "process" func called by the HandleFunc:
1. Decodes http request body into the request type struct
2. Calls the Run method

Get* request types are in the requests_get.go file.  
Put* request types are in requests_put.go file.  
Qry request type is in requests_qry.go file.