// The rec.go file contains funcs that perform actions on an indivual record ( []byte ).
// Funcs recGetStr and recGetInt return requested field's value from the record.
// Func recFind determines if the record meets specified FindConditions.

package bobb

import (
	"log"
	"slices"
	"strings"

	"github.com/valyala/fastjson"
)

const StrToLower = true // optional parm used when calling recGetStr()

// recGetStr returns the string value associated with a field in the record.
func recGetStr(rec []byte, fld string, toLower ...bool) string {
	val := fastjson.GetString(rec, fld)
	if len(toLower) > 0 && toLower[0] {
		val = strings.ToLower(val)
	}
	return val
}

// recGetInt returns the int value associated with a field in the record.
func recGetInt(rec []byte, fld string) int {
	return fastjson.GetInt(rec, fld)
}

// NOTE - in recFind() string values are converted to lower case.
// If this behaviour is not valid for your use case, code must be changed.

// recFind determines if rec value(s) meet all find conditions.
func recFind(rec []byte, conditions []FindCondition) bool {
	var conditionMet bool
	var n int                        // compare result  1:greater, -1:less, 0:equal
	var compareVal, recValStr string // only used for strings, to support StartsWith and Contains ops
	for _, condition := range conditions {
		conditionMet = false
		switch {
		case slices.Contains(StrFindOps, condition.Op):
			compareVal = strings.ToLower(condition.ValStr)
			recValStr = recGetStr(rec, condition.Fld, StrToLower)
			n = strCompare(recValStr, compareVal)
		case slices.Contains(IntFindOps, condition.Op):
			recVal := recGetInt(rec, condition.Fld)
			n = intCompare(recVal, condition.ValInt)
		default:
			log.Println("FindCondition invalid op", condition.Op)
			return false
		}
		switch condition.Op {
		case FindMatches, FindEquals:
			if n == 0 {
				conditionMet = true
			}
		case FindLessThan, FindBefore:
			if n == -1 {
				conditionMet = true
			}
		case FindGreaterThan, FindAfter:
			if n == 1 {
				conditionMet = true
			}
		case FindStartsWith:
			if strings.HasPrefix(recValStr, compareVal) {
				conditionMet = true
			}
		case FindContains:
			if strings.Contains(recValStr, compareVal) {
				conditionMet = true
			}
		}
		if condition.Not {
			if conditionMet { // condition.Not indicates to exclude recs that meet condition
				return false
			} else {
				continue // rec doesn't meet condition, so don't exclude it
			}
		}
		if !conditionMet {
			return false // condition was not met, end recFind
		}
	}
	return true // no condition check returned false
}

// parseRecFind determines if rec value(s) meet all find conditions using already parsed rec.
func parsedRecFind(parsedRec *fastjson.Value, conditions []FindCondition) bool {
	var conditionMet bool
	var n int                        // compare result  1:greater, -1:less, 0:equal
	var compareVal, recValStr string // only used for strings, to support StartsWith and Contains ops
	for _, condition := range conditions {
		conditionMet = false
		switch {
		case slices.Contains(StrFindOps, condition.Op):
			compareVal = strings.ToLower(condition.ValStr)
			strBytes := parsedRec.GetStringBytes(condition.Fld)
			if strBytes == nil {
				recValStr = ""
			} else {
				recValStr = strings.ToLower(string(strBytes))
			}
			n = strCompare(recValStr, compareVal)
		case slices.Contains(IntFindOps, condition.Op):
			recVal := parsedRec.GetInt(condition.Fld)
			n = intCompare(recVal, condition.ValInt)
		default:
			log.Println("FindCondition invalid op", condition.Op)
			return false
		}
		switch condition.Op {
		case FindMatches, FindEquals:
			if n == 0 {
				conditionMet = true
			}
		case FindLessThan, FindBefore:
			if n == -1 {
				conditionMet = true
			}
		case FindGreaterThan, FindAfter:
			if n == 1 {
				conditionMet = true
			}
		case FindStartsWith:
			if strings.HasPrefix(recValStr, compareVal) {
				conditionMet = true
			}
		case FindContains:
			if strings.Contains(recValStr, compareVal) {
				conditionMet = true
			}
		}
		if condition.Not {
			if conditionMet { // condition.Not indicates to exclude recs that meet condition
				return false
			} else {
				continue // rec doesn't meet condition, so don't exclude it
			}
		}
		if !conditionMet {
			return false // condition was not met, end recFind
		}
	}
	return true // no condition check returned false
}
