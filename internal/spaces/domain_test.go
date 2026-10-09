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

func TestPublishedContentInputValidatesPartialTitleAndPrice(t *testing.T) {
	title := "Título editado"
	description := strings.Repeat("Descripción local válida. ", 5)
	capacity := int32(5)
	usageRules := "Respetar el horario y no fumar"
	price := int64(5001)
	for name, input := range map[string]PublishedContentInput{
		"title only":       {Title: &title},
		"description only": {Description: &description},
		"capacity only":    {Capacity: &capacity},
		"rules only":       {UsageRules: &usageRules},
		"price only":       {BasePriceCLP: &price},
		"both":             {Title: &title, BasePriceCLP: &price},
	} {
		if err := input.Validate(); err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
	}
	for name, input := range map[string]PublishedContentInput{
		"empty":             {},
		"blank":             {Title: ptrString("  ")},
		"too long":          {Title: ptrString(strings.Repeat("x", 71))},
		"short description": {Description: ptrString("Descripción breve")},
		"zero capacity":     {Capacity: ptrInt32(0)},
		"blank rules":       {UsageRules: ptrString(" ")},
		"long rules":        {UsageRules: ptrString(strings.Repeat("x", 251))},
		"low price":         {BasePriceCLP: ptrInt64(5000)},
	} {
		if err := input.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s error=%v, want ErrInvalid", name, err)
		}
	}
}

func ptrString(value string) *string { return &value }
func ptrInt64(value int64) *int64    { return &value }
func ptrInt32(value int32) *int32    { return &value }

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

func TestAttributeProfilesValidateOptionalTypedValuesAndRules(t *testing.T) {
	minOne, maxInt := 1.0, 2147483647.0
	p := Profile{CategoryCode: "oficina", SchemaVersion: 1, Attributes: []AttributeDefinition{
		{Code: "puestos_trabajo", Type: "integer", Minimum: &minOne, Maximum: &maxInt},
		{Code: "wifi", Type: "boolean"},
		{Code: "tipo_uso_oficina", Type: "enum", Options: []string{"privada", "compartida"}},
	}}
	for _, values := range []map[string]any{nil, {}, {"wifi": false, "puestos_trabajo": float64(1), "tipo_uso_oficina": "privada"}} {
		if err := p.ValidateAttributes(values); err != nil {
			t.Errorf("valid attributes %v rejected: %v", values, err)
		}
	}
	for _, values := range []map[string]any{{"wifi": "true"}, {"puestos_trabajo": float64(0)}, {"puestos_trabajo": float64(2147483648)}, {"tipo_uso_oficina": "bodega"}, {"extra": true}, {"wifi": nil}} {
		if err := p.ValidateAttributes(values); err == nil {
			t.Errorf("invalid attributes %v accepted", values)
		}
	}
	q := Profile{CategoryCode: "quincho", Attributes: []AttributeDefinition{{Code: "tipo_parrilla", Type: "enum", Options: []string{"carbon", "sin_parrilla"}}, {Code: "parrillas_disponibles", Type: "integer", Minimum: &minOne, Maximum: &maxInt}}}
	if q.ValidateAttributes(map[string]any{"tipo_parrilla": "sin_parrilla", "parrillas_disponibles": float64(1)}) == nil {
		t.Fatal("incompatible grill combination accepted")
	}
	parcel := Profile{CategoryCode: "parcela_eventos", Attributes: []AttributeDefinition{{Code: "superficie_exterior_util_m2", Type: "number", Minimum: &minOne, Maximum: &maxInt, Step: floatPtr(0.01)}}}
	if validateAttributes(parcel, Input{AreaM2: 100, Attributes: map[string]any{"superficie_exterior_util_m2": float64(101)}}) == nil {
		t.Fatal("partial event surface larger than common surface accepted")
	}
}

func floatPtr(v float64) *float64 { return &v }
