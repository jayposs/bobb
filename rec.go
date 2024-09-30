// The rec.go file contains funcs that perform actions on an indivual record.

package bobb

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/valyala/fastjson"
)

const StrToLower = "lower" // optional parm used when calling parsedRecGetStr()
const StrToPlain = "plain" // optional parm used when calling parsedRecGetStr()

// PlainString returns lower case version of input string with non alphanumeric chars removed.
// Ex. Hip-Hop > hiphop
// Used by MergeFlds below.
func PlainString(in string) string {
	mapFunc := func(char rune) rune {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			return char
		}
		return -1
	}
	plainString := strings.Map(mapFunc, in)
	return strings.ToLower(plainString)
}

// MergeFlds is typically used to create index keys composed of multiple flds merged together.
// Type FldFormat defined in types.go (FldName, FldType, Length).
// Separator placed between each value.
// Values are plain strings (lower case alphanumeric, special chars removed).
// Each fld value is truncated or padded if needed to set length.
func MergeFlds(parsedRec *fastjson.Value, flds []FldFormat, separator string) string {
	formattedFlds := make([]string, len(flds))
	var format string
	var intVal int
	var strVal string
	for i, fld := range flds {
		fmtLength := strconv.Itoa(fld.Length)
		switch fld.FldType {
		case "int":
			intVal = parsedRecGetInt(parsedRec, fld.FldName)
			format = "%0" + fmtLength + "d"
			formattedFlds[i] = fmt.Sprintf(format, intVal)
		case "string":
			strVal = parsedRecGetStr(parsedRec, fld.FldName, StrToPlain)
			if len(strVal) > fld.Length {
				strVal = strVal[:fld.Length]
			}
			format = "%-" + fmtLength + "s"
			formattedFlds[i] = fmt.Sprintf(format, strVal)
		default:
			log.Println("MergeFlds, invalid fld type, must be string or int", fld.FldName, fld.FldType)
			return ""
		}
	}
	return strings.Join(formattedFlds, separator)
}

// parsedRecGetStr returns the string value for specified fld.
// Parm option can be either StrToLower or StrToPlain.
func parsedRecGetStr(parsedRec *fastjson.Value, fld string, option ...string) string {
	val := parsedRec.Get(fld)
	if val == nil {
		log.Println("parsedRecGetStr - fld not found", fld)
		return ""
	}
	strBytes, err := val.StringBytes()
	if err != nil || strBytes == nil {
		log.Println("parsedRecGetStr error - ", fld, err)
		return ""
	}
	strVal := string(strBytes)
	if len(option) > 0 {
		if option[0] == StrToLower {
			return strings.ToLower(strVal)
		}
		if option[0] == StrToPlain {
			return PlainString(strVal)
		}
	}
	return strVal
}

// parsedRecGetInt returns the int value for specified fld.
func parsedRecGetInt(parsedRec *fastjson.Value, fld string) int {
	val := parsedRec.Get(fld)
	if val == nil {
		log.Println("parsedRecGetInt - fld not found", fld)
		return 0
	}
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
			switch condition.StrOption {
			case "":
				compareVal = strings.ToLower(condition.ValStr)
				recValStr = parsedRecGetStr(parsedRec, condition.Fld, StrToLower)
			case "plain":
				compareVal = PlainString(condition.ValStr)
				recValStr = parsedRecGetStr(parsedRec, condition.Fld, StrToPlain)
			case "asis":
				compareVal = condition.ValStr
				recValStr = parsedRecGetStr(parsedRec, condition.Fld)
			default:
				log.Println("invalid FindCondition.StrOption", condition.StrOption)
				return false
			}
			n = StrCompare(recValStr, compareVal)
		case slices.Contains(IntFindOps, condition.Op):
			recValInt := parsedRecGetInt(parsedRec, condition.Fld)
			n = recValInt - condition.ValInt
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
			if n < 0 {
				conditionMet = true
			}
		case FindGreaterThan, FindAfter:
			if n > 0 {
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
