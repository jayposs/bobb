package datatypes

import (
	"encoding/json"
	"fmt"

	"github.com/jayposs/bobb"
)

// used by exportcsv/exportcsv.go pgm

// CsvDataRec unMarshals jsonBytes into instance of specific dataType based on bktName.
// All dataTypes must implement the bobb.CsvExport interface (see types.go).
// NOTE - this func is custom per project, using its bucket names and datatypes
func CsvDataRec(rec []byte, bktName string) (bobb.CsvExport, error) {
	var err error
	switch bktName {
	case "location":
		dataRec := Location{}
		err = json.Unmarshal(rec, &dataRec)
		return dataRec, err
	case "request":
		dataRec := Request{}
		err = json.Unmarshal(rec, &dataRec)
		return dataRec, err
	}
	return nil, fmt.Errorf("invalid bktName")
}
