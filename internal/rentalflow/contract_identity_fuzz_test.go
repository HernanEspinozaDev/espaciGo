package rentalflow

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

func FuzzNormalizeContractIdentityAcceptsOnlySafeNames(f *testing.F) {
	for _, name := range [][]byte{
		[]byte("Arrendataria Sintética"),
		[]byte("المالك"),
		[]byte("Nombre\u202eprivado"),
		[]byte("Nombre\u2028Apellido"),
		[]byte("Nombre\nApellido"),
		[]byte(" Nombre"),
		[]byte(strings.Repeat("N", 201)),
		{0xff},
	} {
		f.Add(name)
	}
	f.Fuzz(func(t *testing.T, nameBytes []byte) {
		if len(nameBytes) > 512 {
			nameBytes = nameBytes[:512]
		}
		name := string(nameBytes)
		identity := ContractIdentity{SubjectID: testOwnerID, LegalName: name, RUT: "11.111.111-1"}
		normalized, accepted := normalizeContractIdentity(identity, testOwnerID)

		unsafe := !utf8.ValidString(name) || name == "" || name != strings.TrimSpace(name) || utf8.RuneCountInString(name) > 200
		for _, r := range name {
			unsafe = unsafe || unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
		}
		if unsafe && accepted {
			t.Fatalf("accepted unsafe synthetic legal name bytes %x as %q", nameBytes, normalized.LegalName)
		}
		if !accepted {
			return
		}
		if normalized.SubjectID != testOwnerID || normalized.LegalName != name {
			t.Fatalf("accepted identity changed scope or legal name: %+v", normalized)
		}
		canonicalRut, err := verification.NormalizeRut(normalized.RUT)
		if err != nil || canonicalRut != normalized.RUT {
			t.Fatalf("accepted RUT is not canonical: %q err=%v", normalized.RUT, err)
		}
		again, ok := normalizeContractIdentity(normalized, testOwnerID)
		if !ok || again != normalized {
			t.Fatalf("identity normalization is not idempotent: first=%+v second=%+v accepted=%v", normalized, again, ok)
		}
	})
}

func FuzzNormalizeContractIdentityRUTMatchesCanonicalValidator(f *testing.F) {
	for _, rut := range [][]byte{
		[]byte("11.111.111-1"),
		[]byte("11.111.111-2"),
		[]byte("11111111-1"),
		[]byte(" 11.111.111-1"),
		[]byte("11.111.111-1\n"),
		[]byte(strings.Repeat("1", 21)),
		{0xff},
	} {
		f.Add(rut)
	}
	f.Fuzz(func(t *testing.T, rutBytes []byte) {
		if len(rutBytes) > 64 {
			rutBytes = rutBytes[:64]
		}
		rut := string(rutBytes)
		canonicalRut, err := verification.NormalizeRut(rut)
		expected := err == nil && rut != "" && len(rut) <= 20 && rut == strings.TrimSpace(rut) &&
			utf8.ValidString(rut) && !strings.HasPrefix(canonicalRut, "-")
		for _, r := range rut {
			expected = expected && !unicode.IsControl(r) && !unicode.Is(unicode.Bidi_Control, r) && !unicode.Is(unicode.Zl, r) && !unicode.Is(unicode.Zp, r)
		}

		identity, accepted := normalizeContractIdentity(ContractIdentity{
			SubjectID: testOwnerID, LegalName: "Arrendador Sintético", RUT: rut,
		}, testOwnerID)
		if accepted != expected {
			t.Fatalf("RUT %x accepted=%v, canonical validator expects %v (canonical=%q err=%v)", rutBytes, accepted, expected, canonicalRut, err)
		}
		if accepted && identity.RUT != canonicalRut {
			t.Fatalf("RUT %x normalized to %q, want canonical %q", rutBytes, identity.RUT, canonicalRut)
		}
	})
}
