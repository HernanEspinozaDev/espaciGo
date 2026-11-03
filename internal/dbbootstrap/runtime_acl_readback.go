package dbbootstrap

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// RuntimeACLSource is one applied, checksum-verified migration source used to
// derive the final runtime/PUBLIC ACL contract.
type RuntimeACLSource struct {
	Version int
	SQL     []byte
}

// RuntimeACLRepairableDriftError reports confirmed direct-ACL differences that
// the source-derived replay can reconcile after every other readback check passes.
type RuntimeACLRepairableDriftError struct {
	Count  int
	Detail string
}

func (e *RuntimeACLRepairableDriftError) Error() string {
	if e == nil || strings.TrimSpace(e.Detail) == "" {
		return "runtime ACL has confirmed replayable direct-ACL differences"
	}
	if e.Count > 0 {
		return fmt.Sprintf("runtime ACL has %d confirmed replayable direct-ACL differences: %s", e.Count, e.Detail)
	}
	return "runtime ACL has confirmed replayable direct-ACL differences: " + e.Detail
}

func summarizeRuntimeACLRepairableDrifts(details []string) (int, string) {
	const maxExamples = 5
	count := len(details)
	if count <= maxExamples {
		return count, strings.Join(details, "; ")
	}
	examples := append([]string(nil), details[:maxExamples]...)
	examples = append(examples, fmt.Sprintf("and %d more", count-maxExamples))
	return count, strings.Join(examples, "; ")
}

type runtimeACLGrantKey struct {
	Grantee    string
	ObjectKind string
	Object     string
	Privilege  string
	Column     string
}

type runtimeACLExpectedGrant struct {
	Granted     bool
	GrantOption bool
	Grantors    map[string]struct{}
	Conditions  map[string]struct{}
	DynamicSQL  bool
}

// Migration GRANT statements omit GRANTED BY. The reviewed contract therefore
// requires the recorded grantor to equal each resolved object's owner; this
// policy is verified from PostgreSQL catalogs rather than inferred from SQL.
const runtimeACLGrantorObjectOwner = "$object-owner"

// foldRuntimeACLEffects computes the final direct grants represented by an
// ordered source contract. It intentionally handles only the ACL grammar that
// ParseACLStatementsForGrantee accepts; unknown effects fail closed.
func foldRuntimeACLEffects(effects []RuntimeACLEffect) (map[runtimeACLGrantKey]runtimeACLExpectedGrant, error) {
	state := make(map[runtimeACLGrantKey]runtimeACLExpectedGrant)
	for i, effect := range effects {
		action := strings.ToUpper(strings.TrimSpace(effect.Action))
		kind := strings.ToUpper(strings.TrimSpace(effect.ObjectKind))
		privilege := strings.ToUpper(strings.TrimSpace(effect.Privilege))
		grantee := strings.TrimSpace(effect.Grantee)
		if grantee == "" {
			grantee = "espacigo_runtime"
		}
		var err error
		grantee, err = normalizeACLGrantee(grantee)
		if err != nil {
			return nil, fmt.Errorf("ACL effect %d: %w", i, err)
		}
		if action != "GRANT" && action != "REVOKE" {
			return nil, fmt.Errorf("ACL effect %d has unsupported action", i)
		}
		if effect.Object == "" || !validACLPrivilege(kind, privilege) {
			return nil, fmt.Errorf("ACL effect %d has unsupported object or privilege", i)
		}
		if effect.Column != "" && (kind != "TABLE" || privilege == "ALL PRIVILEGES" ||
			(privilege != "SELECT" && privilege != "INSERT" && privilege != "UPDATE" && privilege != "REFERENCES")) {
			return nil, fmt.Errorf("ACL effect %d has unsupported column scope", i)
		}

		privileges := []string{privilege}
		if privilege == "ALL PRIVILEGES" {
			privileges = allACLPrivileges(kind)
		}
		for _, item := range privileges {
			key := runtimeACLGrantKey{
				Grantee: grantee, ObjectKind: kind, Object: strings.ToLower(effect.Object),
				Privilege: item, Column: strings.ToLower(effect.Column),
			}
			if action == "REVOKE" {
				// Table-level ACLs and column ACLs are separate PostgreSQL catalogs;
				// a table-level revoke does not remove a column-scoped grant.
				revoked := runtimeACLExpectedGrant{DynamicSQL: effect.DynamicSQL}
				if effect.Condition != "" {
					revoked.Conditions = map[string]struct{}{effect.Condition: {}}
				}
				state[key] = revoked
				continue
			}
			grant := state[key]
			grant.Granted = true
			grant.GrantOption = grant.GrantOption || effect.GrantOption
			if effect.Grantor != "" {
				if grant.Grantors == nil {
					grant.Grantors = make(map[string]struct{})
				}
				grant.Grantors[effect.Grantor] = struct{}{}
			}
			if effect.Condition != "" {
				if grant.Conditions == nil {
					grant.Conditions = make(map[string]struct{})
				}
				grant.Conditions[effect.Condition] = struct{}{}
			}
			grant.DynamicSQL = grant.DynamicSQL || effect.DynamicSQL
			state[key] = grant
		}
	}
	if len(state) == 0 && len(effects) > 0 {
		return nil, errors.New("ACL source contract resolves to no grants")
	}
	return state, nil
}

func allACLPrivileges(kind string) []string {
	switch kind {
	case "TABLE":
		return []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"}
	case "SEQUENCE":
		return []string{"USAGE", "SELECT", "UPDATE"}
	case "FUNCTION":
		return []string{"EXECUTE"}
	case "SCHEMA":
		return []string{"USAGE", "CREATE"}
	default:
		return nil
	}
}

// BuildRuntimeACLContract combines immutable applied-migration ACL effects with
// the shared administrative runtime grant source, then folds them in execution
// order. Migration versions must be strictly increasing; callers must first
// verify the migration ledger against the source checksums. Since GRANT
// statements do not carry an explicit grantor, catalog readback enforces the
// reviewed object-owner grantor policy for each resolved object.
func BuildRuntimeACLContract(sources []RuntimeACLSource) (map[runtimeACLGrantKey]runtimeACLExpectedGrant, error) {
	effects, err := extractRuntimeACLSourceEffects(sources)
	if err != nil {
		return nil, err
	}
	for i := range effects {
		effects[i].Grantor = runtimeACLGrantorObjectOwner
	}
	for _, source := range []string{runtimeACLSQL, runtimeACLReplaySQL, runtimeACLOverridesSQL} {
		parsed, err := ParseRuntimeACLStatements(source)
		if err != nil {
			return nil, fmt.Errorf("shared runtime ACL source is invalid: %w", err)
		}
		for i := range parsed {
			parsed[i].Grantor = runtimeACLGrantorObjectOwner
		}
		effects = append(effects, parsed...)
	}
	// PostgreSQL 15+ creates the public schema with USAGE for PUBLIC and
	// ownership by pg_database_owner; this catalog baseline is not a migration
	// GRANT and is checked separately from the replayed postgres grants.
	effects = append(effects, RuntimeACLEffect{
		Action: "GRANT", Grantee: "PUBLIC", ObjectKind: "SCHEMA", Object: "public",
		Privilege: "USAGE", Grantor: runtimeACLGrantorObjectOwner,
	})
	return foldRuntimeACLEffects(effects)
}

