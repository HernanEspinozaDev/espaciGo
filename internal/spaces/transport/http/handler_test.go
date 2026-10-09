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
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/occupancy"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

const ownerA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const ownerB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const draftID = "11111111-1111-4111-8111-111111111111"

type fakeAuth struct{}

func (fakeAuth) Authorize(_ context.Context, token identity.Secret, required identity.Role, _ identity.ActivityKind) (identity.Principal, error) {
	if token == "" {
		return identity.Principal{}, identity.ErrUnauthorized
	}
	if token == "other" {
		return identity.Principal{AccountID: ownerB}, nil
	}
	if required == identity.RoleLandlord && token != "landlord" {
		return identity.Principal{}, identity.ErrForbidden
	}
	return identity.Principal{AccountID: ownerA}, nil
}

type memoryRepo struct {
	owner          string
	draft          spaces.Draft
	publicationErr error
}

func (m *memoryRepo) Categories(context.Context) ([]spaces.Category, error) {
	return []spaces.Category{{Code: "oficina", Name: "Oficina"}, {Code: "sala_multiproposito", Name: "Sala o espacio multipropósito"}}, nil
}
func (m *memoryRepo) Profile(_ context.Context, category string, version int) (spaces.Profile, error) {
	profiles := map[string]spaces.Profile{
		"oficina":             {CategoryCode: "oficina", SchemaVersion: 1, Attributes: []spaces.AttributeDefinition{{Code: "puestos_trabajo", Order: 1, Label: "Puestos de trabajo", Type: "integer", FilterCandidate: false, Minimum: floatPtr(1), Maximum: floatPtr(2147483647)}, {Code: "escritorios", Order: 2, Label: "Escritorios", Type: "integer", FilterCandidate: false, Minimum: floatPtr(0), Maximum: floatPtr(2147483647)}, {Code: "wifi", Order: 3, Label: "Wi-Fi", Type: "boolean", FilterCandidate: false}, {Code: "tipo_uso_oficina", Order: 4, Label: "Tipo de uso", Type: "enum", FilterCandidate: false, Options: []string{"privada", "compartida"}}}},
		"sala_multiproposito": {CategoryCode: "sala_multiproposito", SchemaVersion: 1, Attributes: []spaces.AttributeDefinition{{Code: "proyector", Order: 1, Label: "Proyector", Type: "boolean", FilterCandidate: false}}},
	}
	if p, ok := profiles[category]; ok {
		if category == "oficina" && (version == 0 || version == 2) {
			p.SchemaVersion = 2
		}
		if version > 0 && version != p.SchemaVersion && !(category == "oficina" && version == 1) {
			return spaces.Profile{}, spaces.ErrNotFound
		}
		return p, nil
	}
	return spaces.Profile{}, spaces.ErrNotFound
}
func floatPtr(v float64) *float64 { return &v }
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
func (m *memoryRepo) SetPublicationState(_ context.Context, owner, id, state, _ string) (spaces.Draft, error) {
	if m.publicationErr != nil {
		return spaces.Draft{}, m.publicationErr
	}
	if owner != m.owner || id != draftID {
		return spaces.Draft{}, spaces.ErrNotFound
	}
	if state == "activa" && m.draft.State == "borrador" {
		m.draft.State = state
		return m.draft, nil
	}
	if state == "oculta" && m.draft.State == "activa" {
		m.draft.State = state
		return m.draft, nil
	}
	return spaces.Draft{}, spaces.ErrPublicationConflict
}
func (m *memoryRepo) UpdatePublishedOwn(_ context.Context, owner, id string, in spaces.PublishedContentInput) (spaces.Draft, error) {
	if owner != m.owner || id != draftID || (m.draft.State != "activa" && m.draft.State != "oculta") {
		return spaces.Draft{}, spaces.ErrNotFound
	}
	if in.Title != nil {
		m.draft.Title = *in.Title
	}
	if in.Description != nil {
		m.draft.Description = *in.Description
	}
	if in.Capacity != nil {
		m.draft.Capacity = *in.Capacity
	}
	if in.UsageRules != nil {
		m.draft.UsageRules = *in.UsageRules
	}
	if in.BasePriceCLP != nil {
		m.draft.BasePriceCLP = *in.BasePriceCLP
	}
	return m.draft, nil
}
func draftFromInput(in spaces.Input) spaces.Draft {
	name := map[string]string{"oficina": "Oficina", "sala_multiproposito": "Sala o espacio multipropósito"}[in.CategoryCode]
	return spaces.Draft{CategoryCode: in.CategoryCode, CategoryName: name, Title: in.Title, Description: in.Description, AreaM2: in.AreaM2, Capacity: in.Capacity, UsageRules: in.UsageRules, RateUnit: in.RateUnit, BasePriceCLP: in.BasePriceCLP, Address: in.Address, State: "borrador", AttributeSchemaVersion: in.AttributeSchemaVersion, Attributes: in.Attributes}
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

func TestPublicationRouteRequiresLandlordAndReturnsState(t *testing.T) {
	initial := draftFromInput(spaces.Input{CategoryCode: "oficina", Title: "Oficina local", Description: strings.Repeat("Espacio de prueba para publicación local. ", 3), AreaM2: 12, Capacity: 3, UsageRules: "Sin fumar", RateUnit: "hora", BasePriceCLP: 8000, Address: "Dirección privada", AttributeSchemaVersion: 1, Attributes: map[string]any{}})
	initial.ID = draftID
	repo := &memoryRepo{owner: ownerA, draft: initial}
	svc, err := spaces.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, svc, nil)
	denied := invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication", "user", `{"state":"activa"}`)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("non-landlord got %d: %s", denied.Code, denied.Body.String())
	}
	deniedContent := invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication-content", "user", `{"title":"Intento no autorizado"}`)
	if deniedContent.Code != http.StatusForbidden {
		t.Fatalf("non-landlord published-content edit got %d: %s", deniedContent.Code, deniedContent.Body.String())
	}
	response := invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication", "landlord", `{"state":"activa"}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"activa"`) {
		t.Fatalf("publish got %d: %s", response.Code, response.Body.String())
	}
	openapi, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = yaml.Unmarshal(openapi, &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	assertDraftResponseSchema(t, compileOpenAPISchema(t, schemas["SpaceDraft"]), response.Body.Bytes())
	response = invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication-content", "landlord", `{"title":"Título actualizado","base_price_clp":9000}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"title":"Título actualizado"`) || !strings.Contains(response.Body.String(), `"base_price_clp":9000`) || !strings.Contains(response.Body.String(), `"state":"activa"`) {
		t.Fatalf("published content update got %d: %s", response.Code, response.Body.String())
	}
	assertDraftResponseSchema(t, compileOpenAPISchema(t, schemas["SpaceDraft"]), response.Body.Bytes())
	description, capacity, usageRules := strings.Repeat("Descripción pública local actualizada. ", 4), int32(5), "Respetar los horarios y no fumar"
	detailsBody, _ := json.Marshal(spaces.PublishedContentInput{Description: &description, Capacity: &capacity, UsageRules: &usageRules})
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["PublishedSpaceContentInput"]), detailsBody, "PublishedSpaceContentInput")
	response = invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication-content", "landlord", string(detailsBody))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"capacity":5`) || !strings.Contains(response.Body.String(), `"usage_rules":"Respetar los horarios y no fumar"`) {
		t.Fatalf("published details update got %d: %s", response.Code, response.Body.String())
	}
	assertDraftResponseSchema(t, compileOpenAPISchema(t, schemas["SpaceDraft"]), response.Body.Bytes())
	response = invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication-content", "landlord", `{}`)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty published-content update got %d: %s", response.Code, response.Body.String())
	}
	response = invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication", "landlord", `{"state":"oculta"}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"oculta"`) {
		t.Fatalf("hide got %d: %s", response.Code, response.Body.String())
	}
	assertDraftResponseSchema(t, compileOpenAPISchema(t, schemas["SpaceDraft"]), response.Body.Bytes())
	repo.publicationErr = spaces.ErrEligibilityRequired
	response = invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication", "landlord", `{"state":"activa"}`)
	var apiError struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if response.Code != http.StatusConflict || json.Unmarshal(response.Body.Bytes(), &apiError) != nil || apiError.Error.Code != "eligibility_required" || apiError.Error.RequestID == "" || apiError.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("eligibility error contract got %d header=%q body=%s", response.Code, response.Header().Get("X-Request-ID"), response.Body.String())
	}
	repo.publicationErr = spaces.ErrEnabledFixture
	response = invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID+"/publication", "landlord", `{"state":"oculta"}`)
	if response.Code != http.StatusConflict || json.Unmarshal(response.Body.Bytes(), &apiError) != nil || apiError.Error.Code != "fixture_enabled" || apiError.Error.RequestID == "" || apiError.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("enabled fixture conflict contract got %d header=%q body=%s", response.Code, response.Header().Get("X-Request-ID"), response.Body.String())
	}
}

