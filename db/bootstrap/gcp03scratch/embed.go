package gcp03scratch

import "embed"

//go:embed 001_pre_migrate.sql
var PreMigrate string

//go:embed 002_retire_migrator.sql
var RetireMigrator string

//go:embed 003_database_acl.sql
var DatabaseACL string

//go:embed 004_grant_owner_history.sql
var GrantOwnerHistory string

// Files embeds the fixed, versioned SQL statements used by the GCP-03 scratch
// executor. Callers must not accept SQL text from flags, environment, or stdin.
var Files embed.FS
