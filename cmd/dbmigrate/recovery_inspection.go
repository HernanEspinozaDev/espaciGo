package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"io"
	"net/url"
	"strings"
)

const recoveryInspectionInstance = "espacigo-testprod:southamerica-west1:espacigo-testprod-db"
const recoveryInspectionDatabase = "espacigo_dms_stage_20261020"

type recoveryInspectionOutput struct {
	Format        string                      `json:"format"`
	ReadOnly      bool                        `json:"read_only"`
	Database      string                      `json:"database"`
	ServerVersion string                      `json:"server_version"`
	SessionUser   string                      `json:"session_user"`
	CurrentUser   string                      `json:"current_user"`
	Applied       []migrator.HistoryEntry     `json:"applied"`
	Pending       []migrator.MigrationSummary `json:"pending"`
	Authority     migrator.RecoveryAuthority  `json:"authority"`
}

func validateRecoveryInspectionInvocation(enabled, inventory, schema, plan, release, applyACL, recoverACL, cloudOwner bool, settings connectionSettings, getenv func(string) string) error {
	if !enabled {
		return nil
	}
	if inventory || schema || plan || release || applyACL || recoverACL || cloudOwner {
		return errors.New("-recovery-inspection-only cannot combine with offline, schema, plan, apply, or administrative modes")
	}
	if settings.mode != "cloudsql" || settings.instance != recoveryInspectionInstance || !settings.privateIP || settings.setRole != "espacigo_schema_owner" || settings.passwordFile != "/var/secrets/migration/password" {
		return errors.New("recovery inspection requires the pinned Cloud SQL target and direct session")
	}
	if err := validateRecoveryInspectionURL(settings.databaseURL); err != nil {
		return err
	}
	if err := validateRecoveryInspectionEnvironment(getenv); err != nil {
		return err
	}
	return nil
}
func validateRecoveryInspectionURL(raw string) error {
	if err := cloudsql.ValidateConnectionURL(raw); err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil || strings.TrimPrefix(u.Path, "/") != recoveryInspectionDatabase || u.User == nil || u.User.Username() != "espacigo_migrator_login" {
		return errors.New("recovery inspection requires the pinned database and migrator login")
	}
	return nil
}
func validateRecoveryInspectionEnvironment(getenv func(string) string) error {
	return cloudsql.ValidatePGEnvironment(getenv)
}
func runRecoveryInspection(ctx context.Context, out io.Writer, settings connectionSettings, dir string, getenv func(string) string) error {
	if err := validateRecoveryInspectionInvocation(true, false, false, false, false, false, false, false, settings, getenv); err != nil {
		return err
	}
	raw, password, err := settings.databaseURLAndPasswordFile()
	if err != nil {
		return err
	}
	r, err := migrator.InspectRecoveryCloudSQL(ctx, raw, password, dir, recoveryInspectionInstance, true, "espacigo_schema_owner")
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(r)
}