type fakeCalendar struct{}

func (fakeCalendar) SetTimeZone(_ context.Context, owner, spaceID, zone string) error {
	if owner != ownerA || spaceID != draftID {
		return occupancy.ErrNotFound
	}
	if zone == "bad" {
		return occupancy.ErrInvalid
	}
	return nil
}
func (fakeCalendar) TimeZone(_ context.Context, owner, spaceID string) (string, error) {
	if owner != ownerA || spaceID != draftID {
		return "", occupancy.ErrNotFound
	}
	return "America/Santiago", nil
}
func (fakeCalendar) Availability(_ context.Context, owner, spaceID, from, to string) (occupancy.Availability, error) {
	if owner != ownerA || spaceID != draftID {
		return occupancy.Availability{}, occupancy.ErrNotFound
	}
	return occupancy.Availability{SpaceID: spaceID, TimeZone: "America/Santiago", StartAt: mustTime(from), EndAt: mustTime(to), Available: true}, nil
}
func (fakeCalendar) ListBlocks(_ context.Context, owner, spaceID, from, to string) (occupancy.Calendar, error) {
	if owner != ownerA || spaceID != draftID {
		return occupancy.Calendar{}, occupancy.ErrNotFound
	}
	return occupancy.Calendar{SpaceID: spaceID, TimeZone: "America/Santiago", Items: []occupancy.Block{}}, nil
}
func (fakeCalendar) CreateBlock(_ context.Context, owner, spaceID string, in occupancy.BlockInput) (occupancy.Block, error) {
	if owner != ownerA || spaceID != draftID {
		return occupancy.Block{}, occupancy.ErrNotFound
	}
	return occupancy.Block{ID: draftID, SpaceID: spaceID, TimeZone: "America/Santiago", StartAt: mustTime(in.StartAt), EndAt: mustTime(in.EndAt), Reason: in.Reason}, nil
}
func (fakeCalendar) DeleteBlock(_ context.Context, owner, spaceID, blockID string) error {
	if owner != ownerA || spaceID != draftID || blockID != draftID {
		return occupancy.ErrNotFound
	}
	return nil
}
func mustTime(v string) time.Time { t, _ := time.Parse(time.RFC3339, v); return t.UTC() }

