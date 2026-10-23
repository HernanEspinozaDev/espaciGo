package zohoauth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestNewRequiresExplicitTLSAndReadsMountedCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("application-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Host: "smtp.zoho.com", Port: "587", Security: "starttls", Username: "contacto@hotusoft.com", PasswordFile: path, From: "contacto@hotusoft.com", FromName: "EspaciGo (pruebas)", PublicOrigin: "https://espacigo.testingpage.store"}
	mailer, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if mailer.password != "application-secret" {
		t.Fatal("mounted SMTP credential was not read")
	}
	if url := mailer.actionURL("verification-id", identity.Secret("one-time-token")); url != "https://espacigo.testingpage.store/verificar-correo#token=one-time-token&token_id=verification-id" {
		t.Fatalf("verification link=%q", url)
	}
}

func TestNewRejectsInsecureOrMismatchedSMTPConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("application-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	base := Config{Host: "smtp.zoho.com", Port: "587", Security: "starttls", Username: "contacto@hotusoft.com", PasswordFile: path, From: "contacto@hotusoft.com", FromName: "EspaciGo (pruebas)", PublicOrigin: "https://espacigo.testingpage.store"}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Security = "none" },
		func(c *Config) { c.Port = "25" },
		func(c *Config) { c.Host = "mailpit.internal" },
		func(c *Config) { c.PublicOrigin = "https://espacigo.testingpage.store/path" },
	} {
		cfg := base
		mutate(&cfg)
		if _, err := New(cfg); err == nil {
			t.Fatalf("accepted invalid SMTP config: %+v", cfg)
		}
	}
}
