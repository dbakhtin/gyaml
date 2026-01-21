package gyaml

import (
	"errors"
	"strconv"
	"strings"
)

type NumberType string

const (
	NumberTypeDecimal NumberType = "decimal"
	NumberTypeBinary  NumberType = "binary"
	NumberTypeOctet   NumberType = "octet"
	NumberTypeHex     NumberType = "hex"
	NumberTypeFloat   NumberType = "float"
)

type NumberValue struct {
	Type  NumberType
	Value any
	Text  string
}

func ToNumber(value string) *NumberValue {
	num, err := toNumber(value)
	if err != nil {
		return nil
	}
	return num
}

func isNumber(value string) bool {
	num, err := toNumber(value)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
			return true
		}
		return false
	}
	return num != nil
}

func toNumber(value string) (*NumberValue, error) {
	if len(value) == 0 {
		return nil, nil
	}
	first := value[0]
	//check the first rune, saves a lot of cpu
	if !strings.ContainsRune("0123456789+-.", rune(first)) {
		return nil, nil
	}
	if value[0] == '_' {
		return nil, nil
	}
	dotCount := strings.Count(value, ".")
	if dotCount > 1 {
		return nil, nil
	}

	isNegative := value[0] == '-'
	normalized := strings.ReplaceAll(strings.TrimLeft(value, "+-"), "_", "")

	var (
		typ  NumberType
		base int
	)
	switch {
	case strings.HasPrefix(normalized, "0x"):
		normalized = strings.TrimPrefix(normalized, "0x")
		base = 16
		typ = NumberTypeHex
	case strings.HasPrefix(normalized, "0o"):
		normalized = strings.TrimPrefix(normalized, "0o")
		base = 8
		typ = NumberTypeOctet
	case strings.HasPrefix(normalized, "0b"):
		normalized = strings.TrimPrefix(normalized, "0b")
		base = 2
		typ = NumberTypeBinary
	case strings.HasPrefix(normalized, "0") && len(normalized) > 1 && dotCount == 0:
		base = 8
		typ = NumberTypeOctet
	case dotCount == 1:
		typ = NumberTypeFloat
	default:
		typ = NumberTypeDecimal
		base = 10
	}

	text := normalized
	if isNegative {
		text = "-" + text
	}

	var v any
	if typ == NumberTypeFloat {
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, err
		}
		v = f
	} else if isNegative {
		i, err := strconv.ParseInt(text, base, 64)
		if err != nil {
			return nil, err
		}
		v = i
	} else {
		u, err := strconv.ParseUint(text, base, 64)
		if err != nil {
			return nil, err
		}
		v = u
	}

	return &NumberValue{
		Type:  typ,
		Value: v,
		Text:  text,
	}, nil
}