func calendarHandler(t *testing.T) http.Handler {
	t.Helper()
	svc, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(fakeAuth{}, svc, nil, fakeCalendar{})
}

type priceRepo struct {
	rate pricing.Rate
	sim  pricing.Simulation
}

func (p *priceRepo) CurrentRate(_ context.Context, owner, space string) (pricing.Rate, error) {
	if owner != ownerA || space != draftID {
		return pricing.Rate{}, pricing.ErrNotFound
	}
	return p.rate, nil
}
func (p *priceRepo) RateHistory(_ context.Context, owner, space string) ([]pricing.Rate, error) {
	r, e := p.CurrentRate(context.Background(), owner, space)
	return []pricing.Rate{r}, e
}
func (p *priceRepo) UpdateRate(_ context.Context, owner, space string, in pricing.RateInput) (pricing.Rate, error) {
	if owner != ownerA || space != draftID {
		return pricing.Rate{}, pricing.ErrNotFound
	}
	p.rate.Version++
	p.rate.Unit = in.Unit
	p.rate.Amount = in.Amount
	return p.rate, nil
}
func (p *priceRepo) CreateSimulation(_ context.Context, owner, space, id, zone string, rate pricing.Rate, start, end time.Time, units, subtotal int64) (pricing.Simulation, error) {
	if owner != ownerA || space != draftID {
		return pricing.Simulation{}, pricing.ErrNotFound
	}
	p.sim = pricing.Simulation{ID: id, SpaceID: space, RateVersion: rate.Version, RateUnit: rate.Unit, BasePrice: rate.Amount, BilledUnits: units, Currency: "CLP", Subtotal: subtotal, StartAt: start, EndAt: end, TimeZone: zone, Private: true, CreatedAt: time.Now().UTC()}
	return p.sim, nil
}
func (p *priceRepo) GetSimulation(_ context.Context, owner, space, id string) (pricing.Simulation, error) {
	if owner != ownerA || space != draftID || id != p.sim.ID {
		return pricing.Simulation{}, pricing.ErrNotFound
	}
	return p.sim, nil
}

