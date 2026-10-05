package spaceshttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
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
	return []spaces.Category{{Code: "oficina", Name: "Oficina"}, {Code: "sala_multiproposito", Name: "Sala o espacio multipropósito"}}, nil
}
func (m *memoryRepo) Create(_ context.Context, owner string, in spaces.Input) (spaces.Draft, error) {
	m.owner = owner
	m.draft = draftFromInput(in)
	m.draft.ID, m.draft.State = draftID, "borrador"
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
	m.draft = draftFromInput(in)
	m.draft.ID, m.draft.State = draftID, "borrador"
	return m.draft, nil
}
func draftFromInput(in spaces.Input) spaces.Draft {
	name := map[string]string{"oficina": "Oficina", "sala_multiproposito": "Sala o espacio multipropósito"}[in.CategoryCode]
	return spaces.Draft{CategoryCode: in.CategoryCode, CategoryName: name, Title: in.Title, Description: in.Description, AreaM2: in.AreaM2, Capacity: in.Capacity, UsageRules: in.UsageRules, RateUnit: in.RateUnit, BasePriceCLP: in.BasePriceCLP, Address: in.Address, State: "borrador"}
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
	if got := invoke(h, "POST", "/api/v1/spaces", "user", `{"owner_id":"`+ownerB+`"}`).Code; got != http.StatusBadRequest {
		t.Fatalf("strict JSON owner field status=%d, want 400", got)
	}
}
func TestDraftValidationIsRejectedBeforeRepository(t *testing.T) {
	h := testHandler(t)
	if got := invoke(h, "POST", "/api/v1/spaces", "user", `{"category_code":"oficina"}`).Code; got != 422 {
		t.Fatalf("validation status=%d", got)
	}
}

func TestDraftResponsesValidateAgainstOpenAPISchemaAndCategoryChanges(t *testing.T) {
	openapi, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(openapi, &doc); err != nil {
		t.Fatal(err)
	}
	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	schemaBytes, err := json.Marshal(schemas["SpaceDraft"])
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("space-draft.json", bytes.NewReader(schemaBytes)); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("space-draft.json")
	if err != nil {
		t.Fatal(err)
	}

	h := testHandler(t)
	created := invoke(h, "POST", "/api/v1/spaces", "user", validJSON())
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d: %s", created.Code, created.Body.String())
	}
	assertDraftResponseSchema(t, schema, created.Body.Bytes())

	input := spaces.Input{CategoryCode: "sala_multiproposito", Title: "Sala", Description: strings.Repeat("Sala multipropósito para actividades privadas. ", 3), AreaM2: 10, Capacity: 3, UsageRules: "Sin fumar", RateUnit: "hora", BasePriceCLP: 6001, Address: "Calle 2"}
	body, _ := json.Marshal(input)
	updated := invoke(h, "PUT", "/api/v1/spaces/"+draftID, "user", string(body))
	if updated.Code != http.StatusOK {
		t.Fatalf("update status=%d: %s", updated.Code, updated.Body.String())
	}
	assertDraftResponseSchema(t, schema, updated.Body.Bytes())
	var draft spaces.Draft
	if err := json.Unmarshal(updated.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if draft.CategoryCode != "sala_multiproposito" || draft.CategoryName != "Sala o espacio multipropósito" {
		t.Fatalf("category response=%q/%q", draft.CategoryCode, draft.CategoryName)
	}
}

func assertDraftResponseSchema(t *testing.T, schema *jsonschema.Schema, data []byte) {
	t.Helper()
	var response any
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(response); err != nil {
		t.Fatalf("response does not match OpenAPI SpaceDraft: %v\n%s", err, data)
	}
}

func TestNumericValuesOutsidePostgresRangesReturn422(t *testing.T) {
	h := testHandler(t)
	valid := validJSON()
	cases := []struct{ name, field, value string }{
		{"area above numeric precision", "area_m2", "100000000"},
		{"area below stored precision", "area_m2", "0.009"},
		{"capacity above PostgreSQL integer", "capacity", "2147483648"},
		{"price above PostgreSQL bigint", "base_price_clp", "9223372036854775808"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var payload map[string]json.RawMessage
			if err := json.Unmarshal([]byte(valid), &payload); err != nil {
				t.Fatal(err)
			}
			payload[tc.field] = json.RawMessage(tc.value)
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			response := invoke(h, "POST", "/api/v1/spaces", "user", string(body))
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d, want 422", response.Code)
			}
			if tc.name == "price above PostgreSQL bigint" {
				var payload struct {
					Error struct {
						Code      string `json:"code"`
						Message   string `json:"message"`
						RequestID string `json:"request_id"`
					} `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if payload.Error.RequestID != response.Header().Get("X-Request-ID") || payload.Error.RequestID == "" || payload.Error.Code != "validation_error" {
					t.Fatalf("OpenAPI error response mismatch: %+v headers=%v", payload, response.Header())
				}
			}
		})
	}
}
