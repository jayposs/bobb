package bobb

/*
QryRequest is used to filter and sort records from a bucket.
Data records must be json objects.
See QryRequest type for details.
*/

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/valyala/fastjson"
	bolt "go.etcd.io/bbolt"
)

// SortKey is used by QryRequest to sort results.
// Only fields of type string or int are currently supported.
// String values are converted to "plain" string (lowercase, alphanumeric).
type SortKey struct {
	Fld        string // name of field
	Dir        string // direction (asc/desc) and field type (str/int)
	UseDefault string // controls what value is used when fld NotFound or IsNull, see codes.go
}

// FindCondition is used by QryRequest to define select criteria.
// Each record's Fld value is compared to FindCondition value.
// See codes.go for Find* op code constants.
type FindCondition struct {
	Fld        string   // field containing compare value
	Op         string   // defines match operation and value type
	ValStr     string   // for string ops, this value also converted based on StrOption
	ValInt     int      // for int Ops
	StrList    []string // used by op FindInStrList
	IntList    []int    // used by op FindInIntList
	Not        bool     // exclude records that meet condition
	UseDefault string   // controls what default value is used, see Default* codes in codes.go
	StrOption  string   // controls string conversion, see Str* codes in codes.go, default StrLowerCase
}

type FindGroup []FindCondition // QryRequest can have multiple FindGroups that are ORed together

type Join struct {
	JoinBkt    string // name of related bkt where value is pulled from
	JoinFld    string // fld in primary rec containing key value of join rec
	FromFld    string // fld in join rec where value comes from
	ToFld      string // fld in primary rec where value is loaded
	UseDefault bool   // if join problem, use default value, no error
}

// QryRequest is used to filter and sort recs from a bkt.
// Start/End keys define range of keys to read.
// If StartKey == EndKey, key prefix must match StartKey.
type QryRequest struct {
	BktName         string      // primary data bkt
	IndexBkt        string      // optional index bkt name, start/end keys use index
	Criteria        []FindGroup // if a record meets all conditions in any FindGroup, it is included in results
	SortKeys        []SortKey   // defines sort order, if omitted ressults returned in key order
	StartKey        string      // begin range, 1st key >=
	EndKey          string      // end range, last key <=
	Limit           int         // limits results before sort step
	Top             int         // limits results after sort step
	ErrLimit        int         // run stops when ErrLimit exceeded, default 0, settings.MaxErrs limit if -1
	JoinsBeforeFind []Join      // joined values can be used in find step (adds processing time)
	JoinsAfterFind  []Join      // joined values can be used for sort step but not find step
	CountOnly       bool        // if true, Response.Recs is nil, count in Response.GetCnt
}

func (req QryRequest) IsUpdtReq() bool {
	return false
}

// SortRec is used when QryRequest has SortKeys
type SortRec struct {
	SortOn []string // Values extracted from record using SortKeys
	Value  []byte   // record value
}

