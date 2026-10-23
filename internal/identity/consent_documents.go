package identity

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
)

//go:embed consentdocs/*.json
var consentFiles embed.FS

type ConsentSection struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}
type ConsentDocument struct {
	ID       string           `json:"id"`
	Code     string           `json:"code"`
	Type     string           `json:"type"`
	Title    string           `json:"title"`
	Scope    string           `json:"scope"`
	Status   string           `json:"status"`
	Sections []ConsentSection `json:"sections"`
	SHA256   string           `json:"-"`
}

// PublicConsentDocuments are immutable readable evaluation documents. The HTTP
// boundary checks their exact bytes against published DB versions before use.
func PublicConsentDocuments() ([]ConsentDocument, error) {
	result := []ConsentDocument{}
	for _, name := range []string{"terms", "privacy"} {
		data, err := consentFiles.ReadFile("consentdocs/" + name + ".json")
		if err != nil {
			return nil, err
		}
		var document ConsentDocument
		if err := json.Unmarshal(data, &document); err != nil {
			return nil, err
		}
		digest := sha256.Sum256(data)
		document.SHA256 = hex.EncodeToString(digest[:])
		result = append(result, document)
	}
	return result, nil
}