func TestPrivatePriceSimulationRoutesAndOpenAPISchemas(t *testing.T) {
	repo := &priceRepo{rate: pricing.Rate{SpaceID: draftID, Version: 1, Unit: "hora", Amount: 8000, Currency: "CLP", CreatedAt: time.Now().UTC()}}
	service, err := pricing.NewService(repo, fakeCalendar{}, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesSvc, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlerWithPricing(fakeAuth{}, spacesSvc, []string{"http://localhost"}, fakeCalendar{}, service)
	base := "/api/v1/spaces/" + draftID
	read := invoke(h, "GET", base+"/tariff", "user", "")
	if read.Code != 200 || !strings.Contains(read.Body.String(), `"version":1`) {
		t.Fatalf("rate response %d %s", read.Code, read.Body.String())
	}
	updated := invoke(h, "PUT", base+"/tariff", "user", `{"rate_unit":"hora","base_price":9000}`)
	if updated.Code != 200 || !strings.Contains(updated.Body.String(), `"version":2`) {
		t.Fatalf("rate update %d %s", updated.Code, updated.Body.String())
	}
	sim := invoke(h, "POST", base+"/price-simulations", "user", `{"start_at":"2030-04-01T12:00:00Z","end_at":"2030-04-01T13:30:00Z"}`)
	if sim.Code != 201 || !strings.Contains(sim.Body.String(), `"billed_units":2`) || !strings.Contains(sim.Body.String(), `"base_subtotal":18000`) || !strings.Contains(sim.Body.String(), `"private":true`) {
		t.Fatalf("simulation %d %s", sim.Code, sim.Body.String())
	}
	if foreign := invoke(h, "GET", base+"/tariff", "other", ""); foreign.Code != 404 {
		t.Fatalf("foreign tariff status %d", foreign.Code)
	}
	openapi, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = yaml.Unmarshal(openapi, &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["Rate"]), read.Body.Bytes(), "Rate")
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["PriceSimulation"]), sim.Body.Bytes(), "PriceSimulation")
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