func (req *QryRequest) Run(tx *bolt.Tx) (*Response, error) {

	resp := new(Response)

	bkt := openBkt(tx, resp, req.BktName)
	if bkt == nil {
		return resp, nil
	}
	var index *bolt.Bucket
	if req.IndexBkt != "" {
		index = openBkt(tx, resp, req.IndexBkt)
		if index == nil {
			return resp, nil
		}
	}
	var err error

	var validatedCriteria []FindGroup
	if len(req.Criteria) > 0 {
		validatedCriteria = make([]FindGroup, len(req.Criteria))
		for i, group := range req.Criteria {
			validatedCriteria[i], err = validateFindConditions(group)
			if err != nil {
				resp.Status = StatusFail
				resp.Msg = fmt.Sprintf("invalid Criteria group %d - %s", i, err.Error())
				return resp, nil
			}
		}
	}

	var validatedSortKeys []SortKey
	// validate and set defaults
	validatedSortKeys, err = validateSortKeys(req.SortKeys)
	if err != nil {
		resp.Status = StatusFail
		resp.Msg = "invalid SortKeys- " + err.Error()
		return resp, nil
	}

	var sortRecs []SortRec
	if len(validatedSortKeys) > 0 {
		sortRecs = make([]SortRec, 0, InitialRespRecsSize)
	} else {
		resp.Recs = make([][]byte, 0, InitialRespRecsSize)
	}
	parser := parserPool.Get() // defined in util.go
	defer parserPool.Put(parser)

	var joinParser *fastjson.Parser // used by loadJoinValues func
	if len(req.JoinsBeforeFind) > 0 || len(req.JoinsAfterFind) > 0 {
		joinParser = parserPool.Get()
		defer parserPool.Put(joinParser)
	}

	if req.ErrLimit == -1 { // see server/bobb_settings.json for MaxErrs value (defined in util.go)
		req.ErrLimit = MaxErrs
	}

	var parsedRec *fastjson.Value
	var bErr *BobbErr
	var keep bool // used to indicate if rec meets either FindConditions, FindOrConditions
	var sortVals []string

	Trace("__ Qry find start __")

	var k, v []byte // key, value returned by readLoop

	readLoop := NewReadLoop(bkt, index)
	k, v, bErr = readLoop.Start(req.StartKey, req.EndKey, req.Limit)
	if bErr != nil {
		resp.Errs = append(resp.Errs, *bErr)
		k, v, bErr = readLoop.Next()
	}

	for k != nil {
		if len(resp.Errs) > req.ErrLimit {
			resp.Status = StatusFail
			resp.Msg = "too many errors, see resp.Errs for details"
			return resp, nil
		}
		if bErr != nil { // triggered when readLoop returns errCode
			resp.Errs = append(resp.Errs, *bErr)
			k, v, bErr = readLoop.Next()
			continue
		}
		// parse data record
		parsedRec, err = parser.ParseBytes(v)
		if err != nil {
			bErr = e(ErrParseRec, err.Error(), k, v)
			resp.Errs = append(resp.Errs, *bErr)
			k, v, bErr = readLoop.Next()
			continue
		}

		// add joined values before find step
		if len(req.JoinsBeforeFind) > 0 {
			v, bErr = loadJoinValues(tx, parsedRec, req.JoinsBeforeFind, joinParser)
			if bErr != nil {
				bErr.Key, bErr.Val = k, v
				resp.Errs = append(resp.Errs, *bErr)
				k, v, bErr = readLoop.Next()
				continue
			}
		}

		if len(validatedCriteria) == 0 {
			keep = true // no criteria, all recs meet criteria
		} else {
			for _, findGroup := range validatedCriteria {
				keep, bErr = parsedRecFind(parsedRec, findGroup)
				if bErr != nil || keep { // if error or rec meets criteria, no need to check other findGroups
					break
				}
			}
		}
		if bErr != nil {
			bErr.Key, bErr.Val = k, v
			resp.Errs = append(resp.Errs, *bErr)
			k, v, bErr = readLoop.Next()
			continue
		}
		if !keep {
			k, v, bErr = readLoop.Next()
			continue
		}
		readLoop.Count++

		// add joined values after find step
		if len(req.JoinsAfterFind) > 0 {
			v, bErr = loadJoinValues(tx, parsedRec, req.JoinsAfterFind, joinParser)
			if bErr != nil {
				bErr.Key, bErr.Val = k, v
				resp.Errs = append(resp.Errs, *bErr)
				k, v, bErr = readLoop.Next()
				continue
			}
		}

		// if Sorting, extract values used for sorting, else add value to resp.Recs
		if len(validatedSortKeys) > 0 {
			sortVals, bErr = extractSortVals(parsedRec, validatedSortKeys)
			if bErr != nil {
				bErr.Key, bErr.Val = k, v
				resp.Errs = append(resp.Errs, *bErr)
				k, v, bErr = readLoop.Next()
				continue
			}
			sortRecs = append(sortRecs, SortRec{SortOn: sortVals, Value: v})
		} else {
			resp.Recs = append(resp.Recs, v)
		}

		k, v, bErr = readLoop.Next()
	}

	if readLoop.NextKey != nil { // ReadLoop.NextKey is loaded by readLoop.Next() at end of range.
		resp.NextKey = string(readLoop.NextKey)
	}
	Trace("__ Qry find done __")

	if len(validatedSortKeys) > 0 {
		qrySort(validatedSortKeys, sortRecs)
		count := len(sortRecs)
		if req.Top > 0 && req.Top < count {
			count = req.Top
		}
		resp.Recs = make([][]byte, count)
		for i := range count {
			resp.Recs[i] = sortRecs[i].Value
		}
	}
	resp.GetCnt = len(resp.Recs)
	if req.CountOnly {
		resp.Recs = nil // clear recs
	}
	if len(resp.Errs) > 0 {
		resp.Status = StatusWarning
		resp.Msg = "see resp.Errs for details"
	} else {
		resp.Status = StatusOk
	}
	return resp, nil
}

