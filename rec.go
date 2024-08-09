// The rec.go file contains funcs that perform actions on an indivual record.

package bobb

import (
	"log"
	"slices"
	"strings"

	"github.com/valyala/fastjson"
)

const StrToLower = true // optional parm used when calling recGetStr()

// parsedRecGetStr returns the string value for specified fld.
// Optional toLower bool parm used to return lower case value.
func parsedRecGetStr(parsedRec *fastjson.Value, fld string, toLower ...bool) string {
	val := parsedRec.Get(fld)
	strBytes, err := val.StringBytes()
	if err != nil || strBytes == nil {
		log.Println("parsedRecGetStr error - ", fld, err)
		return ""
	}
	strVal := string(strBytes)
	if len(toLower) > 0 && toLower[0] {
		return strings.ToLower(strVal)
	}
	return strVal
}

// parsedRecGetInt returns the int value for specified fld.
func parsedRecGetInt(parsedRec *fastjson.Value, fld string) int {
	val := parsedRec.Get(fld)
	intVal, err := val.Int()
	if err != nil {
		log.Println("parsedRecGetInt error - ", fld, err)
		return 0
	}
	return intVal
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
			recValStr = parsedRecGetStr(parsedRec, condition.Fld, StrToLower)
			n = strCompare(recValStr, compareVal)
		case slices.Contains(IntFindOps, condition.Op):
			recVal := parsedRecGetInt(parsedRec, condition.Fld)
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
