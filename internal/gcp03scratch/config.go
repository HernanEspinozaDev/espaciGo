package gcp03scratch

import (
	"errors"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
)

const (
	ProjectID                 = "espacigo-testprod"
	ProjectNumber             = "447373903153"
	Region                    = "southamerica-west1"
	InstanceName              = "espacigo-gcp03-pilot01-pg"
	DatabaseName              = "espacigo_scratch"
	MigratorLogin             = "espacigo_scratch_migrator"
	DatabaseOwnerLogin        = "postgres"
	RuntimeRole               = "espacigo_runtime"
	GADMRole                  = "espacigo_gadm_bootstrap"
	InstanceConnection        = ProjectID + ":" + Region + ":" + InstanceName
	TargetConfirmation        = InstanceConnection + "/" + DatabaseName
	DatabaseURL               = "postgres://" + MigratorLogin + "@localhost:5432/" + DatabaseName + "?sslmode=disable"
	DatabaseOwnerURL          = "postgres://" + DatabaseOwnerLogin + "@localhost:5432/" + DatabaseName + "?sslmode=disable"
	PasswordFile              = "/secrets/migrator/password"
	DatabaseOwnerPasswordFile = "/secrets/database-owner/password"
	MigrationCount            = 61
)

type Phase string

const (
	PhaseBootstrap                 Phase = "bootstrap"
	PhaseDatabaseACL               Phase = "database_acl"
	PhaseMigrate                   Phase = "migrate"
	PhaseFinalize                  Phase = "finalize"
	PhaseRetire                    Phase = "retire"
	PhaseRoleCommentPrepare        Phase = "role_comment_prepare"
	PhaseRoleCommentAfterReconcile Phase = "role_comment_after_reconcile"
)

var scratchPasswordPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

type Config struct {
	Phase        Phase
	DatabaseURL  string
	PasswordFile string
	Instance     string
	PrivateIP    bool
	Login        string
}

func LoadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("GCP-03 environment is unavailable")
	}
	phase := Phase(strings.TrimSpace(getenv("GCP03_PHASE")))
	switch phase {
	case PhaseBootstrap, PhaseDatabaseACL, PhaseMigrate, PhaseFinalize, PhaseRetire,
		PhaseRoleCommentPrepare, PhaseRoleCommentAfterReconcile:
	default:
		return Config{}, errors.New("GCP03_PHASE must name a supported scratch phase")
	}
	if getenv("GCP03_CONFIRM_TARGET") != TargetConfirmation {
		return Config{}, errors.New("GCP-03 target confirmation does not match the reviewed scratch instance and database")
	}
	if getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME") != InstanceConnection {
		return Config{}, errors.New("GCP-03 Cloud SQL instance does not match the reviewed scratch instance")
	}
	login, databaseURL, passwordFile := MigratorLogin, DatabaseURL, PasswordFile
	if phase == PhaseDatabaseACL || phase == PhaseRetire {
		login = DatabaseOwnerLogin
		databaseURL = DatabaseOwnerURL
		passwordFile = DatabaseOwnerPasswordFile
	}
	if getenv("DATABASE_URL") != databaseURL {
		return Config{}, errors.New("GCP-03 database URL does not match the reviewed scratch database")
	}
	if getenv("DATABASE_PASSWORD_FILE") != passwordFile {
		return Config{}, errors.New("GCP-03 password must come from the scratch secret mount")
	}
	if getenv("CLOUD_SQL_PRIVATE_IP") != "true" {
		return Config{}, errors.New("GCP-03 requires the Cloud SQL private IP connector")
	}
	for _, key := range []string{"GCP03_SQL", "GCP03_SQL_FILE", "GCP03_SQL_STDIN"} {
		if getenv(key) != "" {
			return Config{}, errors.New("GCP-03 does not accept SQL text or SQL file overrides")
		}
	}
	if err := cloudsql.ValidatePGEnvironment(getenv); err != nil {
		return Config{}, err
	}
	if err := cloudsql.ValidateConnectionURL(databaseURL); err != nil {
		return Config{}, errors.New("GCP-03 database URL failed validation")
	}
	return Config{
		Phase:        phase,
		DatabaseURL:  databaseURL,
		PasswordFile: passwordFile,
		Instance:     InstanceConnection,
		PrivateIP:    true,
		Login:        login,
	}, nil
}

func IsRoleCommentProbePhase(phase Phase) bool {
	return phase == PhaseRoleCommentPrepare || phase == PhaseRoleCommentAfterReconcile
}

func ReadPassword(path string) (string, error) {
	if path != PasswordFile && path != DatabaseOwnerPasswordFile {
		return "", errors.New("GCP-03 password path is not the reviewed secret mount")
	}
	value, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("could not read GCP-03 scratch password secret")
	}
	return parsePassword(strings.NewReader(string(value)))
}

func parsePassword(reader io.Reader) (string, error) {
	value, err := io.ReadAll(io.LimitReader(reader, 256))
	if err != nil {
		return "", errors.New("could not read GCP-03 scratch password secret")
	}
	password := string(value)
	if !scratchPasswordPattern.MatchString(password) {
		return "", errors.New("GCP-03 scratch password secret has an invalid format")
	}
	return password, nil
}
