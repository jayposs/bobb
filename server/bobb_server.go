// Program server.go accepts http requests from client programs and interacts with the bbolt db.
// All requests use the Post method.
// All responses are instances of bobb.Response.
// The dbHandler func calls appropriate request handler in view_handlers.go or updt_handlers.go

package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"bobb"

	bolt "go.etcd.io/bbolt"
)

var settings struct {
	DBPath           string `json:"dbPath"`  // location & name of db file
	Port             string `json:"port"`    // what port server listens on
	Trace            string `json:"trace"`   // if "on" calls to bobb.Trace will write to log
	LogPath          string `json:"logPath"` // if not "", log output will be to this file
	CompressResponse bool   `json:"compressResponse"`
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

	// *** GET REQUEST ROUTING *****************************************************

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.GetRequest
		dbHandler(bobb.OpGet, &request, w, r)
	})
	http.HandleFunc("/getone", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.GetOneRequest
		dbHandler(bobb.OpGetOne, &request, w, r)
	})
	http.HandleFunc("/getall", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.GetAllRequest
		dbHandler(bobb.OpGetAll, &request, w, r)
	})
	http.HandleFunc("/getallkeys", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.GetAllKeysRequest
		dbHandler(bobb.OpGetAllKeys, &request, w, r)
	})
	http.HandleFunc("/getindex", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.GetIndexRequest
		dbHandler(bobb.OpGetIndex, &request, w, r)
	})

	// *** PUT REQUEST ROUTING *****************************************************

	http.HandleFunc("/put", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.PutRequest
		dbHandler(bobb.OpPut, &request, w, r)
	})
	http.HandleFunc("/putone", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.PutOneRequest
		dbHandler(bobb.OpPutOne, &request, w, r)
	})
	http.HandleFunc("/putindex", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.PutIndexRequest
		dbHandler(bobb.OpPutIndex, &request, w, r)
	})
	http.HandleFunc("/putbkts", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.PutBktsRequest
		dbHandler(bobb.OpPutBkts, &request, w, r)
	})

	// *** QRY REQUEST ROUTING *****************************************************

	http.HandleFunc("/qry", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.QryRequest
		dbHandler(bobb.OpQry, &request, w, r)
	})
	http.HandleFunc("/qryindex", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.QryIndexRequest
		dbHandler(bobb.OpQryIndex, &request, w, r)
	})

	// *** OTHER REQUEST ROUTING *****************************************************

	http.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.DeleteRequest
		dbHandler(bobb.OpDelete, &request, w, r)
	})
	http.HandleFunc("/bkt", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.BktRequest
		dbHandler(bobb.OpBkt, &request, w, r)
	})
	http.HandleFunc("/export", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.ExportRequest
		dbHandler(bobb.OpExport, &request, w, r)
	})
	http.HandleFunc("/copydb", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.CopyDBRequest
		dbHandler(bobb.OpCopyDB, &request, w, r)
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("invalid request url:", r.RequestURI)
		http.Error(w, "invalid request url", http.StatusNotFound)
	})

	// *** EXPERIMENTAL REQUEST ROUTING *****************************************************

	http.HandleFunc("/getvalues", func(w http.ResponseWriter, r *http.Request) {
		var request bobb.GetValuesRequest
		dbHandler(bobb.OpGetValues, &request, w, r)
	})

	// --- END REQUEST ROUTING --------------------------------------------------------------

	bobb.ServerStatus.Set("running") // see util.go

	fmt.Println("waiting for requests ...")
	log.Println(http.ListenAndServe(":"+settings.Port, nil))
}