// buildF5RuntimeACLContract preserves the approved pre-V81 PUBLIC baseline on
// the exact F5 target while retaining PUBLIC ACL effects authored by new,
// applied migrations. Runtime-role effects remain source-derived for all
// versions. The caller must separately verify the pinned F5 baseline.
func buildF5RuntimeACLContract(sources []RuntimeACLSource) (map[runtimeACLGrantKey]runtimeACLExpectedGrant, error) {
	effects, err := extractF5RuntimeACLSourceEffects(sources)
	if err != nil {
		return nil, err
	}
	for i := range effects {
		effects[i].Grantor = runtimeACLGrantorObjectOwner
	}
	for _, source := range []string{runtimeACLSQL, runtimeACLReplaySQL, runtimeACLOverridesSQL} {
		parsed, err := ParseRuntimeACLStatements(source)
		if err != nil {
			return nil, fmt.Errorf("shared runtime ACL source is invalid: %w", err)
		}
		for i := range parsed {
			parsed[i].Grantor = runtimeACLGrantorObjectOwner
		}
		effects = append(effects, parsed...)
	}
	return foldRuntimeACLEffects(effects)
}

func extractF5RuntimeACLSourceEffects(sources []RuntimeACLSource) ([]RuntimeACLEffect, error) {
	var effects []RuntimeACLEffect
	previousVersion := 0
	for _, source := range sources {
		if source.Version <= previousVersion || len(source.SQL) == 0 {
			return nil, errors.New("runtime ACL sources must have unique positive versions and non-empty SQL")
		}
		previousVersion = source.Version
		runtimeEffects, err := ExtractMigrationRuntimeACL(source.Version, source.SQL)
		if err != nil {
			return nil, err
		}
		publicEffects, err := ExtractMigrationACLForGrantee(source.Version, source.SQL, "PUBLIC")
		if err != nil {
			return nil, err
		}
		effects = append(effects, runtimeEffects...)
		if source.Version >= 81 {
			for _, effect := range publicEffects {
				if effect.Condition == "database-not:"+runtimeACLF5DatabaseName &&
					effect.Action == "REVOKE" && effect.ObjectKind == "FUNCTION" &&
					effect.Object == "public.document_verification_space_owner_changed()" &&
					effect.Privilege == "EXECUTE" {
					continue
				}
				effects = append(effects, effect)
			}
		}
	}
	return effects, nil
}

func extractRuntimeACLSourceEffects(sources []RuntimeACLSource) ([]RuntimeACLEffect, error) {
	var effects []RuntimeACLEffect
	previousVersion := 0
	for _, source := range sources {
		if source.Version <= previousVersion || len(source.SQL) == 0 {
			return nil, errors.New("runtime ACL sources must have unique positive versions and non-empty SQL")
		}
		previousVersion = source.Version
		runtimeEffects, err := ExtractMigrationRuntimeACL(source.Version, source.SQL)
		if err != nil {
			return nil, err
		}
		publicEffects, err := ExtractMigrationACLForGrantee(source.Version, source.SQL, "PUBLIC")
		if err != nil {
			return nil, err
		}
		effects = append(effects, runtimeEffects...)
		effects = append(effects, publicEffects...)
	}
	return effects, nil
}

// ApplyRuntimeACLFromMigrations replays only the parsed runtime/PUBLIC ACL
// effects from the applied migration sources. It is intended for the guarded
// administrator ACL transaction before the shared runtime overrides are run.
func ApplyRuntimeACLFromMigrations(ctx context.Context, conn ScratchPermissionDB, sources []RuntimeACLSource) error {
	if conn == nil {
		return errors.New("runtime ACL migration executor is required")
	}
	effects, err := extractRuntimeACLSourceEffects(sources)
	if err != nil {
		return err
	}
	for i, effect := range effects {
		key := runtimeACLGrantKey{Grantee: effect.Grantee, ObjectKind: effect.ObjectKind, Object: effect.Object, Privilege: effect.Privilege, Column: effect.Column}
		if _, err := resolveRuntimeACLObject(ctx, conn, key); err != nil {
			return fmt.Errorf("migration ACL effect %d references an unavailable object: %w", i, err)
		}
		statement, err := formatRuntimeACLEffect(effect)
		if err != nil {
			return fmt.Errorf("migration ACL effect %d is invalid: %w", i, err)
		}
		if _, err := conn.Exec(ctx, statement); err != nil {
			return fmt.Errorf("migration ACL effect %d could not be applied: %w", i, err)
		}
	}
	return nil
}

// ApplyRuntimeACLFromMigrationsF5 replays runtime ACL effects from all applied
// migrations and PUBLIC effects only from V81 onward. It checks the exact F5
// PUBLIC/default baseline before and after replay; callers must use a
// transaction and roll it back if a later readback fails.
func ApplyRuntimeACLFromMigrationsF5(ctx context.Context, conn ScratchPermissionDB, sources []RuntimeACLSource) error {
	if conn == nil {
		return errors.New("F5 runtime ACL migration executor is required")
	}
	if err := verifyRuntimeACLF5PublicBaseline(ctx, conn); err != nil {
		return err
	}
	effects, err := extractF5RuntimeACLSourceEffects(sources)
	if err != nil {
		return err
	}
	for i, effect := range effects {
		key := runtimeACLGrantKey{Grantee: effect.Grantee, ObjectKind: effect.ObjectKind, Object: effect.Object, Privilege: effect.Privilege, Column: effect.Column}
		if _, err := resolveRuntimeACLObject(ctx, conn, key); err != nil {
			return fmt.Errorf("F5 migration ACL effect %d references an unavailable object: %w", i, err)
		}
		statement, err := formatRuntimeACLEffect(effect)
		if err != nil {
			return fmt.Errorf("F5 migration ACL effect %d is invalid: %w", i, err)
		}
		if _, err := conn.Exec(ctx, statement); err != nil {
			return fmt.Errorf("F5 migration ACL effect %d could not be applied: %w", i, err)
		}
	}
	if err := verifyRuntimeACLF5PublicBaseline(ctx, conn); err != nil {
		return fmt.Errorf("F5 PUBLIC/default ACL baseline changed during ACL replay: %w", err)
	}
	return nil
}

