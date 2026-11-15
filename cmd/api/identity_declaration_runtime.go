package main

import (
	"errors"

	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification/rutkeyfile"
)

const defaultIdentityDeclarationConsentVersion = "id-consent-20261108"
const defaultIdentityDeclarationRetention = 30 * 24 * time.Hour

func identityDeclarationRetentionPolicy(consentAt time.Time) time.Time {
	return consentAt.UTC().Add(defaultIdentityDeclarationRetention)
}

func loadIdentityDeclarationKeyring(getenv func(string) string) (*declarationcrypto.Keyring, error) {
	path := strings.TrimSpace(getenv("IDENTITY_DECLARATION_KEYRING_FILE"))
	if path == "" {
		path = strings.TrimSpace(getenv("LOCAL_IDENTITY_DECLARATION_KEYRING_FILE"))
	}
	if path == "" {
		return nil, nil
	}
	keyring, err := declarationcrypto.LoadFile(path)
	if err != nil {
		return nil, errors.New("identity declaration keyring configuration is invalid")
	}
	return keyring, nil
}

func identityDeclarationConsentVersion(getenv func(string) string) (string, error) {
	version := strings.TrimSpace(getenv("IDENTITY_DECLARATION_CONSENT_VERSION"))
	if version == "" {
		return defaultIdentityDeclarationConsentVersion, nil
	}
	if len(version) > 20 || strings.ContainsAny(version, " \t\r\n") {
		return "", errors.New("identity declaration consent version is invalid")
	}
	return version, nil
}

func loadRuntimeRutHMACKeyring(getenv func(string) string) (*verification.RutHMACKeyring, error) {
	path := strings.TrimSpace(getenv("IDENTITY_RUT_HMAC_KEYRING_FILE"))
	if path == "" {
		path = strings.TrimSpace(getenv("LOCAL_IDENTITY_RUT_HMAC_KEYRING_FILE"))
	}
	secret := strings.TrimSpace(getenv("IDENTITY_RUT_HMAC_SECRET"))
	if path != "" {
		if secret != "" {
			return nil, errors.New("identity RUT HMAC configuration is ambiguous")
		}
		keys, err := rutkeyfile.Load(path)
		if err != nil {
			return nil, errors.New("identity RUT HMAC configuration is invalid")
		}
		return keys, nil
	}
	keys, err := verification.NewRutHMACKeyring("v1", map[string][]byte{"v1": []byte(secret)})
	if err != nil {
		return nil, errors.New("identity RUT HMAC configuration is invalid")
	}
	return keys, nil
}

func newRuntimeIdentityDeclarationService(getenv func(string) string, repo verification.DeclarationRevisionRepository, now func() time.Time) (*verification.DeclarationService, error) {
	keyring, err := loadIdentityDeclarationKeyring(getenv)
	if err != nil || keyring == nil {
		return nil, err
	}
	consentVersion, err := identityDeclarationConsentVersion(getenv)
	if err != nil {
		return nil, err
	}
	rutKeys, err := loadRuntimeRutHMACKeyring(getenv)
	if err != nil {
		return nil, err
	}
	return verification.NewDeclarationServiceWithRutKeyring(repo, keyring, credentials.Generator{}, now, identityDeclarationRetentionPolicy, consentVersion, rutKeys)
}
