package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/jayposs/bobb"
	bo "github.com/jayposs/bobb/client"
)

const (
	qryTestBkt  = "qry_test"
	qryJoinBkt  = "qry_join_test"
	qryZipIndex = "qry_test_zip_index"
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

// qryLocs is the controlled dataset shared across all QryRequest subtests.
var qryLocs []Location

// qryNonNullStr is a non-nil target used to set NullTest on select records.
var qryNonNullStr = "set"

// ids extracts Id fields from a slice of Location records for order assertions.
func ids(locs []Location) []string {
	result := make([]string, len(locs))
	for i, loc := range locs {
		result[i] = loc.Id
	}
	return result
}

// setupQryTest deletes prior state, registers a zip index, loads 10 Location records
// and 3 Request records (for join subtests). Called once at the top of TestQry.
func setupQryTest(t *testing.T, httpClient *http.Client) {
	t.Helper()

	bo.DeleteBkt(httpClient, qryTestBkt)
	bo.DeleteBkt(httpClient, qryJoinBkt)
	bo.DeleteBkt(httpClient, qryZipIndex)
	bo.DeleteBkt(httpClient, qryZipIndex+"_inverted")

	// Register a zip index before loading data so PutRequest auto-creates entries.
	// KeySuffixWidth=4 makes each index key unique: "78701|0001", "78701|0002", etc.
	setting := bobb.IndexSetting{
		DataBkt:        qryTestBkt,
		IndexBkt:       qryZipIndex,
		KeySuffixWidth: 4,
		KeyFlds: []bobb.FldFormat{
			{FldName: "zip", FldType: bobb.FldTypeStr, Length: 5, StrOption: bobb.StrAsIs, UseDefault: bobb.DefaultAlways},
		},
	}
	resp, err := bo.Run(httpClient, bobb.OpIndexSetting, bobb.IndexSettingRequest{IndexSettings: []bobb.IndexSetting{setting}})
	if err := checkResp_qry_test(resp, err, "setupQryTest - IndexSettingRequest"); err != nil {
		t.Fatal(err)
	}

	// 10 Location records with fully known field values.
	// NullTest: nil  → JSON null  (records 001–008)
	// NullTest: &str → JSON value (records 009, 010)
	//
	// LocationType distribution: 1→{001,003,006,009}, 2→{002,005,008}, 3→{004,007,010}
	// State distribution:        TX→{001,005,008}, MA→{002}, IL→{003}, CO→{004},
	//                            AZ→{006}, WI→{007}, IN→{009}, FL→{010}
	qryLocs = []Location{
		{Id: "001", City: "Austin", St: "TX", Zip: "78701", LocationType: 1, LastActionDt: "2023-01-15", Address: "100 Main St"},
		{Id: "002", City: "Boston", St: "MA", Zip: "02101", LocationType: 2, LastActionDt: "2023-03-20", Address: "200 Oak Ave"},
		{Id: "003", City: "Chicago", St: "IL", Zip: "60601", LocationType: 1, LastActionDt: "2023-06-01", Address: "300 Elm Rd"},
		{Id: "004", City: "Denver", St: "CO", Zip: "80201", LocationType: 3, LastActionDt: "2022-12-10", Address: "400 Pine Dr"},
		{Id: "005", City: "Austin", St: "TX", Zip: "78702", LocationType: 2, LastActionDt: "2024-02-28", Address: "500 Cedar Blvd"},
		{Id: "006", City: "Flagstaff", St: "AZ", Zip: "86001", LocationType: 1, LastActionDt: "2023-09-05", Address: "600 Spruce Ln"},
		{Id: "007", City: "Green Bay", St: "WI", Zip: "54301", LocationType: 3, LastActionDt: "2023-04-14", Address: "700 Birch Way"},
		{Id: "008", City: "Houston", St: "TX", Zip: "77001", LocationType: 2, LastActionDt: "2022-11-30", Address: "800 Walnut St"},
		{Id: "009", City: "Indianapolis", St: "IN", Zip: "46201", LocationType: 1, LastActionDt: "2024-05-19", Address: "900 Maple Ct", NullTest: &qryNonNullStr},
		{Id: "010", City: "Jacksonville", St: "FL", Zip: "32201", LocationType: 3, LastActionDt: "2023-07-22", Address: "1000 Ash Ave", NullTest: &qryNonNullStr},
	}
	resp, err = bo.Put(httpClient, qryTestBkt, bo.SliceToJson(qryLocs), nil)
	if err := checkResp_qry_test(resp, err, "setupQryTest - Put locations"); err != nil {
		t.Fatal(err)
	}

	// 3 Request records for join subtests.
	// req001, req002 → Austin (001, 005); req003 → Houston (008)
	joinRecs := []Request{
		{Id: "req001", LocationId: "001", Description: "routine inspection"},
		{Id: "req002", LocationId: "005", Description: "urgent repair"},
		{Id: "req003", LocationId: "008", Description: "routine maintenance"},
	}
	resp, err = bo.Put(httpClient, qryJoinBkt, bo.SliceToJson(joinRecs), nil)
	if err := checkResp_qry_test(resp, err, "setupQryTest - Put join recs"); err != nil {
		t.Fatal(err)
	}
}

func teardownQryTest(httpClient *http.Client) {
	bo.DeleteBkt(httpClient, qryTestBkt)
	bo.DeleteBkt(httpClient, qryJoinBkt)
	bo.DeleteBkt(httpClient, qryZipIndex)
	bo.DeleteBkt(httpClient, qryZipIndex+"_inverted")
}

func TestQry(t *testing.T) {
	bo.BaseURL = "http://localhost:50555/"
	bo.Debug = false

	httpClient := &http.Client{}

	setupQryTest(t, httpClient)
	defer teardownQryTest(httpClient)

	// -----------------------------------------------------------------------
	t.Run("FindStr", func(t *testing.T) {
		// FindStartsWith: city starts with "a" → Austin 001, 005 = 2
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindStartsWith, "a"),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindStartsWith"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 2 {
			t.Errorf("FindStartsWith: expected 2, got %d", resp.GetCnt)
		}

		// FindEndsWith: city ends with "n" → Austin (001,005), Boston (002), Houston (008) = 4
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindEndsWith, "n"),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindEndsWith"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 4 {
			t.Errorf("FindEndsWith: expected 4, got %d", resp.GetCnt)
		}

		// FindContains: city contains "in" → Austin (001,005), Indianapolis (009) = 3
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindContains, "in"),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindContains"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 3 {
			t.Errorf("FindContains: expected 3, got %d", resp.GetCnt)
		}

		// FindContainsWord: city contains whole word "bay" → Green Bay (007) = 1
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindContainsWord, "bay"),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindContainsWord"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 1 {
			t.Errorf("FindContainsWord: expected 1, got %d", resp.GetCnt)
		}
		results := bo.JsonToSlice(resp.Recs, Location{})
		if results[0].Id != "007" {
			t.Errorf("FindContainsWord: expected id 007, got %s", results[0].Id)
		}

		// FindMatches: city matches "austin" → 001, 005 = 2
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindMatches, "austin"),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindMatches"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 2 {
			t.Errorf("FindMatches: expected 2, got %d", resp.GetCnt)
		}

		// FindBefore: city (lowercase) before "chicago" → Austin (001,005), Boston (002) = 3
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindBefore, "chicago"),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindBefore"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 3 {
			t.Errorf("FindBefore: expected 3, got %d", resp.GetCnt)
		}

		// FindAfter: city (lowercase) after "houston" → Indianapolis (009), Jacksonville (010) = 2
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindAfter, "houston"),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindAfter"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 2 {
			t.Errorf("FindAfter: expected 2, got %d", resp.GetCnt)
		}

		// FindInStrList: st in [TX, FL] → 001, 005, 008 (TX), 010 (FL) = 4
		// default StrLowerCase lowercases both record values and list values
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "st", bobb.FindInStrList, []string{"TX", "FL"}),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindInStrList"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 4 {
			t.Errorf("FindInStrList: expected 4, got %d", resp.GetCnt)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("FindInt", func(t *testing.T) {
		// FindEquals: locationType == 1 → 001, 003, 006, 009 = 4
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "locationType", bobb.FindEquals, 1),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindEquals"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 4 {
			t.Errorf("FindEquals: expected 4, got %d", resp.GetCnt)
		}

		// FindLessThan: locationType < 2 → type-1 records = 4
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "locationType", bobb.FindLessThan, 2),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindLessThan"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 4 {
			t.Errorf("FindLessThan: expected 4, got %d", resp.GetCnt)
		}

		// FindGreaterThan: locationType > 2 → type-3 records (004, 007, 010) = 3
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "locationType", bobb.FindGreaterThan, 2),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindGreaterThan"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 3 {
			t.Errorf("FindGreaterThan: expected 3, got %d", resp.GetCnt)
		}

		// FindInIntList: locationType in [1, 3] → 001, 003, 004, 006, 007, 009, 010 = 7
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "locationType", bobb.FindInIntList, []int{1, 3}),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindInIntList"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 7 {
			t.Errorf("FindInIntList: expected 7, got %d", resp.GetCnt)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("FindSpecial", func(t *testing.T) {
		// FindExists: nulltest field is present in every record (no omitempty), even when null = 10
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "nulltest", bobb.FindExists, ""),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindExists"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 10 {
			t.Errorf("FindExists: expected 10, got %d", resp.GetCnt)
		}

		// FindIsNull: nulltest is JSON null for records 001–008 = 8
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "nulltest", bobb.FindIsNull, ""),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindIsNull"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 8 {
			t.Errorf("FindIsNull: expected 8, got %d", resp.GetCnt)
		}

		// Not flag: city NOT starts with "a" → all except Austin records (001, 005) = 8
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindStartsWith, "a", bobb.FindNot),
			},
		})
		if err := checkResp_qry_test(resp, err, "Not flag"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 8 {
			t.Errorf("Not flag: expected 8, got %d", resp.GetCnt)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("FindOrConditions", func(t *testing.T) {
		// A record is kept if it meets FindConditions OR FindOrConditions.
		// FindConditions:   city == "austin"       → 001, 005
		// FindOrConditions: locationType == 3      → 004, 007, 010
		// Union:                                     001, 004, 005, 007, 010 = 5
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "city", bobb.FindMatches, "austin"),
				bo.Find(nil, "locationType", bobb.FindEquals, 3),
			},
		})
		if err := checkResp_qry_test(resp, err, "FindOrConditions"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 5 {
			t.Errorf("FindOrConditions: expected 5, got %d", resp.GetCnt)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("StrOptions", func(t *testing.T) {
		// StrAsIs: city "Green Bay" matches record "Green Bay" exactly = 1
		criteria1 := bobb.FindGroup{
			bobb.FindCondition{Fld: "city", Op: bobb.FindMatches, ValStr: "Green Bay", StrOption: bobb.StrAsIs},
		}
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			Criteria: []bobb.FindGroup{criteria1},
		})
		if err := checkResp_qry_test(resp, err, "StrAsIs match"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 1 {
			t.Errorf("StrAsIs match: expected 1, got %d", resp.GetCnt)
		}

		// StrAsIs: "green bay" (wrong case) does not match "Green Bay" = 0
		criteria2 := bobb.FindGroup{
			bobb.FindCondition{Fld: "city", Op: bobb.FindMatches, ValStr: "green bay", StrOption: bobb.StrAsIs},
		}
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			Criteria: []bobb.FindGroup{criteria2},
		})
		if err := checkResp_qry_test(resp, err, "StrAsIs no-match"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 0 {
			t.Errorf("StrAsIs no-match: expected 0, got %d", resp.GetCnt)
		}

		// StrPlain: "Green-Bay" → PlainString → "greenbay" matches record "Green Bay" → "greenbay" = 1
		criteria3 := bobb.FindGroup{
			bobb.FindCondition{Fld: "city", Op: bobb.FindMatches, ValStr: "Green Bay", StrOption: bobb.StrPlain},
		}
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			Criteria: []bobb.FindGroup{criteria3},
		})
		if err := checkResp_qry_test(resp, err, "StrPlain"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 1 {
			t.Errorf("StrPlain: expected 1, got %d", resp.GetCnt)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("SortStr", func(t *testing.T) {
		// Sort city ascending: Austin, Austin, Boston, …, Jacksonville
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			SortKeys: bo.Sort(nil, "city", bobb.SortAscStr),
		})
		if err := checkResp_qry_test(resp, err, "SortAscStr"); err != nil {
			t.Error(err)
		}
		results := bo.JsonToSlice(resp.Recs, Location{})
		if len(results) != 10 {
			t.Fatalf("SortAscStr: expected 10 recs, got %d", len(results))
		}
		if results[0].City != "Austin" || results[1].City != "Austin" {
			t.Errorf("SortAscStr: expected first two Austin, got %s, %s", results[0].City, results[1].City)
		}
		if results[9].City != "Jacksonville" {
			t.Errorf("SortAscStr: expected last Jacksonville, got %s", results[9].City)
		}
		for i := 1; i < len(results); i++ {
			if results[i].City < results[i-1].City {
				t.Errorf("SortAscStr: order broken at pos %d: %q before %q", i, results[i-1].City, results[i].City)
			}
		}

		// Sort city descending: Jacksonville, …, Austin, Austin
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			SortKeys: bo.Sort(nil, "city", bobb.SortDescStr),
		})
		if err := checkResp_qry_test(resp, err, "SortDescStr"); err != nil {
			t.Error(err)
		}
		results = bo.JsonToSlice(resp.Recs, Location{})
		if results[0].City != "Jacksonville" {
			t.Errorf("SortDescStr: expected first Jacksonville, got %s", results[0].City)
		}
		for i := 1; i < len(results); i++ {
			if results[i].City > results[i-1].City {
				t.Errorf("SortDescStr: order broken at pos %d: %q after %q", i, results[i].City, results[i-1].City)
			}
		}

		// Multi-key sort: locationType asc, then city asc within each type.
		// Expected first: type-1 Austin (001); last: type-3 Jacksonville (010).
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			SortKeys: bo.Sort(bo.Sort(nil, "locationType", bobb.SortAscInt), "city", bobb.SortAscStr),
		})
		if err := checkResp_qry_test(resp, err, "MultiSort"); err != nil {
			t.Error(err)
		}
		results = bo.JsonToSlice(resp.Recs, Location{})
		if results[0].Id != "001" {
			t.Errorf("MultiSort: expected first id 001, got %s (city=%s type=%d)", results[0].Id, results[0].City, results[0].LocationType)
		}
		if results[9].Id != "010" {
			t.Errorf("MultiSort: expected last id 010, got %s (city=%s type=%d)", results[9].Id, results[9].City, results[9].LocationType)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("SortInt", func(t *testing.T) {
		// Sort locationType ascending: 1,1,1,1,2,2,2,3,3,3
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			SortKeys: bo.Sort(nil, "locationType", bobb.SortAscInt),
		})
		if err := checkResp_qry_test(resp, err, "SortAscInt"); err != nil {
			t.Error(err)
		}
		results := bo.JsonToSlice(resp.Recs, Location{})
		if len(results) != 10 {
			t.Fatalf("SortAscInt: expected 10 recs, got %d", len(results))
		}
		if results[0].LocationType != 1 {
			t.Errorf("SortAscInt: expected first locationType 1, got %d", results[0].LocationType)
		}
		if results[9].LocationType != 3 {
			t.Errorf("SortAscInt: expected last locationType 3, got %d", results[9].LocationType)
		}
		for i := 1; i < len(results); i++ {
			if results[i].LocationType < results[i-1].LocationType {
				t.Errorf("SortAscInt: order broken at pos %d", i)
			}
		}

		// Sort locationType descending: 3,3,3,2,2,2,1,1,1,1
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			SortKeys: bo.Sort(nil, "locationType", bobb.SortDescInt),
		})
		if err := checkResp_qry_test(resp, err, "SortDescInt"); err != nil {
			t.Error(err)
		}
		results = bo.JsonToSlice(resp.Recs, Location{})
		if results[0].LocationType != 3 {
			t.Errorf("SortDescInt: expected first locationType 3, got %d", results[0].LocationType)
		}
		if results[9].LocationType != 1 {
			t.Errorf("SortDescInt: expected last locationType 1, got %d", results[9].LocationType)
		}
		for i := 1; i < len(results); i++ {
			if results[i].LocationType > results[i-1].LocationType {
				t.Errorf("SortDescInt: order broken at pos %d", i)
			}
		}
	})

	// -----------------------------------------------------------------------
	t.Run("LimitTop", func(t *testing.T) {
		// Limit 3, no sort: first 3 records in key order → 001, 002, 003
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Limit:   3,
		})
		if err := checkResp_qry_test(resp, err, "Limit"); err != nil {
			t.Error(err)
		}
		results := bo.JsonToSlice(resp.Recs, Location{})
		if !slices.Equal(ids(results), []string{"001", "002", "003"}) {
			t.Errorf("Limit: expected [001 002 003], got %v", ids(results))
		}
		// NextKey must point to the next record after the limit
		if resp.NextKey != "004" {
			t.Errorf("NextKey: expected 004, got %q", resp.NextKey)
		}

		// Top 3 with city sort: 3 alphabetically first cities → Austin(x2), Boston
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			SortKeys: bo.Sort(nil, "city", bobb.SortAscStr),
			Top:      3,
		})
		if err := checkResp_qry_test(resp, err, "Top"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 3 {
			t.Errorf("Top: expected 3, got %d", resp.GetCnt)
		}
		results = bo.JsonToSlice(resp.Recs, Location{})
		if results[0].City != "Austin" || results[2].City != "Boston" {
			t.Errorf("Top: unexpected order: cities are %v", func() []string {
				c := make([]string, len(results))
				for i, r := range results {
					c[i] = r.City
				}
				return c
			}())
		}

		// Limit 5 + city sort: Limit restricts candidates to the first 5 keys (001–005),
		// then sorts those: Austin(001), Austin(005), Boston(002), Chicago(003), Denver(004).
		// Denver must be last since no record after 005 is included.
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			Limit:    5,
			SortKeys: bo.Sort(nil, "city", bobb.SortAscStr),
		})
		if err := checkResp_qry_test(resp, err, "Limit+Sort"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 5 {
			t.Errorf("Limit+Sort: expected 5, got %d", resp.GetCnt)
		}
		results = bo.JsonToSlice(resp.Recs, Location{})
		if results[4].City != "Denver" {
			t.Errorf("Limit+Sort: expected last city Denver, got %s", results[4].City)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("KeyRange", func(t *testing.T) {
		// StartKey "003", EndKey "007" → records 003, 004, 005, 006, 007 = 5
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			StartKey: "003",
			EndKey:   "007",
		})
		if err := checkResp_qry_test(resp, err, "KeyRange"); err != nil {
			t.Error(err)
		}
		results := bo.JsonToSlice(resp.Recs, Location{})
		if !slices.Equal(ids(results), []string{"003", "004", "005", "006", "007"}) {
			t.Errorf("KeyRange: expected 003–007, got %v", ids(results))
		}

		// StartKey == EndKey == "005": ReadLoop uses prefix match, returns only record 005
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			StartKey: "005",
			EndKey:   "005",
		})
		if err := checkResp_qry_test(resp, err, "PrefixMatch"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 1 {
			t.Errorf("PrefixMatch: expected 1, got %d", resp.GetCnt)
		}
		results = bo.JsonToSlice(resp.Recs, Location{})
		if results[0].Id != "005" {
			t.Errorf("PrefixMatch: expected id 005, got %s", results[0].Id)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("CountOnly", func(t *testing.T) {
		// locationType == 1: 001, 003, 006, 009 = 4; CountOnly returns count, Recs is nil
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "locationType", bobb.FindEquals, 1),
			},
			CountOnly: true,
		})
		if err := checkResp_qry_test(resp, err, "CountOnly"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 4 {
			t.Errorf("CountOnly: expected GetCnt 4, got %d", resp.GetCnt)
		}
		if resp.Recs != nil {
			t.Errorf("CountOnly: expected nil Recs, got %d recs", len(resp.Recs))
		}
	})

	// -----------------------------------------------------------------------
	t.Run("JoinsBeforeFind", func(t *testing.T) {
		// Join Request.locationId → Location.city → stored as "location_city" in request record.
		// FindCondition on location_city (available because join runs before find).
		// city starts with "a" → Austin locations (001, 005) → req001, req002 = 2
		join := bobb.Join{
			JoinBkt: qryTestBkt,
			JoinFld: "locationId",
			FromFld: "city",
			ToFld:   "location_city",
		}
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:         qryJoinBkt,
			JoinsBeforeFind: []bobb.Join{join},
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "location_city", bobb.FindStartsWith, "a"),
			},
		})
		if err := checkResp_qry_test(resp, err, "JoinsBeforeFind"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 2 {
			t.Errorf("JoinsBeforeFind: expected 2, got %d", resp.GetCnt)
		}
		// Verify joined field appears in the returned record
		var req Request
		if jsonErr := json.Unmarshal(resp.Recs[0], &req); jsonErr != nil {
			t.Fatalf("JoinsBeforeFind: unmarshal error: %v", jsonErr)
		}
		if req.LocationCity == "" {
			t.Error("JoinsBeforeFind: location_city not present in returned record")
		}
	})

	// -----------------------------------------------------------------------
	t.Run("JoinsAfterFind", func(t *testing.T) {
		// Find first: description contains "urgent" → req002 only (Austin, 005).
		// Then join adds location_city to that record; join value is NOT available during find.
		join := bobb.Join{
			JoinBkt: qryTestBkt,
			JoinFld: "locationId",
			FromFld: "city",
			ToFld:   "location_city",
		}
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:        qryJoinBkt,
			JoinsAfterFind: []bobb.Join{join},
			Criteria: []bobb.FindGroup{
				bo.Find(nil, "description", bobb.FindContains, "urgent"),
			},
		})
		if err := checkResp_qry_test(resp, err, "JoinsAfterFind"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 1 {
			t.Errorf("JoinsAfterFind: expected 1, got %d", resp.GetCnt)
		}
		var req Request
		if jsonErr := json.Unmarshal(resp.Recs[0], &req); jsonErr != nil {
			t.Fatalf("JoinsAfterFind: unmarshal error: %v", jsonErr)
		}
		if req.LocationCity != "Austin" {
			t.Errorf("JoinsAfterFind: expected location_city Austin, got %q", req.LocationCity)
		}
	})

	// -----------------------------------------------------------------------
	t.Run("IndexBkt", func(t *testing.T) {
		// Zip index keys: "78701|NNNN" (001) and "78702|NNNN" (005).
		// Range "78"–"79" captures both Austin zip codes and nothing else.
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			IndexBkt: qryZipIndex,
			StartKey: "78",
			EndKey:   "79",
		})
		if err := checkResp_qry_test(resp, err, "IndexBkt"); err != nil {
			t.Error(err)
		}
		if resp.GetCnt != 2 {
			t.Errorf("IndexBkt: expected 2, got %d", resp.GetCnt)
		}
		results := bo.JsonToSlice(resp.Recs, Location{})
		for _, r := range results {
			if r.City != "Austin" {
				t.Errorf("IndexBkt: expected Austin record, got city=%s id=%s", r.City, r.Id)
			}
		}
	})

	// -----------------------------------------------------------------------
	t.Run("ErrorHandling", func(t *testing.T) {
		// Missing bucket → StatusFail
		resp, err := bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: "nonexistent_bkt_qrytest",
		})
		if err != nil {
			t.Errorf("missing bkt: unexpected http error: %v", err)
		}
		if resp.Status != bobb.StatusFail {
			t.Errorf("missing bkt: expected StatusFail, got %s", resp.Status)
		}

		// Invalid FindCondition.Op → validation fails before scan → StatusFail
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName: qryTestBkt,
			Criteria: []bobb.FindGroup{
				{bobb.FindCondition{Fld: "city", Op: "badop"}},
			},
		})
		if err != nil {
			t.Errorf("invalid op: unexpected http error: %v", err)
		}
		if resp.Status != bobb.StatusFail {
			t.Errorf("invalid op: expected StatusFail, got %s", resp.Status)
		}

		// DefaultNever on a field absent from every record: each record produces an error.
		// ErrLimit=-1 (MaxErrs) lets all records be scanned → StatusWarning + populated Errs.
		criteria := bobb.FindGroup{
			bobb.FindCondition{Fld: "nonexistent_str_fld", Op: bobb.FindEquals, ValInt: 1, UseDefault: bobb.DefaultNever},
		}
		resp, err = bo.Run(httpClient, bobb.OpQry, bobb.QryRequest{
			BktName:  qryTestBkt,
			Criteria: []bobb.FindGroup{criteria},
			ErrLimit: -1,
		})
		if err != nil {
			t.Errorf("DefaultNever: unexpected http error: %v", err)
		}
		if resp.Status != bobb.StatusWarning {
			t.Errorf("DefaultNever: expected StatusWarning, got %s", resp.Status)
		}
		if len(resp.Errs) == 0 {
			t.Error("DefaultNever: expected errors in resp.Errs, got none")
		}
	})
}

// checkResp_qry_test calls log.Fatalln if the response status is not Ok or err is non-nil.
func checkResp_qry_test(resp *bobb.Response, err error, funcName string) error {
	if resp == nil {
		return fmt.Errorf("%s: nil response, error: %v", funcName, err)
	}
	if !(resp.Status == bobb.StatusOk && err == nil) {
		return fmt.Errorf("%s: failed: %s, error: %v", funcName, resp.Msg, err)
	}
	return nil
}
