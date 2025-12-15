package bobb

import (
	"errors"
	"slices"
)

var ErrBadInputData error = errors.New("data error - bad json or no key") // used by put funcs when input data has problems

// Request Operations
const (
	OpBkt        = "bkt"
	OpGet        = "get"
	OpGetOne     = "getone"
	OpGetAll     = "getall"
	OpGetAllKeys = "getallkeys"
	OpQry        = "qry"
	OpPut        = "put"
	OpPutOne     = "putone"
	OpPutBkts    = "putbkts"
	OpPutIndex   = "putindex"
	OpDelete     = "delete"
	OpExport     = "export"
	OpClose      = "close"
	OpCopyDB     = "copydb"
)

// Response Status Values
const (
	StatusOk      = "ok"
	StatusFail    = "fail"
	StatusWarning = "warning"
)

// SortKey Dir Codes
const (
	SortAscStr  = "ascstr"
	SortDescStr = "descstr"
	SortAscInt  = "ascint"
	SortDescInt = "descint"
)

var StrSortCodes = []string{SortAscStr, SortDescStr}
var IntSortCodes = []string{SortAscInt, SortDescInt}
var AscSortCodes = []string{SortAscInt, SortAscStr}
var DescSortCodes = []string{SortDescInt, SortDescStr}

var AllSortCodes = slices.Concat(StrSortCodes, IntSortCodes)

// FindCondition Op Codes
const (
	FindContains   = "contains"      // str
	FindMatches    = "matches"       // str
	FindStartsWith = "startswith"    // str
	FindBefore     = "before"        // str
	FindAfter      = "after"         // str
	FindInStrList  = "findinstrlist" // str

	FindLessThan    = "lessthan"      // int
	FindGreaterThan = "greaterthan"   // int
	FindEquals      = "equals"        // int
	FindInIntList   = "findinintlist" // int

	FindExists = "exists" // any type
	FindIsNull = "isnull" // any type

	FindNot = true // used to set FindCondition.Not field
)

var StrFindOps = []string{FindContains, FindMatches, FindStartsWith, FindBefore, FindAfter, FindInStrList}
var IntFindOps = []string{FindLessThan, FindGreaterThan, FindEquals, FindInIntList}
var AllFindOps = slices.Concat(StrFindOps, IntFindOps, []string{FindExists, FindIsNull})

// Bobb Error Codes, Used for BobbError.ErrCode value
const (
	ErrNotFound    = "notfound"    // specified key not found in bkt
	ErrIndexRef    = "indexref"    // index value not key in bkt
	ErrParseRec    = "parserec"    // error parsing record
	ErrFldNotFound = "fldnotfound" // fld not found in record
	ErrFldIsNull   = "fldisnull"   // value of fld is null
	ErrFldType     = "fldtype"     // fld type in bkt rec does not match req fld type
	ErrJoinBkt     = "joinbkt"     // join bkt not found
	ErrJoinFld     = "joinfld"     // join fld invalid
	ErrJoinKey     = "joinkey"     // join key not found in join bkt
	ErrJoinParse   = "joinparse"   // error parsing join record
	ErrJoinFromFld = "joinfromfld" // join from fld invalid
)

// UseDefault Codes, controls value returned when record field not found or is null
const (
	DefaultAlways   = "always"   // on not found or null use zero value
	DefaultNever    = "never"    // return error if notfound or null
	DefaultIsNull   = "isnull"   // if null, use zero value, not found is error
	DefaultNotFound = "notfound" // if not found, use zero value, null is error
)

var AllDefaultCodes = []string{DefaultAlways, DefaultNever, DefaultIsNull, DefaultNotFound}

const (
	StrLowerCase = "lowercase" // value converted to lowercase
	StrPlain     = "plain"     // value converted to lowercase + non alphanumeric removed
	StrAsIs      = "asis"      // value used asis, no changes
)

var AllStrOptions = []string{StrLowerCase, StrPlain, StrAsIs}

// values for BktRequest Operation
const (
	BktCreate  = "create"  // create bucket
	BktDelete  = "delete"  // delete bucket
	BktNextSeq = "nextseq" // get next sequence number(s)
	BktList    = "list"    // get list of all buckets in db
	BktCount   = "count"   // get count of keys in a bucket
)