func TestCalendarAPIConfiguresQueriesCreatesAndDeletesWithOwnerChecks(t *testing.T) {
	h := calendarHandler(t)
	base := "/api/v1/spaces/" + draftID + "/availability"
	if got := invoke(h, "PUT", base, "", `{"time_zone":"America/Santiago"}`).Code; got != 401 {
		t.Fatalf("unauthenticated configuration status=%d", got)
	}
	if got := invoke(h, "PUT", base, "user", `{"time_zone":"America/Santiago","owner_id":"`+ownerB+`"}`).Code; got != 400 {
		t.Fatalf("unknown field status=%d", got)
	}
	if got := invoke(h, "PUT", base, "other", `{"time_zone":"America/Santiago"}`).Code; got != 404 {
		t.Fatalf("foreign timezone configuration status=%d", got)
	}
	configured := invoke(h, "PUT", base, "user", `{"time_zone":"America/Santiago"}`)
	if configured.Code != 200 || !strings.Contains(configured.Body.String(), `"time_zone":"America/Santiago"`) {
		t.Fatalf("configure status=%d %s", configured.Code, configured.Body.String())
	}
	timeZone := invoke(h, "GET", base+"/timezone", "user", "")
	if timeZone.Code != 200 || !strings.Contains(timeZone.Body.String(), `"time_zone":"America/Santiago"`) {
		t.Fatalf("read configured zone status=%d %s", timeZone.Code, timeZone.Body.String())
	}
	query := base + "?from=2030-04-01T12%3A00%3A00Z&to=2030-04-01T13%3A00%3A00Z"
	availability := invoke(h, "GET", query, "user", "")
	if availability.Code != 200 || !strings.Contains(availability.Body.String(), `"available":true`) {
		t.Fatalf("availability status=%d %s", availability.Code, availability.Body.String())
	}
	if got := invoke(h, "GET", base+"?from=2030-04-01T12%3A00%3A00Z", "user", "").Code; got != 400 {
		t.Fatalf("incomplete query status=%d", got)
	}
	if got := invoke(h, "GET", base+"/timezone", "other", "").Code; got != 404 {
		t.Fatalf("foreign timezone query status=%d", got)
	}
	blocks := "/api/v1/spaces/" + draftID + "/availability/blocks"
	created := invoke(h, "POST", blocks, "user", `{"start_at":"2030-04-01T12:00:00Z","end_at":"2030-04-01T13:00:00Z","reason":"Mantención"}`)
	if created.Code != 201 {
		t.Fatalf("create block status=%d %s", created.Code, created.Body.String())
	}
	openapi, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(openapi, &document); err != nil {
		t.Fatal(err)
	}
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["AvailabilityConfig"]), []byte(`{"time_zone":"America/Santiago"}`), "AvailabilityConfig")
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["AvailabilityConfigResponse"]), configured.Body.Bytes(), "AvailabilityConfigResponse")
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["ManualBlock"]), created.Body.Bytes(), "ManualBlock")
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["AvailabilityResult"]), availability.Body.Bytes(), "AvailabilityResult")
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["ManualBlockInput"]), []byte(`{"start_at":"2030-04-01T12:00:00Z","end_at":"2030-04-01T13:00:00Z","reason":"Mantención"}`), "ManualBlockInput")
	listed := invoke(h, "GET", blocks+"?from=2030-04-01T12%3A00%3A00Z&to=2030-04-01T13%3A00%3A00Z", "user", "")
	if listed.Code != 200 {
		t.Fatalf("list blocks status=%d %s", listed.Code, listed.Body.String())
	}
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["ManualBlockList"]), listed.Body.Bytes(), "ManualBlockList")
	if got := invoke(h, "DELETE", blocks+"/"+draftID, "other", "").Code; got != 404 {
		t.Fatalf("foreign block delete status=%d", got)
	}
	if got := invoke(h, "DELETE", blocks+"/"+draftID, "user", "").Code; got != 204 {
		t.Fatalf("delete status=%d", got)
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
	profileSchema := compileOpenAPISchema(t, schemas["SpaceAttributeProfile"])
	inputSchema := compileOpenAPISchema(t, schemas["SpaceDraftInput"])

	h := testHandler(t)
	profileResponse := invoke(h, "GET", "/api/v1/spaces/categories/oficina/attributes", "user", "")
	if profileResponse.Code != http.StatusOK {
		t.Fatalf("profile status=%d %s", profileResponse.Code, profileResponse.Body.String())
	}
	assertJSONSchema(t, profileSchema, profileResponse.Body.Bytes(), "SpaceAttributeProfile")
	validInputBody, err := json.Marshal(spaces.Input{CategoryCode: "oficina", Title: "Oficina", Description: strings.Repeat("Espacio de trabajo disponible. ", 4), AreaM2: 10, Capacity: 2, UsageRules: "No fumar", RateUnit: "dia", BasePriceCLP: 6001, Address: "Calle 1", AttributeSchemaVersion: 1, Attributes: map[string]any{"puestos_trabajo": float64(4), "wifi": false}})
	if err != nil {
		t.Fatal(err)
	}
	assertJSONSchema(t, inputSchema, validInputBody, "SpaceDraftInput")
	created := invoke(h, "POST", "/api/v1/spaces", "user", validJSON())
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d: %s", created.Code, created.Body.String())
	}
	assertDraftResponseSchema(t, schema, created.Body.Bytes())

	input := spaces.Input{CategoryCode: "sala_multiproposito", AttributeSchemaVersion: 1, Title: "Sala", Description: strings.Repeat("Sala multipropósito para actividades privadas. ", 3), AreaM2: 10, Capacity: 3, UsageRules: "Sin fumar", RateUnit: "hora", BasePriceCLP: 6001, Address: "Calle 2"}
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
	assertJSONSchema(t, schema, data, "SpaceDraft")
}

