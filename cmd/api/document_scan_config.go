package main

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	documentScanAddressEnv  = "DOCUMENT_VERIFICATION_CLAMAV_ADDRESS"
	documentScanMaxAgeEnv   = "DOCUMENT_VERIFICATION_CLAMAV_MAX_SIGNATURE_AGE"
	maxDocumentSignatureAge = 48 * time.Hour
)

type documentScanConfig struct {
	address         string
	maxSignatureAge time.Duration
}

func documentScanConfigFromEnv(getenv func(string) string) (documentScanConfig, bool, error) {
	if getenv == nil {
		return documentScanConfig{}, false, errors.New("document scanner environment is unavailable")
	}
	address := getenv(documentScanAddressEnv)
	maxAgeText := getenv(documentScanMaxAgeEnv)
	if address == "" && maxAgeText == "" {
		return documentScanConfig{}, false, nil
	}
	if strings.TrimSpace(address) != address || strings.TrimSpace(maxAgeText) != maxAgeText || address == "" || maxAgeText == "" {
		return documentScanConfig{}, false, errors.New("document scanner configuration is incomplete")
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return documentScanConfig{}, false, errors.New("document scanner address must be an explicit private IP and port")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() && !ip.IsPrivate() {
		return documentScanConfig{}, false, errors.New("document scanner address must be loopback or private")
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return documentScanConfig{}, false, errors.New("document scanner port is invalid")
	}
	maxAge, err := time.ParseDuration(maxAgeText)
	if err != nil || maxAge <= 0 || maxAge > maxDocumentSignatureAge {
		return documentScanConfig{}, false, errors.New("document scanner signature age is invalid")
	}
	return documentScanConfig{address: address, maxSignatureAge: maxAge}, true, nil
}
