package spaceshttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	m.draft.UpdatedAt = time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
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
func (m *memoryRepo) UpdateOwnIfRevision(_ context.Context, owner, id string, expected time.Time, in spaces.Input) (spaces.Draft, error) {
	if owner != m.owner || id != draftID {
		return spaces.Draft{}, spaces.ErrNotFound
	}
	if !m.draft.UpdatedAt.Equal(expected) {
		return spaces.Draft{}, spaces.ErrPreconditionFailed
	}
	m.draft = draftFromInput(in)
	m.draft.ID, m.draft.State = draftID, "borrador"
	m.draft.UpdatedAt = expected.Add(time.Second)
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
func (m *memoryRepo) DeleteOwn(_ context.Context, owner, id, correlationID string) (spaces.Deletion, error) {
	if m.publicationErr != nil {
		return spaces.Deletion{}, m.publicationErr
	}
	if owner != m.owner || id != draftID {
		return spaces.Deletion{}, spaces.ErrNotFound
	}
	if m.draft.State != "activa" && m.draft.State != "oculta" {
		return spaces.Deletion{}, spaces.ErrPublicationConflict
	}
	m.draft.State = "eliminada"
	return spaces.Deletion{SpaceID: id, State: "eliminada", Removed: []string{"catalogo"}, Preserved: []string{"historicos"}}, nil
}
func draftFromInput(in spaces.Input) spaces.Draft {
	name := map[string]string{"oficina": "Oficina", "sala_multiproposito": "Sala o espacio multipropósito"}[in.CategoryCode]
	return spaces.Draft{CategoryCode: in.CategoryCode, CategoryName: name, Title: in.Title, Description: in.Description, AreaM2: in.AreaM2, Capacity: in.Capacity, UsageRules: in.UsageRules, RateUnit: in.RateUnit, BasePriceCLP: in.BasePriceCLP, Address: in.Address, State: "borrador", AttributeSchemaVersion: in.AttributeSchemaVersion, Attributes: in.Attributes}
}
func validJSON() string {
	b, _ := json.Marshal(spaces.Input{CategoryCode: "oficina", Title: "Oficina", Description: strings.Repeat("Espacio de trabajo. ", 7), AreaM2: 10, Capacity: 3, UsageRules: "Sin fumar", RateUnit: "dia", BasePriceCLP: 6001, Address: "Calle 1"})
	return string(b)
}
func versionedDraftJSON(title string) string {
	b, _ := json.Marshal(spaces.Input{CategoryCode: "oficina", Title: title, Description: strings.Repeat("Espacio de trabajo. ", 7), AreaM2: 10, Capacity: 3, UsageRules: "Sin fumar", RateUnit: "dia", BasePriceCLP: 6001, Address: "Calle 1", AttributeSchemaVersion: 1, Attributes: map[string]any{}})
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
	initial.UpdatedAt = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
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
	response := invokeIfMatch(h, "/api/v1/spaces/"+draftID+"/publication", "landlord", `{"state":"activa"}`, draftETag(repo.draft))
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
	response = invokeIfMatch(h, "/api/v1/spaces/"+draftID+"/publication-content", "landlord", `{"title":"Título actualizado","base_price_clp":9000}`, draftETag(repo.draft))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"title":"Título actualizado"`) || !strings.Contains(response.Body.String(), `"base_price_clp":9000`) || !strings.Contains(response.Body.String(), `"state":"activa"`) {
		t.Fatalf("published content update got %d: %s", response.Code, response.Body.String())
	}
	assertDraftResponseSchema(t, compileOpenAPISchema(t, schemas["SpaceDraft"]), response.Body.Bytes())
	description, capacity, usageRules := strings.Repeat("Descripción pública local actualizada. ", 4), int32(5), "Respetar los horarios y no fumar"
	detailsBody, _ := json.Marshal(spaces.PublishedContentInput{Description: &description, Capacity: &capacity, UsageRules: &usageRules})
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["PublishedSpaceContentInput"]), detailsBody, "PublishedSpaceContentInput")
	response = invokeIfMatch(h, "/api/v1/spaces/"+draftID+"/publication-content", "landlord", string(detailsBody), draftETag(repo.draft))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"capacity":5`) || !strings.Contains(response.Body.String(), `"usage_rules":"Respetar los horarios y no fumar"`) {
		t.Fatalf("published details update got %d: %s", response.Code, response.Body.String())
	}
	assertDraftResponseSchema(t, compileOpenAPISchema(t, schemas["SpaceDraft"]), response.Body.Bytes())
	response = invokeIfMatch(h, "/api/v1/spaces/"+draftID+"/publication-content", "landlord", `{}`, draftETag(repo.draft))
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty published-content update got %d: %s", response.Code, response.Body.String())
	}
	response = invokeIfMatch(h, "/api/v1/spaces/"+draftID+"/publication", "landlord", `{"state":"oculta"}`, draftETag(repo.draft))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"oculta"`) {
		t.Fatalf("hide got %d: %s", response.Code, response.Body.String())
	}
	assertDraftResponseSchema(t, compileOpenAPISchema(t, schemas["SpaceDraft"]), response.Body.Bytes())
	var apiError struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	repo.publicationErr = spaces.ErrEnabledFixture
	response = invokeIfMatch(h, "/api/v1/spaces/"+draftID+"/publication", "landlord", `{"state":"oculta"}`, draftETag(repo.draft))
	if response.Code != http.StatusConflict || json.Unmarshal(response.Body.Bytes(), &apiError) != nil || apiError.Error.Code != "fixture_enabled" || apiError.Error.RequestID == "" || apiError.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("enabled fixture conflict contract got %d header=%q body=%s", response.Code, response.Header().Get("X-Request-ID"), response.Body.String())
	}
	repo.publicationErr = spaces.ErrTimeZoneRequired
	response = invokeIfMatch(h, "/api/v1/spaces/"+draftID+"/publication", "landlord", `{"state":"activa"}`, draftETag(repo.draft))
	if response.Code != http.StatusConflict || json.Unmarshal(response.Body.Bytes(), &apiError) != nil || apiError.Error.Code != "time_zone_required" || !strings.Contains(response.Body.String(), "IANA") || apiError.Error.RequestID == "" || apiError.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("missing time zone error contract got %d header=%q body=%s", response.Code, response.Header().Get("X-Request-ID"), response.Body.String())
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
func (fakeCalendar) SetTimeZoneCAS(_ context.Context, owner, spaceID, zone string, expected time.Time) (time.Time, error) {
	if err := (fakeCalendar{}).SetTimeZone(context.Background(), owner, spaceID, zone); err != nil {
		return time.Time{}, err
	}
	return expected.Add(time.Second), nil
}

type statefulTimezoneCalendar struct {
	fakeCalendar
	revision time.Time
	zone     string
	calls    int
	stale    bool
}

func (c *statefulTimezoneCalendar) SetTimeZoneCAS(_ context.Context, owner, spaceID, zone string, expected time.Time) (time.Time, error) {
	c.calls++
	if owner != ownerA || spaceID != draftID {
		return time.Time{}, occupancy.ErrNotFound
	}
	if c.stale || !c.revision.Equal(expected) {
		return time.Time{}, occupancy.ErrPreconditionFailed
	}
	if zone == "bad" {
		return time.Time{}, occupancy.ErrInvalid
	}
	if c.zone != zone {
		c.zone = zone
		c.revision = c.revision.Add(time.Second)
	}
	return c.revision, nil
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
	revision         time.Time
	rate             pricing.Rate
	sim              pricing.Simulation
	calendarConflict bool
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
	if p.revision.IsZero() {
		p.revision = p.rate.CreatedAt
	}
	if p.calendarConflict && in.Unit != "hora" {
		return pricing.Rate{}, pricing.ErrCalendarConflict
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
	initialRevision := time.Now().UTC()
	repo := &priceRepo{revision: initialRevision, rate: pricing.Rate{SpaceID: draftID, Version: 1, Unit: "hora", Amount: 8000, Currency: "CLP", CreatedAt: initialRevision}}
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
	read := invoke(h, "GET", base+"/tariff", "landlord", "")
	if read.Code != 200 || !strings.Contains(read.Body.String(), `"version":1`) {
		t.Fatalf("rate response %d %s", read.Code, read.Body.String())
	}
	// Tariff ETag is intentionally not implemented in this handler yet; supply
	// the revision-aware strong tag directly from the fake's known revision.
	tag := draftETag(spaces.Draft{ID: draftID, UpdatedAt: repo.revision})
	repo.calendarConflict = true
	conflict := invokeIfMatch(h, base+"/tariff", "landlord", `{"rate_unit":"dia","base_price":9000}`, tag)
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), `"code":"tariff_calendar_conflict"`) {
		t.Fatalf("calendar-incompatible tariff response %d %s", conflict.Code, conflict.Body.String())
	}
	repo.calendarConflict = false
	updated := invokeIfMatch(h, base+"/tariff", "landlord", `{"rate_unit":"hora","base_price":9000}`, tag)
	if updated.Code != 200 || !strings.Contains(updated.Body.String(), `"version":2`) {
		t.Fatalf("rate update %d %s", updated.Code, updated.Body.String())
	}
	sim := invoke(h, "POST", base+"/price-simulations", "landlord", `{"start_at":"2030-04-01T12:00:00Z","end_at":"2030-04-01T13:30:00Z"}`)
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
func invokeIfMatch(h http.Handler, path, token, body, tag string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("If-Match", tag)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestDraftRoutesRequireSessionAndReturnNotFoundAcrossOwners(t *testing.T) {
	repository := &memoryRepo{}
	service, err := spaces.NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, service, []string{"http://localhost"})
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
	if got, want := created.Header().Get("ETag"), draftETag(repository.draft); got != want || got == "" {
		t.Fatalf("create ETag=%q, want %q", got, want)
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
func TestDraftPutMissingIfMatchPrecedesMalformedJSON(t *testing.T) {
	repo := &memoryRepo{owner: ownerA, draft: draftFromInput(spaces.Input{CategoryCode: "oficina", Title: "Original"})}
	repo.draft.ID, repo.draft.UpdatedAt = draftID, time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	svc, err := spaces.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, svc, nil)
	response := invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID, "user", `{malformed`)
	if response.Code != http.StatusPreconditionRequired || !strings.Contains(response.Body.String(), `"code":"precondition_required"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repo.draft.Title != "Original" {
		t.Fatalf("mutated title: %q", repo.draft.Title)
	}
}

