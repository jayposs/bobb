package main

import (
	"net/http"

	"github.com/jayposs/bobb"
)

func experimentalRoutes() {

	// *** EXPERIMENTAL REQUEST ROUTING *****************************************************

	http.HandleFunc("/getvalues", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.GetValuesRequest
		process(bobb.OpGetValues, &req, w, r)
	})

	http.HandleFunc("/searchkeys", func(w http.ResponseWriter, r *http.Request) {
		var req bobb.SearchKeysRequest
		process(bobb.OpSearchKeys, &req, w, r)
	})
}
