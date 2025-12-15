// Data types used by demo.go.
// Records in db are json.Marshalled instances of these types.

package main

import (
	"errors"
)

// Request type used to test joins with Location
type Request struct {
	Id              string `json:"id"`
	LocationId      string `json:"locationId"` // key of related rec in Location bkt
	Description     string `json:"description"`
	LocationSt      string `json:"location_st,omitempty"`      // loaded from joined Location
	LocationCity    string `json:"location_city,omitempty"`    // loaded from joined Location
	LocationAddress string `json:"location_address,omitempty"` // loaded from joined Location
}

func (rec Request) RecId() string {
	return rec.Id
}

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
}

func (rec Location) RecId() string {
	return rec.Id
}

// Update method uses a map to update specific fields in a Location record.
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

type Order struct {
	Id         string `json:"id"` // customerid_bktseqno
	OrderDate  string `json:"orderDate"`
	CustomerId string `json:"customerId"`
}

func (rec Order) RecId() string {
	return rec.Id
}

type OrderItem struct {
	Id        string `json:"id"` // orderid_itemno
	OrderId   string `json:"orderId"`
	ItemNo    int    `json:"itemNo"`
	ProductId string `json:"productId"`
	Qty       int    `json:"qty"`
}

func (rec OrderItem) RecId() string {
	return rec.Id
}
