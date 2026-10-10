package postgres_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adminlocal"
	adminlocalhttp "github.com/HernanEspinozaDev/espaciGo/internal/adminlocal/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

func TestLocalAdminBlockRevokesSessionsAndAllowsOnlyRestrictedLogin(t *testing.T) {
	h := newAuthHarness(t)
	adminID := h.register(t, "admin-block@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	adminSession := h.login(t, "admin-block@ejemplo.invalid")
	targetID := h.register(t, "blocked-owner@ejemplo.invalid")
	h.verify(t)
	oldSession := h.login(t, "blocked-owner@ejemplo.invalid")
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.token_accion(id,usuario_id,proposito,token_hash,creado_en,expira_en) VALUES(gen_random_uuid(),$1,'recuperar_clave',repeat('a',64),$2::timestamptz,$2::timestamptz+interval '15 minutes')`, targetID, h.now); err != nil {
		t.Fatalf("seed active action token for revocation assertion: %v", err)
	}
	runtimePool := newRuntimePool(t, h)
	service, err := adminlocal.New(runtimePool, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.BlockAccount(h.ctx, adminID, targetID, "riesgo_seguridad", "block-request-1"); err != nil {
		t.Fatalf("block account: %v", err)
	}
	var revokedTokens int
	if err = h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.token_accion WHERE usuario_id=$1 AND invalidado_en IS NOT NULL AND consumido_en IS NULL`, targetID).Scan(&revokedTokens); err != nil || revokedTokens != 1 {
		t.Fatalf("block did not invalidate active action token: count=%d err=%v", revokedTokens, err)
	}
	if _, err = h.service.Authorize(h.ctx, oldSession.Token, "", identity.AutomaticPolling); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatalf("pre-block session remained active: %v", err)
	}
	restricted, err := h.service.Login(h.ctx, identity.LoginInput{Email: "blocked-owner@ejemplo.invalid", Password: "Synthetic#123"})
	if err != nil {
		t.Fatalf("restricted login: %v", err)
	}
	if !restricted.RestrictedMode {
		t.Fatal("login did not identify restricted mode")
	}
	principal, err := h.service.Authorize(h.ctx, restricted.Token, "", identity.AutomaticPolling)
	if err != nil || !principal.RestrictedMode {
		t.Fatalf("restricted session lookup principal=%+v err=%v", principal, err)
	}
	if _, err = h.service.Authorize(h.ctx, restricted.Token, "", identity.UserOperation); !errors.Is(err, identity.ErrRestrictedMode) {
		t.Fatalf("ordinary operation allowed without route allow-list: %v", err)
	}
	allowedCtx := identity.WithRestrictedReservationAccess(h.ctx, targetID)
	if _, err = h.service.Authorize(allowedCtx, restricted.Token, "", identity.UserOperation); err != nil {
		t.Fatalf("existing-reservation route denied after transport allow-list: %v", err)
	}
	period := adminlocal.Period{From: h.now.Add(-time.Hour), Until: h.now.Add(time.Hour), TimeZone: "America/Santiago"}
	report, err := service.ReservationReport(h.ctx, adminID, "report-request-1", period)
	if err != nil {
		t.Fatalf("admin read-only report: %v", err)
	}
	if report.Notice != adminlocal.SafetyNotice || report.Currency != "CLP" {
		t.Fatalf("report omitted local safety contract: %+v", report)
	}
	finance, err := service.FinanceReport(h.ctx, adminID, "finance-request-1", period)
	if err != nil {
		t.Fatalf("admin read-only finance report: %v", err)
	}
	if finance.Notice != adminlocal.SafetyNotice || finance.Currency != "CLP" || len(finance.Confirmed) != 0 || len(finance.Pending) != 0 {
		t.Fatalf("unexpected empty-fixture finance report: %+v", finance)
	}
	adminHandler := adminlocalhttp.NewHandler(h.service, service, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/local/reports/reservations?from="+h.now.Add(-time.Hour).Format(time.RFC3339)+"&until="+h.now.Add(time.Hour).Format(time.RFC3339)+"&timezone=UTC", nil)
	request.Header.Set("Authorization", "Bearer "+string(adminSession.Token))
	response := httptest.NewRecorder()
	adminHandler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("X-Request-ID") == "" || !strings.Contains(response.Body.String(), adminlocal.SafetyNotice) {
		t.Fatalf("admin report HTTP route=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	if _, err = service.ReservationReport(h.ctx, targetID, "denied-report", period); !errors.Is(err, adminlocal.ErrForbidden) {
		t.Fatalf("non-admin report was not denied: %v", err)
	}
	if _, err = service.UnblockAccount(h.ctx, adminID, targetID, "revision_concluida", "unblock-request-1"); err != nil {
		t.Fatalf("unblock account: %v", err)
	}
	if _, err = h.service.Authorize(h.ctx, restricted.Token, "", identity.AutomaticPolling); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatalf("unblock restored a restricted session instead of requiring new login: %v", err)
	}
	normal, err := h.service.Login(h.ctx, identity.LoginInput{Email: "blocked-owner@ejemplo.invalid", Password: "Synthetic#123"})
	if err != nil || normal.RestrictedMode {
		t.Fatalf("fresh post-unblock login mode=%v err=%v", normal.RestrictedMode, err)
	}
	var history, audits int
	if err = h.pool.QueryRow(h.ctx, `SELECT (SELECT count(*) FROM public.bloqueo_cuenta_historial_local WHERE cuenta_id=$1),(SELECT count(*) FROM public.evento_auditoria_local WHERE recurso_tipo='cuenta' AND recurso_id=$1)`, targetID).Scan(&history, &audits); err != nil {
		t.Fatal(err)
	}
	if history != 2 || audits != 2 {
		t.Fatalf("block/unblock history=%d audit=%d, want two each; report audit uses collection resource", history, audits)
	}
	var reportAudits int
	if err = h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.evento_auditoria_local WHERE accion IN ('admin.reports.reservations','admin.reports.finance') AND recurso_tipo LIKE 'coleccion_%'`).Scan(&reportAudits); err != nil {
		t.Fatal(err)
	}
	if reportAudits != 3 {
		t.Fatalf("successful report requests logged %d audit events, want three", reportAudits)
	}
}

func TestAdminBlockSharesAccountLockAndPreventsLaterReservationGate(t *testing.T) {
	h := newAuthHarness(t)
	adminID := h.register(t, "admin-race@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	targetID := h.register(t, "renter-race@ejemplo.invalid")
	h.verify(t)
	service, err := adminlocal.New(h.pool, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	hold := holdAccountRowLock(t, h, targetID)
	done := make(chan error, 1)
	go func() {
		_, e := service.BlockAccount(h.ctx, adminID, targetID, "revision_administrativa", "race-block")
		done <- e
	}()
	waitForAccountLockWait(t, h)
	hold()
	if err = <-done; err != nil {
		t.Fatalf("waiting account block: %v", err)
	}
	// A new reservation/quote transaction locks the same user rows and reads
	// this state before it writes. Once the block commits it must fail closed.
	tx, err := h.pool.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(h.ctx)
	if _, err = tx.Exec(h.ctx, `SELECT id FROM public.usuario WHERE id=$1 FOR UPDATE`, targetID); err != nil {
		t.Fatal(err)
	}
	var blocked bool
	if err = tx.QueryRow(h.ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, targetID).Scan(&blocked); err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("new reservation gate did not observe the committed block")
	}
}