func formatRuntimeACLEffect(effect RuntimeACLEffect) (string, error) {
	action := strings.ToUpper(strings.TrimSpace(effect.Action))
	kind := strings.ToUpper(strings.TrimSpace(effect.ObjectKind))
	privilege := strings.ToUpper(strings.TrimSpace(effect.Privilege))
	grantee := strings.TrimSpace(effect.Grantee)
	if grantee == "" {
		grantee = "espacigo_runtime"
	}
	grantee, err := normalizeACLGrantee(grantee)
	if err != nil {
		return "", err
	}
	if action != "GRANT" && action != "REVOKE" || !validACLPrivilege(kind, privilege) {
		return "", errors.New("unsupported ACL action or privilege")
	}
	if effect.GrantOption && action != "GRANT" {
		return "", errors.New("REVOKE cannot carry a grant option")
	}
	revokeBehavior := strings.ToUpper(strings.TrimSpace(effect.RevokeBehavior))
	if action == "GRANT" && revokeBehavior != "" ||
		(action == "REVOKE" && revokeBehavior != "" && revokeBehavior != "CASCADE" && revokeBehavior != "RESTRICT") {
		return "", errors.New("unsupported ACL revoke behavior")
	}
	object := strings.ToLower(strings.TrimSpace(effect.Object))
	if kind == "FUNCTION" {
		if err := validateACLFunctionIdentity(object); err != nil {
			return "", err
		}
	} else if kind == "SCHEMA" {
		if !regexp.MustCompile(`^[a-z_][a-z0-9_]*$`).MatchString(object) {
			return "", errors.New("invalid schema identity")
		}
	} else if !aclIdent.MatchString(object) {
		return "", errors.New("invalid object identity")
	}
	column := strings.ToLower(strings.TrimSpace(effect.Column))
	if column != "" && (kind != "TABLE" || !regexp.MustCompile(`^[a-z_][a-z0-9_]*$`).MatchString(column) ||
		(privilege != "SELECT" && privilege != "INSERT" && privilege != "UPDATE" && privilege != "REFERENCES")) {
		return "", errors.New("invalid column ACL scope")
	}
	objectKind := kind
	if kind == "TABLE" && column != "" {
		privilege += " (" + column + ")"
	}
	direction := "TO"
	if action == "REVOKE" {
		direction = "FROM"
	}
	statement := fmt.Sprintf("%s %s ON %s %s %s %s", action, privilege, objectKind, object, direction, grantee)
	if effect.GrantOption {
		statement += " WITH GRANT OPTION"
	}
	if revokeBehavior != "" {
		statement += " " + revokeBehavior
	}
	return statement, nil
}

type runtimeACLResolvedKey struct {
	Grantee, ObjectKind, ObjectID, Privilege, Column string
}

type runtimeACLObservedGrant struct {
	GrantOption              bool
	Default                  bool
	InitialPrivilegeBaseline bool
	Grantor                  string
	Owner                    string
}

func runtimeACLGrantorMatches(expected string, actual runtimeACLObservedGrant) bool {
	if expected == runtimeACLGrantorObjectOwner {
		return actual.Owner != "" && actual.Grantor == actual.Owner
	}
	return actual.Grantor == expected
}

func allowImplicitDefaultPublicFunctionExecute(key runtimeACLResolvedKey, actual runtimeACLObservedGrant, modeled bool) bool {
	return !modeled && actual.Default && !actual.GrantOption && key.Grantee == "PUBLIC" &&
		key.ObjectKind == "FUNCTION" && key.Privilege == "EXECUTE" &&
		actual.Owner != "" && actual.Grantor == actual.Owner
}

func runtimeACLEffectivePrivilegeExpected(runtimeGrant, publicGrant bool) bool {
	return runtimeGrant || publicGrant
}

func ignoreUnmodeledF5PublicACL(key runtimeACLResolvedKey, preserveF5Baseline bool) bool {
	return preserveF5Baseline && key.Grantee == "PUBLIC"
}

// VerifyRuntimeACLReadback compares source-derived direct grants and explicit
// revocations with PostgreSQL catalogs, checks effective privileges, verifies
// the API-login-to-runtime SET ROLE membership, and rejects runtime ownership
// or unsafe role attributes. The caller must first validate the migration
// ledger/checksums and hold the migration advisory lock.
func VerifyRuntimeACLReadback(ctx context.Context, conn ScratchPermissionDB, sources []RuntimeACLSource) error {
	if conn == nil {
		return errors.New("runtime ACL readback connection is required")
	}
	expected, err := BuildRuntimeACLContract(sources)
	if err != nil {
		return fmt.Errorf("derive runtime ACL contract: %w", err)
	}
	return verifyRuntimeACLReadbackContract(ctx, conn, expected, false)
}

// VerifyRuntimeACLReadbackF5 is scoped to the explicitly approved synthetic
// F5 target. Historical PUBLIC ACLs are covered by the pinned full catalog
// fingerprint; newly added V81+ PUBLIC effects remain source-derived.
func VerifyRuntimeACLReadbackF5(ctx context.Context, conn ScratchPermissionDB, sources []RuntimeACLSource) error {
	if conn == nil {
		return errors.New("F5 runtime ACL readback connection is required")
	}
	if err := verifyRuntimeACLF5PublicBaseline(ctx, conn); err != nil {
		return err
	}
	expected, err := buildF5RuntimeACLContract(sources)
	if err != nil {
		return fmt.Errorf("derive F5 runtime ACL contract: %w", err)
	}
	return verifyRuntimeACLReadbackContract(ctx, conn, expected, true)
}

