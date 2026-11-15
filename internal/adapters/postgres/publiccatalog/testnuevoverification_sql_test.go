package publiccatalogpg

import (
	"os"
	"strings"
	"testing"
)

func TestPublicVerificationSQLDerivesCurrentOwnerBadges(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, fragment := range []string{
		"identity_case.purpose='identity'",
		"identity_case.subject_id=owner.id",
		"space_case.purpose='space'",
		"space_case.space_id=e.id",
		"space_case.subject_id=owner.id",
		"state='aprobada'",
		"verified_at<=now()",
		"verified_until>now()",
		"document_expires_at>now()",
		"revoked_at IS NULL",
		"superseded_at IS NULL",
		"AS identity_verified",
		"AS space_verified",
		"verification.identity_verified,verification.space_verified",
		"&v.Verification.IdentityVerified, &v.Verification.SpaceVerified",
	} {
		if !strings.Contains(text, fragment) {
			t.Errorf("repository SQL/scan missing %q", fragment)
		}
	}
	if strings.Contains(text, "elegibilidad_verificacion_local") || strings.Contains(text, "tipo='kyc'") {
		t.Fatal("legacy KYC eligibility must not gate the public catalog")
	}
	for _, guard := range []string{
		"e.estado='activa'",
		"bloqueo_cuenta_administrativo_local",
		"reserva_ensayo_local_fixture",
		"owner.estado='activo'",
	} {
		if !strings.Contains(text, guard) {
			t.Errorf("required public eligibility guard removed: %s", guard)
		}
	}
}

func TestPublicVerificationSQLUsesLatestFinalizedDecision(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	const declaration = "const eligiblePublicSQL = `"
	start := strings.Index(text, declaration)
	if start < 0 {
		t.Fatal("eligible public SQL declaration not found")
	}
	start += len(declaration)
	end := strings.Index(text[start:], "`")
	if end < 0 {
		t.Fatal("eligible public SQL declaration is not terminated")
	}
	eligibleSQL := text[start : start+end]
	if !strings.Contains(text, "` + eligiblePublicSQL + `") {
		t.Fatal("eligible public SQL is not included in a query")
	}
	for _, decision := range []struct {
		name     string
		fragment string
	}{
		{
			name: "identity",
			fragment: "WHERE purpose='identity' AND subject_id=owner.id\n" +
				"         AND state IN ('aprobada','revocada','vencida')\n" +
				"       ORDER BY updated_at DESC,id DESC LIMIT 1",
		},
		{
			name: "space",
			fragment: "WHERE purpose='space' AND space_id=e.id AND subject_id=owner.id\n" +
				"         AND state IN ('aprobada','revocada','vencida')\n" +
				"       ORDER BY updated_at DESC,id DESC LIMIT 1",
		},
	} {
		t.Run(decision.name, func(t *testing.T) {
			if !strings.Contains(eligibleSQL, decision.fragment) {
				t.Fatalf("latest finalized %s decision must be selected deterministically", decision.name)
			}
		})
	}
}