func dbHandler(op string, request any, w http.ResponseWriter, r *http.Request) {

	if bobb.ServerStatus.Get() != "running" {
		http.Error(w, "server not accepting requests", http.StatusServiceUnavailable)
		return
	}
	bobb.Trace(op + " == request started ==")

	jsonContent, err := io.ReadAll(r.Body) // -> []byte
	if err != nil {
		log.Println("readall of request body failed", op, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(jsonContent, request)
	if err != nil {
		log.Println("json.Unmarshal failed", op, err)
		log.Println(string(jsonContent))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var response *bobb.Response
	var dbErr, txErr error
	var jsonData []byte
	var jsonErr error
	switch op {

	// *** GET OPERATIONS - SEE VIEW_HANDLERS.GO *****************************************************

	case bobb.OpGet:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.Get(tx, request.(*bobb.GetRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})
	case bobb.OpGetOne:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.GetOne(tx, request.(*bobb.GetOneRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})
	case bobb.OpGetAll:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.GetAll(tx, request.(*bobb.GetAllRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})
	case bobb.OpGetAllKeys:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.GetAllKeys(tx, request.(*bobb.GetAllKeysRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})
	case bobb.OpGetIndex:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.GetIndex(tx, request.(*bobb.GetIndexRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})

	// *** PUT OPERATIONS - SEE UPDT_HANDLERS.GO ****************************************************

	case bobb.OpPut:
		txErr = db.Update(func(tx *bolt.Tx) error {
			response, dbErr = bobb.Put(tx, request.(*bobb.PutRequest))
			jsonData, jsonErr = json.Marshal(response)
			return dbErr
		})
	case bobb.OpPutOne:
		txErr = db.Update(func(tx *bolt.Tx) error {
			response, dbErr = bobb.PutOne(tx, request.(*bobb.PutOneRequest))
			jsonData, jsonErr = json.Marshal(response)
			return dbErr
		})
	case bobb.OpPutIndex:
		txErr = db.Update(func(tx *bolt.Tx) error {
			response, dbErr = bobb.PutIndex(tx, request.(*bobb.PutIndexRequest))
			jsonData, jsonErr = json.Marshal(response)
			return dbErr
		})

	// *** QRY OPERATIONS - SEE VIEW_HANDLERS.GO *****************************************************

	case bobb.OpQry:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.Qry(tx, request.(*bobb.QryRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})
	case bobb.OpQryIndex:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.QryIndex(tx, request.(*bobb.QryIndexRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})

	// *** OTHER OPERATIONS *****************************************************

	case bobb.OpDelete:
		txErr = db.Update(func(tx *bolt.Tx) error {
			response, dbErr = bobb.Delete(tx, request.(*bobb.DeleteRequest))
			jsonData, jsonErr = json.Marshal(response)
			return dbErr
		})
	case bobb.OpBkt:
		txErr = db.Update(func(tx *bolt.Tx) error {
			response, dbErr = bobb.Bkt(tx, request.(*bobb.BktRequest))
			jsonData, jsonErr = json.Marshal(response)
			return dbErr
		})
	case bobb.OpExport:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.Export(tx, request.(*bobb.ExportRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})
	case bobb.OpCopyDB:
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.CopyDB(tx, request.(*bobb.CopyDBRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})
	case bobb.OpPutBkts:
		txErr = db.Update(func(tx *bolt.Tx) error {
			response, dbErr = bobb.PutBkts(tx, request.(*bobb.PutBktsRequest))
			jsonData, jsonErr = json.Marshal(response)
			return dbErr
		})

	// *** EXPERIMENTAL OPERATIONS - SEE EXPERIMENTAL.GO *****************************************************

	case bobb.OpGetValues: // GetValues is experimental request, see experimental.go
		txErr = db.View(func(tx *bolt.Tx) error {
			response = bobb.GetValues(tx, request.(*bobb.GetValuesRequest))
			jsonData, jsonErr = json.Marshal(response)
			return nil
		})
	}
	// when dbErr == bobb.DataError
	//  db.Update func returned non nil dbErr to cause rollback of updates
	//  this error indicates a problem with the input data, not a database error
	//  we want normal response to be returned to requestor rather than a server error
	if dbErr == bobb.DataError {
		log.Println("dataerror detected")
		dbErr = nil
		txErr = nil
	}
	if dbErr != nil {
		log.Println("DB Error Occured - Update transaction rolled  back", dbErr)
		http.Error(w, dbErr.Error(), http.StatusInternalServerError)
		return
	}
	if txErr != nil {
		log.Println("DB Transaction Error Occured", txErr)
		http.Error(w, txErr.Error(), http.StatusInternalServerError)
		return
	}
	// NOTE - marshalling is done before exiting bbolt transaction
	// references to db values are not preserved after txn ends
	if jsonErr != nil {
		log.Println("json.Marshal response failed", err)
		log.Println(response)
		return
	}
	if settings.CompressResponse {
		compressResponse(jsonData, w)
	} else {
		w.Write(jsonData)
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

	if settings.LogPath != "" {
		logFile, err = os.Create(settings.LogPath)
		if err != nil {
			log.Fatalln("logfile create failed", err)
		}
		log.SetOutput(logFile)
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

// compressResponse returns compressed http response
func compressResponse(jsonData []byte, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "gzip")
	compressor := gzipWriterPool.Get().(*gzip.Writer)
	compressor.Reset(w)
	_, err := compressor.Write(jsonData)
	compressor.Close()
	gzipWriterPool.Put(compressor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
