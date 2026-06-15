// Program server.go accepts http requests from client programs and interacts with the bbolt db.
// All requests use the Post method.
// All responses are instances of bobb.Response.

package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jayposs/bobb"

	bolt "go.etcd.io/bbolt"
)

var settings struct {
	DBPath              string `json:"dbPath"`              // location & name of db file
	Port                string `json:"port"`                // what port server listens on
	Trace               string `json:"trace"`               // if "on" calls to bobb.Trace will write to log
	LogPath             string `json:"logPath"`             // if not "", log output will be to this file
	CompressResponse    bool   `json:"compressResponse"`    // if true, response is compressed using gzip
	InitialRespRecsSize int    `json:"initialRespRecsSize"` // initial size of Response.Recs slice
	MaxErrs             int    `json:"maxErrs"`             // used if request ErrLimit is -1
	KeySuffixWidth      int    `json:"keySuffixWidth"`      // width of zero-padded suffix for keys, see PutRequest.AddKeySuffix
	DefaultKeyFld       string `json:"defaultKeyFld"`       // if request doesn't specify key field, this will be used
}
var db *bolt.DB
var logFile *os.File

var gzipWriterPool = &sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(nil)
	},
}

var mux *http.ServeMux = http.NewServeMux()

func main() {
	var err error

	// if cmd flag -settings not entered, pgm will look for bobb-settings.json in current dir.
	pathPrefix := flag.String("settings", "", "add -settings cmd line option to specify dir where bobb-settings.json is located")
	flag.Parse()
	loadSettings(*pathPrefix) // loadSettings func is below
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

	// catchall for invalid request url
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("invalid request url:", r.URL)
		http.Error(w, "invalid request url", http.StatusNotFound)
	})

	// --- END REQUEST ROUTING --------------------------------------------------------------

	bobb.ServerStatus.Set("running") // see util.go

	fmt.Println("waiting for requests ...")
	// log.Println(http.ListenAndServe(":"+settings.Port, nil))

	// Configure and instantiate the http.Server explicitly
	srv := &http.Server{
		Addr:         ":" + settings.Port,
		Handler:      mux, // Pass your custom router here
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	// 1. Run server in a goroutine so it doesn't block
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 2. Wait for interrupt signal (Ctrl+C or SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// 3. Create a timeout context for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 4. Shutdown gracefully
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exiting")
}

// Func process executes the request.
// Parm req is pointer to request type which implements bobb.Request interface.
func process(op string, req bobb.Request, w http.ResponseWriter, r *http.Request) {

	if bobb.ServerStatus.Get() != "running" {
		http.Error(w, "server not accepting requests", http.StatusServiceUnavailable)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println("Error decoding JSON", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var response *bobb.Response
	var err error

	// NOTE - updt vs view requests
	// updt(put) req cannot have refs to bolt values in response
	//		writeResponse is executed outside bolt trans
	// view(get,qry) req can have refs to bolt values in response
	//		writeResponse is executed inside bolt trans
	// response refs to bolt values are valid only inside a bolt trans
	// bolt allows concurrent View trans but not concurrent Update trans,
	//    updt trans holds the database's exclusive write-lock for the entire duration of the network I/O
	if req.IsUpdtReq() {
		db.Update(func(tx *bolt.Tx) error {
			response, err = req.Run(tx)
			return err
		})
		if err != nil && err != bobb.ErrBadInputData {
			log.Println("DB Error Occured - Update transaction rolled  back", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			writeResponse(response, w) // executed outside db transaction
		}
	} else {
		db.View(func(tx *bolt.Tx) error {
			response, err = req.Run(tx) // View requests always return nil err
			writeResponse(response, w)  // executed inside db transaction
			return err
		})
	}
	bobb.Trace(op + " == request complete ==")
}

func loadSettings(pathPrefix string) {
	var fileName string
	pathPrefix = strings.TrimSpace(pathPrefix)
	if pathPrefix != "" && !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}
	fileName = pathPrefix + "bobb_settings.json"
	log.Println("bobb settings file:", fileName)

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
	if settings.InitialRespRecsSize < 1 {
		settings.InitialRespRecsSize = 400
	}
	if settings.MaxErrs < 1 {
		settings.MaxErrs = 10
	}
	if settings.KeySuffixWidth < 1 {
		settings.KeySuffixWidth = 8
	}
	if settings.DefaultKeyFld == "" {
		settings.DefaultKeyFld = "id"
	}
	bobb.DefaultKeyFld = settings.DefaultKeyFld
	bobb.InitialRespRecsSize = settings.InitialRespRecsSize
	bobb.MaxErrs = settings.MaxErrs
	bobb.KeySuffixWidth = settings.KeySuffixWidth
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
