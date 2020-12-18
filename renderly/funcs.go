package renderly

import "fmt"

func FuncMap(names ...string) map[string]interface{} {
	funcMap := map[string]interface{}{
		"map":       fnMap,
		"mapsplode": fnMapsplode,
		"slice":     fnSlice,
		"errorf":    fnErrorf,
	}
	if len(names) == 0 {
		return funcMap
	}
	customMap := make(map[string]interface{})
	for _, name := range names {
		if fn, ok := funcMap[name]; ok {
			customMap[name] = fn
		}
	}
	return customMap
}

func fnMap(keyvalues ...interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	isKey := true
	var key string
	for i, arg := range keyvalues {
		switch arg := arg.(type) {
		case mapsploded:
			for k, v := range fnMap(arg.keyvalues...) {
				result[k] = v
			}
			continue
		case []interface{}:
			if isKey {
				// TODO: handle nested keys
			}
		}
		if isKey && i+1 == len(keyvalues) {
			// drop arg if it is a key and there are no more values
			break
		}
		if isKey {
			key = fmt.Sprint(arg)
		} else {
			result[key] = arg
		}
		isKey = !isKey
	}
	return result
}

type mapsploded struct {
	keyvalues []interface{}
}

func fnMapsplode(m map[string]interface{}, keys ...string) mapsploded {
	result := mapsploded{}
	if len(keys) == 0 {
		for k, v := range m {
			result.keyvalues = append(result.keyvalues, k, v)
		}
		return result
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			result.keyvalues = append(result.keyvalues, k, v)
		}
	}
	return result
}

func fnSlice(a ...interface{}) []interface{} {
	return a
}

func fnErrorf(format string, a ...interface{}) (string, error) {
	return "", fmt.Errorf(format, a...)
}