func TestDraftPutRequiresIfMatchAndDoesNotMutate(t *testing.T) {
	repo := &memoryRepo{owner: ownerA, draft: draftFromInput(spaces.Input{CategoryCode: "oficina", Title: "Original"})}
	repo.draft.ID, repo.draft.UpdatedAt = draftID, time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	svc, err := spaces.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, svc, nil)
	response := invoke(h, http.MethodPut, "/api/v1/spaces/"+draftID, "user", versionedDraftJSON("Changed"))
	if response.Code != http.StatusPreconditionRequired {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if repo.draft.Title != "Original" {
		t.Fatalf("mutated title: %q", repo.draft.Title)
	}
}

func TestDraftGetETagAndVersionedPut(t *testing.T) {
	repo := &memoryRepo{owner: ownerA, draft: draftFromInput(spaces.Input{CategoryCode: "oficina", Title: "Original"})}
	repo.draft.ID, repo.draft.UpdatedAt = draftID, time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	svc, err := spaces.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, svc, nil)
	get := invoke(h, http.MethodGet, "/api/v1/spaces/"+draftID, "user", "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET status=%d", get.Code)
	}
	tag := get.Header().Get("ETag")
	if tag == "" {
		t.Fatal("GET missing ETag")
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+draftID, strings.NewReader(versionedDraftJSON("Changed")))
	request.Header.Set("Authorization", "Bearer user")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("If-Match", tag)
	put := httptest.NewRecorder()
	h.ServeHTTP(put, request)
	if put.Code != http.StatusOK || !strings.Contains(put.Body.String(), `"title":"Changed"`) {
		t.Fatalf("PUT status=%d body=%s", put.Code, put.Body.String())
	}
	newTag := put.Header().Get("ETag")
	if newTag == "" || newTag == tag {
		t.Fatalf("ETag did not advance: old=%q new=%q", tag, newTag)
	}
	if _, err := parseIfMatch([]string{newTag}, draftID); err != nil {
		t.Fatalf("invalid returned ETag %q: %v", newTag, err)
	}
}

func TestDraftPutRejectsBadPreconditionsWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name, tag string
		status    int
	}{
		{"stale", draftETag(spaces.Draft{ID: draftID, UpdatedAt: time.Date(2025, 1, 2, 3, 4, 4, 0, time.UTC)}), http.StatusPreconditionFailed},
		{"malformed", `not-an-etag`, http.StatusBadRequest},
		{"other resource", draftETag(spaces.Draft{ID: "22222222-2222-4222-8222-222222222222", UpdatedAt: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)}), http.StatusPreconditionFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memoryRepo{owner: ownerA, draft: draftFromInput(spaces.Input{CategoryCode: "oficina", Title: "Original"})}
			repo.draft.ID, repo.draft.UpdatedAt = draftID, time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
			svc, _ := spaces.NewService(repo)
			h := NewHandler(fakeAuth{}, svc, nil)
			r := httptest.NewRequest(http.MethodPut, "/api/v1/spaces/"+draftID, strings.NewReader(versionedDraftJSON("Changed")))
			r.Header.Set("Authorization", "Bearer user")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("If-Match", tc.tag)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if repo.draft.Title != "Original" {
				t.Fatalf("mutated title: %q", repo.draft.Title)
			}
		})
	}
}

func TestDraftValidationIsRejectedBeforeRepository(t *testing.T) {
	h := testHandler(t)
	if got := invoke(h, "POST", "/api/v1/spaces", "user", `{"category_code":"oficina"}`).Code; got != 422 {
		t.Fatalf("validation status=%d", got)
	}
}

