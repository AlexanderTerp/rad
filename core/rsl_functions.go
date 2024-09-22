package core

import (
	"fmt"
	"strings"
)

// RunRslNonVoidFunction returns pointers to values e.g. *string
func RunRslNonVoidFunction(i *MainInterpreter, function Token, values []interface{}) interface{} {
	functionName := function.GetLexeme()
	switch functionName {
	case "len":
		return runLen(i, function, values)
	case "today_date": // todo is this name good? current_date? date?
		return RClock.Now().Format("2006-01-02")
	case "today_year":
		return int64(RClock.Now().Year())
	case "today_month":
		return int64(RClock.Now().Month())
	case "today_day":
		return int64(RClock.Now().Day())
	case "today_hour":
		return int64(RClock.Now().Hour())
	case "today_minute":
		return int64(RClock.Now().Minute())
	case "today_second":
		return int64(RClock.Now().Second())
	case "epoch_seconds":
		return RClock.Now().Unix()
	case "epoch_millis":
		return RClock.Now().UnixMilli()
	case "epoch_nanos":
		return RClock.Now().UnixNano()
	case "replace":
		return runReplace(i, function, values)
	case "join":
		return runJoin(i, function, values)
	case "upper":
		return strings.ToUpper(ToPrintable(values[0]))
	case "lower":
		return strings.ToLower(ToPrintable(values[0]))
	case "starts_with":
		if len(values) != 2 {
			i.error(function, "starts_with() takes exactly two arguments")
		}
		return strings.HasPrefix(ToPrintable(values[0]), ToPrintable(values[1]))
	case "ends_with":
		if len(values) != 2 {
			i.error(function, "ends_with() takes exactly two arguments")
		}
		return strings.HasSuffix(ToPrintable(values[0]), ToPrintable(values[1]))
	case "contains":
		if len(values) != 2 {
			i.error(function, "contains() takes exactly two arguments")
		}
		return strings.Contains(ToPrintable(values[0]), ToPrintable(values[1]))
	case "pick":
		return runPick(i, function, values)
	default:
		i.error(function, fmt.Sprintf("Unknown function: %v", functionName))
		panic(UNREACHABLE)
	}
}

func RunRslFunction(i *MainInterpreter, function Token, values []interface{}) {
	functionName := function.GetLexeme()
	switch functionName {
	case "print": // todo would be nice to make this a reference to a var that GoLand can find
		runPrint(values)
	case "debug":
		runDebug(values)
	default:
		RunRslNonVoidFunction(i, function, values)
	}
}

func runPrint(values []interface{}) {
	output := resolveOutputString(values)
	RP.Print(output)
}

func runDebug(values []interface{}) {
	output := resolveOutputString(values)
	RP.ScriptDebug(output)
}

func resolveOutputString(values []interface{}) string {
	output := ""

	if len(values) == 0 {
		output = "\n"
	} else {
		for _, v := range values {
			output += ToPrintable(v) + " "
		}
		output = output[:len(output)-1] // remove last space
		output = output + "\n"
	}
	return output
}

func runLen(i *MainInterpreter, function Token, values []interface{}) int64 {
	if len(values) != 1 {
		i.error(function, "len() takes exactly one argument")
	}
	switch v := values[0].(type) {
	case string:
		return int64(len(v))
	case []string:
		return int64(len(v))
	case []int64:
		return int64(len(v))
	case []float64:
		return int64(len(v))
	default:
		i.error(function, "len() takes a string or array")
		panic(UNREACHABLE)
	}
}

func runReplace(i *MainInterpreter, function Token, values []interface{}) interface{} {
	if len(values) != 3 {
		i.error(function, "replace() takes exactly three arguments")
	}

	subject := ToPrintable(values[0])
	oldRegex := ToPrintable(values[1])
	newRegex := ToPrintable(values[2])

	return Replace(i, function, subject, oldRegex, newRegex)
}

func runJoin(i *MainInterpreter, function Token, values []interface{}) interface{} {
	if len(values) < 2 {
		i.error(function, "join() takes at least two arguments")
	}

	prefix := ""
	suffix := ""
	if len(values) == 3 {
		prefix = ToPrintable(values[2])
	} else if len(values) == 4 {
		prefix = ToPrintable(values[2])
		suffix = ToPrintable(values[3])
	}

	var arr []string
	switch values[0].(type) {
	case []string:
		arr = values[0].([]string)
	case []int64:
		ints := values[0].([]int64)
		for _, v := range ints {
			arr = append(arr, ToPrintable(v))
		}
	case []float64:
		floats := values[0].([]float64)
		for _, v := range floats {
			arr = append(arr, ToPrintable(v))
		}
	default:
		i.error(function, "join() takes an array as the first argument")
	}

	separator := ToPrintable(values[1])

	return prefix + strings.Join(arr, separator) + suffix
}
