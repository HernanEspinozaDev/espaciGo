package credentials_test

import (
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
)

func TestCredentialsProvideRandom256BitSecretsAndUUIDs(t *testing.T) {
	generator := credentials.Generator{}
	first, err := generator.Token()
	if err != nil {
		t.Fatal(err)
	}
	second, err := generator.Token()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(first))
	if err != nil || len(decoded) != 32 || first == second {
		t.Fatal("tokens lack expected entropy/encoding")
	}
	id, err := generator.ID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) {
		t.Fatal("invalid UUID credential identifier")
	}
}
