package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
)

func validDatabaseRoleName(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	for i, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' ||
			(i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

type connectionSettings struct {
	databaseURL  string
	passwordFile string
	mode         string
	instance     string
	privateIP    bool
	setRole      string
	timeout      time.Duration
}

func loadConnectionSettings(getenv func(string) string) (connectionSettings, error) {
	settings := connectionSettings{
		databaseURL: getenv("DATABASE_URL"),
		mode:        strings.TrimSpace(getenv("DATABASE_CONNECTION_MODE")),
		setRole:     strings.TrimSpace(getenv("DATABASE_SET_ROLE")),
		timeout:     10 * time.Minute,
	}
	if settings.setRole != "" && !validDatabaseRoleName(settings.setRole) {
		return connectionSettings{}, errors.New("DATABASE_SET_ROLE must be a simple PostgreSQL role name")
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
	if err := cloudsql.ValidatePGEnvironment(getenv); err != nil {
		return connectionSettings{}, err
	}
	if err := cloudsql.ValidateConnectionURL(settings.databaseURL); err != nil {
		return connectionSettings{}, err
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

func (settings connectionSettings) databaseURLAndPasswordFile() (string, string, error) {
	password, err := os.ReadFile(settings.passwordFile)
	if err != nil {
		return "", "", errors.New("could not read database password file")
	}
	passwordValue := strings.TrimSpace(string(password))
	if passwordValue == "" {
		return "", "", errors.New("database password file is empty")
	}
	return settings.databaseURL, passwordValue, nil
}
