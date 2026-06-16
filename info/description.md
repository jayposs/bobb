## bobb — JSON Database with HTTP Access

A lightweight JSON database server for Go applications. It wraps bbolt, a proven embedded key-value store and adds an HTTP layer, a query engine, and automatic secondary index management on top of it.  

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

Process flow is very straight forward. Example http routes, they all work the same way:
```
    mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetRequest
		process(bobb.OpGet, &req, w, r)
	})
	mux.HandleFunc("/put", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.PutRequest
		process(bobb.OpPut, &req, w, r)
	})
	mux.HandleFunc("/qry", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.QryRequest
		process(bobb.OpQry, &req, w, r)
	})
```
All request types have Run method.
The "process" func called by the HandleFunc:
1. Decodes http request body into the request type struct
2. Calls the Run method
3. Returns the standard Response type

RequestTypes of same operation family (ex. Get*, Put*) are in requests_opfamily.go.
For example Get, GetAll, GetOne request types are in requests_get.go 

### Motivation
Need for small, simple document style database that works easily with Go. Has feature set that includes common database operations such as queries with indexes. 
