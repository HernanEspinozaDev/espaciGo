package verification

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func validDeclaration() DeclarationPayload {
	return DeclarationPayload{FirstName: "Ana", LastName: "Pérez", DateOfBirth: "1990-01-02", RUT: "12345670-K", IDCountry: "CHL", DocumentType: "ID"}
}

func TestValidateAdultBirthDate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		birth   string
		now     time.Time
		wantErr bool
	}{
		{"exact eighteenth birthday", "2008-06-15", time.Date(2026, 6, 15, 23, 0, 0, 0, time.FixedZone("offset", -7*3600)), false},
		{"one day short", "2008-06-16", time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC), true},
		{"leap day before anniversary", "2008-02-29", time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC), true},
		{"leap day anniversary reached", "2008-02-29", time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC), false},
		{"too old", "1899-12-31", time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC), true},
		{"zero reference", "2000-01-01", time.Time{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAdultBirthDate(tc.birth, tc.now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateAdultBirthDate() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestDeclarationPayloadRoundTrip(t *testing.T) {
	want := validDeclaration()
	encoded, err := EncodeDeclarationPayload(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeDeclarationPayload(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip mismatch: got %#v", got)
	}
}

func TestEncodeDeclarationPayloadRejectsFutureBirthDate(t *testing.T) {
	p := validDeclaration()
	p.DateOfBirth = time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	if _, err := EncodeDeclarationPayload(p); err == nil {
		t.Fatal("accepted a future civil date of birth")
	}
}

func TestEncodeDeclarationPayloadValidation(t *testing.T) {
	cases := map[string]func(*DeclarationPayload){
		"missing first name":              func(p *DeclarationPayload) { p.FirstName = "" },
		"missing last name":               func(p *DeclarationPayload) { p.LastName = " \t" },
		"first name over 150 code points": func(p *DeclarationPayload) { p.FirstName = strings.Repeat("界", 151) },
		"last name over 150 code points":  func(p *DeclarationPayload) { p.LastName = strings.Repeat("a", 151) },
		"invalid date":                    func(p *DeclarationPayload) { p.DateOfBirth = "1990-02-30" },
		"noncanonical rut":                func(p *DeclarationPayload) { p.RUT = "12.345.678-k" },
		"invalid rut":                     func(p *DeclarationPayload) { p.RUT = "12345678-0" },
		"wrong country":                   func(p *DeclarationPayload) { p.IDCountry = "CH" },
		"wrong document type":             func(p *DeclarationPayload) { p.DocumentType = "PASSPORT" },
		"invalid utf8":                    func(p *DeclarationPayload) { p.FirstName = string([]byte{0xff}) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := validDeclaration()
			mutate(&p)
			if _, err := EncodeDeclarationPayload(p); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDeclarationStrictJSON(t *testing.T) {
	valid, err := json.Marshal(validDeclaration())
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{
		append(append([]byte(nil), valid[:len(valid)-1]...), []byte(`,"extra":"value"}`)...),
		append(append([]byte(nil), valid...), []byte(` {}`)...),
		[]byte(`null`),
	} {
		if _, err := DecodeDeclarationPayload(raw); err == nil {
			t.Fatalf("accepted invalid JSON: %s", raw)
		}
	}
	if _, err := DecodeDeclarationPayload([]byte("\xff")); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
	encoded, err := EncodeDeclarationPayload(validDeclaration())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, valid) {
		t.Fatalf("encoding is not deterministic JSON: %s", encoded)
	}
}

func TestBuildDeclarationAAD(t *testing.T) {
	const account = "123e4567-e89b-12d3-a456-426614174000"
	const declaration = "123e4567-e89b-12d3-a456-426614174001"
	a, err := BuildDeclarationAAD(account, declaration, 1, "1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildDeclarationAAD(account, declaration, 1, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) || len(a) == 0 {
		t.Fatal("AAD must be stable and nonempty")
	}
	for _, tc := range []struct {
		name, account, declaration string
		revision                   int64
		schema                     string
	}{
		{"bad account UUID", "not-uuid", declaration, 1, "1"},
		{"bad declaration UUID", account, "not-uuid", 1, "1"},
		{"zero revision", account, declaration, 0, "1"},
		{"negative revision", account, declaration, -1, "1"},
		{"wrong schema", account, declaration, 1, "2"},
		{"empty schema", account, declaration, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BuildDeclarationAAD(tc.account, tc.declaration, tc.revision, tc.schema); err == nil {
				t.Fatal("expected invalid AAD input error")
			}
		})
	}
	changed, err := BuildDeclarationAAD(account, declaration, 2, "1")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, changed) {
		t.Fatal("AAD must bind revision")
	}
	changed, err = BuildDeclarationAAD(account, "123e4567-e89b-12d3-a456-426614174002", 1, "1")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, changed) {
		t.Fatal("AAD must bind declaration ID")
	}
}

func TestDeclarationErrorsDoNotExposePII(t *testing.T) {
	p := validDeclaration()
	p.FirstName = "private-name"
	p.RUT = "private-rut"
	_, err := EncodeDeclarationPayload(p)
	if err == nil {
		t.Fatal("expected invalid RUT")
	}
	if strings.Contains(err.Error(), p.FirstName) || strings.Contains(err.Error(), p.RUT) {
		t.Fatal("error exposed PII")
	}
}
