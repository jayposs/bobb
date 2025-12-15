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
func MergeFlds(parsedRec *fastjson.Value, flds []FldFormat, separator string) (mergedVal string) {
	formattedFlds := make([]string, len(flds))
	var format string
	var intVal int
	var strVal string
	var useDefault string = DefaultAlways
	for i, fld := range flds {
		fmtLength := strconv.Itoa(fld.Length)
		switch fld.FldType {
		case "int":
			intVal, _ = parsedRecGetInt(parsedRec, fld.FldName, useDefault) // err ignored
			format = "%0" + fmtLength + "d"
			formattedFlds[i] = fmt.Sprintf(format, intVal)
		case "string":
			strVal, _ = parsedRecGetStr(parsedRec, fld.FldName, useDefault, StrPlain)
			if len(strVal) > fld.Length {
				strVal = strVal[:fld.Length]
			}
			format = "%-" + fmtLength + "s"
			formattedFlds[i] = fmt.Sprintf(format, strVal)
		default:
			log.Println("MergeFlds, invalid fld type, must be string or int", fld.FldName, fld.FldType)
			return
		}
	}
	mergedVal = strings.Join(formattedFlds, separator)
	return
}

// parsedRecGetStr returns the string value for specified fld.
// Parm "option" controls conversion, see Str* codes in codes.go
// Parm useDefault controls how fld not found or null is handled.
func parsedRecGetStr(parsedRec *fastjson.Value, fld string, useDefault string, option ...string) (strVal string, bErr *BobbErr) {
	val := parsedRec.Get(fld)
	if val == nil { // fld not found in rec
		if useDefault == DefaultAlways || useDefault == DefaultNotFound {
			return // returns ""
		}
		emsg := fmt.Sprintf("parsedRecGetStr - fld %s not found", fld)
		bErr = e(ErrFldNotFound, emsg, nil, nil)
		return
	}
	if val.Type() == fastjson.TypeNull {
		if useDefault == DefaultAlways || useDefault == DefaultIsNull {
			return // returns ""
		}
		emsg := fmt.Sprintf("parsedRecGetStr - fld %s has null value", fld)
		bErr = e(ErrFldIsNull, emsg, nil, nil)
		return
	}
	if val.Type() != fastjson.TypeString {
		emsg := fmt.Sprintf("parsedRecGetStr - fld %s not a string", fld)
		bErr = e(ErrFldType, emsg, nil, nil)
		return
	}
	strBytes := val.GetStringBytes()
	if len(option) > 0 {
		switch option[0] {
		case StrLowerCase:
			strVal = strings.ToLower(string(strBytes))
		case StrPlain:
			strVal = PlainString(string(strBytes))
		}
	} else {
		strVal = string(strBytes)
	}
	return
}

// parsedRecGetInt returns the int value for specified fld.
func parsedRecGetInt(parsedRec *fastjson.Value, fld string, useDefault string) (intVal int, bErr *BobbErr) {
	val := parsedRec.Get(fld)

	if val == nil { // fld not found in rec
		if useDefault == DefaultAlways || useDefault == DefaultNotFound {
			return // returns 0
		}
		emsg := fmt.Sprintf("parsedRecGetInt - fld %s not found", fld)
		bErr = e(ErrFldNotFound, emsg, nil, nil)
		return
	}

	if val.Type() == fastjson.TypeNull {
		if useDefault == DefaultAlways || useDefault == DefaultIsNull {
			return // returns 0
		}
		emsg := fmt.Sprintf("parsedRecGetInt - fld %s has null value", fld)
		bErr = e(ErrFldIsNull, emsg, nil, nil)
		return
	}

	var err error
	intVal, err = val.Int()
	if err != nil {
		emsg := fmt.Sprintf("parsedRecGetInt - fld %s not int, %s", fld, err.Error())
		bErr = e(ErrFldNotFound, emsg, nil, nil)
		return
	}
	return
}

var nStrOps = []string{FindMatches, FindBefore, FindAfter}
var nIntOps = []string{FindEquals, FindLessThan, FindGreaterThan}

// parseRecFind determines if rec value(s) meet all find conditions using already parsed rec.
func parsedRecFind(parsedRec *fastjson.Value, conditions []FindCondition) (keep bool, bErr *BobbErr) {
	var conditionMet bool
	var n int // compare result  1:greater, -1:less, 0:equal
	var recValStr, compareValStr string
	var recValInt int
	for _, condition := range conditions {
		conditionMet = false
		switch {
		case slices.Contains(StrFindOps, condition.Op):
			switch condition.StrOption {
			case StrLowerCase:
				recValStr, bErr = parsedRecGetStr(parsedRec, condition.Fld, condition.UseDefault, StrLowerCase)
				if condition.Op == FindInStrList {
					for i, val := range condition.StrList {
						condition.StrList[i] = strings.ToLower(val)
					}
				} else {
					compareValStr = strings.ToLower(condition.ValStr)
				}
			case StrPlain:
				recValStr, bErr = parsedRecGetStr(parsedRec, condition.Fld, condition.UseDefault, StrPlain)
				if condition.Op == FindInStrList {
					for i, val := range condition.StrList {
						condition.StrList[i] = PlainString(val)
					}
				} else {
					compareValStr = PlainString(condition.ValStr)
				}
			case StrAsIs:
				recValStr, bErr = parsedRecGetStr(parsedRec, condition.Fld, condition.UseDefault)
				compareValStr = condition.ValStr
			default:
				log.Panicln("invalid findCondition.StrOption", condition.StrOption) // should already be validated
			}
			if bErr != nil {
				return
			}
			if slices.Contains(nStrOps, condition.Op) {
				n = strings.Compare(recValStr, compareValStr)
			}
		case slices.Contains(IntFindOps, condition.Op):
			recValInt, bErr = parsedRecGetInt(parsedRec, condition.Fld, condition.UseDefault)
			if bErr != nil {
				return
			}
			if slices.Contains(nIntOps, condition.Op) {
				n = recValInt - condition.ValInt
			}
		}

		switch condition.Op {
		case FindExists, FindIsNull:
			val := parsedRec.Get(condition.Fld)
			log.Println(condition.Op, val, condition.Fld)
			if val != nil {
				if condition.Op == FindExists {
					conditionMet = true
				} else if val.Type() == fastjson.TypeNull {
					conditionMet = true
				}
			}
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
			if strings.HasPrefix(recValStr, compareValStr) {
				conditionMet = true
			}
		case FindContains:
			if strings.Contains(recValStr, compareValStr) {
				conditionMet = true
			}
		case FindInStrList:
			if slices.Contains(condition.StrList, recValStr) {
				conditionMet = true
			}
		case FindInIntList:
			if slices.Contains(condition.IntList, recValInt) {
				conditionMet = true
			}
		default:
			log.Panicln("parsedRecFind, invalid Op code, ", condition.Op) // should already be validated
		}
		if condition.Not {
			if conditionMet { // condition.Not indicates to exclude recs that meet condition
				return
			} else {
				continue // rec doesn't meet condition, so don't exclude it
			}
		}
		if !conditionMet {
			return // condition was not met, end recFind
		}
	}
	keep = true
	return // rec meets all conditions
}
