// Program server.go accepts http requests from client programs and interacts with the bbolt db.
// All requests use the Post method.
// All responses are instances of bobb.Response.

package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jayposs/bobb"

	bolt "go.etcd.io/bbolt"
)

var settings struct {
	DBPath           string `json:"dbPath"`  // location & name of db file
	Port             string `json:"port"`    // what port server listens on
	Trace            string `json:"trace"`   // if "on" calls to bobb.Trace will write to log
	LogPath          string `json:"logPath"` // if not "", log output will be to this file
	CompressResponse bool   `json:"compressResponse"`
	MaxErrs          int    `json:"maxErrs"` // used if request ErrLimit is -1
}
var db *bolt.DB
var logFile *os.File

var gzipWriterPool = &sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(nil)
	},
}

func main() {
	var err error

	// if cmd flag -settings not entered, pgm will look for bobb-settings.json in current dir.
	settingsPath := flag.String("settings", "", "add -settings cmd line option to specify where bobb-settings.json is located")
	flag.Parse()
	loadSettings(*settingsPath + "bobb_settings.json") // loadSettings func is below
	fmt.Println("-- Settings --")
	fmt.Printf("%+v\n", settings)

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("bobb_server starting")

	dbOptions := bolt.Options{FreelistType: bolt.FreelistMapType}
	db, err = bolt.Open(settings.DBPath, 0600, &dbOptions)
	if err != nil {
		log.Fatalln(err)
	}

	stdRoutes() // set http request routing for standard requests, see routes_std.go

	experimentalRoutes() // routing for experimental requests, see routes_experimental.go

	customRoutes() // routing for custom requests, see routes_custom.go

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("invalid request url:", r.RequestURI)
		http.Error(w, "invalid request url", http.StatusNotFound)
	})

	// --- END REQUEST ROUTING --------------------------------------------------------------

	bobb.ServerStatus.Set("running") // see util.go

	fmt.Println("waiting for requests ...")
	log.Println(http.ListenAndServe(":"+settings.Port, nil))
}

// Func process executes the request.
// Parm req is pointer to request type which implements bobb.Request interface.
func process(op string, req bobb.Request, w http.ResponseWriter, r *http.Request) {

	if bobb.ServerStatus.Get() != "running" {
		http.Error(w, "server not accepting requests", http.StatusServiceUnavailable)
		return
	}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		log.Println("Error decoding JSON", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// updateFunc is called for update Requests
	var updateFunc = func(tx *bolt.Tx) error {
		response, err := req.Run(tx)
		if err == nil || err == bobb.ErrBadInputData { // allow bobb.ErrBadInputData to be returned to caller in normal response
			writeResponse(response, w)
		} else {
			log.Println("DB Error Occured - Update transaction rolled  back", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return err // non nil err will cause bbolt to rollback the transaction
	}

	// viewFunc is called for read Requests
	var viewFunc = func(tx *bolt.Tx) error {
		response, _ := req.Run(tx) // View requests always return nil err
		writeResponse(response, w)
		return nil
	}

	if req.IsUpdtReq() {
		db.Update(updateFunc)
	} else {
		db.View(viewFunc)
	}

	bobb.Trace(op + " == request complete ==")
}

func loadSettings(fileName string) {
	jsonSettings, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatalln("error opening Settings File", err)
	}
	err = json.Unmarshal(jsonSettings, &settings)
	if err != nil {
		log.Fatalln("json.Unmarshal error on jsonSettings", err)
	}
	bobb.TraceStatus.Set(settings.Trace)

	// CONSIDER OPTION TO APPEND TO LOG FILE RATHER THAN OVERWRITE ###
	if settings.LogPath != "" {
		logFile, err = os.Create(settings.LogPath)
		if err != nil {
			log.Fatalln("logfile create failed", err)
		}
		log.SetOutput(logFile)
	}

	if settings.MaxErrs > 0 {
		bobb.MaxErrs = settings.MaxErrs
	}
}

// shutDown will wait 10 seconds to allow current requests to finish and block future requests.
// The database file will then be closed.
func shutDown() {
	bobb.ServerStatus.Set("down")
	log.Println("shutdown process started, waiting 10 secs ...")
	time.Sleep(10 * time.Second)
	if err := db.Close(); err != nil {
		log.Fatalln("error closing db file", err)
	}
	log.Println("db closed")
	if logFile != nil {
		logFile.Close()
	}
}

// writeResponse returns response to client
func writeResponse(resp *bobb.Response, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")

	if settings.CompressResponse {
		w.Header().Set("Content-Encoding", "gzip")

		compressor := gzipWriterPool.Get().(*gzip.Writer)
		compressor.Reset(w)

		encoder := json.NewEncoder(compressor)
		err := encoder.Encode(resp)
		if err != nil {
			log.Println("json encoding failed, with compression", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		err = compressor.Close()
		if err != nil {
			log.Println("compressor.Close() failed", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		gzipWriterPool.Put(compressor)
	} else {
		encoder := json.NewEncoder(w)
		err := encoder.Encode(resp)
		if err != nil {
			log.Println("json encoding failed, no compression", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
