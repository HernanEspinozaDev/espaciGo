package main

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
)

type connectionSettings struct {
	databaseURL  string
	passwordFile string
	mode         string
	instance     string
	privateIP    bool
	timeout      time.Duration
}

func loadConnectionSettings(getenv func(string) string) (connectionSettings, error) {
	settings := connectionSettings{
		databaseURL: getenv("DATABASE_URL"),
		mode:        strings.TrimSpace(getenv("DATABASE_CONNECTION_MODE")),
		timeout:     10 * time.Minute,
	}
	if strings.TrimSpace(settings.databaseURL) == "" {
		return connectionSettings{}, errors.New("DATABASE_URL is required")
	}
	if settings.mode == "" {
		settings.mode = "direct"
	}
	if timeoutValue := strings.TrimSpace(getenv("DBMIGRATE_TIMEOUT")); timeoutValue != "" {
		timeout, err := time.ParseDuration(timeoutValue)
		if err != nil || timeout <= 0 || timeout > 4*time.Hour {
			return connectionSettings{}, errors.New("DBMIGRATE_TIMEOUT must be a positive duration no greater than 4h")
		}
		settings.timeout = timeout
	}
	switch settings.mode {
	case "direct":
		return settings, nil
	case "cloudsql":
	default:
		return connectionSettings{}, errors.New("DATABASE_CONNECTION_MODE must be direct or cloudsql")
	}
	settings.instance = strings.TrimSpace(getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"))
	if !cloudsql.ValidInstanceConnectionName(settings.instance) {
		return connectionSettings{}, errors.New("CLOUD_SQL_INSTANCE_CONNECTION_NAME must use project:region:instance format")
	}
	settings.passwordFile = strings.TrimSpace(getenv("DATABASE_PASSWORD_FILE"))
	if settings.passwordFile == "" {
		return connectionSettings{}, errors.New("DATABASE_PASSWORD_FILE is required for Cloud SQL mode")
	}
	privateIPValue := strings.TrimSpace(getenv("CLOUD_SQL_PRIVATE_IP"))
	if privateIPValue != "" {
		privateIP, err := strconv.ParseBool(privateIPValue)
		if err != nil {
			return connectionSettings{}, errors.New("CLOUD_SQL_PRIVATE_IP must be true or false")
		}
		settings.privateIP = privateIP
	}
	return settings, nil
}

func (settings connectionSettings) databaseURLWithPasswordFile() (string, error) {
	parsed, err := url.Parse(settings.databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.User == nil || parsed.User.Username() == "" {
		return "", errors.New("Cloud SQL DATABASE_URL must be a PostgreSQL URL with a username")
	}
	if _, hasInlinePassword := parsed.User.Password(); hasInlinePassword {
		return "", errors.New("Cloud SQL DATABASE_URL must not contain an inline password")
	}
	password, err := os.ReadFile(settings.passwordFile)
	if err != nil {
		return "", errors.New("could not read database password file")
	}
	passwordValue := strings.TrimSpace(string(password))
	if passwordValue == "" {
		return "", errors.New("database password file is empty")
	}
	parsed.User = url.UserPassword(parsed.User.Username(), passwordValue)
	return parsed.String(), nil
}