func compileOpenAPISchema(t *testing.T, schemaValue any) *jsonschema.Schema {
	t.Helper()
	schemaBytes, err := json.Marshal(schemaValue)
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	name := "compiled.json"
	if err := compiler.AddResource(name, bytes.NewReader(schemaBytes)); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(name)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func assertJSONSchema(t *testing.T, schema *jsonschema.Schema, data []byte, name string) {
	t.Helper()
	var response any
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(response); err != nil {
		t.Fatalf("JSON does not match OpenAPI %s: %v\n%s", name, err, data)
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

func TestCategoryProfilesAndOptionalTypedAttributes(t *testing.T) {
	h := testHandler(t)
	profile := invoke(h, "GET", "/api/v1/spaces/categories/oficina/attributes", "user", "")
	if profile.Code != 200 || !strings.Contains(profile.Body.String(), `"schema_version":2`) {
		t.Fatalf("profile response %d %s", profile.Code, profile.Body.String())
	}
	v1 := invoke(h, "GET", "/api/v1/spaces/categories/oficina/attributes/1", "user", "")
	if v1.Code != 200 || !strings.Contains(v1.Body.String(), `"schema_version":1`) {
		t.Fatalf("v1 profile response %d %s", v1.Code, v1.Body.String())
	}
	if missing := invoke(h, "GET", "/api/v1/spaces/categories/oficina/attributes/3", "user", ""); missing.Code != 404 {
		t.Fatalf("missing profile version status=%d", missing.Code)
	}
	for _, attrs := range []string{`{"puestos_trabajo":4,"escritorios":0,"wifi":false,"tipo_uso_oficina":"privada"}`, `{}`, `{"wifi":false}`} {
		body := strings.TrimSuffix(validJSON(), "}") + `,"attributes":` + attrs + `}`
		response := invoke(h, "POST", "/api/v1/spaces", "user", body)
		if response.Code != 201 {
			t.Fatalf("valid optional attributes rejected: %d %s", response.Code, response.Body.String())
		}
		if strings.Contains(attrs, `"escritorios":0`) && !strings.Contains(response.Body.String(), `"escritorios":0`) {
			t.Fatalf("zero value was not retained distinctly: %s", response.Body.String())
		}
	}
	for _, attrs := range []string{`{"puestos_trabajo":"4"}`, `{"tipo_uso_oficina":"industrial"}`, `{"campo_desconocido":true}`, `{"puestos_trabajo":0}`, `{"wifi":null}`} {
		body := strings.TrimSuffix(validJSON(), "}") + `,"attributes":` + attrs + `}`
		response := invoke(h, "POST", "/api/v1/spaces", "user", body)
		if response.Code != 422 {
			t.Fatalf("invalid attributes %s status=%d want 422: %s", attrs, response.Code, response.Body.String())
		}
	}
	largeValue, _ := json.Marshal(map[string]any{"extra": strings.Repeat("x", 17*1024)})
	oversized := strings.TrimSuffix(validJSON(), "}") + `,"attributes":` + string(largeValue) + `}`
	if response := invoke(h, "POST", "/api/v1/spaces", "user", oversized); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized attributes status=%d want 422", response.Code)
	}
}

func TestUpdateDraftKeepsItsStoredProfileVersion(t *testing.T) {
	repo := &memoryRepo{}
	svc, err := spaces.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, svc, nil)
	if response := invoke(h, "POST", "/api/v1/spaces", "user", validJSON()); response.Code != 201 {
		t.Fatalf("create status=%d %s", response.Code, response.Body.String())
	}
	// Simulate an existing draft that predates the currently latest profile.
	repo.draft.AttributeSchemaVersion = 1
	updated := invoke(h, "PUT", "/api/v1/spaces/"+draftID, "user", validJSON())
	if updated.Code != 200 || !strings.Contains(updated.Body.String(), `"attribute_schema_version":1`) {
		t.Fatalf("v1 update status=%d %s", updated.Code, updated.Body.String())
	}
	if repo.draft.AttributeSchemaVersion != 1 {
		t.Fatalf("update implicitly converted profile version: %d", repo.draft.AttributeSchemaVersion)
	}
}
