package verification

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizeRut validates and returns a RUT in canonical 12345678-K form.
func NormalizeRut(rut string) (string, error) {
	r := strings.Map(func(c rune) rune {
		if c == '.' || unicode.IsSpace(c) {
			return -1
		}
		return c
	}, strings.ToUpper(strings.TrimSpace(rut)))
	r = strings.ReplaceAll(r, "–", "-")
	parts := strings.Split(r, "-")
	if len(parts) != 2 || len(parts[0]) < 1 || len(parts[0]) > 8 || len(parts[1]) != 1 {
		return "", ErrInvalid
	}
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return "", ErrInvalid
		}
	}
	dv := parts[1][0]
	if !(dv >= '0' && dv <= '9' || dv == 'K') {
		return "", ErrInvalid
	}
	if rutDV(parts[0]) != dv {
		return "", ErrInvalid
	}
	return fmt.Sprintf("%s-%c", strings.TrimLeft(parts[0], "0"), dv), nil
}

func ValidateRut(rut string) bool { _, err := NormalizeRut(rut); return err == nil }

func rutDV(digits string) byte {
	sum, factor := 0, 2
	for i := len(digits) - 1; i >= 0; i-- {
		sum += int(digits[i]-'0') * factor
		factor++
		if factor > 7 {
			factor = 2
		}
	}
	switch v := 11 - sum%11; v {
	case 11:
		return '0'
	case 10:
		return 'K'
	default:
		return byte('0' + v)
	}
}