func verifyRuntimeACLReadbackContract(ctx context.Context, conn ScratchPermissionDB, expected map[runtimeACLGrantKey]runtimeACLExpectedGrant, preserveF5Baseline bool) error {
	if err := verifyRuntimeACLRoleStructure(ctx, conn, preserveF5Baseline); err != nil {
		return err
	}
	if err := verifyRuntimeACLConditions(ctx, conn, expected); err != nil {
		return err
	}
	if err := rejectUnmodeledRuntimeACLClasses(ctx, conn, preserveF5Baseline); err != nil {
		return err
	}
	if err := verifyRuntimeACLDatabaseCatalog(ctx, conn, preserveF5Baseline); err != nil {
		return err
	}
	if err := verifyRuntimeACLInitialPrivilegeBaselines(ctx, conn); err != nil {
		return err
	}
	resolved := make(map[runtimeACLResolvedKey]runtimeACLExpectedGrant, len(expected))
	checkedOwners := make(map[string]struct{})
	repairableDirectACLDrifts := make(map[runtimeACLResolvedKey]struct{})
	var repairableDriftDetails []string
	markRepairableDrift := func(key runtimeACLResolvedKey, detail string) {
		effectiveKey := key
		effectiveKey.Grantee = "espacigo_runtime"
		repairableDirectACLDrifts[effectiveKey] = struct{}{}
		repairableDriftDetails = append(repairableDriftDetails, detail)
	}
	for key, grant := range expected {
		objectID, err := resolveRuntimeACLObject(ctx, conn, key)
		if err != nil {
			return fmt.Errorf("resolve ACL object %s %s: %w", key.ObjectKind, key.Object, err)
		}
		ownerKey := key.ObjectKind + ":" + objectID
		if _, checked := checkedOwners[ownerKey]; !checked {
			if err := verifyRuntimeACLObjectOwner(ctx, conn, key.ObjectKind, objectID); err != nil {
				return fmt.Errorf("verify ACL object owner %s %s: %w", key.ObjectKind, key.Object, err)
			}
			checkedOwners[ownerKey] = struct{}{}
		}
		resolved[runtimeACLResolvedKey{key.Grantee, key.ObjectKind, objectID, key.Privilege, key.Column}] = grant
	}
	observed, err := readRuntimeACLDirectGrants(ctx, conn)
	if err != nil {
		return err
	}
	for key, actuals := range observed {
		want, modeled := resolved[key]
		if !modeled {
			if ignoreUnmodeledF5PublicACL(key, preserveF5Baseline) {
				continue
			}
			allInitialPrivilegeBaseline := len(actuals) > 0
			for _, actual := range actuals {
				allInitialPrivilegeBaseline = allInitialPrivilegeBaseline && actual.InitialPrivilegeBaseline
			}
			if allInitialPrivilegeBaseline {
				ownerKey := key.ObjectKind + ":" + key.ObjectID
				if _, checked := checkedOwners[ownerKey]; !checked {
					if err := verifyRuntimeACLObjectOwner(ctx, conn, key.ObjectKind, key.ObjectID); err != nil {
						return fmt.Errorf("verify extension ACL object owner: %w", err)
					}
					checkedOwners[ownerKey] = struct{}{}
				}
				continue
			}
			if len(actuals) == 1 && allowImplicitDefaultPublicFunctionExecute(key, actuals[0], false) {
				ownerKey := key.ObjectKind + ":" + key.ObjectID
				if _, checked := checkedOwners[ownerKey]; !checked {
					if err := verifyRuntimeACLObjectOwner(ctx, conn, key.ObjectKind, key.ObjectID); err != nil {
						return fmt.Errorf("verify default ACL object owner: %w", err)
					}
					checkedOwners[ownerKey] = struct{}{}
				}
				continue
			}
			return fmt.Errorf("unexpected direct ACL grant: grantee=%s kind=%s object_oid=%s privilege=%s column=%s", key.Grantee, key.ObjectKind, key.ObjectID, key.Privilege, key.Column)
		}
		if !want.Granted {
			return fmt.Errorf("direct ACL grant remains although the source contract revokes it: grantee=%s kind=%s object_oid=%s privilege=%s column=%s", key.Grantee, key.ObjectKind, key.ObjectID, key.Privilege, key.Column)
		}
		if len(want.Grantors) != len(actuals) {
			return fmt.Errorf("direct ACL grantor count differs from source contract: grantee=%s kind=%s object_oid=%s privilege=%s", key.Grantee, key.ObjectKind, key.ObjectID, key.Privilege)
		}
		for _, actual := range actuals {
			if allowImplicitDefaultPublicFunctionExecute(key, actual, true) {
				return fmt.Errorf("source-modeled function ACL remains only an implicit default: object_oid=%s", key.ObjectID)
			}
			if actual.GrantOption != want.GrantOption {
				return fmt.Errorf("direct ACL grant option differs from source contract: grantee=%s kind=%s object_oid=%s privilege=%s", key.Grantee, key.ObjectKind, key.ObjectID, key.Privilege)
			}
			grantorMatches := false
			for expectedGrantor := range want.Grantors {
				if runtimeACLGrantorMatches(expectedGrantor, actual) {
					grantorMatches = true
					break
				}
			}
			if !grantorMatches {
				return fmt.Errorf("direct ACL grantor differs from source contract: grantee=%s kind=%s object_oid=%s privilege=%s grantor=%s", key.Grantee, key.ObjectKind, key.ObjectID, key.Privilege, actual.Grantor)
			}
			if actual.Owner == "espacigo_runtime" || actual.Grantor == "espacigo_runtime" {
				return fmt.Errorf("ACL owner/grantor is outside the reviewed owner/grantor contract: kind=%s object_oid=%s", key.ObjectKind, key.ObjectID)
			}
		}
	}
	for key, want := range resolved {
		_, exists := observed[key]
		if runtimeACLDirectDifferenceRepairable(want.Granted, exists) {
			markRepairableDrift(key, fmt.Sprintf("required grant is missing: grantee=%s kind=%s object_oid=%s privilege=%s column=%s", key.Grantee, key.ObjectKind, key.ObjectID, key.Privilege, key.Column))
		}
	}
	if err := verifyRuntimeACLEffectivePrivileges(ctx, conn, resolved, observed, repairableDirectACLDrifts); err != nil {
		return err
	}
	if len(repairableDriftDetails) > 0 {
		count, detail := summarizeRuntimeACLRepairableDrifts(repairableDriftDetails)
		return &RuntimeACLRepairableDriftError{Count: count, Detail: detail}
	}
	return nil
}

