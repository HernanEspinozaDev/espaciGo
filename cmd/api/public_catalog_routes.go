package main

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func localPublicCatalogKey(path string) ([]byte, error) {
	fail := errors.New("standalone public catalog requires a private stable cursor key file")
	if !filepath.IsAbs(path) {
		return nil, fail
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, fail
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fail
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil || len(key) != 32 {
		return nil, fail
	}
	return key, nil
}

// Public reads can be exercised independently from booking/payment trials.
// The explicit flag authorizes only an exact isolated local target.
func localPublicCatalogEnabled(getenv func(string) string, profile runtimeProfileName) (bool, error) {
	value := getenv("LOCAL_PUBLIC_CATALOG_TRIAL")
	if value == "" {
		return false, nil
	}
	if value != "1" || getenv("LOCAL_AUTH_PROTOTYPE") != "1" || getenv("LOCAL_BOOKING_TRIAL") != "" || getenv("APP_PROFILE") != "" {
		return false, errors.New("standalone public catalog requires the exact isolated synthetic target")
	}
	httpAddr := getenv("HTTP_ADDR")
	authorizedTarget := profile == runtimeProfileWorkerFree && isAuthorizedF5Runtime(getenv, httpAddr) ||
		profile == runtimeProfileWizardLocal && isAuthorizedWizardRuntime(getenv, httpAddr)
	if !authorizedTarget {
		return false, errors.New("standalone public catalog requires the exact isolated synthetic target")
	}
	return true, nil
}
