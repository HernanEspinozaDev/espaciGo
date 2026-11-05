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