func verifyRuntimeACLConditions(ctx context.Context, conn ScratchPermissionDB, expected map[runtimeACLGrantKey]runtimeACLExpectedGrant) error {
	checkedRoles := make(map[string]struct{})
	checkedDatabases := make(map[string]struct{})
	for _, grant := range expected {
		for condition := range grant.Conditions {
			if database, ok := strings.CutPrefix(condition, "database-not:"); ok {
				if database != runtimeACLF5DatabaseName {
					return errors.New("runtime ACL source contains an unsupported database condition")
				}
				if _, checked := checkedDatabases[database]; checked {
					continue
				}
				var applies bool
				if err := conn.QueryRow(ctx, `SELECT pg_catalog.current_database()<>$1`, database).Scan(&applies); err != nil || !applies {
					return fmt.Errorf("runtime ACL source database condition is false or unreadable: database-not:%s", database)
				}
				checkedDatabases[database] = struct{}{}
				continue
			}
			role, ok := strings.CutPrefix(condition, "role-exists:")
			if !ok || !aclRoleIdent.MatchString(role) {
				return errors.New("runtime ACL source contains an unsupported unresolved condition")
			}
			if _, checked := checkedRoles[role]; checked {
				continue
			}
			var exists bool
			if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=$1)`, role).Scan(&exists); err != nil || !exists {
				return fmt.Errorf("runtime ACL source condition is false or unreadable: role-exists:%s", role)
			}
			checkedRoles[role] = struct{}{}
		}
	}
	return nil
}

func verifyRuntimeACLObjectOwner(ctx context.Context, conn ScratchPermissionDB, kind, objectID string) error {
	var owner string
	var query string
	switch kind {
	case "TABLE", "SEQUENCE":
		query = `SELECT r.rolname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_roles r ON r.oid=c.relowner WHERE c.oid=$1::oid`
	case "FUNCTION":
		query = `SELECT r.rolname FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::oid`
	case "SCHEMA":
		query = `SELECT r.rolname FROM pg_catalog.pg_namespace n JOIN pg_catalog.pg_roles r ON r.oid=n.nspowner WHERE n.oid=$1::oid`
	default:
		return fmt.Errorf("unsupported ACL owner object kind %q", kind)
	}
	if err := conn.QueryRow(ctx, query, objectID).Scan(&owner); err != nil {
		return errors.New("could not read ACL object owner")
	}
	switch owner {
	case "postgres", "pg_database_owner", "espacigo_schema_owner", "espacigo_document_verification_fn":
		return nil
	default:
		return fmt.Errorf("unexpected ACL object owner %q", owner)
	}
}

func rejectUnmodeledRuntimeACLClasses(ctx context.Context, conn ScratchPermissionDB, preserveF5Baseline bool) error {
	var found bool
	err := conn.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
		          CROSS JOIN LATERAL pg_catalog.aclexplode(t.typacl) a
		          WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND a.grantee IN (0, 'espacigo_runtime'::regrole)
		            AND (NOT $1 OR a.grantee<>0))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		          CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) a
		          WHERE n.nspname IN ('pg_catalog','information_schema') AND a.grantee='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_attribute att
		          JOIN pg_catalog.pg_class c ON c.oid=att.attrelid
		          JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		          CROSS JOIN LATERAL pg_catalog.aclexplode(att.attacl) a
		          WHERE n.nspname IN ('pg_catalog','information_schema')
		            AND att.attnum>0 AND NOT att.attisdropped AND a.grantee='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
		          CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) a
		          WHERE n.nspname IN ('pg_catalog','information_schema') AND a.grantee='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
		          WHERE n.nspname IN ('pg_catalog','information_schema') AND a.grantee='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
		          CROSS JOIN LATERAL pg_catalog.aclexplode(t.typacl) a
		          WHERE n.nspname IN ('pg_catalog','information_schema') AND a.grantee='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_language l CROSS JOIN LATERAL pg_catalog.aclexplode(l.lanacl) a
		          WHERE a.grantee IN (0, 'espacigo_runtime'::regrole) AND (NOT $1 OR a.grantee<>0))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_foreign_data_wrapper f CROSS JOIN LATERAL pg_catalog.aclexplode(f.fdwacl) a
		          WHERE a.grantee IN (0, 'espacigo_runtime'::regrole) AND (NOT $1 OR a.grantee<>0))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_foreign_server s CROSS JOIN LATERAL pg_catalog.aclexplode(s.srvacl) a
		          WHERE a.grantee IN (0, 'espacigo_runtime'::regrole) AND (NOT $1 OR a.grantee<>0))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_tablespace s CROSS JOIN LATERAL pg_catalog.aclexplode(s.spcacl) a
		          WHERE a.grantee IN (0, 'espacigo_runtime'::regrole) AND (NOT $1 OR a.grantee<>0))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_largeobject_metadata l CROSS JOIN LATERAL pg_catalog.aclexplode(l.lomacl) a
		          WHERE a.grantee IN (0, 'espacigo_runtime'::regrole) AND (NOT $1 OR a.grantee<>0))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_parameter_acl p CROSS JOIN LATERAL pg_catalog.aclexplode(p.paracl) a
		          WHERE a.grantee IN (0, 'espacigo_runtime'::regrole) AND (NOT $1 OR a.grantee<>0))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_default_acl d CROSS JOIN LATERAL pg_catalog.aclexplode(d.defaclacl) a
		          WHERE a.grantee IN (0, 'espacigo_runtime'::regrole))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_default_acl d
		          WHERE d.defaclrole='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_default_acl d
		          LEFT JOIN pg_catalog.pg_namespace n ON n.oid=d.defaclnamespace
		          WHERE d.defaclobjtype='f' AND (d.defaclnamespace=0 OR n.nspname NOT IN ('pg_catalog','information_schema')))
		`, preserveF5Baseline).Scan(&found)
	if err != nil {
		return errors.New("could not inspect non-table runtime/PUBLIC ACL catalogs")
	}
	if found {
		return errors.New("runtime/PUBLIC privileges exist on unmodeled database, type, service, large-object, parameter, or default ACLs")
	}
	return nil
}

func verifyRuntimeACLInitialPrivilegeBaselines(ctx context.Context, conn ScratchPermissionDB) error {
	var differs bool
	err := conn.QueryRow(ctx, `WITH extension_objects AS (
		SELECT 'pg_catalog.pg_class'::regclass AS classoid, c.oid AS objoid, 0::smallint AS objsubid, c.relacl AS acl
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname NOT IN ('pg_catalog','information_schema')
		  AND EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_class'::regclass AND ip.objoid=c.oid AND ip.objsubid=0 AND ip.privtype='e')
		UNION ALL
		SELECT 'pg_catalog.pg_class'::regclass, c.oid, a.attnum, a.attacl
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
		WHERE n.nspname NOT IN ('pg_catalog','information_schema')
		  AND EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_class'::regclass AND ip.objoid=c.oid AND ip.objsubid=a.attnum AND ip.privtype='e')
		UNION ALL
		SELECT 'pg_catalog.pg_proc'::regclass, p.oid, 0::smallint, p.proacl
		FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog','information_schema')
		  AND EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_proc'::regclass AND ip.objoid=p.oid AND ip.objsubid=0 AND ip.privtype='e')
		UNION ALL
		SELECT 'pg_catalog.pg_namespace'::regclass, n.oid, 0::smallint, n.nspacl
		FROM pg_catalog.pg_namespace n
		WHERE n.nspname NOT IN ('pg_catalog','information_schema')
		  AND EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_namespace'::regclass AND ip.objoid=n.oid AND ip.objsubid=0 AND ip.privtype='e')
	), current_acl AS (
		SELECT o.classoid,o.objoid,o.objsubid,a.grantee,a.grantor,a.privilege_type,a.is_grantable
		FROM extension_objects o CROSS JOIN LATERAL pg_catalog.aclexplode(o.acl) a
		WHERE a.grantee IN (0,'espacigo_runtime'::regrole)
	), initial_acl AS (
		SELECT ip.classoid,ip.objoid,ip.objsubid,a.grantee,a.grantor,a.privilege_type,a.is_grantable
		FROM pg_catalog.pg_init_privs ip
		JOIN extension_objects o ON o.classoid=ip.classoid AND o.objoid=ip.objoid AND o.objsubid=ip.objsubid
		CROSS JOIN LATERAL pg_catalog.aclexplode(ip.initprivs) a
		WHERE ip.privtype='e'
		  AND ip.classoid IN ('pg_catalog.pg_class'::regclass,'pg_catalog.pg_proc'::regclass,'pg_catalog.pg_namespace'::regclass)
		  AND a.grantee IN (0,'espacigo_runtime'::regrole)
	)
	SELECT EXISTS (SELECT * FROM current_acl EXCEPT SELECT * FROM initial_acl)
	    OR EXISTS (SELECT * FROM initial_acl EXCEPT SELECT * FROM current_acl)`).Scan(&differs)
	if err != nil {
		return errors.New("could not compare recorded initial runtime/PUBLIC ACLs with pg_init_privs")
	}
	if differs {
		return errors.New("runtime/PUBLIC ACLs differ from their recorded initial privileges")
	}
	return nil
}