func qrySort(sortKeys []SortKey, sortRecs []SortRec) {
	Trace("~ qry sort start ~")

	sortDir := make([]int, len(sortKeys))
	for i, sortKey := range sortKeys {
		if slices.Contains(DescSortCodes, sortKey.Dir) {
			sortDir[i] = -1
		} else {
			sortDir[i] = 1
		}
	}
	slices.SortFunc(sortRecs, func(a, b SortRec) (n int) { // slices pkg added in Go 1.21
		for i := range sortKeys {
			n = strings.Compare(a.SortOn[i], b.SortOn[i])
			if n == 0 {
				continue // sort vals match
			}
			n = sortDir[i] * n
			break
		}
		return
	})
	Trace("~ qry sort done ~")
}

// loadJoinValues adds values from a different bucket to parsed primary data record.
// The joins parm specifies join information. See Join type for details.
// If UseDefault true, on error, no join value(s) added to rec and no error returned.
// Client side UnMarshal will load default (zero value) into struct join flds.
func loadJoinValues(tx *bolt.Tx, parsedRec *fastjson.Value, joins []Join, joinParser *fastjson.Parser) (recBytes []byte, bErr *BobbErr) {
	var err error
	var prevJoinBkt string // name of prevJoinBkt
	var prevJoinFld string
	var joinBkt *bolt.Bucket
	var joinRec []byte          // record from join bkt
	var joinKey []byte          // key used to get join record
	var joinVal *fastjson.Value // value from join bkt
	var parsedJoinRec *fastjson.Value

	for _, join := range joins {

		// Note - if the JoinBkt and JoinFld are same as prev join, then the parsedJoinRec is reused
		if join.JoinBkt != prevJoinBkt {
			joinBkt = tx.Bucket([]byte(join.JoinBkt))
			if joinBkt == nil {
				emsg := fmt.Sprintf("invalid join bkt, %s", join.JoinBkt)
				bErr = e(ErrJoinBkt, emsg, nil, nil)
				return
			}
			prevJoinBkt = join.JoinBkt
			prevJoinFld = ""
		}
		if join.JoinFld != prevJoinFld {
			joinKey = parsedRec.GetStringBytes(join.JoinFld) // get key of record in join bkt
			if joinKey == nil {
				if join.UseDefault {
					continue
				}
				emsg := fmt.Sprintf("invalid join fld, %s", join.JoinFld)
				bErr = e(ErrJoinFld, emsg, nil, nil)
				return
			}
			joinRec = joinBkt.Get(joinKey) // get record from join bkt
			if joinRec == nil {
				if join.UseDefault {
					continue
				}
				emsg := fmt.Sprintf("join key %s not in join bkt %s", string(joinKey), join.JoinBkt)
				bErr = e(ErrJoinKey, emsg, nil, nil)
				return
			}
			parsedJoinRec, err = joinParser.ParseBytes(joinRec) // parse join record
			if err != nil {
				if join.UseDefault {
					continue
				}
				emsg := fmt.Sprintf("error parsing join rec, key %s, val %s", joinKey, string(joinRec))
				bErr = e(ErrJoinParse, emsg, nil, nil)
				return
			}
			prevJoinFld = join.JoinFld
		}
		joinVal = parsedJoinRec.Get(join.FromFld)
		if joinVal == nil {
			if join.UseDefault {
				continue
			}
			emsg := fmt.Sprintf("join from fld not found, key %s, FromFld %s", joinKey, join.FromFld)
			bErr = e(ErrJoinFromFld, emsg, nil, nil)
			return
		}
		parsedRec.Set(join.ToFld, joinVal)
	}
	recBytes = parsedRec.MarshalTo(nil)
	return
}

