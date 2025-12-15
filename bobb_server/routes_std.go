package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jayposs/bobb"
)

func stdRoutes() {

	// *** VIEW REQUEST ROUTING *****************************************************

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetRequest
		process(bobb.OpGet, &req, w, r)
	})
	http.HandleFunc("/getone", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetOneRequest
		process(bobb.OpGetOne, &req, w, r)
	})
	http.HandleFunc("/getall", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetAllRequest
		process(bobb.OpGetAll, &req, w, r)
	})
	http.HandleFunc("/getallkeys", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetAllKeysRequest
		process(bobb.OpGetAllKeys, &req, w, r)
	})
	http.HandleFunc("/"+bobb.OpQry, func(w http.ResponseWriter, r *http.Request) {
		var req bobb.QryRequest
		process(bobb.OpQry, &req, w, r)
	})

	// *** UPDATE REQUEST ROUTING *****************************************************

	http.HandleFunc("/put", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.PutRequest
		process(bobb.OpPut, &req, w, r)
	})
	http.HandleFunc("/putone", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.PutOneRequest
		process(bobb.OpPutOne, &req, w, r)
	})
	http.HandleFunc("/putindex", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.PutIndexRequest
		process(bobb.OpPutIndex, &req, w, r)
	})
	http.HandleFunc("/putbkts", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.PutBktsRequest
		process(bobb.OpPutBkts, &req, w, r)
	})
	http.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.DeleteRequest
		process(bobb.OpDelete, &req, w, r)
	})

	// *** OTHER REQUEST ROUTING *****************************************************

	http.HandleFunc("/bkt", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.BktRequest
		process(bobb.OpBkt, &req, w, r)
	})
	http.HandleFunc("/export", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.ExportRequest
		process(bobb.OpExport, &req, w, r)
	})
	http.HandleFunc("/copydb", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.CopyDBRequest
		process(bobb.OpCopyDB, &req, w, r)
	})

	// activates Trace() calls in code, see scripts/traceon.sh
	http.HandleFunc("/traceon", func(w http.ResponseWriter, r *http.Request) {
		bobb.TraceStatus.Set("on") // see util.go
		log.Println("tracing turned on")
		w.WriteHeader(http.StatusOK)
	})
	// deactivates Trace() calls in code, see scripts/traceoff.sh
	http.HandleFunc("/traceoff", func(w http.ResponseWriter, r *http.Request) {
		bobb.TraceStatus.Set("off") // see util.go
		log.Println("tracing turned off")
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		shutDown()
		os.Exit(0)
	})
}
