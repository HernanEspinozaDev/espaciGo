package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// runNativeRotationClaimRace holds the real keyset lock until PostgreSQL proves
// both independent API backends are queued behind it, then checks the committed
// winner and rolled-back loser using the separately authorized owner pool.
func runNativeRotationClaimRace(t *testing.T, ctx context.Context, appPool, otherPool, cleanupPool *pgxpool.Pool, tracked []*nativeDeclarationIntentRepository, payload verification.DeclarationPayload, mkService func(string, verification.DeclarationRevisionRepository) *verification.DeclarationService, accountA, accountB string) {
	t.Helper()
	if appPool == nil || otherPool == nil || cleanupPool == nil || len(tracked) != 2 || tracked[0] == nil || tracked[1] == nil || accountA == "" || accountB == "" || accountA == accountB {
		t.Fatal("native rotation race requires two accounts, tracked slots, and three pools")
	}
	// The journals remain separate; each call must also reach its own single-connection pool.
	tracked[1].delegate = identitydeclaration.New(otherPool)
	oldService := mkService("r2-native-old", tracked[0])
	newService := mkService("r2-native-new", tracked[1])
	if oldService == nil || newService == nil || oldService.RutKeyring() == nil || newService.RutKeyring() == nil {
		t.Fatal("native rotation services or keyrings unavailable")
	}
	oldSet, err := oldService.RutKeyring().Fingerprints(payload.RUT)
	if err != nil {
		t.Fatal("cannot derive old-service fingerprint set")
	}
	newSet, err := newService.RutKeyring().Fingerprints(payload.RUT)
	if err != nil || oldSet.Active == newSet.Active {
		t.Fatal("services do not demonstrate distinct active fingerprint versions")
	}

	var apiPIDs [2]int
	for i, pool := range []*pgxpool.Pool{appPool, otherPool} {
		if err := pool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&apiPIDs[i]); err != nil {
			t.Fatal("cannot identify API backend")
		}
	}
	holder, err := cleanupPool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal("cannot begin authorized lock-holder transaction")
	}
	var holderPID int
	if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		_ = holder.Rollback(context.Background())
		t.Fatal("cannot identify lock-holder backend")
	}
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "espacigo:rut-hmac-claim-keyset:v1"); err != nil {
		_ = holder.Rollback(context.Background())
		t.Fatal("cannot acquire actual global claim-keyset advisory lock")
	}

	childCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	results := make(chan nativeRotationRaceResult, 2)
	started := make(chan struct{}, 2)
	start := make(chan struct{})
	var releaseStart sync.Once
	accounts := [2]string{accountA, accountB}
	services := [2]*verification.DeclarationService{oldService, newService}
	for i := range accounts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			started <- struct{}{}
			<-start
			saved, saveErr := services[i].Save(childCtx, accounts[i], payload, true, defaultIdentityDeclarationConsentVersion)
			results <- nativeRotationRaceResult{accountID: accounts[i], saved: saved, err: saveErr, service: services[i]}
		}(i)
	}
	var joinOnce sync.Once
	join := func() {
		joinOnce.Do(func() {
			cancel()
			_ = holder.Rollback(context.Background())
			releaseStart.Do(func() { close(start) })
			wg.Wait()
		})
	}
	defer join()
	// Wait until both Save goroutines are ready, then allow them to contend.
	<-started
	<-started
	releaseStart.Do(func() { close(start) })
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blockedA, blockedB bool
		// cleanupPool has MaxConns=1 and its only connection is held by
		// this transaction. Inspect through that transaction, not the pool.
		if err := holder.QueryRow(ctx, `SELECT
			EXISTS (SELECT 1 FROM pg_catalog.pg_locks l WHERE l.pid=$1 AND l.locktype='advisory' AND NOT l.granted AND $3=ANY(pg_catalog.pg_blocking_pids(l.pid))),
			EXISTS (SELECT 1 FROM pg_catalog.pg_locks l WHERE l.pid=$2 AND l.locktype='advisory' AND NOT l.granted AND $3=ANY(pg_catalog.pg_blocking_pids(l.pid)))`, apiPIDs[0], apiPIDs[1], holderPID).Scan(&blockedA, &blockedB); err != nil {
			t.Fatal("cannot inspect advisory-lock waiters")
		}
		if blockedA && blockedB {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("both API backends were not proven waiting on the held global advisory lock within five seconds")
		}
		select {
		case <-ctx.Done():
			t.Fatal("context ended while proving PostgreSQL waiters")
		case <-time.After(20 * time.Millisecond):
		}
	}

	if err := holder.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal("cannot release global advisory-lock holder")
	}
	wg.Wait()
	close(results)
	var outcomes []nativeRotationRaceResult
	for result := range results {
		outcomes = append(outcomes, result)
	}
	if len(outcomes) != 2 {
		t.Fatal("race did not return exactly two Save outcomes")
	}
	var winner, loser *nativeRotationRaceResult
	for i := range outcomes {
		result := &outcomes[i]
		if result.err == nil {
			if winner != nil {
				t.Fatal("both concurrent Saves succeeded")
			}
			winner = result
		} else if errors.Is(result.err, verification.ErrConflict) {
			if loser != nil {
				t.Fatal("both concurrent Saves conflicted")
			}
			loser = result
		} else {
			t.Fatal("concurrent Save failed with a non-conflict error")
		}
	}
	if winner == nil || loser == nil || winner.accountID == loser.accountID || !canonicalNativeUUID(winner.saved.ID) || !tracked[0].intentWritten || !tracked[1].intentWritten {
		t.Fatal("race did not produce one journaled winner and one journaled loser")
	}
	winnerSet, err := winner.service.RutKeyring().Fingerprints(payload.RUT)
	if err != nil {
		t.Fatal("cannot derive winner fingerprint")
	}
	var revisions, claims, audits, loserRows int
	err = cleanupPool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=ANY($1::uuid[])),
		(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=ANY($1::uuid[])),
		(SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=ANY($1::uuid[]) AND recurso_tipo='identity_declaration' AND accion='kyc.identity_declaration.save'),
		(SELECT (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$2::uuid)+(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$2::uuid)+(SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$2::uuid AND recurso_tipo='identity_declaration' AND accion='kyc.identity_declaration.save'))`, []string{accountA, accountB}, loser.accountID).Scan(&revisions, &claims, &audits, &loserRows)
	if err != nil || revisions != 1 || claims != 1 || audits != 1 || loserRows != 0 {
		t.Fatal("owner readback did not show exactly one atomic declaration, claim, and save audit for the race")
	}
	var storedFingerprint string
	if err := cleanupPool.QueryRow(ctx, `SELECT fingerprint FROM public.identity_rut_claim WHERE account_id=$1::uuid AND declaration_id=$2::uuid`, winner.accountID, winner.saved.ID).Scan(&storedFingerprint); err != nil || storedFingerprint != winnerSet.Active {
		t.Fatal("winning claim fingerprint does not match the winner's active key version")
	}
	t.Log("PASS: both independent API backends observed waiting on the global keyset lock; one atomic Save winner and one conflict across active HMAC versions")
}

type nativeRotationRaceResult struct {
	accountID string
	saved     verification.SavedDeclaration
	err       error
	service   *verification.DeclarationService
}
