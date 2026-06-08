package datatypes

import (
	"encoding/json"
	"log"

	"github.com/jayposs/bobb"
)

// funcs used by exportcsv/exportcsv.go pgm

func toCsvExport(rec []byte, dataRec any) bobb.CsvExport {
	if err := json.Unmarshal(rec, dataRec); err != nil {
		log.Println("toCsvExport ", err)
	}
	return dataRec.(bobb.CsvExport)
}

// CsvDataRec unMarshals jsonBytes into instance of specific dataType based on bktName.
// All dataTypes must implement the bobb.CsvExport interface (see types.go).
// NOTE - this func is custom per project, using its bucket names and datatypes
func CsvDataRec(rec []byte, bktName string) bobb.CsvExport {
	switch bktName {
	case "location":
		return toCsvExport(rec, Location{})
	}
	return nil
}