// extractSortVals extracts values to be sorted from parsedRec into a slice of strings.
// Integer values are converted to a string and leading zeros added to ensure all int vals are same length.
func extractSortVals(parsedRec *fastjson.Value, sortKeys []SortKey) (sortVals []string, bErr *BobbErr) {
	sortVals = make([]string, 0, len(sortKeys))
	var sortVal string
	var intVal int
	for _, sortKey := range sortKeys {
		switch {
		case slices.Contains(StrSortCodes, sortKey.Dir): // Dir contains both direction and fld type
			sortVal, bErr = parsedRecGetStr(parsedRec, sortKey.Fld, sortKey.UseDefault, StrPlain)
		case slices.Contains(IntSortCodes, sortKey.Dir):
			intVal, bErr = parsedRecGetInt(parsedRec, sortKey.Fld, sortKey.UseDefault)
			// note - underscores are ignored in numeric literals, added for readability, negative values will sort correctly with this approach
			// allows for sorting of int values as strings while preserving numeric order
			sortVal = fmt.Sprintf("%020d", intVal+1_000_000_000_000_000_000) // converts 3456 to 100000000000003456
		default:
			log.Panicln("invalid sortkey dir", sortKey.Dir) // should already be validated
		}
		if bErr != nil {
			return
		}
		sortVals = append(sortVals, sortVal)
	}
	return
}

// validateFindConditions validates values and loads defaults.
func validateFindConditions(conditions []FindCondition) ([]FindCondition, error) {
	validatedConditions := make([]FindCondition, len(conditions))
	for i, condition := range conditions {
		if !slices.Contains(AllFindOps, condition.Op) {
			return nil, fmt.Errorf("invalid find operation: %s", condition.Op)
		}
		if condition.StrOption == "" {
			condition.StrOption = StrLowerCase
		}
		if !slices.Contains(AllStrOptions, condition.StrOption) {
			return nil, fmt.Errorf("invalid string option: %s", condition.StrOption)
		}
		if condition.UseDefault == "" {
			condition.UseDefault = DefaultAlways
		}
		if !slices.Contains(AllDefaultCodes, condition.UseDefault) {
			return nil, fmt.Errorf("invalid default code: %s", condition.UseDefault)
		}
		if condition.Op == FindInStrList && len(condition.StrList) == 0 {
			return nil, fmt.Errorf("FindInStrList has empty string list")
		}
		if condition.Op == FindInIntList && len(condition.IntList) == 0 {
			return nil, fmt.Errorf("FindInIntList has empty integer list")
		}
		if condition.StrOption == StrLowerCase {
			condition.ValStr = strings.ToLower(condition.ValStr)
			for i, s := range condition.StrList {
				condition.StrList[i] = strings.ToLower(s)
			}
		}
		if condition.StrOption == StrPlain {
			condition.ValStr = PlainString(condition.ValStr)
			for i, s := range condition.StrList {
				condition.StrList[i] = PlainString(s)
			}
		}

		validatedConditions[i] = condition
	}
	return validatedConditions, nil
}

// validateSortKeys validates values and loads defaults.
func validateSortKeys(sortKeys []SortKey) ([]SortKey, error) {
	validatedSortKeys := make([]SortKey, len(sortKeys))
	for i, sortKey := range sortKeys {
		if sortKey.Fld == "" {
			return nil, fmt.Errorf("SortKey missing Fld")
		}
		if sortKey.Dir == "" {
			return nil, fmt.Errorf("SortKey.Dir is not set for fld %s", sortKey.Fld)
		}
		if !slices.Contains(AllSortCodes, sortKey.Dir) {
			return nil, fmt.Errorf("invalid SortKey.Dir: %s, for fld %s", sortKey.Dir, sortKey.Fld)
		}
		if sortKey.UseDefault == "" {
			sortKey.UseDefault = DefaultAlways
		}
		if !slices.Contains(AllDefaultCodes, sortKey.UseDefault) {
			return nil, fmt.Errorf("invalid SortKey.UseDefault: %s, for fld %s", sortKey.UseDefault, sortKey.Fld)
		}
		validatedSortKeys[i] = sortKey
	}
	return validatedSortKeys, nil
}
