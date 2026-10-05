package verificationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type testIDs struct{ n int }

func (g *testIDs) ID() (string, error) {
	g.n++
	return fmt.Sprintf("11111111-1111-4111-8111-%012x", g.n), nil
}

type testAuth struct{}

func (testAuth) Authorize(_ context.Context, token identity.Secret, required identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if token == "" {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	p := identity.Principal{AccountID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Roles: []identity.Role{"arrendatario"}}
	if token == "other" {
		p.AccountID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	}
	if token == "admin" {
		p.AccountID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		p.Roles = []identity.Role{"administrador"}
	}
	if required != "" && (len(p.Roles) == 0 || p.Roles[0] != required) {
		return identity.Principal{}, identity.ErrForbidden
	}
	return p, nil
}

type repo struct {
	cases map[string]verification.Case
	keys  map[string]string
}

func newRepo() *repo { return &repo{map[string]verification.Case{}, map[string]string{}} }
func (r *repo) Create(_ context.Context, c verification.Case) (verification.Case, error) {
	k := c.OwnerID + c.Idempotency
	if old := r.keys[k]; old != "" {
		return r.cases[old], nil
	}
	r.cases[c.ID] = c
	r.keys[k] = c.ID
	return c, nil
}
func (r *repo) GetOwn(_ context.Context, owner, id string) (verification.Case, error) {
	c, ok := r.cases[id]
	if !ok || c.OwnerID != owner {
		return verification.Case{}, verification.ErrNotFound
	}
	return c, nil
}
func (r *repo) ListOwn(_ context.Context, owner string) ([]verification.Case, error) {
	out := []verification.Case{}
	for _, c := range r.cases {
		if c.OwnerID == owner {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *repo) ListPending(_ context.Context) ([]verification.Case, error) {
	out := []verification.Case{}
	for _, c := range r.cases {
		if c.State == "en_revision" {
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *repo) Review(_ context.Context, id, reviewer string, approved bool, reason string) (verification.Case, error) {
	c, ok := r.cases[id]
	if !ok {
		return verification.Case{}, verification.ErrNotFound
	}
	if c.State != "en_revision" {
		return verification.Case{}, verification.ErrConflict
	}
	c.ReviewerID = reviewer
	c.ResolvedAt = timePtr(time.Unix(3, 0))
	if approved {
		c.State = "aprobada"
	} else {
		c.State = "rechazada"
		c.ReasonCode = reason
	}
	r.cases[id] = c
	return c, nil
}
func (r *repo) Retry(_ context.Context, owner, prior string, c verification.Case) (verification.Case, error) {
	old, e := r.GetOwn(context.Background(), owner, prior)
	if e != nil {
		return verification.Case{}, e
	}
	if old.State != "rechazada" {
		return verification.Case{}, verification.ErrConflict
	}
	c.Type = old.Type
	c.RetryOf = prior
	r.cases[c.ID] = c
	return c, nil
}
func timePtr(t time.Time) *time.Time { return &t }
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	svc, e := verification.NewService(newRepo(), &testIDs{}, verification.LocalFixtureProvider{}, func() time.Time { return time.Unix(1, 0) })
	if e != nil {
		t.Fatal(e)
	}
	return NewHandler(testAuth{}, svc, []string{"http://localhost:8081"})
}

type evidenceRepoFake struct {
	items map[string]verification.Evidence
}

func evidenceKey(caseID, id string) string { return caseID + ":" + id }
func (r *evidenceRepoFake) CreateEvidence(_ context.Context, item verification.Evidence) (verification.Evidence, error) {
	r.items[evidenceKey(item.VerificationID, item.ID)] = item
	return item, nil
}
func (r *evidenceRepoFake) ListOwnEvidence(_ context.Context, owner, caseID string) ([]verification.Evidence, error) {
	return r.list(caseID), nil
}
func (r *evidenceRepoFake) ListReviewEvidence(_ context.Context, caseID string) ([]verification.Evidence, error) {
	return r.list(caseID), nil
}
func (r *evidenceRepoFake) list(caseID string) []verification.Evidence {
	out := []verification.Evidence{}
	for _, item := range r.items {
		if item.VerificationID == caseID {
			out = append(out, item)
		}
	}
	return out
}
func (r *evidenceRepoFake) GetOwnEvidence(_ context.Context, owner, caseID, id string) (verification.Evidence, error) {
	item, ok := r.items[evidenceKey(caseID, id)]
	if !ok || owner != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		return verification.Evidence{}, verification.ErrNotFound
	}
	return item, nil
}
func (r *evidenceRepoFake) GetReviewEvidence(_ context.Context, caseID, id string) (verification.Evidence, error) {
	item, ok := r.items[evidenceKey(caseID, id)]
	if !ok {
		return verification.Evidence{}, verification.ErrNotFound
	}
	return item, nil
}
func (r *evidenceRepoFake) DeleteOwnEvidence(_ context.Context, owner, caseID, id string) error {
	if _, err := r.GetOwnEvidence(context.Background(), owner, caseID, id); err != nil {
		return err
	}
	delete(r.items, evidenceKey(caseID, id))
	return nil
}
func (r *evidenceRepoFake) DeleteReviewEvidence(_ context.Context, caseID, id string) error {
	if _, err := r.GetReviewEvidence(context.Background(), caseID, id); err != nil {
		return err
	}
	delete(r.items, evidenceKey(caseID, id))
	return nil
}

type evidenceStoreFake struct{ blobs map[string][]byte }

func (s *evidenceStoreFake) Put(_ context.Context, id string, data []byte) error {
	s.blobs[id] = data
	return nil
}
func (s *evidenceStoreFake) Get(_ context.Context, id string) ([]byte, error) {
	data, ok := s.blobs[id]
	if !ok {
		return nil, fmt.Errorf("missing blob")
	}
	return data, nil
}
func (s *evidenceStoreFake) Delete(_ context.Context, id string) error {
	delete(s.blobs, id)
	return nil
}

func TestEvidenceAPIIsPrivateOwnerScopedAndExplicitlyCleanable(t *testing.T) {
	caseRepo := newRepo()
	caseID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	caseRepo.cases[caseID] = verification.Case{ID: caseID, OwnerID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Type: "kyc", State: "en_revision"}
	ids := &testIDs{}
	service, err := verification.NewService(caseRepo, ids, verification.LocalFixtureProvider{}, func() time.Time { return time.Unix(7, 0) })
	if err != nil {
		t.Fatal(err)
	}
	metadata := &evidenceRepoFake{items: map[string]verification.Evidence{}}
	storage := &evidenceStoreFake{blobs: map[string][]byte{}}
	evidence, err := verification.NewEvidenceService(caseRepo, metadata, storage, ids, func() time.Time { return time.Unix(8, 0) })
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(testAuth{}, service, []string{"http://localhost:8081"}, evidence)
	if got := call(h, "GET", "/api/v1/verifications/"+caseID+"/evidence", "", "", "").Code; got != 401 {
		t.Fatalf("unauthenticated evidence query status=%d", got)
	}
	if got := call(h, "POST", "/api/v1/verifications/"+caseID+"/evidence", "user", "", `{"fixture_code":"real-file"}`).Code; got != 422 {
		t.Fatalf("arbitrary input status=%d", got)
	}
	created := call(h, "POST", "/api/v1/verifications/"+caseID+"/evidence", "user", "", `{"fixture_code":"synthetic-png-v1"}`)
	if created.Code != 201 {
		t.Fatalf("upload status=%d %s", created.Code, created.Body)
	}
	var item verification.Evidence
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.FixtureCode != verification.SyntheticEvidenceFixture || len(storage.blobs[item.ID]) == 0 {
		t.Fatalf("fixture not persisted: %+v", item)
	}
	if got := call(h, "GET", "/api/v1/verifications/"+caseID+"/evidence/"+item.ID, "other", "", "").Code; got != 404 {
		t.Fatalf("foreign content status=%d", got)
	}
	if got := call(h, "GET", "/api/v1/admin/verifications/"+caseID+"/evidence/"+item.ID, "admin", "", "").Code; got != 200 {
		t.Fatalf("review content status=%d", got)
	}
	if got := call(h, "GET", "/api/v1/admin/verifications/"+caseID+"/evidence", "admin", "", "").Code; got != 200 {
		t.Fatalf("review evidence list status=%d", got)
	}
	if got := call(h, "GET", "/api/v1/admin/verifications/"+caseID+"/evidence/"+item.ID, "user", "", "").Code; got != 403 {
		t.Fatalf("non-review content status=%d", got)
	}
	if got := call(h, "DELETE", "/api/v1/verifications/"+caseID+"/evidence/"+item.ID, "user", "", "").Code; got != 204 {
		t.Fatalf("explicit cleanup status=%d", got)
	}
	if len(storage.blobs) != 0 || len(metadata.items) != 0 {
		t.Fatal("explicit cleanup did not remove blob and metadata")
	}
}
func call(h http.Handler, method, path, token, key, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestKYCAPIAuthOwnershipAndReview(t *testing.T) {
	h := newHandler(t)
	if got := call(h, "POST", "/api/v1/verifications", "", "request-0001", `{"type":"kyc"}`).Code; got != 401 {
		t.Fatalf("unauth status=%d", got)
	}
	unauth := call(h, "GET", "/api/v1/verifications", "", "", "")
	if unauth.Header().Get("WWW-Authenticate") != "Bearer" || unauth.Header().Get("X-Request-ID") == "" {
		t.Fatal("unauthenticated response is missing standard headers")
	}
	created := call(h, "POST", "/api/v1/verifications", "user", "request-0001", `{"type":"kyc"}`)
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var item verification.Case
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.State != "en_revision" || item.Provider != "local-fixture-v1" {
		t.Fatalf("case=%+v", item)
	}
	replay := call(h, "POST", "/api/v1/verifications", "user", "request-0001", `{"type":"kyc"}`)
	var replayed verification.Case
	if replay.Code != 201 || json.Unmarshal(replay.Body.Bytes(), &replayed) != nil || replayed.ID != item.ID {
		t.Fatalf("idempotent replay: %d %s", replay.Code, replay.Body)
	}
	if got := call(h, "POST", "/api/v1/verifications", "user", "request-0001", `{"type":"kyb"}`).Code; got != 409 {
		t.Fatalf("idempotency payload mismatch status=%d", got)
	}
	if got := call(h, "GET", "/api/v1/verifications/"+item.ID, "other", "", "").Code; got != 404 {
		t.Fatalf("owner isolation status=%d", got)
	}
	if got := call(h, "GET", "/api/v1/admin/verifications", "user", "", "").Code; got != 403 {
		t.Fatalf("admin role status=%d", got)
	}
	if got := call(h, "POST", "/api/v1/admin/verifications/"+item.ID+"/review", "admin", "", `{"decision":"rechazada","reason_code":""}`).Code; got != 422 {
		t.Fatalf("missing rejection reason status=%d", got)
	}
	resolved := call(h, "POST", "/api/v1/admin/verifications/"+item.ID+"/review", "admin", "", `{"decision":"rechazada","reason_code":"antecedentes_incompletos"}`)
	if resolved.Code != 200 {
		t.Fatalf("review: %d %s", resolved.Code, resolved.Body)
	}
	retry := call(h, "POST", "/api/v1/verifications/"+item.ID+"/retry", "user", "retry-0001", `{"corrected":true}`)
	if retry.Code != 201 {
		t.Fatalf("retry: %d %s", retry.Code, retry.Body)
	}
}

func TestKYCAPIRejectsUnknownFieldsAndQuery(t *testing.T) {
	h := newHandler(t)
	if got := call(h, "POST", "/api/v1/verifications?owner=other", "user", "request-0001", `{"type":"kyc"}`).Code; got != 400 {
		t.Fatalf("query status=%d", got)
	}
	if got := call(h, "POST", "/api/v1/verifications", "user", "request-0001", `{"type":"kyc","rut":"12.345.678-9"}`).Code; got != 400 {
		t.Fatalf("unknown sensitive field status=%d", got)
	}
	if _, err := verification.NewService(nil, &testIDs{}, verification.LocalFixtureProvider{}, time.Now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("nil repo error=%v", err)
	}
}
