package spaceshttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
)

const ownerA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const ownerB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const draftID = "11111111-1111-4111-8111-111111111111"

type fakeAuth struct{}

func (fakeAuth) Authorize(_ context.Context, token identity.Secret, _ identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if token == "" {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	if token == "other" {
		return identity.Principal{AccountID: ownerB}, nil
	}
	return identity.Principal{AccountID: ownerA}, nil
}

type memoryRepo struct {
	owner string
	draft spaces.Draft
}

func (m *memoryRepo) Categories(context.Context) ([]spaces.Category, error) {
	return []spaces.Category{{Code: "oficina", Name: "Oficina"}}, nil
}
func (m *memoryRepo) Create(_ context.Context, owner string, in spaces.Input) (spaces.Draft, error) {
	m.owner = owner
	m.draft = spaces.Draft{ID: draftID, CategoryCode: in.CategoryCode, Title: in.Title, State: "borrador"}
	return m.draft, nil
}
func (m *memoryRepo) ListOwn(_ context.Context, owner string) ([]spaces.Draft, error) {
	if owner != m.owner {
		return []spaces.Draft{}, nil
	}
	return []spaces.Draft{m.draft}, nil
}
func (m *memoryRepo) GetOwn(_ context.Context, owner, id string) (spaces.Draft, error) {
	if owner != m.owner || id != draftID {
		return spaces.Draft{}, spaces.ErrNotFound
	}
	return m.draft, nil
}
func (m *memoryRepo) UpdateOwn(_ context.Context, owner, id string, in spaces.Input) (spaces.Draft, error) {
	if owner != m.owner || id != draftID {
		return spaces.Draft{}, spaces.ErrNotFound
	}
	m.draft.Title = in.Title
	return m.draft, nil
}
func validJSON() string {
	b, _ := json.Marshal(spaces.Input{CategoryCode: "oficina", Title: "Oficina", Description: strings.Repeat("Espacio de trabajo. ", 7), AreaM2: 10, Capacity: 3, UsageRules: "Sin fumar", RateUnit: "dia", BasePriceCLP: 6001, Address: "Calle 1"})
	return string(b)
}
func testHandler(t *testing.T) http.Handler {
	t.Helper()
	svc, e := spaces.NewService(&memoryRepo{})
	if e != nil {
		t.Fatal(e)
	}
	return NewHandler(fakeAuth{}, svc, []string{"http://localhost"})
}
func invoke(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestDraftRoutesRequireSessionAndReturnNotFoundAcrossOwners(t *testing.T) {
	h := testHandler(t)
	if got := invoke(h, "GET", "/api/v1/spaces", "", "").Code; got != 401 {
		t.Fatalf("unauthenticated status=%d", got)
	}
	created := invoke(h, "POST", "/api/v1/spaces", "user", validJSON())
	if created.Code != 201 {
		t.Fatalf("create status=%d: %s", created.Code, created.Body.String())
	}
	if !strings.Contains(created.Body.String(), `"state":"borrador"`) {
		t.Fatalf("not a draft: %s", created.Body.String())
	}
	if got := invoke(h, "GET", "/api/v1/spaces/"+draftID, "other", "").Code; got != 404 {
		t.Fatalf("cross-owner status=%d", got)
	}
	if got := invoke(h, "PUT", "/api/v1/spaces/"+draftID, "other", validJSON()).Code; got != 404 {
		t.Fatalf("cross-owner edit status=%d", got)
	}
	list := invoke(h, "GET", "/api/v1/spaces", "other", "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"items":[]`) {
		t.Fatalf("other owner list=%d %s", list.Code, list.Body.String())
	}
	if got := invoke(h, "POST", "/api/v1/spaces", "user", `{"owner_id":"`+ownerB+`"}`).Code; got != 422 && got != 400 {
		t.Fatalf("client owner field status=%d", got)
	}
}
func TestDraftValidationIsRejectedBeforeRepository(t *testing.T) {
	h := testHandler(t)
	if got := invoke(h, "POST", "/api/v1/spaces", "user", `{"category_code":"oficina"}`).Code; got != 422 {
		t.Fatalf("validation status=%d", got)
	}
}
