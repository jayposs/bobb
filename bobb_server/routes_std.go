package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jayposs/bobb"
)

func stdRoutes() {

	// *** VIEW REQUEST ROUTING *****************************************************

	mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetRequest
		process(bobb.OpGet, &req, w, r)
	})
	mux.HandleFunc("/getone", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetOneRequest
		process(bobb.OpGetOne, &req, w, r)
	})
	mux.HandleFunc("/getall", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetAllRequest
		process(bobb.OpGetAll, &req, w, r)
	})
	mux.HandleFunc("/getallkeys", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetAllKeysRequest
		process(bobb.OpGetAllKeys, &req, w, r)
	})
	mux.HandleFunc("/qry", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.QryRequest
		process(bobb.OpQry, &req, w, r)
	})
	mux.HandleFunc("/verifyindex", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.VerifyIndexRequest
		process(bobb.OpVerifyIndex, &req, w, r)
	})

	// *** UPDATE REQUEST ROUTING *****************************************************

	mux.HandleFunc("/put", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.PutRequest
		process(bobb.OpPut, &req, w, r)
	})
	mux.HandleFunc("/putindex", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.PutIndexRequest
		process(bobb.OpPutIndex, &req, w, r)
	})
	mux.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.DeleteRequest
		process(bobb.OpDelete, &req, w, r)
	})
	mux.HandleFunc("/indexsetting", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.IndexSettingRequest
		process(bobb.OpIndexSetting, &req, w, r)
	})
	mux.HandleFunc("/indexrequest", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.IndexRequest
		process(bobb.OpIndexRequest, &req, w, r)
	})
	// *** OTHER REQUEST ROUTING *****************************************************

	mux.HandleFunc("/bkt", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.BktRequest
		process(bobb.OpBkt, &req, w, r)
	})
	mux.HandleFunc("/export", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.ExportRequest
		process(bobb.OpExport, &req, w, r)
	})
	mux.HandleFunc("/copydb", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.CopyDBRequest
		process(bobb.OpCopyDB, &req, w, r)
	})

	// activates Trace() calls in code, see scripts/traceon.sh
	mux.HandleFunc("/traceon", func(w http.ResponseWriter, r *http.Request) {
		bobb.TraceStatus.Set("on") // see util.go
		log.Println("tracing turned on")
		w.WriteHeader(http.StatusOK)
	})
	// deactivates Trace() calls in code, see scripts/traceoff.sh
	mux.HandleFunc("/traceoff", func(w http.ResponseWriter, r *http.Request) {
		bobb.TraceStatus.Set("off") // see util.go
		log.Println("tracing turned off")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		shutDown()
		os.Exit(0)
	})
}
