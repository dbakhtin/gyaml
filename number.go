package gyaml

import (
	"strconv"
)

func isNumber(value string) bool {
	if len(value) == 0 {
		return false
	}
	first := value[0]
	//check the first rune, save cpu and nature
	switch first {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '+', '-', '.':
	default:
		return false
	}
	dotCount := 0
	eCount := 0
	//since isNumber is used frequently, count manually instead of calling strings.Count
	for i := range value {
		if value[i] == '.' {
			dotCount++
		}
		if value[i] == 'e' {
			eCount++
		}
	}
	if dotCount > 1 || eCount > 1 {
		return false
	}

	if first == '-' || first == '+' {
		value = value[1:]
	}

	if dotCount == 1 || eCount == 1 {
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	}
	_, err := strconv.ParseUint(value, 0, 64)
	return err == nil
}
