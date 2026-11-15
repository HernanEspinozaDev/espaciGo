package pricing

import (
	"errors"
	"testing"
)

func TestValidateRateAcceptsCanonicalUnitsAboveMinimum(t *testing.T) {
	for _, unit := range []string{"hora", "dia", "mes"} {
		t.Run(unit, func(t *testing.T) {
			if err := validateRate(RateInput{Unit: unit, Amount: 5001}); err != nil {
				t.Fatalf("valid rate unit %q above minimum returned %v", unit, err)
			}
		})
	}
}

func TestValidateRateRejectsAmountAtOrBelowMinimum(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount int64
	}{
		{name: "negative", amount: -1},
		{name: "zero", amount: 0},
		{name: "minimum boundary", amount: 5000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRate(RateInput{Unit: "hora", Amount: tc.amount}); !errors.Is(err, ErrInvalid) {
				t.Fatalf("rate amount %d error=%v, want ErrInvalid", tc.amount, err)
			}
		})
	}
}

func TestValidateRateRejectsNonCanonicalUnits(t *testing.T) {
	for _, unit := range []string{"", "HORA", "hour", "horas", "día"} {
		t.Run(unit, func(t *testing.T) {
			if err := validateRate(RateInput{Unit: unit, Amount: 5001}); !errors.Is(err, ErrInvalid) {
				t.Fatalf("rate unit %q error=%v, want ErrInvalid", unit, err)
			}
		})
	}
}
