// example pgm directly interacting with database

package main

import (
	"log"

	"github.com/jayposs/bobb"
	bolt "go.etcd.io/bbolt"
)

func main() {

	db, err := bolt.Open("../bobb_server/demo_copy.db", 0600, nil)
	if err != nil {
		log.Fatalln("db open failed", err)
	}
	bktName := "location"
	indexName := "location_zip_index"
	startKey := "10000" // index key
	endKey := "40000"
	limit := 0

	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(bktName))
		if bkt == nil {
			log.Fatalln("bkt not found", bktName)
		}
		index := tx.Bucket([]byte(indexName))
		if index == nil {
			log.Fatalln("indexBkt not found", indexName)
		}
		readLoop := bobb.NewReadLoop(bkt, index)
		k, v, bErr := readLoop.Start(startKey, endKey, limit)
		for k != nil {
			if bErr != nil {
				log.Println("error", bErr)
				break
			}
			log.Println(string(v))
			k, v, bErr = readLoop.Next()
		}
		return nil
	})
}
