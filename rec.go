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
// Type FldFormat defined in types.go (FldName, FldType, Length, StrOption, UseDefault).
// Separator placed between each value.
// Each fld value is truncated or padded if needed to set length.
// UseDefault controls what happens when fld is not found or null in parsedRec.
// Ex. DefaultNull would return 0 for FldTypeInt, or "" for FldTypeStr if fld value is null.
// DefaultNever would return error if fld value is null or fld not found.
func MergeFlds(parsedRec *fastjson.Value, flds []FldFormat, separator string) (mergedVal string, err error) {
	var bErr *BobbErr
	var format string
	var intVal int
	var strVal string
	var strOption string
	var useDefault string
	formattedFlds := make([]string, len(flds))
	for i, fld := range flds {
		useDefault = fld.UseDefault
		if fld.UseDefault == "" {
			useDefault = DefaultAlways
		}
		if !slices.Contains(AllDefaultCodes, useDefault) {
			return "", fmt.Errorf("MergeFlds, invalid UseDefault code for fld %s, must be one of bobb.DefaultAlways, bobb.DefaultNever, bobb.DefaultIsNull, bobb.DefaultNotFound", fld.FldName)
		}
		fmtLength := strconv.Itoa(fld.Length)
		switch fld.FldType {
		case FldTypeInt:
			intVal, bErr = parsedRecGetInt(parsedRec, fld.FldName, useDefault)
			if bErr != nil {
				return "", fmt.Errorf("MergeFlds, error getting int value for fld %s, %s", fld.FldName, bErr.Msg)
			}
			format = "%0" + fmtLength + "d"
			formattedFlds[i] = fmt.Sprintf(format, intVal)
		case FldTypeStr:
			strOption = fld.StrOption
			if strOption == "" {
				strOption = StrLowerCase
			}
			if !slices.Contains(AllStrOptions, strOption) {
				return "", fmt.Errorf("MergeFlds, invalid StrOption for fld %s, must be one of bobb.StrPlain, bobb.StrLowerCase, bobb.StrAsIs", fld.FldName)
			}
			strVal, bErr = parsedRecGetStr(parsedRec, fld.FldName, useDefault, strOption)
			if bErr != nil {
				return "", fmt.Errorf("MergeFlds, error getting str value for fld %s, %s", fld.FldName, bErr.Msg)
			}
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
// Parm useDefault controls how fld not found or null is handled (whether ""/no error or error is returned).
func parsedRecGetStr(parsedRec *fastjson.Value, fld string, useDefault string, option ...string) (returnVal string, bErr *BobbErr) {
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
	strBytes, err := val.StringBytes()
	if err != nil {
		emsg := fmt.Sprintf("parsedRecGetStr - fld %s error getting string bytes, %s", fld, err.Error())
		bErr = e(ErrFldType, emsg, nil, nil)
		return
	}
	returnVal = string(strBytes)
	if len(option) > 0 {
		switch option[0] {
		case StrLowerCase:
			returnVal = strings.ToLower(returnVal)
		case StrPlain:
			returnVal = PlainString(returnVal)
		}
	}
	return
}

// parsedRecGetInt returns the int value for specified fld.
// UseDefault controls how fld not found or null is handled (whether 0/no error or error is returned).
func parsedRecGetInt(parsedRec *fastjson.Value, fld string, useDefault string) (returnVal int, bErr *BobbErr) {
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
	returnVal, err := val.Int()
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
	var recValStr string
	var recValInt int
	for _, condition := range conditions {
		conditionMet = false
		switch {
		case slices.Contains(StrFindOps, condition.Op):
			switch condition.StrOption {
			case StrLowerCase:
				recValStr, bErr = parsedRecGetStr(parsedRec, condition.Fld, condition.UseDefault, StrLowerCase)
			case StrPlain:
				recValStr, bErr = parsedRecGetStr(parsedRec, condition.Fld, condition.UseDefault, StrPlain)
			case StrAsIs:
				recValStr, bErr = parsedRecGetStr(parsedRec, condition.Fld, condition.UseDefault)
			default:
				log.Panicln("invalid findCondition.StrOption", condition.StrOption) // should already be validated
			}
			if bErr != nil {
				return
			}
			if slices.Contains(nStrOps, condition.Op) { // n indicates if recValStr is less than, equal to, or greater than compareValStr
				n = strings.Compare(recValStr, condition.ValStr)
			}
		case slices.Contains(IntFindOps, condition.Op):
			recValInt, bErr = parsedRecGetInt(parsedRec, condition.Fld, condition.UseDefault)
			if bErr != nil {
				return
			}
			if slices.Contains(nIntOps, condition.Op) { // n indicates if recValInt is less than, equal to, or greater than condition.ValInt
				n = recValInt - condition.ValInt
			}
		}
		// log.Println("parsedRecFind, condition", condition, "recValStr", recValStr, "condition.ValStr", condition.ValStr, "n", n)

		switch condition.Op {
		case FindExists, FindIsNull:
			val := parsedRec.Get(condition.Fld)
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
			if strings.HasPrefix(recValStr, condition.ValStr) {
				conditionMet = true
			}
		case FindEndsWith:
			if strings.HasSuffix(recValStr, condition.ValStr) {
				conditionMet = true
			}
		case FindContains:
			if strings.Contains(recValStr, condition.ValStr) {
				conditionMet = true
			}
		case FindContainsWord:
			words := strings.Fields(recValStr)
			if slices.Contains(words, condition.ValStr) {
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

	// log.Println("parsedRecFind, rec meets all conditions, keep it")

	return // rec meets all conditions
}
