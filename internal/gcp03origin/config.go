package gcp03origin

import (
	"errors"
	"fmt"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
)

const (
	ProjectID           = "espacigo-testprod"
	Region              = "southamerica-west1"
	RunID               = "g03o-1017"
	InstanceName        = "espacigo-gcp03-origin-g03o-1017-pg"
	DatabaseName        = "espacigo_scratch"
	AdminLogin          = "postgres"
	ObserverServiceAcct = "g03o-observer-g03o-1017@espacigo-testprod.iam.gserviceaccount.com"
	ObserverLogin       = "g03o-observer-g03o-1017@espacigo-testprod.iam"
	InstanceConnection  = ProjectID + ":" + Region + ":" + InstanceName
	TargetConfirmation  = InstanceConnection + "/" + DatabaseName
	AdminDatabaseURL    = "postgres://postgres@localhost:5432/espacigo_scratch?sslmode=disable"
	ObserverDatabaseURL = "postgres://g03o-observer-g03o-1017%40espacigo-testprod.iam@localhost:5432/espacigo_scratch?sslmode=disable"
	AdminPasswordFile   = "/secrets/admin/password"
)

type Phase string

const (
	PhaseBootstrap       Phase = "bootstrap"
	PhaseObserve         Phase = "observe"
	PhaseMembership      Phase = "membership"
	PhaseMembershipCheck Phase = "membership-check"
)

type Config struct {
	Phase        Phase
	Login        string
	DatabaseURL  string
	PasswordFile string
	Instance     string
	PrivateIP    bool
}

func LoadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("GCP-03 origin environment is unavailable")
	}
	phase := Phase(strings.TrimSpace(getenv("GCP03_ORIGIN_PHASE")))
	if phase != PhaseBootstrap && phase != PhaseObserve && phase != PhaseMembership && phase != PhaseMembershipCheck {
		return Config{}, errors.New("GCP03_ORIGIN_PHASE must be bootstrap, membership, membership-check, or observe")
	}
	if ValidateConfirmation(getenv("GCP03_ORIGIN_CONFIRM_TARGET")) != nil || getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME") != InstanceConnection || getenv("CLOUD_SQL_PRIVATE_IP") != "true" {
		return Config{}, errors.New("GCP-03 origin target confirmation does not match the fixed private rehearsal target")
	}
	for _, key := range []string{"GCP03_SQL", "GCP03_SQL_FILE", "GCP03_SQL_STDIN"} {
		if getenv(key) != "" {
			return Config{}, errors.New("GCP-03 origin does not accept SQL overrides")
		}
	}
	if err := cloudsql.ValidatePGEnvironment(getenv); err != nil {
		return Config{}, err
	}
	config := Config{Phase: phase, Instance: InstanceConnection, PrivateIP: true}
	if phase == PhaseBootstrap || phase == PhaseMembership || phase == PhaseMembershipCheck {
		config.Login, config.DatabaseURL, config.PasswordFile = AdminLogin, AdminDatabaseURL, AdminPasswordFile
	} else {
		config.Login, config.DatabaseURL = ObserverLogin, ObserverDatabaseURL
	}
	if getenv("DATABASE_URL") != config.DatabaseURL || getenv("DATABASE_CONNECTION_MODE") != "cloudsql" {
		return Config{}, errors.New("GCP-03 origin database connection does not match the fixed Cloud SQL target")
	}
	if (phase == PhaseBootstrap || phase == PhaseMembership || phase == PhaseMembershipCheck) && getenv("DATABASE_PASSWORD_FILE") != AdminPasswordFile {
		return Config{}, errors.New("GCP-03 origin admin phase requires the fixed admin secret mount")
	}
	if phase == PhaseObserve && getenv("DATABASE_PASSWORD_FILE") != "" {
		return Config{}, errors.New("GCP-03 origin observer must not receive a password file")
	}
	if err := cloudsql.ValidateConnectionURL(config.DatabaseURL); err != nil {
		return Config{}, fmt.Errorf("GCP-03 origin database URL is invalid")
	}
	return config, nil
}

func ValidateConfirmation(value string) error {
	if value != TargetConfirmation {
		return errors.New("GCP-03 origin target confirmation is invalid")
	}
	return nil
}
