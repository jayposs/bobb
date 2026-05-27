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

                                                                                                                    