package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type config struct {
	httpAddr             string
	dbHost               string
	dbPort               uint16
	dbName               string
	dbUser               string
	dbPassword           string
	dbSSLMode            string
	dbSSLRootCert        string
	dbSSLCert            string
	dbSSLKey             string
	allowedOrigins       []string
	frontendPublicOrigin string
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{httpAddr: getenv("HTTP_ADDR")}
	if cfg.httpAddr == "" {
		cfg.httpAddr = ":8080"
	}
	if strings.TrimSpace(cfg.httpAddr) != cfg.httpAddr {
		return config{}, errors.New("HTTP_ADDR must not contain surrounding whitespace")
	}

	var err error
	if cfg.dbHost, err = required(getenv, "DATABASE_HOST"); err != nil {
		return config{}, err
	}
	portValue := getenv("DATABASE_PORT")
	if portValue == "" {
		cfg.dbPort = 5432
	} else {
		port, parseErr := strconv.ParseUint(portValue, 10, 16)
		if parseErr != nil || port == 0 {
			return config{}, errors.New("DATABASE_PORT must be between 1 and 65535")
		}
		cfg.dbPort = uint16(port)
	}
	if cfg.dbName, err = required(getenv, "DATABASE_NAME"); err != nil {
		return config{}, err
	}
	if cfg.dbUser, err = required(getenv, "DATABASE_USER"); err != nil {
		return config{}, err
	}
	cfg.dbSSLMode = strings.TrimSpace(getenv("DATABASE_SSLMODE"))
	if cfg.dbSSLMode == "" {
		cfg.dbSSLMode = "disable"
	}
	switch cfg.dbSSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return config{}, errors.New("DATABASE_SSLMODE must be disable, allow, prefer, require, verify-ca, or verify-full")
	}
	cfg.dbSSLRootCert = strings.TrimSpace(getenv("DATABASE_SSLROOTCERT"))
	cfg.dbSSLCert = strings.TrimSpace(getenv("DATABASE_SSLCERT"))
	cfg.dbSSLKey = strings.TrimSpace(getenv("DATABASE_SSLKEY"))
	if (cfg.dbSSLCert == "") != (cfg.dbSSLKey == "") {
		return config{}, errors.New("DATABASE_SSLCERT and DATABASE_SSLKEY must be configured together")
	}
	passwordPath, err := required(getenv, "DATABASE_PASSWORD_FILE")
	if err != nil {
		return config{}, err
	}
	password, err := os.ReadFile(passwordPath)
	if err != nil {
		return config{}, errors.New("could not read database password file")
	}
	cfg.dbPassword = strings.TrimSpace(string(password))
	if cfg.dbPassword == "" {
		return config{}, errors.New("database password file is empty")
	}

	for _, value := range strings.Split(getenv("CORS_ALLOWED_ORIGINS"), ",") {
		origin := strings.TrimSpace(value)
		if origin == "" {
			continue
		}
		if !validOrigin(origin) {
			return config{}, fmt.Errorf("invalid CORS origin %q", origin)
		}
		cfg.allowedOrigins = append(cfg.allowedOrigins, origin)
	}
	cfg.frontendPublicOrigin = strings.TrimSpace(getenv("FRONTEND_PUBLIC_ORIGIN"))
	if cfg.frontendPublicOrigin == "" {
		cfg.frontendPublicOrigin = "http://127.0.0.1:4200"
	}
	if !validOrigin(cfg.frontendPublicOrigin) {
		return config{}, errors.New("FRONTEND_PUBLIC_ORIGIN must be an HTTP(S) origin without path, query, or fragment")
	}
	return cfg, nil
}

func required(getenv func(string) string, key string) (string, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func validOrigin(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Path == "" || parsed.Path == "/"
}

func (cfg config) databaseURL() string {
	endpoint := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.dbUser, cfg.dbPassword),
		Host:   net.JoinHostPort(cfg.dbHost, strconv.Itoa(int(cfg.dbPort))),
		Path:   "/" + cfg.dbName,
	}
	query := url.Values{}
	query.Set("sslmode", cfg.dbSSLMode)
	if cfg.dbSSLRootCert != "" {
		query.Set("sslrootcert", cfg.dbSSLRootCert)
	}
	if cfg.dbSSLCert != "" {
		query.Set("sslcert", cfg.dbSSLCert)
		query.Set("sslkey", cfg.dbSSLKey)
	}
	endpoint.RawQuery = query.Encode()
	return endpoint.String()
}