func verifyRuntimeACLDatabaseCatalog(ctx context.Context, conn ScratchPermissionDB, preserveF5Baseline bool) error {
	if preserveF5Baseline {
		return verifyRuntimeACLDatabaseCatalogF5(ctx, conn)
	}
	var valid bool
	err := conn.QueryRow(ctx, `WITH current_acl AS (
		SELECT a.grantee, a.grantor, a.privilege_type, a.is_grantable
		FROM pg_catalog.pg_database d
		CROSS JOIN LATERAL pg_catalog.aclexplode(
			COALESCE(d.datacl, pg_catalog.acldefault('d', d.datdba))) a
		WHERE d.datname=current_database()
	)
	SELECT d.datdba='postgres'::regrole
	   AND EXISTS (SELECT 1 FROM current_acl a
	               WHERE a.grantee='espacigo_runtime'::regrole
	                 AND a.grantor='postgres'::regrole
	                 AND a.privilege_type='CONNECT' AND NOT a.is_grantable)
	   AND NOT EXISTS (SELECT 1 FROM current_acl a
	                   WHERE a.grantee='espacigo_runtime'::regrole
	                     AND (a.privilege_type<>'CONNECT' OR a.grantor<>'postgres'::regrole OR a.is_grantable))
	   AND EXISTS (SELECT 1 FROM current_acl a
	               WHERE a.grantee=0 AND a.grantor='postgres'::regrole
	                 AND a.privilege_type='CONNECT' AND NOT a.is_grantable)
	   AND NOT EXISTS (SELECT 1 FROM current_acl a
	                   WHERE a.grantee=0
	                     AND (a.privilege_type<>'CONNECT' OR a.grantor<>'postgres'::regrole OR a.is_grantable))
	FROM pg_catalog.pg_database d WHERE d.datname=current_database()`).Scan(&valid)
	if err != nil || !valid {
		return errors.New("direct database ACL grants exceed CONNECT-only runtime/PUBLIC policy")
	}
	return nil
}

func verifyRuntimeACLDatabaseCatalogF5(ctx context.Context, conn ScratchPermissionDB) error {
	var valid bool
	err := conn.QueryRow(ctx, `WITH current_acl AS (
		SELECT a.grantee, a.grantor, a.privilege_type, a.is_grantable
		FROM pg_catalog.pg_database d
		CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba))) a
		WHERE d.datname=pg_catalog.current_database()
	)
	SELECT d.datdba='postgres'::regrole
	   AND NOT EXISTS (SELECT 1 FROM current_acl a WHERE a.grantee='espacigo_runtime'::regrole)
	   AND pg_catalog.has_database_privilege('espacigo_runtime',d.datname,'CONNECT')
	   AND NOT pg_catalog.has_database_privilege('espacigo_runtime',d.datname,'CREATE')
	   AND pg_catalog.has_database_privilege('espacigo_runtime',d.datname,'TEMPORARY') =
	       EXISTS (SELECT 1 FROM current_acl a WHERE a.grantee=0 AND a.privilege_type='TEMPORARY')
	FROM pg_catalog.pg_database d WHERE d.datname=pg_catalog.current_database()`).Scan(&valid)
	if err != nil || !valid {
		return errors.New("F5 runtime database privileges differ from the pinned PUBLIC baseline or include direct grants")
	}
	return nil
}

