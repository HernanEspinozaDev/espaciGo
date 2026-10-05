package spaces

import (
	"errors"
	"strings"
	"testing"
)

func validInput() Input {
	return Input{CategoryCode: "oficina", Title: "Oficina", Description: strings.Repeat("Espacio de trabajo disponible para arriendo. ", 3), AreaM2: 12.5, Capacity: 4, UsageRules: "No fumar", RateUnit: "hora", BasePriceCLP: 6000, Address: "Av. Prueba 123"}
}
func TestDraftInputRequiresAllCU15FieldsAndLimits(t *testing.T) {
	if err := validInput().Validate(); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	checks := []struct {
		name   string
		mutate func(*Input)
	}{
		{"title", func(i *Input) { i.Title = strings.Repeat("a", 71) }},
		{"description", func(i *Input) { i.Description = "breve" }},
		{"area", func(i *Input) { i.AreaM2 = 0 }},
		{"category", func(i *Input) { i.CategoryCode = "" }},
		{"capacity", func(i *Input) { i.Capacity = 0 }},
		{"rules", func(i *Input) { i.UsageRules = strings.Repeat("a", 251) }},
		{"rate", func(i *Input) { i.RateUnit = "week" }},
		{"price", func(i *Input) { i.BasePriceCLP = 5000 }},
		{"address", func(i *Input) { i.Address = " " }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			in := validInput()
			check.mutate(&in)
			if !errors.Is(in.Validate(), ErrInvalid) {
				t.Fatal("expected invalid field to be rejected")
			}
		})
	}
}

func TestDraftAreaAndPriceMatchPostgresNumericRanges(t *testing.T) {
	for _, area := range []float64{0.01, 99999999.99} {
		in := validInput()
		in.AreaM2 = area
		if err := in.Validate(); err != nil {
			t.Errorf("PostgreSQL numeric boundary area %v rejected: %v", area, err)
		}
	}
	for _, area := range []float64{0.009, 12.345, 100000000} {
		in := validInput()
		in.AreaM2 = area
		if err := in.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("out-of-range area %v err=%v", area, err)
		}
	}
	in := validInput()
	in.BasePriceCLP = int64(^uint64(0) >> 1)
	if err := in.Validate(); err != nil {
		t.Fatalf("max PostgreSQL bigint rejected: %v", err)
	}
}
