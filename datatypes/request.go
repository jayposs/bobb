package datatypes

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
