package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// cleanupNativeApprovalFixture removes only the bounded, caller-authorized
// declaration/attempt fixture. It deliberately does not infer trust from rows.
func cleanupNativeApprovalFixture(ctx context.Context, pool *pgxpool.Pool, declarations []nativeDeclarationIntent, attempt *nativeAttemptIntent) error {
	if pool == nil || len(declarations) < 1 || len(declarations) > 2 {
		return errors.New("native approval cleanup requires a pool and one or two declaration receipts")
	}
	accountID := declarations[0].AccountID
	ids, correlations := make([]string, 0, len(declarations)), make([]string, 0, len(declarations))
	seenID, seenCorrelation := map[string]bool{}, map[string]bool{}
	for _, d := range declarations {
		if !canonicalNativeUUID(d.AccountID) || d.AccountID != accountID || !canonicalNativeUUID(d.DeclarationID) || strings.TrimSpace(d.CorrelationID) != d.CorrelationID || len(d.CorrelationID) == 0 || len(d.CorrelationID) > 120 || seenID[d.DeclarationID] || seenCorrelation[d.CorrelationID] {
			return errors.New("native approval declaration receipts are not a bounded canonical scope")
		}
		seenID[d.DeclarationID], seenCorrelation[d.CorrelationID] = true, true
		ids, correlations = append(ids, d.DeclarationID), append(correlations, d.CorrelationID)
	}
	if attempt != nil {
		d := declarations[0]
		if !canonicalNativeUUID(attempt.AccountID) || attempt.AccountID != accountID || !canonicalNativeUUID(attempt.DeclarationID) || attempt.DeclarationID != d.DeclarationID || !canonicalNativeUUID(attempt.AttemptID) || !canonicalNativeUUID(attempt.CorrelationID) || attempt.CorrelationID == d.CorrelationID || attempt.Provider != "didit" || attempt.Environment != "SANDBOX" || attempt.ApplicationID != "local-kyc-test-app" || attempt.WorkflowID != "local-kyc-test-workflow" || attempt.WorkflowVersion != "1" {
			return errors.New("native approval attempt receipt does not match the trusted declaration")
		}
	}
	var sessionUser, currentUser string
	if err := pool.QueryRow(ctx, `SELECT session_user,current_user`).Scan(&sessionUser, &currentUser); err != nil {
		return err
	}
	if sessionUser != "espacigo_migrator_login" || currentUser != "espacigo_schema_owner" {
		return errors.New("native approval cleanup requires authorized migrator/schema-owner identity")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var database, engine string
	var oid int64
	if err := tx.QueryRow(ctx, `SELECT current_database(),(SELECT oid::bigint FROM pg_database WHERE datname=current_database()),current_setting('server_version_num')`).Scan(&database, &oid, &engine); err != nil {
		return err
	}
	if database != identityDeclarationNativeTarget || oid != identityDeclarationNativeTargetOID || engine != "180007" {
		return errors.New("native approval cleanup target mismatch")
	}
	// Lock every surviving declaration in the supplied receipt scope; absence is
	// permitted after an earlier successful purge, but never discovers by account.
	rows, err := tx.Query(ctx, `SELECT id::text FROM public.identity_declaration_revision WHERE account_id=$1::uuid ORDER BY revision DESC FOR UPDATE`, accountID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if !seenID[id] {
			rows.Close()
			return errors.New("unexpected declaration revision for authorized cleanup account")
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	// Audit rows are scoped by receipt correlations OR exact declaration resource;
	// reject any unrelated/malformed row before deleting anything.
	ar, err := tx.Query(ctx, `SELECT actor_id::text,recurso_tipo,recurso_id::text,accion,correlacion_id FROM public.evento_auditoria_local WHERE correlacion_id=ANY($1::text[]) OR (actor_id=$2::uuid AND recurso_tipo='identity_declaration' AND recurso_id=ANY($3::uuid[]))`, correlations, accountID, ids)
	if err != nil {
		return err
	}
	auditSeen := map[string]bool{}
	for ar.Next() {
		var actor, typ, rid, action, corr string
		if err := ar.Scan(&actor, &typ, &rid, &action, &corr); err != nil {
			ar.Close()
			return err
		}
		d, ok := func() (nativeDeclarationIntent, bool) {
			for _, x := range declarations {
				if x.DeclarationID == rid && x.CorrelationID == corr {
					return x, true
				}
			}
			return nativeDeclarationIntent{}, false
		}()
		if !ok || actor != accountID || typ != "identity_declaration" || action != "kyc.identity_declaration.save" || rid != d.DeclarationID {
			ar.Close()
			return errors.New("declaration audit outside exact receipt scope")
		}
		auditSeen[corr] = true
	}
	if err := ar.Err(); err != nil {
		ar.Close()
		return err
	}
	ar.Close()
	// Validate all attempts for this account against the single trusted attempt;
	// never adopt an attempt merely because it belongs to the fixture account.
	type attemptRow struct {
		id, decl, status, provider, env, app, workflow, workflowVersion, corr, session, keyVersion, algorithm string
		encrypted                                                                                             bool
	}
	var found []attemptRow
	arows, err := tx.Query(ctx, `SELECT id::text,COALESCE(declaration_id::text,''),status,provider,environment,application_id,workflow_id,workflow_version,correlation_id::text,COALESCE(provider_session_id,''),COALESCE(decision_key_version,''),COALESCE(decision_algorithm,''),encrypted_decision IS NOT NULL FROM public.identity_provider_attempt WHERE account_id=$1::uuid ORDER BY id FOR UPDATE`, accountID)
	if err != nil {
		return err
	}
	for arows.Next() {
		var x attemptRow
		if err := arows.Scan(&x.id, &x.decl, &x.status, &x.provider, &x.env, &x.app, &x.workflow, &x.workflowVersion, &x.corr, &x.session, &x.keyVersion, &x.algorithm, &x.encrypted); err != nil {
			arows.Close()
			return err
		}
		if attempt == nil || x.id != attempt.AttemptID {
			arows.Close()
			return errors.New("unexpected provider attempt for authorized cleanup account")
		}
		if x.decl != "" && x.decl != attempt.DeclarationID {
			arows.Close()
			return errors.New("attempt declaration differs from receipt")
		}
		if x.id != attempt.AttemptID || x.provider != attempt.Provider || x.env != attempt.Environment || x.app != attempt.ApplicationID || x.workflow != attempt.WorkflowID || x.workflowVersion != attempt.WorkflowVersion || x.corr != attempt.CorrelationID || (x.decl == "" && !nativeApprovalDetachedStateAllowed(x.status, x.keyVersion, x.algorithm, x.encrypted)) {
			arows.Close()
			return errors.New("attempt row does not match trusted receipt or detached terminal invariant")
		}
		found = append(found, x)
	}
	if err := arows.Err(); err != nil {
		arows.Close()
		return err
	}
	arows.Close()
	if len(found) > 1 {
		return errors.New("multiple rows match trusted attempt")
	}
	if attempt != nil && len(found) == 1 && !canonicalNativeUUID(found[0].id) {
		return errors.New("attempt row ID is not canonical")
	}
	var claimFingerprint string
	claimRows, err := tx.Query(ctx, `SELECT fingerprint,COALESCE(declaration_id::text,''),verified_at IS NOT NULL FROM public.identity_rut_claim WHERE account_id=$1::uuid ORDER BY fingerprint FOR UPDATE`, accountID)
	if err != nil {
		return err
	}
	for claimRows.Next() {
		var fp, decl string
		var verified bool
		if err := claimRows.Scan(&fp, &decl, &verified); err != nil {
			claimRows.Close()
			return err
		}
		if !nativeApprovalClaimScopeAllowed(decl, verified, seenID, len(found) == 1 && found[0].decl == "") {
			claimRows.Close()
			return errors.New("claim cannot be bound to the authorized attempt/declaration")
		}
		if claimFingerprint != "" {
			claimRows.Close()
			return errors.New("multiple identity claims in bounded cleanup scope")
		}
		claimFingerprint = fp
	}
	if err := claimRows.Err(); err != nil {
		claimRows.Close()
		return err
	}
	claimRows.Close()
	var sessionKey string
	if len(found) == 1 && found[0].session != "" {
		if !canonicalNativeUUID(found[0].session) || len("didit_session_deletion:"+found[0].session) > 120 {
			return errors.New("provider session deletion key is invalid")
		}
		sessionKey = "didit_session_deletion:" + found[0].session
	}
	// Inspect both aggregate-id and dedup-key hits; only the exact expected
	// event linked to this exact attempt is eligible for deletion.
	if len(found) == 1 {
		var bad int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM public.outbox_evento_local WHERE agregado_id=$1::uuid OR clave_deduplicacion=$2`, found[0].id, sessionKey).Scan(&bad)
		if err != nil {
			return err
		}
		if bad > 0 {
			var exact int
			err = tx.QueryRow(ctx, `SELECT count(*) FROM public.outbox_evento_local WHERE agregado_tipo='didit_session' AND agregado_id=$1::uuid AND tipo='didit.session_remote_deletion' AND clave_deduplicacion=$2`, found[0].id, sessionKey).Scan(&exact)
			if err != nil {
				return err
			}
			if sessionKey == "" || exact != bad || exact > 1 {
				return errors.New("outbox row outside exact attempt/session deletion scope")
			}
			if _, err = tx.Exec(ctx, `DELETE FROM public.outbox_evento_local WHERE agregado_tipo='didit_session' AND agregado_id=$1::uuid AND tipo='didit.session_remote_deletion' AND clave_deduplicacion=$2`, found[0].id, sessionKey); err != nil {
				return err
			}
		}
	}
	if len(found) == 1 {
		if _, err = tx.Exec(ctx, `DELETE FROM public.identity_provider_event WHERE attempt_id=$1::uuid`, found[0].id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM public.identity_provider_attempt WHERE id=$1::uuid AND account_id=$2::uuid`, found[0].id, accountID); err != nil {
			return err
		}
	}
	for _, d := range declarations {
		if auditSeen[d.CorrelationID] {
			if _, err = tx.Exec(ctx, `DELETE FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration' AND recurso_id=$2::uuid AND accion='kyc.identity_declaration.save' AND correlacion_id=$3`, accountID, d.DeclarationID, d.CorrelationID); err != nil {
				return err
			}
		}
	}
	if claimFingerprint != "" {
		if _, err = tx.Exec(ctx, `DELETE FROM public.identity_rut_claim WHERE fingerprint=$1 AND account_id=$2::uuid`, claimFingerprint, accountID); err != nil {
			return err
		}
	}
	for i := len(ids) - 1; i >= 0; i-- {
		id := ids[i]
		if _, err = tx.Exec(ctx, `DELETE FROM public.identity_declaration_revision WHERE id=$1::uuid AND account_id=$2::uuid`, id, accountID); err != nil {
			return err
		}
	}
	var left int64
	if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.identity_declaration_revision WHERE id=ANY($2::uuid[]) AND account_id=$1::uuid)+(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$1::uuid)+(SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$1::uuid AND recurso_tipo='identity_declaration' AND (correlacion_id=ANY($3::text[]) OR recurso_id=ANY($2::uuid[])))+(SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$1::uuid)+(SELECT count(*) FROM public.identity_provider_event e JOIN public.identity_provider_attempt a ON a.id=e.attempt_id WHERE a.account_id=$1::uuid)+(SELECT count(*) FROM public.outbox_evento_local WHERE agregado_id::text=ANY($4::text[]) OR clave_deduplicacion=$5)`, accountID, ids, correlations, func() []string {
		if attempt != nil {
			return []string{attempt.AttemptID}
		}
		return []string{}
	}(), sessionKey).Scan(&left); err != nil {
		return err
	}
	if left != 0 {
		return fmt.Errorf("native approval cleanup left %d scoped artifact(s)", left)
	}
	return tx.Commit(ctx)
}

func nativeApprovalDetachedStateAllowed(status, key, algorithm string, encrypted bool) bool {
	return (status == "APPROVED" || status == "REJECTED" || status == "EXPIRED") && key == "" && algorithm == "" && !encrypted
}

func nativeApprovalClaimScopeAllowed(declarationID string, verified bool, known map[string]bool, detachedTrusted bool) bool {
	if declarationID != "" {
		return known[declarationID]
	}
	return verified && detachedTrusted
}
