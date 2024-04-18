package bobb

// Request Operations
const (
	OpBkt        = "bkt"
	OpGet        = "get"
	OpGetOne     = "getone"
	OpGetAll     = "getall"
	OpGetAllKeys = "getallkeys"
	OpGetIndex   = "getindex"
	OpQry        = "qry"
	OpQryIndex   = "qryindex"
	OpPut        = "put"
	OpPutOne     = "putone"
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

// FindCondition Op Codes
const (
	FindContains   = "contains"   // str
	FindMatches    = "matches"    // str
	FindStartsWith = "startswith" // str
	FindBefore     = "before"     // str
	FindAfter      = "after"      // str

	FindLessThan    = "lessthan"    // int
	FindGreaterThan = "greaterthan" // int
	FindEquals      = "equals"      // int
)

var StrFindOps = []string{FindContains, FindMatches, FindStartsWith, FindBefore, FindAfter}
var IntFindOps = []string{FindLessThan, FindGreaterThan, FindEquals}
