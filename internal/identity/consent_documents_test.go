package identity

import (
	"os"
	"strings"
	"testing"
)

func TestConsentDocumentsMatchAdditivePublishedVersions(t *testing.T) {
	docs, err := PublicConsentDocuments()
	if err != nil || len(docs) != 2 {
		t.Fatal("read consent documents")
	}
	migration, err := os.ReadFile("../../db/migrations/V000075__auth_evaluation_consent_documents.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if !strings.Contains(string(migration), d.ID) || !strings.Contains(string(migration), d.SHA256) || len(d.Sections) < 5 || d.Status == "" {
			t.Fatal("document publication/content integrity mismatch")
		}
	}
}