func TestDraft422ReturnsSafeFieldCodesWithoutSubmittedValues(t *testing.T) {
	h := testHandler(t)
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(validJSON()), &payload); err != nil {
		t.Fatal(err)
	}
	privateValue := "contenido privado que no debe volver en la respuesta"
	payload["description"], _ = json.Marshal(privateValue)
	payload["area_m2"] = json.RawMessage("12.345")
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	response := invoke(h, http.MethodPost, "/api/v1/spaces", "user", string(body))
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		Error struct {
			Code    string              `json:"code"`
			Details []spaces.FieldError `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "validation_error" {
		t.Fatalf("error code=%q", envelope.Error.Code)
	}
	want := map[string]string{"description": "min_length", "area_m2": "step"}
	for _, field := range envelope.Error.Details {
		if want[field.Field] != field.Code {
			t.Errorf("unexpected field error %+v", field)
		}
		delete(want, field.Field)
	}
	if len(want) != 0 {
		t.Errorf("missing field errors: %v; response=%s", want, response.Body.String())
	}
	if strings.Contains(response.Body.String(), privateValue) || strings.Contains(response.Body.String(), "no-reflect") || strings.Contains(response.Body.String(), "client_secret") {
		t.Fatal("field validation response reflected submitted content")
	}
	if err := json.Unmarshal([]byte(validJSON()), &payload); err != nil {
		t.Fatal(err)
	}
	payload["attributes"] = json.RawMessage(`{"client_secret":"no-reflect"}`)
	body, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	response = invoke(h, http.MethodPost, "/api/v1/spaces", "user", string(body))
	if response.Code != http.StatusUnprocessableEntity || strings.Contains(response.Body.String(), "no-reflect") || strings.Contains(response.Body.String(), "client_secret") || !strings.Contains(response.Body.String(), `"field":"attributes","code":"unknown_field"`) {
		t.Fatalf("unknown profile attribute response=%d %s", response.Code, response.Body.String())
	}
}

func TestTimezoneCASPreconditionsAndRevision(t *testing.T) {
	initial := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	base := "/api/v1/spaces/" + draftID + "/availability"
	newHandler := func(c *statefulTimezoneCalendar) http.Handler {
		repo := &memoryRepo{owner: ownerA, draft: spaces.Draft{ID: draftID, UpdatedAt: initial}}
		svc, err := spaces.NewService(repo)
		if err != nil {
			t.Fatal(err)
		}
		return NewHandler(fakeAuth{}, svc, nil, c)
	}
	t.Run("missing precondition precedes malformed body", func(t *testing.T) {
		c := &statefulTimezoneCalendar{revision: initial}
		r := invoke(newHandler(c), "PUT", base, "user", "not-json")
		if r.Code != http.StatusPreconditionRequired || c.calls != 0 {
			t.Fatalf("status=%d calls=%d body=%s", r.Code, c.calls, r.Body.String())
		}
	})
	t.Run("unexpected query is rejected before timezone CAS", func(t *testing.T) {
		c := &statefulTimezoneCalendar{revision: initial}
		tag := DraftETag(spaces.Draft{ID: draftID, UpdatedAt: initial})
		r := invokeIfMatch(newHandler(c), base+"?page=1", "user", `{"time_zone":"Europe/Paris"}`, tag)
		if r.Code != http.StatusBadRequest || !strings.Contains(r.Body.String(), `"code":"invalid_request"`) || c.calls != 0 || !c.revision.Equal(initial) || c.zone != "" {
			t.Fatalf("status=%d calls=%d revision=%s zone=%q body=%s", r.Code, c.calls, c.revision, c.zone, r.Body.String())
		}
	})
	for _, tc := range []struct {
		name, tag string
		status    int
	}{
		{"malformed", "not-an-etag", http.StatusBadRequest},
		{"wrong space", DraftETag(spaces.Draft{ID: "22222222-2222-4222-8222-222222222222", UpdatedAt: initial}), http.StatusPreconditionFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &statefulTimezoneCalendar{revision: initial}
			r := invokeIfMatch(newHandler(c), base, "user", `{"time_zone":"Europe/Paris"}`, tc.tag)
			if r.Code != tc.status || c.calls != 0 || !c.revision.Equal(initial) || c.zone != "" {
				t.Fatalf("status=%d calls=%d revision=%s zone=%q body=%s", r.Code, c.calls, c.revision, c.zone, r.Body.String())
			}
		})
	}
	t.Run("repository stale CAS", func(t *testing.T) {
		c := &statefulTimezoneCalendar{revision: initial}
		r := invokeIfMatch(newHandler(c), base, "user", `{"time_zone":"Europe/Paris"}`, DraftETag(spaces.Draft{ID: draftID, UpdatedAt: initial.Add(-time.Second)}))
		if r.Code != http.StatusPreconditionFailed || c.calls != 1 || !c.revision.Equal(initial) || c.zone != "" {
			t.Fatalf("status=%d calls=%d body=%s", r.Code, c.calls, r.Body.String())
		}
	})
	t.Run("success advances ETag and no-op preserves it", func(t *testing.T) {
		c := &statefulTimezoneCalendar{revision: initial, zone: "America/Santiago"}
		h := newHandler(c)
		old := DraftETag(spaces.Draft{ID: draftID, UpdatedAt: initial})
		r := invokeIfMatch(h, base, "user", `{"time_zone":"Europe/Paris"}`, old)
		want := DraftETag(spaces.Draft{ID: draftID, UpdatedAt: initial.Add(time.Second)})
		if r.Code != 200 || r.Header().Get("ETag") != want || c.zone != "Europe/Paris" {
			t.Fatalf("status=%d ETag=%q want=%q body=%s", r.Code, r.Header().Get("ETag"), want, r.Body.String())
		}
		r = invokeIfMatch(h, base, "user", `{"time_zone":"Europe/Paris"}`, want)
		if r.Code != 200 || r.Header().Get("ETag") != want {
			t.Fatalf("no-op status=%d ETag=%q want=%q", r.Code, r.Header().Get("ETag"), want)
		}
	})
	t.Run("foreign owner remains hidden", func(t *testing.T) {
		c := &statefulTimezoneCalendar{revision: initial}
		r := invokeIfMatch(newHandler(c), base, "other", `{"time_zone":"Europe/Paris"}`, DraftETag(spaces.Draft{ID: draftID, UpdatedAt: initial}))
		if r.Code != 404 || c.calls != 0 {
			t.Fatalf("status=%d calls=%d body=%s", r.Code, c.calls, r.Body.String())
		}
	})
}

func TestCalendarAPIConfiguresQueriesCreatesAndDeletesWithOwnerChecks(t *testing.T) {
	initialRevision := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	repo := &memoryRepo{owner: ownerA, draft: spaces.Draft{ID: draftID, UpdatedAt: initialRevision}}
	service, err := spaces.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, service, nil, fakeCalendar{})
	base := "/api/v1/spaces/" + draftID + "/availability"
	tag := DraftETag(spaces.Draft{ID: draftID, UpdatedAt: initialRevision})
	if got := invoke(h, "PUT", base, "", `{"time_zone":"America/Santiago"}`).Code; got != 401 {
		t.Fatalf("unauthenticated configuration status=%d", got)
	}
	missing := invoke(h, "PUT", base, "user", "not-json")
	if missing.Code != http.StatusPreconditionRequired || !strings.Contains(missing.Body.String(), `"code":"precondition_required"`) {
		t.Fatalf("missing precondition status=%d body=%s", missing.Code, missing.Body.String())
	}
	if got := invokeIfMatch(h, base, "user", `{"time_zone":"America/Santiago","owner_id":"`+ownerB+`"}`, tag).Code; got != 400 {
		t.Fatalf("unknown field status=%d", got)
	}
	if got := invoke(h, "PUT", base, "other", `{"time_zone":"America/Santiago"}`).Code; got != 404 {
		t.Fatalf("foreign timezone configuration status=%d", got)
	}
	configured := invokeIfMatch(h, base, "user", `{"time_zone":"America/Santiago"}`, tag)
	if configured.Code != 200 || !strings.Contains(configured.Body.String(), `"time_zone":"America/Santiago"`) {
		t.Fatalf("configure status=%d %s", configured.Code, configured.Body.String())
	}
	if got, want := configured.Header().Get("ETag"), DraftETag(spaces.Draft{ID: draftID, UpdatedAt: initialRevision.Add(time.Second)}); got != want {
		t.Fatalf("timezone response ETag=%q, want %q", got, want)
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
	for _, tc := range []struct {
		name string
		path string
	}{
		{"missing from", blocks + "?to=2030-04-01T13%3A00%3A00Z"},
		{"missing to", blocks + "?from=2030-04-01T12%3A00%3A00Z"},
		{"empty to", blocks + "?from=2030-04-01T12%3A00%3A00Z&to="},
		{"duplicate from", blocks + "?from=2030-04-01T12%3A00%3A00Z&from=2030-04-01T12%3A00%3A00Z&to=2030-04-01T13%3A00%3A00Z"},
		{"unexpected query parameter", blocks + "?from=2030-04-01T12%3A00%3A00Z&to=2030-04-01T13%3A00%3A00Z&page=2"},
	} {
		t.Run("list blocks rejects "+tc.name, func(t *testing.T) {
			invalid := invoke(h, http.MethodGet, tc.path, "user", "")
			if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), `"code":"invalid_request"`) {
				t.Fatalf("list blocks query %q status=%d body=%s", tc.name, invalid.Code, invalid.Body.String())
			}
		})
	}
	assertJSONSchema(t, compileOpenAPISchema(t, schemas["ManualBlockList"]), listed.Body.Bytes(), "ManualBlockList")
	if got := invoke(h, "DELETE", blocks+"/"+draftID, "other", "").Code; got != 404 {
		t.Fatalf("foreign block delete status=%d", got)
	}
	if got := invoke(h, "DELETE", blocks+"/"+draftID, "user", "").Code; got != 204 {
		t.Fatalf("delete status=%d", got)
	}
}

type countingOccupancyRepository struct {
	occupancy.Repository
	availabilityCalls   int
	listBlocksCalls     int
	createBlockCalls    int
	setTimeZoneCalls    int
	setTimeZoneCASCalls int
	timeZoneCalls       int
	deleteBlockCalls    int
	listOwner           string
	listSpaceID         string
	lastStart           time.Time
	lastEnd             time.Time
}

func (r *countingOccupancyRepository) SetTimeZone(context.Context, string, string, string) error {
	r.setTimeZoneCalls++
	return nil
}

func (r *countingOccupancyRepository) SetTimeZoneCAS(context.Context, string, string, string, time.Time) (time.Time, error) {
	r.setTimeZoneCASCalls++
	return time.Time{}, nil
}

func (r *countingOccupancyRepository) TimeZone(context.Context, string, string) (string, error) {
	r.timeZoneCalls++
	return "America/Santiago", nil
}

func (r *countingOccupancyRepository) Availability(_ context.Context, _, spaceID string, start, end time.Time) (occupancy.Availability, error) {
	r.availabilityCalls++
	return occupancy.Availability{SpaceID: spaceID, TimeZone: "America/Santiago", StartAt: start, EndAt: end, Available: true}, nil
}

func (r *countingOccupancyRepository) ListBlocks(_ context.Context, owner, spaceID string, start, end time.Time) (occupancy.Calendar, error) {
	r.listBlocksCalls++
	r.listOwner, r.listSpaceID = owner, spaceID
	r.lastStart, r.lastEnd = start, end
	return occupancy.Calendar{SpaceID: spaceID, TimeZone: "America/Santiago", Items: []occupancy.Block{}}, nil
}

func (r *countingOccupancyRepository) CreateBlock(_ context.Context, _, _, _ string, _, _ time.Time, _ string) (occupancy.Block, error) {
	r.createBlockCalls++
	return occupancy.Block{}, nil
}

func (r *countingOccupancyRepository) DeleteBlock(context.Context, string, string, string) error {
	r.deleteBlockCalls++
	return nil
}

func TestCalendarRejectsUnsupportedMethodAndResourceRoutesWithoutRepositoryCalls(t *testing.T) {
	repository := &countingOccupancyRepository{}
	calendar, err := occupancy.NewService(repository, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesService, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, spacesService, nil, calendar)
	base := "/api/v1/spaces/" + draftID + "/availability"
	cases := []struct {
		name, method, path string
	}{
		{name: "POST availability", method: http.MethodPost, path: base},
		{name: "PUT blocks collection", method: http.MethodPut, path: base + "/blocks"},
		{name: "DELETE blocks collection", method: http.MethodDelete, path: base + "/blocks"},
		{name: "POST timezone resource", method: http.MethodPost, path: base + "/timezone"},
		{name: "GET individual block resource", method: http.MethodGet, path: base + "/blocks/" + draftID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := invoke(h, tc.method, tc.path, "user", "")
			if response.Code != http.StatusNotFound {
				t.Fatalf("status=%d body=%s, want 404", response.Code, response.Body.String())
			}
		})
	}
	if repository.setTimeZoneCalls != 0 || repository.setTimeZoneCASCalls != 0 || repository.timeZoneCalls != 0 ||
		repository.availabilityCalls != 0 || repository.listBlocksCalls != 0 || repository.createBlockCalls != 0 || repository.deleteBlockCalls != 0 {
		t.Fatalf("unsupported calendar routes reached repository: %+v", repository)
	}
}

func TestCalendarRouteFailsClosedWithoutServiceOrOnMalformedPath(t *testing.T) {
	spacesService, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/spaces/" + draftID + "/availability"
	withoutCalendar := NewHandler(fakeAuth{}, spacesService, nil)
	response := invoke(withoutCalendar, http.MethodGet, base, "user", "")
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("missing calendar service status=%d body=%s", response.Code, response.Body.String())
	}

	repository := &countingOccupancyRepository{}
	calendar, err := occupancy.NewService(repository, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, spacesService, nil, calendar)
	for _, tc := range []struct{ name, path string }{
		{name: "wrong path prefix", path: "/outside/availability"},
		{name: "wrong resource segment", path: "/api/v1/spaces/" + draftID + "/other/availability"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := invoke(h, http.MethodGet, tc.path, "user", "")
			if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if repository.setTimeZoneCalls != 0 || repository.setTimeZoneCASCalls != 0 || repository.timeZoneCalls != 0 ||
		repository.availabilityCalls != 0 || repository.listBlocksCalls != 0 || repository.createBlockCalls != 0 || repository.deleteBlockCalls != 0 {
		t.Fatalf("malformed calendar routes reached repository: %+v", repository)
	}
}

func TestCalendarBlockListForwardsAuthenticatedOwnerAndSpaceScope(t *testing.T) {
	repository := &countingOccupancyRepository{}
	calendar, err := occupancy.NewService(repository, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesService, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, spacesService, nil, calendar)
	path := "/api/v1/spaces/" + draftID + "/availability/blocks?from=2030-01-01T10%3A00%3A00Z&to=2030-01-01T11%3A00%3A00Z"

	for i, tc := range []struct {
		name, token, owner string
	}{
		{name: "primary owner", token: "user", owner: ownerA},
		{name: "other authenticated owner", token: "other", owner: ownerB},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := invoke(h, http.MethodGet, path, tc.token, "")
			if response.Code != http.StatusOK || repository.listBlocksCalls != i+1 {
				t.Fatalf("status=%d listCalls=%d body=%s", response.Code, repository.listBlocksCalls, response.Body.String())
			}
			if repository.listOwner != tc.owner || repository.listSpaceID != draftID {
				t.Fatalf("repository scope owner=%q space=%q, want owner=%q space=%q", repository.listOwner, repository.listSpaceID, tc.owner, draftID)
			}
		})
	}
}

func TestCalendarBlockRangeRejectsInvalidTimestampsBeforeRepositoryRead(t *testing.T) {
	repository := &countingOccupancyRepository{}
	calendar, err := occupancy.NewService(repository, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesService, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, spacesService, nil, calendar)
	base := "/api/v1/spaces/" + draftID + "/availability/blocks"
	for _, tc := range []struct {
		name string
		from string
		to   string
	}{
		{name: "date without time", from: "2030-01-01", to: "2030-01-02T00:00:00Z"},
		{name: "non-UTC offset", from: "2030-01-01T10:00:00-03:00", to: "2030-01-01T11:00:00-03:00"},
		{name: "impossible date", from: "2030-02-30T10:00:00Z", to: "2030-03-01T10:00:00Z"},
		{name: "equal bounds", from: "2030-01-01T10:00:00Z", to: "2030-01-01T10:00:00Z"},
		{name: "reversed bounds", from: "2030-01-01T11:00:00Z", to: "2030-01-01T10:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := base + "?from=" + url.QueryEscape(tc.from) + "&to=" + url.QueryEscape(tc.to)
			response := invoke(h, http.MethodGet, path, "user", "")
			if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"code":"validation_error"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if repository.listBlocksCalls != 0 {
				t.Fatalf("invalid range reached repository %d times", repository.listBlocksCalls)
			}
		})
	}

	availabilityReversed := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+draftID+"/availability", nil)
	availabilityReversed.URL.RawQuery = "from=2030-01-01T11%3A00%3A00Z&to=2030-01-01T10%3A00%3A00Z"
	availabilityReversed.Header.Set("Authorization", "Bearer user")
	availabilityRangeResponse := httptest.NewRecorder()
	h.ServeHTTP(availabilityRangeResponse, availabilityReversed)
	if availabilityRangeResponse.Code != http.StatusUnprocessableEntity || !strings.Contains(availabilityRangeResponse.Body.String(), `"code":"validation_error"`) {
		t.Fatalf("availability reversed range status=%d body=%s", availabilityRangeResponse.Code, availabilityRangeResponse.Body.String())
	}
	if repository.availabilityCalls != 0 {
		t.Fatalf("invalid availability range reached repository %d times", repository.availabilityCalls)
	}

	availabilityOffset := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+draftID+"/availability", nil)
	availabilityOffset.URL.RawQuery = "from=2030-01-01T10%3A00%3A00-03%3A00&to=2030-01-01T11%3A00%3A00-03%3A00"
	availabilityOffset.Header.Set("Authorization", "Bearer user")
	availabilityOffsetResponse := httptest.NewRecorder()
	h.ServeHTTP(availabilityOffsetResponse, availabilityOffset)
	if availabilityOffsetResponse.Code != http.StatusUnprocessableEntity || !strings.Contains(availabilityOffsetResponse.Body.String(), `"code":"validation_error"`) {
		t.Fatalf("availability non-UTC range status=%d body=%s", availabilityOffsetResponse.Code, availabilityOffsetResponse.Body.String())
	}
	if repository.availabilityCalls != 0 {
		t.Fatalf("non-UTC availability range reached repository %d times", repository.availabilityCalls)
	}

	invalidSpace := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/not-a-uuid/availability", nil)
	invalidSpace.URL.RawQuery = "from=2030-01-01T13%3A00%3A00Z&to=2030-01-01T14%3A00%3A00Z"
	invalidSpace.Header.Set("Authorization", "Bearer user")
	invalidSpaceResponse := httptest.NewRecorder()
	h.ServeHTTP(invalidSpaceResponse, invalidSpace)
	if invalidSpaceResponse.Code != http.StatusUnprocessableEntity || !strings.Contains(invalidSpaceResponse.Body.String(), `"code":"validation_error"`) {
		t.Fatalf("invalid space ID status=%d body=%s", invalidSpaceResponse.Code, invalidSpaceResponse.Body.String())
	}
	if repository.availabilityCalls != 0 {
		t.Fatalf("invalid space ID reached repository %d times", repository.availabilityCalls)
	}

	malformed := httptest.NewRequest(http.MethodGet, base, nil)
	malformed.URL.RawQuery = "from=2030-01-01T13%3A00%3A00Z&to=2030-01-01T14%3A00%3A00Z&%ZZ"
	malformed.Header.Set("Authorization", "Bearer user")
	malformedResponse := httptest.NewRecorder()
	h.ServeHTTP(malformedResponse, malformed)
	if malformedResponse.Code != http.StatusBadRequest || !strings.Contains(malformedResponse.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("malformed query status=%d body=%s", malformedResponse.Code, malformedResponse.Body.String())
	}
	if repository.listBlocksCalls != 0 {
		t.Fatalf("malformed query reached repository %d times", repository.listBlocksCalls)
	}

	availabilityMalformed := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+draftID+"/availability", nil)
	availabilityMalformed.URL.RawQuery = "from=2030-01-01T13%3A00%3A00Z&to=2030-01-01T14%3A00%3A00Z&%ZZ"
	availabilityMalformed.Header.Set("Authorization", "Bearer user")
	availabilityResponse := httptest.NewRecorder()
	h.ServeHTTP(availabilityResponse, availabilityMalformed)
	if availabilityResponse.Code != http.StatusBadRequest || !strings.Contains(availabilityResponse.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("availability malformed query status=%d body=%s", availabilityResponse.Code, availabilityResponse.Body.String())
	}
	if repository.availabilityCalls != 0 {
		t.Fatalf("malformed availability query reached repository %d times", repository.availabilityCalls)
	}

	valid := invoke(h, http.MethodGet, base+"?from=2030-01-01T13%3A00%3A00Z&to=2030-01-01T14%3A00%3A00Z", "user", "")
	if valid.Code != http.StatusOK || repository.listBlocksCalls != 1 {
		t.Fatalf("valid range status=%d repository calls=%d body=%s", valid.Code, repository.listBlocksCalls, valid.Body.String())
	}
	wantStart := time.Date(2030, 1, 1, 13, 0, 0, 0, time.UTC)
	wantEnd := wantStart.Add(time.Hour)
	if !repository.lastStart.Equal(wantStart) || !repository.lastEnd.Equal(wantEnd) {
		t.Fatalf("repository range = %s .. %s, want %s .. %s", repository.lastStart, repository.lastEnd, wantStart, wantEnd)
	}
}

func TestManualBlockCreateHTTPRejectsInvalidInputBeforeRepositoryWrite(t *testing.T) {
	repository := &countingOccupancyRepository{}
	calendar, err := occupancy.NewService(repository, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesService, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, spacesService, nil, calendar)
	base := "/api/v1/spaces/" + draftID + "/availability/blocks"
	valid := occupancy.BlockInput{
		StartAt: "2030-04-01T12:00:00Z",
		EndAt:   "2030-04-01T13:00:00Z",
		Reason:  "Mantención",
	}
	cases := []struct {
		name  string
		input occupancy.BlockInput
	}{
		{name: "date without time", input: occupancy.BlockInput{StartAt: "2030-04-01", EndAt: valid.EndAt, Reason: valid.Reason}},
		{name: "non-UTC offset", input: occupancy.BlockInput{StartAt: "2030-04-01T09:00:00-03:00", EndAt: "2030-04-01T10:00:00-03:00", Reason: valid.Reason}},
		{name: "equal bounds", input: occupancy.BlockInput{StartAt: valid.StartAt, EndAt: valid.StartAt, Reason: valid.Reason}},
		{name: "inverted bounds", input: occupancy.BlockInput{StartAt: valid.EndAt, EndAt: valid.StartAt, Reason: valid.Reason}},
		{name: "empty trimmed reason", input: occupancy.BlockInput{StartAt: valid.StartAt, EndAt: valid.EndAt, Reason: " \t "}},
		{name: "reason exceeds Unicode codepoint limit", input: occupancy.BlockInput{StartAt: valid.StartAt, EndAt: valid.EndAt, Reason: strings.Repeat("ñ", 501)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			response := invoke(h, http.MethodPost, base, "user", string(body))
			if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"code":"validation_error"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if repository.createBlockCalls != 0 {
				t.Fatalf("invalid block reached repository %d times", repository.createBlockCalls)
			}
		})
	}
}

func TestManualBlockCreateHTTPRejectsUnexpectedBodyAndQueryBeforeRepositoryWrite(t *testing.T) {
	repository := &countingOccupancyRepository{}
	calendar, err := occupancy.NewService(repository, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesService, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, spacesService, nil, calendar)
	base := "/api/v1/spaces/" + draftID + "/availability/blocks"
	validBody := `{"start_at":"2030-04-01T12:00:00Z","end_at":"2030-04-01T13:00:00Z","reason":"Mantención"}`
	cases := []struct {
		name, path, body string
	}{
		{name: "unknown owner field", path: base, body: `{"start_at":"2030-04-01T12:00:00Z","end_at":"2030-04-01T13:00:00Z","reason":"Mantención","owner_id":"` + ownerB + `"}`},
		{name: "multiple JSON values", path: base, body: validBody + ` {}`},
		{name: "body exceeds request size limit", path: base, body: validBody + strings.Repeat(" ", 64*1024)},
		{name: "unexpected query parameter", path: base + "?owner_id=" + ownerB, body: validBody},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := invoke(h, http.MethodPost, tc.path, "user", tc.body)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_request"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if repository.createBlockCalls != 0 {
				t.Fatalf("invalid HTTP request reached repository %d times", repository.createBlockCalls)
			}
		})
	}
}

func TestManualBlockCreateHTTPRejectsNonJSONBeforeRepositoryWrite(t *testing.T) {
	repository := &countingOccupancyRepository{}
	calendar, err := occupancy.NewService(repository, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesService, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, spacesService, nil, calendar)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/spaces/"+draftID+"/availability/blocks",
		strings.NewReader(`{"start_at":"2030-04-01T12:00:00Z","end_at":"2030-04-01T13:00:00Z","reason":"Mantención"}`),
	)
	request.Header.Set("Authorization", "Bearer user")
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)

	if response.Code != http.StatusUnsupportedMediaType || !strings.Contains(response.Body.String(), `"code":"unsupported_media_type"`) {
		t.Fatalf("status=%d body=%s, want 415 unsupported_media_type", response.Code, response.Body.String())
	}
	if repository.createBlockCalls != 0 {
		t.Fatalf("non-JSON request reached repository %d times", repository.createBlockCalls)
	}
}

type calendarErrorStub struct {
	fakeCalendar
	err error
}

func (c calendarErrorStub) SetTimeZoneCAS(context.Context, string, string, string, time.Time) (time.Time, error) {
	return time.Time{}, c.err
}

func (c calendarErrorStub) ListBlocks(context.Context, string, string, string, string) (occupancy.Calendar, error) {
	return occupancy.Calendar{}, c.err
}

func (c calendarErrorStub) CreateBlock(context.Context, string, string, occupancy.BlockInput) (occupancy.Block, error) {
	return occupancy.Block{}, c.err
}

func (c calendarErrorStub) DeleteBlock(context.Context, string, string, string) error {
	return c.err
}

// This test characterizes the existing HTTP error contract for calendar
// operations; repository details must remain sanitized at the transport edge.
func TestCalendarHTTPMapsAvailabilityErrorsWithoutChangingContract(t *testing.T) {
	revision := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	base := "/api/v1/spaces/" + draftID + "/availability"
	cases := []struct {
		name, method, path, body, code string
		err                            error
		status                         int
	}{
		{
			name: "missing timezone while listing blocks", method: http.MethodGet,
			path: base + "/blocks?from=2030-01-01T13%3A00%3A00Z&to=2030-01-01T14%3A00%3A00Z",
			err:  occupancy.ErrTimezoneRequired, status: http.StatusConflict, code: "time_zone_required",
		},
		{
			name: "overlapping manual block", method: http.MethodPost, path: base + "/blocks",
			body: `{"start_at":"2030-01-01T13:00:00Z","end_at":"2030-01-01T14:00:00Z","reason":"synthetic"}`,
			err:  occupancy.ErrConflict, status: http.StatusConflict, code: "occupancy_conflict",
		},
		{
			name: "missing block deletion", method: http.MethodDelete, path: base + "/blocks/00000000-0000-4000-8000-000000000003",
			err: occupancy.ErrNotFound, status: http.StatusNotFound, code: "not_found",
		},
		{
			name: "timezone cannot be replaced while in use", method: http.MethodPut, path: base,
			body: `{"time_zone":"America/Santiago"}`,
			err:  occupancy.ErrTimezoneInUse, status: http.StatusConflict, code: "time_zone_in_use",
		},
		{
			name: "locked timezone remains locked", method: http.MethodPut, path: base,
			body: `{"time_zone":"America/Santiago"}`,
			err:  occupancy.ErrTimezoneLocked, status: http.StatusConflict, code: "time_zone_locked",
		},
		{
			name: "unknown calendar failure is sanitized", method: http.MethodGet,
			path: base + "/blocks?from=2030-01-01T13%3A00%3A00Z&to=2030-01-01T14%3A00%3A00Z",
			err:  errors.New("synthetic repository detail"), status: http.StatusInternalServerError, code: "internal_error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memoryRepo{owner: ownerA, draft: spaces.Draft{ID: draftID, UpdatedAt: revision}}
			service, err := spaces.NewService(repo)
			if err != nil {
				t.Fatal(err)
			}
			h := NewHandler(fakeAuth{}, service, nil, calendarErrorStub{err: tc.err})
			var response *httptest.ResponseRecorder
			if tc.method == http.MethodPut {
				tag := DraftETag(spaces.Draft{ID: draftID, UpdatedAt: revision})
				response = invokeIfMatch(h, tc.path, "user", tc.body, tag)
			} else {
				response = invoke(h, tc.method, tc.path, "user", tc.body)
			}
			if response.Code != tc.status || !strings.Contains(response.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("status=%d body=%s, want status=%d code=%s", response.Code, response.Body.String(), tc.status, tc.code)
			}
			if strings.Contains(response.Body.String(), "synthetic repository detail") {
				t.Fatalf("raw repository error leaked: %s", response.Body.String())
			}
		})
	}
}

type manualBlockCaptureRepository struct {
	occupancy.Repository
	owner, spaceID, blockID, reason string
	start, end                      time.Time
}

func (r *manualBlockCaptureRepository) CreateBlock(_ context.Context, owner, spaceID, blockID string, start, end time.Time, reason string) (occupancy.Block, error) {
	r.owner, r.spaceID, r.blockID = owner, spaceID, blockID
	r.start, r.end, r.reason = start, end, reason
	return occupancy.Block{
		ID: blockID, SpaceID: spaceID, TimeZone: "America/Santiago",
		StartAt: start, EndAt: end, Reason: reason,
	}, nil
}

func TestManualBlockCreateHTTPForwardsValidatedUTCBlockAndReturnsRepositoryBlock(t *testing.T) {
	repository := &manualBlockCaptureRepository{}
	calendar, err := occupancy.NewService(repository, credentials.Generator{})
	if err != nil {
		t.Fatal(err)
	}
	spacesService, err := spaces.NewService(&memoryRepo{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, spacesService, nil, calendar)
	body := `{"start_at":"2030-04-01T12:00:00.123456789Z","end_at":"2030-04-01T13:00:00.987654321Z","reason":"  mantención  "}`
	response := invoke(h, http.MethodPost, "/api/v1/spaces/"+draftID+"/availability/blocks", "user", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	var got occupancy.Block
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode created block: %v", err)
	}
	wantStart := time.Date(2030, time.April, 1, 12, 0, 0, 123456789, time.UTC)
	wantEnd := time.Date(2030, time.April, 1, 13, 0, 0, 987654321, time.UTC)
	if repository.owner != ownerA || repository.spaceID != draftID || repository.blockID == "" ||
		!repository.start.Equal(wantStart) || !repository.end.Equal(wantEnd) || repository.reason != "mantención" {
		t.Fatalf("repository received owner=%q space=%q id=%q start=%s end=%s reason=%q", repository.owner, repository.spaceID, repository.blockID, repository.start, repository.end, repository.reason)
	}
	if got.ID != repository.blockID || got.SpaceID != draftID || got.TimeZone != "America/Santiago" ||
		!got.StartAt.Equal(wantStart) || !got.EndAt.Equal(wantEnd) || got.Reason != "mantención" {
		t.Fatalf("response block=%+v, want repository readback", got)
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

	repository := &memoryRepo{}
	service, err := spaces.NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, service, []string{"http://localhost"})
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
	if strings.Contains(created.Body.String(), `"cover_photo_id"`) {
		t.Fatalf("unselected cover should remain omitted: %s", created.Body.String())
	}
	selectedCoverID := "11111111-1111-4111-8111-111111111111"
	repository.draft.CoverPhotoID = &selectedCoverID
	coverResponse := invoke(h, "GET", "/api/v1/spaces/"+draftID, "user", "")
	if coverResponse.Code != http.StatusOK || !strings.Contains(coverResponse.Body.String(), `"cover_photo_id":"`+selectedCoverID+`"`) {
		t.Fatalf("selected cover response status=%d body=%s", coverResponse.Code, coverResponse.Body.String())
	}
	assertDraftResponseSchema(t, schema, coverResponse.Body.Bytes())

	input := spaces.Input{CategoryCode: "sala_multiproposito", AttributeSchemaVersion: 1, Title: "Sala", Description: strings.Repeat("Sala multipropósito para actividades privadas. ", 3), AreaM2: 10, Capacity: 3, UsageRules: "Sin fumar", RateUnit: "hora", BasePriceCLP: 6001, Address: "Calle 2"}
	body, _ := json.Marshal(input)
	current := invoke(h, http.MethodGet, "/api/v1/spaces/"+draftID, "user", "")
	updated := invokeIfMatch(h, "/api/v1/spaces/"+draftID, "user", string(body), current.Header().Get("ETag"))
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

func TestOwnPublicationDeleteResponseAndErrorsMatchContract(t *testing.T) {
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
	receiptSchema := compileOpenAPISchema(t, schemas["SpaceDeletionReceipt"])

	repository := &memoryRepo{owner: ownerA, draft: spaces.Draft{ID: draftID, State: "activa"}}
	service, err := spaces.NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(fakeAuth{}, service, []string{"http://localhost"})
	deleted := invoke(h, http.MethodDelete, "/api/v1/spaces/"+draftID, "landlord", "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	assertJSONSchema(t, receiptSchema, deleted.Body.Bytes(), "SpaceDeletionReceipt")
	if foreign := invoke(h, http.MethodDelete, "/api/v1/spaces/"+draftID, "other", ""); foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign delete status=%d body=%s", foreign.Code, foreign.Body.String())
	}
	repository.draft.State = "activa"
	repository.publicationErr = spaces.ErrDeleteBlocked
	blocked := invoke(h, http.MethodDelete, "/api/v1/spaces/"+draftID, "landlord", "")
	if blocked.Code != http.StatusConflict || !strings.Contains(blocked.Body.String(), `"code":"publication_delete_blocked"`) {
		t.Fatalf("blocked delete status=%d body=%s", blocked.Code, blocked.Body.String())
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
	current := invoke(h, http.MethodGet, "/api/v1/spaces/"+draftID, "user", "")
	updated := invokeIfMatch(h, "/api/v1/spaces/"+draftID, "user", validJSON(), current.Header().Get("ETag"))
	if updated.Code != 200 || !strings.Contains(updated.Body.String(), `"attribute_schema_version":1`) {
		t.Fatalf("v1 update status=%d %s", updated.Code, updated.Body.String())
	}
	if repo.draft.AttributeSchemaVersion != 1 {
		t.Fatalf("update implicitly converted profile version: %d", repo.draft.AttributeSchemaVersion)
	}
}
