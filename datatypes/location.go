package datatypes

import (
	"errors"
	"slices"
	"strconv"
)

// Agent is internal to Location
type Agent struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type Location struct {
	Id           string   `json:"id"`
	Address      string   `json:"address"`
	City         string   `json:"city"`
	St           string   `json:"st"`
	Zip          string   `json:"zip"`
	LocationType int      `json:"locationType"`
	LastActionDt string   `json:"lastActionDt"` // "yyyy-mm-dd"
	Notes        []string `json:"notes"`
	LocAgent     Agent    `json:"agent"`
	NullTest     *string  `json:"nulltest"` // used for testing FindIsNull
	A            string   `json:"a"`        // used for random testing
	B            string   `json:"b"`
	C            string   `json:"c"`
	D            string   `json:"d"`
	ManagerId    string   `json:"managerId"`
	ManagerName  string   `json:"manager_name,omitempty"`  // used for join testing in bigqry.go
	ManagerLevel string   `json:"manager_level,omitempty"` // used for join testing in bigqry.go
	Long1        string   // used for random testing
	Long2        string
	Long3        string
	Int1         int
	Int2         int
}

// RecId method required for
func (rec Location) RecId() string {
	return rec.Id
}

func (rec *Location) CsvHeader(includeJoins bool) []string {
	csvHeader := []string{"Id", "Address", "City", "St", "Zip", "LocType"}
	if includeJoins {
		return slices.Concat(csvHeader, []string{"ManagerName"})
	}
	return csvHeader
}

func (rec *Location) CsvData(includeJoins bool) []string {
	locType := strconv.Itoa(rec.LocationType)
	csvData := []string{rec.Id, rec.Address, rec.City, rec.St, rec.Zip, locType}
	if includeJoins {
		return slices.Concat(csvData, []string{rec.ManagerName})
	}
	return csvData
}

// Update method uses a map to update specific fields in a Location record.
// This method is just an example of how an update of specific values might work.
func (rec *Location) Update(updates map[string]any) error {
	var err error
	var ok bool
	for k, v := range updates {
		switch k {
		case "address":
			rec.Address, ok = v.(string)
		case "city":
			rec.City, ok = v.(string)
		case "st":
			rec.St, ok = v.(string)
		case "zip":
			rec.Zip, ok = v.(string)
		case "locationType":
			rec.LocationType, ok = v.(int)
		case "lastActionDt":
			rec.LastActionDt, ok = v.(string)
		case "notes":
			rec.Notes, ok = v.([]string)
		default:
			err = errors.New("Location.Update invalid update fld name:" + k)
		}
		if !ok {
			err = errors.New(("Location.Update invalid value type for:" + k))
			break
		}
	}
	return err
}

type Manager struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Level string `json:"level"`
}

func (rec Manager) RecId() string {
	return rec.Id
}