func verifyRuntimeACLRoleStructure(ctx context.Context, conn ScratchPermissionDB, preserveF5Baseline bool) error {
	var safe bool
	err := conn.QueryRow(ctx, `SELECT
		runtime.rolcanlogin = false AND runtime.rolsuper = false
		AND runtime.rolcreatedb = false AND runtime.rolcreaterole = false
		AND runtime.rolreplication = false AND runtime.rolbypassrls = false AND NOT runtime.rolinherit
		AND api.rolcanlogin = true AND api.rolsuper = false
		AND api.rolcreatedb = false AND api.rolcreaterole = false
		AND api.rolreplication = false AND api.rolbypassrls = false AND NOT api.rolinherit
		AND migrator.rolcanlogin = true AND migrator.rolsuper = false
		AND migrator.rolcreatedb = false AND migrator.rolcreaterole = false
		AND migrator.rolreplication = false AND migrator.rolbypassrls = false AND NOT migrator.rolinherit
		AND owner_role.rolcanlogin = false AND owner_role.rolsuper = false
		AND owner_role.rolcreatedb = false AND owner_role.rolcreaterole = false
		AND owner_role.rolreplication = false AND owner_role.rolbypassrls = false AND NOT owner_role.rolinherit
		AND helper.rolcanlogin = false AND helper.rolsuper = false
		AND helper.rolcreatedb = false AND helper.rolcreaterole = false
		AND helper.rolreplication = false AND helper.rolbypassrls = false AND NOT helper.rolinherit
		AND (SELECT count(*) FROM pg_catalog.pg_auth_members m
		     WHERE m.roleid IN (runtime.oid, owner_role.oid, helper.oid, migrator.oid)
		        OR m.member IN (runtime.oid, owner_role.oid, helper.oid, migrator.oid, api.oid)) = 3
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m
		     WHERE (m.roleid IN (runtime.oid, owner_role.oid, helper.oid, migrator.oid)
		         OR m.member IN (runtime.oid, owner_role.oid, helper.oid, migrator.oid, api.oid))
		       AND (NOT ((m.roleid=runtime.oid AND m.member=api.oid)
		         OR (m.roleid=owner_role.oid AND m.member=migrator.oid)
		         OR (m.roleid=helper.oid AND m.member=owner_role.oid))
		         OR ((m.roleid=runtime.oid OR m.roleid=owner_role.oid OR m.roleid=helper.oid)
		             AND (NOT m.set_option OR m.inherit_option OR m.admin_option))))
		FROM pg_catalog.pg_roles runtime
		JOIN pg_catalog.pg_roles api ON api.rolname='espacigo_api_login'
		JOIN pg_catalog.pg_roles migrator ON migrator.rolname='espacigo_migrator_login'
		JOIN pg_catalog.pg_roles owner_role ON owner_role.rolname='espacigo_schema_owner'
		JOIN pg_catalog.pg_roles helper ON helper.rolname='espacigo_document_verification_fn'
		WHERE runtime.rolname='espacigo_runtime'`).Scan(&safe)
	if err != nil || !safe {
		return errors.New("runtime/API role attributes or exact SET ROLE membership differ from the reviewed contract")
	}
	var runtimeOwnsObjects bool
	err = conn.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n WHERE n.nspowner = 'espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c WHERE c.relowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p WHERE p.proowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_type t WHERE t.typowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_database d WHERE d.datdba='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_collation c WHERE c.collowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_conversion c WHERE c.conowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_event_trigger e WHERE e.evtowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_extension e WHERE e.extowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_foreign_data_wrapper f WHERE f.fdwowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_foreign_server s WHERE s.srvowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_language l WHERE l.lanowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_largeobject_metadata l WHERE l.lomowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_opclass o WHERE o.opcowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_operator o WHERE o.oprowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_opfamily o WHERE o.opfowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_publication p WHERE p.pubowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_statistic_ext s WHERE s.stxowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_subscription s WHERE s.subowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_tablespace t WHERE t.spcowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_ts_config c WHERE c.cfgowner='espacigo_runtime'::regrole)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_ts_dict d WHERE d.dictowner='espacigo_runtime'::regrole)`).Scan(&runtimeOwnsObjects)
	if err != nil || runtimeOwnsObjects {
		return errors.New("runtime role owns a database, schema, or catalog object")
	}
	var databaseACLValid bool
	err = conn.QueryRow(ctx, `SELECT pg_catalog.has_database_privilege('espacigo_runtime', current_database(), 'CONNECT')
		AND NOT pg_catalog.has_database_privilege('espacigo_runtime', current_database(), 'CREATE')
		AND ($1 OR NOT pg_catalog.has_database_privilege('espacigo_runtime', current_database(), 'TEMPORARY'))
		AND pg_catalog.has_schema_privilege('espacigo_runtime', 'public', 'USAGE')
		AND NOT pg_catalog.has_schema_privilege('espacigo_runtime', 'public', 'CREATE')`, preserveF5Baseline).Scan(&databaseACLValid)
	if err != nil || !databaseACLValid {
		return errors.New("runtime database/schema effective privileges differ from the least-privilege contract")
	}
	return nil
}

func resolveRuntimeACLObject(ctx context.Context, conn ScratchPermissionDB, key runtimeACLGrantKey) (string, error) {
	var id uint32
	var err error
	switch key.ObjectKind {
	case "TABLE", "SEQUENCE":
		err = conn.QueryRow(ctx, `SELECT COALESCE(pg_catalog.to_regclass($1)::oid, 0)`, key.Object).Scan(&id)
	case "FUNCTION":
		err = conn.QueryRow(ctx, `SELECT COALESCE(pg_catalog.to_regprocedure($1)::oid, 0)`, key.Object).Scan(&id)
	case "SCHEMA":
		err = conn.QueryRow(ctx, `SELECT oid FROM pg_catalog.pg_namespace WHERE nspname=$1`, strings.TrimPrefix(key.Object, "public.")).Scan(&id)
	default:
		return "", fmt.Errorf("unsupported ACL object kind %q", key.ObjectKind)
	}
	if err != nil || id == 0 {
		return "", errors.New("ACL object does not exist")
	}
	return fmt.Sprint(id), nil
}

func readRuntimeACLDirectGrants(ctx context.Context, conn ScratchPermissionDB) (map[runtimeACLResolvedKey][]runtimeACLObservedGrant, error) {
	rows, err := conn.Query(ctx, `WITH acl_rows AS (
		SELECT 'TABLE'::text AS kind, c.oid::text AS object_id, 'espacigo_runtime'::text AS grantee,
		       a.privilege_type AS privilege, ''::text AS column_name, a.is_grantable AS grantable,
		       false AS default_acl, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_class'::regclass AND ip.objoid=c.oid AND ip.privtype='e') AS extension_owned,
		       grantor.rolname AS grantor, owner.rolname AS owner
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor
		JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND c.relkind <> 'S' AND a.grantee='espacigo_runtime'::regrole
		UNION ALL
		SELECT 'SEQUENCE', c.oid::text, 'espacigo_runtime', a.privilege_type, '', a.is_grantable, false, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_class'::regclass AND ip.objoid=c.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND c.relkind='S' AND a.grantee='espacigo_runtime'::regrole
		UNION ALL
		SELECT 'TABLE', c.oid::text, 'espacigo_runtime', a.privilege_type, att.attname, a.is_grantable, false, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_class'::regclass AND ip.objoid=c.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_catalog.pg_attribute att ON att.attrelid=c.oid AND att.attnum>0 AND NOT att.attisdropped
		CROSS JOIN LATERAL pg_catalog.aclexplode(att.attacl) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND a.grantee='espacigo_runtime'::regrole
		UNION ALL
		SELECT 'FUNCTION', p.oid::text, 'espacigo_runtime', a.privilege_type, '', a.is_grantable, p.proacl IS NULL, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_proc'::regclass AND ip.objoid=p.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
		CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl, pg_catalog.acldefault('f', p.proowner))) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=p.proowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND a.grantee='espacigo_runtime'::regrole
		UNION ALL
		SELECT 'SCHEMA', n.oid::text, 'espacigo_runtime', a.privilege_type, '', a.is_grantable, false, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_namespace'::regclass AND ip.objoid=n.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=n.nspowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND a.grantee='espacigo_runtime'::regrole
		UNION ALL
		SELECT 'TABLE', c.oid::text, 'PUBLIC', a.privilege_type, '', a.is_grantable, false, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_class'::regclass AND ip.objoid=c.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND c.relkind <> 'S' AND a.grantee=0
		UNION ALL
		SELECT 'SEQUENCE', c.oid::text, 'PUBLIC', a.privilege_type, '', a.is_grantable, false, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_class'::regclass AND ip.objoid=c.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND c.relkind='S' AND a.grantee=0
		UNION ALL
		SELECT 'TABLE', c.oid::text, 'PUBLIC', a.privilege_type, att.attname, a.is_grantable, false, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_class'::regclass AND ip.objoid=c.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_catalog.pg_attribute att ON att.attrelid=c.oid AND att.attnum>0 AND NOT att.attisdropped
		CROSS JOIN LATERAL pg_catalog.aclexplode(att.attacl) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND a.grantee=0
		UNION ALL
		SELECT 'FUNCTION', p.oid::text, 'PUBLIC', a.privilege_type, '', a.is_grantable, p.proacl IS NULL, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_proc'::regclass AND ip.objoid=p.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
		CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl, pg_catalog.acldefault('f', p.proowner))) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=p.proowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND a.grantee=0
		UNION ALL
		SELECT 'SCHEMA', n.oid::text, 'PUBLIC', a.privilege_type, '', a.is_grantable, false, EXISTS (SELECT 1 FROM pg_catalog.pg_init_privs ip WHERE ip.classoid='pg_catalog.pg_namespace'::regclass AND ip.objoid=n.oid AND ip.privtype='e'), grantor.rolname, owner.rolname
		FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) a
		JOIN pg_catalog.pg_roles grantor ON grantor.oid=a.grantor JOIN pg_catalog.pg_roles owner ON owner.oid=n.nspowner
		WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND a.grantee=0
	)
	SELECT kind, object_id, grantee, privilege, column_name, grantable, default_acl, extension_owned, grantor, owner FROM acl_rows`)
	if err != nil {
		return nil, errors.New("could not read direct runtime/PUBLIC ACLs from PostgreSQL catalogs")
	}
	defer rows.Close()
	observed := make(map[runtimeACLResolvedKey][]runtimeACLObservedGrant)
	for rows.Next() {
		var kind, objectID, grantee, privilege, column, grantor, owner string
		var grantable, defaultACL, extensionOwned bool
		if err := rows.Scan(&kind, &objectID, &grantee, &privilege, &column, &grantable, &defaultACL, &extensionOwned, &grantor, &owner); err != nil {
			return nil, errors.New("could not decode direct runtime/PUBLIC ACL catalog rows")
		}
		key := runtimeACLResolvedKey{grantee, kind, objectID, privilege, column}
		observed[key] = append(observed[key], runtimeACLObservedGrant{GrantOption: grantable, Default: defaultACL, InitialPrivilegeBaseline: extensionOwned, Grantor: grantor, Owner: owner})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.New("runtime/PUBLIC ACL catalog read failed")
	}
	return observed, nil
}

func runtimeACLDirectDifferenceRepairable(expectedGrant, observedGrant bool) bool {
	return expectedGrant && !observedGrant
}

func runtimeACLDirectDriftExplainsEffectiveMismatch(key runtimeACLResolvedKey, expectedEffective, actualEffective bool, repairable map[runtimeACLResolvedKey]struct{}) bool {
	_, exists := repairable[key]
	return exists && expectedEffective && !actualEffective
}

func verifyRuntimeACLEffectivePrivileges(ctx context.Context, conn ScratchPermissionDB, expected map[runtimeACLResolvedKey]runtimeACLExpectedGrant, observed map[runtimeACLResolvedKey][]runtimeACLObservedGrant, repairableDirectACLDrifts map[runtimeACLResolvedKey]struct{}) error {
	effectiveKeys := make(map[runtimeACLResolvedKey]struct{}, len(expected)+len(observed))
	for key := range expected {
		key.Grantee = "espacigo_runtime"
		effectiveKeys[key] = struct{}{}
	}
	for key := range observed {
		key.Grantee = "espacigo_runtime"
		effectiveKeys[key] = struct{}{}
	}
	for key := range effectiveKeys {
		runtimeGrant := expected[key]
		_, runtimeObserved := observed[key]
		publicKey := runtimeACLResolvedKey{"PUBLIC", key.ObjectKind, key.ObjectID, key.Privilege, key.Column}
		publicGrant, publicTouched := expected[publicKey]
		_, publicObserved := observed[publicKey]
		wantEffective := runtimeACLEffectivePrivilegeExpected(
			runtimeGrant.Granted || runtimeObserved,
			publicGrant.Granted || publicObserved,
		)
		if key.ObjectKind == "FUNCTION" && key.Privilege == "EXECUTE" && !publicTouched && !publicObserved {
			// The built-in default function ACL grants EXECUTE to PUBLIC unless
			// the source contract explicitly revokes it.
			wantEffective = true
		}
		var effective bool
		var query string
		switch key.ObjectKind {
		case "TABLE":
			if key.Column != "" {
				query = `SELECT pg_catalog.has_column_privilege('espacigo_runtime', $1::oid, $2, $3)`
				err := conn.QueryRow(ctx, query, key.ObjectID, key.Column, key.Privilege).Scan(&effective)
				if err != nil {
					return errors.New("could not read effective column privilege from PostgreSQL catalogs")
				}
				if effective != wantEffective && !runtimeACLDirectDriftExplainsEffectiveMismatch(key, wantEffective, effective, repairableDirectACLDrifts) {
					return fmt.Errorf("effective column privilege differs from source contract: object_oid=%s column=%s privilege=%s", key.ObjectID, key.Column, key.Privilege)
				}
				continue
			}
			query = `SELECT pg_catalog.has_table_privilege('espacigo_runtime', $1::oid, $2)`
		case "SEQUENCE":
			query = `SELECT pg_catalog.has_sequence_privilege('espacigo_runtime', $1::oid, $2)`
		case "FUNCTION":
			query = `SELECT pg_catalog.has_function_privilege('espacigo_runtime', $1::oid, $2)`
		case "SCHEMA":
			query = `SELECT pg_catalog.has_schema_privilege('espacigo_runtime', $1::oid, $2)`
		default:
			return fmt.Errorf("effective ACL check does not support object kind %s", key.ObjectKind)
		}
		if err := conn.QueryRow(ctx, query, key.ObjectID, key.Privilege).Scan(&effective); err != nil {
			return errors.New("could not read effective privilege from PostgreSQL catalogs")
		}
		if effective != wantEffective && !runtimeACLDirectDriftExplainsEffectiveMismatch(key, wantEffective, effective, repairableDirectACLDrifts) {
			return fmt.Errorf("effective privilege differs from source contract: kind=%s object_oid=%s privilege=%s", key.ObjectKind, key.ObjectID, key.Privilege)
		}
	}
	var functionsMatchCatalog bool
	err := conn.QueryRow(ctx, `SELECT NOT EXISTS (
		SELECT 1 FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog','information_schema')
		  AND pg_catalog.has_function_privilege('espacigo_runtime', p.oid, 'EXECUTE')
		      <> EXISTS (
		          SELECT 1 FROM pg_catalog.aclexplode(COALESCE(p.proacl, pg_catalog.acldefault('f', p.proowner))) a
		          WHERE a.grantee IN (0, 'espacigo_runtime'::regrole) AND a.privilege_type='EXECUTE'
		      )
	)`).Scan(&functionsMatchCatalog)
	if err != nil || !functionsMatchCatalog {
		return errors.New("effective function privileges differ from direct/default ACL catalogs")
	}
	return nil
}
