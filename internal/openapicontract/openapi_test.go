package openapicontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/HernanEspinozaDev/espaciGo/internal/publiccatalog"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	"gopkg.in/yaml.v3"
)

var httpMethods = map[string]struct{}{
	"get": {}, "post": {}, "put": {}, "patch": {}, "delete": {}, "head": {}, "options": {}, "trace": {},
}

func TestPublicSpaceSchemasHaveExactPropertyAndRequiredAllowLists(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	schemas := nodeAt(t, nodeAt(t, document.Content[0], "components"), "schemas")
	cases := []struct {
		name       string
		properties []string
		required   []string
	}{
		{
			name: "PublicSpace",
			properties: []string{
				"space_id", "category_code", "category_name", "title", "description", "area_m2", "rate_unit",
				"base_price_clp", "currency", "region_code", "region_name", "comuna_code", "comuna_name",
				"estimated_total_clp", "cover_image_id", "features", "images", "verification",
			},
			required: []string{
				"space_id", "category_code", "category_name", "title", "description", "area_m2", "rate_unit",
				"base_price_clp", "currency", "estimated_total_clp",
			},
		},
		{
			name: "PublicSpaceDetail",
			properties: []string{
				"space_id", "category_code", "category_name", "title", "description", "area_m2", "rate_unit",
				"base_price_clp", "currency", "region_code", "region_name", "comuna_code", "comuna_name",
				"estimated_total_clp", "usage_rules", "features", "images", "weekly_schedule", "verification",
			},
			required: []string{
				"space_id", "category_code", "category_name", "title", "description", "area_m2", "rate_unit",
				"base_price_clp", "currency", "estimated_total_clp", "usage_rules", "features", "images",
			},
		},
		{
			name:       "PublicDocumentVerification",
			properties: []string{"identity_verified", "space_verified"},
			required:   []string{"identity_verified", "space_verified"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := nodeAt(t, schemas, tc.name)
			if got := scalarAt(t, schema, "additionalProperties"); got != "false" {
				t.Fatalf("%s additionalProperties=%q, want false", tc.name, got)
			}
			assertYAMLStringSet(t, nodeAt(t, schema, "properties"), tc.properties)
			assertYAMLStringSet(t, nodeAt(t, schema, "required"), tc.required)
		})
	}
	for _, name := range []string{"PublicSpace", "PublicSpaceDetail"} {
		verification := nodeAt(t, nodeAt(t, nodeAt(t, schemas, name), "properties"), "verification")
		if got := scalarAt(t, verification, "$ref"); got != "#/components/schemas/PublicDocumentVerification" {
			t.Errorf("%s.verification ref=%q", name, got)
		}
	}
}

func TestPublicSpaceImageRefSchemaDefinesAdapterSourceEnum(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	schemas := nodeAt(t, nodeAt(t, document.Content[0], "components"), "schemas")
	schema := nodeAt(t, schemas, "PublicSpaceImageRef")
	if got := scalarAt(t, schema, "additionalProperties"); got != "false" {
		t.Fatalf("PublicSpaceImageRef additionalProperties=%q, want false", got)
	}
	assertYAMLStringSet(t, nodeAt(t, schema, "properties"), []string{"image_id", "is_cover", "source"})
	assertYAMLStringSet(t, nodeAt(t, schema, "required"), []string{"image_id", "is_cover", "source"})
	source := nodeAt(t, nodeAt(t, schema, "properties"), "source")
	if got := scalarAt(t, source, "type"); got != "string" {
		t.Fatalf("PublicSpaceImageRef.source type=%q, want string", got)
	}
	assertYAMLStringSet(t, nodeAt(t, source, "enum"), []string{"synthetic", "media"})
}

func assertYAMLStringSet(t *testing.T, node *yaml.Node, want []string) {
	t.Helper()
	got := make(map[string]struct{}, len(want))
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			got[node.Content[i].Value] = struct{}{}
		}
	} else if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			got[item.Value] = struct{}{}
		}
	} else {
		t.Fatalf("expected YAML mapping or sequence, got kind %d", node.Kind)
	}
	if len(got) != len(want) {
		t.Fatalf("keys=%v, want exactly %v", got, want)
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q from %v", key, got)
		}
	}
}

func TestOpenAPIContractParsesWithUniqueOperationsAndResponses(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse OpenAPI YAML: %v", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		t.Fatal("OpenAPI document root must be a mapping")
	}
	root := document.Content[0]
	if got := scalarAt(t, root, "openapi"); got != "3.1.0" {
		t.Fatalf("OpenAPI version=%q, want 3.1.0", got)
	}
	paths := nodeAt(t, root, "paths")
	if paths.Kind != yaml.MappingNode || len(paths.Content) == 0 {
		t.Fatal("OpenAPI paths must be a non-empty mapping")
	}
	operationIDs := map[string]string{}
	operations := 0
	for i := 0; i < len(paths.Content); i += 2 {
		path, item := paths.Content[i].Value, paths.Content[i+1]
		if item.Kind != yaml.MappingNode {
			t.Fatalf("path item %s must be a mapping", path)
		}
		for j := 0; j < len(item.Content); j += 2 {
			method, operation := strings.ToLower(item.Content[j].Value), item.Content[j+1]
			if _, ok := httpMethods[method]; !ok {
				continue
			}
			operations++
			id := scalarAt(t, operation, "operationId")
			if prior, exists := operationIDs[id]; exists {
				t.Fatalf("duplicate OpenAPI operationId %q at %s and %s", id, prior, path)
			}
			operationIDs[id] = path
			responses := nodeAt(t, operation, "responses")
			if responses.Kind != yaml.MappingNode || len(responses.Content) == 0 {
				t.Fatalf("%s %s must define at least one response", method, path)
			}
		}
	}
	if operations == 0 {
		t.Fatal("OpenAPI document has no HTTP operations")
	}
}

func TestPublishedRentalContractsSeparatePublicAvailabilityFromAuthenticatedDraft(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	root := document.Content[0]
	paths := nodeAt(t, root, "paths")

	availability := nodeAt(t, paths, "/spaces/{spaceID}/rental-availability")
	availabilityGet := nodeAt(t, availability, "get")
	if got := scalarAt(t, availabilityGet, "operationId"); got != "listPublicRentalAvailability" {
		t.Fatalf("availability operationId=%q", got)
	}
	security := nodeAt(t, availabilityGet, "security")
	if security.Kind != yaml.SequenceNode || len(security.Content) != 0 {
		t.Fatal("public rental availability must explicitly disable inherited authentication")
	}
	availabilityOK := nodeAt(t, nodeAt(t, availabilityGet, "responses"), "200")
	availabilityCache := nodeAt(t, nodeAt(t, availabilityOK, "headers"), "Cache-Control")
	if got := scalarAt(t, nodeAt(t, availabilityCache, "schema"), "const"); got != "no-store" {
		t.Fatalf("public availability Cache-Control const=%q", got)
	}

	quotes := nodeAt(t, paths, "/rental-quotes")
	quotePost := nodeAt(t, quotes, "post")
	if got := scalarAt(t, quotePost, "operationId"); got != "createRentalQuote" {
		t.Fatalf("quote operationId=%q", got)
	}
	quoteCreated := nodeAt(t, nodeAt(t, quotePost, "responses"), "201")
	quoteCache := nodeAt(t, nodeAt(t, quoteCreated, "headers"), "Cache-Control")
	if got := scalarAt(t, nodeAt(t, quoteCache, "schema"), "const"); got != "no-store" {
		t.Fatalf("rental quote Cache-Control const=%q", got)
	}

	previewPath := nodeAt(t, paths, "/rental-quotes/{quoteID}/contract-preview")
	previewPost := nodeAt(t, previewPath, "post")
	if got := scalarAt(t, previewPost, "operationId"); got != "previewRentalContract" {
		t.Fatalf("contract preview operationId=%q", got)
	}
	responses := nodeAt(t, previewPost, "responses")
	previewOK := nodeAt(t, responses, "200")
	previewCache := nodeAt(t, nodeAt(t, previewOK, "headers"), "Cache-Control")
	if got := scalarAt(t, nodeAt(t, previewCache, "schema"), "const"); got != "no-store" {
		t.Fatalf("contract preview Cache-Control const=%q", got)
	}
	for _, code := range []string{"200", "401", "403", "404", "405", "409", "503"} {
		_ = nodeAt(t, responses, code)
	}
	components := nodeAt(t, root, "components")
	schemas := nodeAt(t, components, "schemas")
	assertSchemaPropertyKeys(t, schemas, "RentalInterval", "start_at", "end_at")
	assertSchemaPropertyKeys(t, schemas, "PublicRentalAvailability", "space_id", "time_zone", "rate_unit", "checked_at", "items")
	assertSchemaPropertyKeys(t, schemas, "RentalAvailabilityEnvelope", "data")
	assertSchemaPropertyKeys(t, schemas, "CreateRentalQuoteRequest", "space_id", "start_at", "end_at")
	assertSchemaPropertyKeys(t, schemas, "RentalQuote", "id", "space_id", "space_title", "rate_unit", "unit_price_clp", "currency", "units", "subtotal_clp", "start_at", "end_at", "time_zone", "created_at", "expires_at")
	assertSchemaPropertyKeys(t, schemas, "RentalQuoteEnvelope", "data")
	assertSchemaPropertyKeys(t, schemas, "RentalContractPartyPreview", "role", "legal_name", "rut")
	assertSchemaPropertyKeys(t, schemas, "RentalContractPreview", "state", "quote_id", "space_id", "space_title", "rate_unit", "unit_price_clp", "units", "subtotal_clp", "currency", "start_at", "end_at", "time_zone", "parties", "generated_at")
	assertSchemaPropertyKeys(t, schemas, "RentalContractPreviewEnvelope", "data")
	assertSchemaRequiredKeys(t, schemas, "RentalInterval", "start_at", "end_at")
	assertSchemaRequiredKeys(t, schemas, "PublicRentalAvailability", "space_id", "time_zone", "rate_unit", "checked_at", "items")
	assertSchemaRequiredKeys(t, schemas, "RentalAvailabilityEnvelope", "data")
	assertSchemaRequiredKeys(t, schemas, "CreateRentalQuoteRequest", "space_id", "start_at", "end_at")
	assertSchemaRequiredKeys(t, schemas, "RentalQuote", "id", "space_id", "space_title", "rate_unit", "unit_price_clp", "currency", "units", "subtotal_clp", "start_at", "end_at", "time_zone", "created_at", "expires_at")
	assertSchemaRequiredKeys(t, schemas, "RentalQuoteEnvelope", "data")
	assertSchemaRequiredKeys(t, schemas, "RentalContractPartyPreview", "role", "legal_name", "rut")
	assertSchemaRequiredKeys(t, schemas, "RentalContractPreview", "state", "quote_id", "space_id", "space_title", "rate_unit", "unit_price_clp", "units", "subtotal_clp", "currency", "start_at", "end_at", "time_zone", "parties", "generated_at")
	assertSchemaRequiredKeys(t, schemas, "RentalContractPreviewEnvelope", "data")
	for _, schemaName := range []string{"RentalInterval", "PublicRentalAvailability", "RentalAvailabilityEnvelope", "CreateRentalQuoteRequest", "RentalQuote", "RentalQuoteEnvelope", "RentalContractPartyPreview", "RentalContractPreview", "RentalContractPreviewEnvelope"} {
		schema := nodeAt(t, schemas, schemaName)
		if got := scalarAt(t, schema, "additionalProperties"); got != "false" {
			t.Fatalf("rental schema %s additionalProperties=%q, want false", schemaName, got)
		}
	}
	rootSecurity := nodeAt(t, root, "security")
	if rootSecurity.Kind != yaml.SequenceNode || len(rootSecurity.Content) == 0 {
		t.Fatal("authenticated rental operations require the global bearer security policy")
	}
	for name, operation := range map[string]*yaml.Node{"quote": quotePost, "preview": previewPost} {
		if security, ok := optionalMappingValue(operation, "security"); ok && security.Kind == yaml.SequenceNode && len(security.Content) == 0 {
			t.Fatalf("authenticated rental %s must not disable inherited security", name)
		}
	}
	availabilitySchema := nodeAt(t, schemas, "PublicRentalAvailability")
	availabilityProperties := nodeAt(t, availabilitySchema, "properties")
	for _, forbidden := range []string{"owner_id", "occupant_id", "reservation_id", "block_reason"} {
		if hasMappingKey(availabilityProperties, forbidden) {
			t.Fatalf("public availability must not expose %q", forbidden)
		}
	}
	quoteSchema := nodeAt(t, schemas, "RentalQuote")
	quoteProperties := nodeAt(t, quoteSchema, "properties")
	if hasMappingKey(quoteProperties, "rate_version") {
		t.Fatal("public rental quote must not expose the internal rate revision")
	}
	for _, forbidden := range []string{"owner_id", "renter_id"} {
		if hasMappingKey(quoteProperties, forbidden) {
			t.Fatalf("rental quote must not expose %q", forbidden)
		}
	}
	requestSchema := nodeAt(t, schemas, "CreateRentalQuoteRequest")
	requestProperties := nodeAt(t, requestSchema, "properties")
	for _, forbidden := range []string{"owner_id", "renter_id", "return_url"} {
		if hasMappingKey(requestProperties, forbidden) {
			t.Fatalf("rental quote request must not accept caller-controlled %q", forbidden)
		}
	}
	preview := nodeAt(t, schemas, "RentalContractPreview")
	properties := nodeAt(t, preview, "properties")
	for _, key := range []string{"quote_id", "space_id", "time_zone", "start_at", "end_at", "subtotal_clp", "parties", "state"} {
		_ = nodeAt(t, properties, key)
	}
	for _, forbidden := range []string{"signature", "payment", "reservation_id", "occupancy_id"} {
		if hasMappingKey(properties, forbidden) {
			t.Fatalf("contract preview must not expose %q in this phase", forbidden)
		}
	}
	parties := nodeAt(t, properties, "parties")
	if scalarAt(t, parties, "minItems") != "2" || scalarAt(t, parties, "maxItems") != "2" {
		t.Fatal("contract preview must contain exactly two participant identity snapshots")
	}
	partyItems := nodeAt(t, parties, "items")
	if scalarAt(t, partyItems, "$ref") != "#/components/schemas/RentalContractPartyPreview" {
		t.Fatal("contract preview parties must use the minimized party schema")
	}
}

func TestRuntimeJSONDTOFieldsMatchOpenAPISchemas(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	schemas := nodeAt(t, nodeAt(t, document.Content[0], "components"), "schemas")
	startAt := time.Date(2030, time.January, 2, 13, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)
	createdAt := startAt.Add(-time.Hour)
	interval := rentalflow.Interval{StartAt: startAt, EndAt: endAt}
	availability := rentalflow.Availability{
		SpaceID: "11111111-1111-4111-8111-111111111111", TimeZone: "America/Santiago", RateUnit: "hora",
		CheckedAt: createdAt, Items: []rentalflow.Interval{interval},
	}
	quote := rentalflow.Quote{
		ID: "33333333-3333-4333-8333-333333333333", SpaceID: availability.SpaceID, SpaceTitle: "Sala sintética",
		RateUnit: "hora", UnitPriceCLP: 12000, Currency: "CLP", Units: 1, SubtotalCLP: 12000,
		StartAt: startAt, EndAt: endAt, TimeZone: availability.TimeZone, CreatedAt: createdAt, ExpiresAt: startAt,
	}
	owner := rentalflow.ContractParty{Role: "arrendador", LegalName: "Propietario Sintético", RUT: "12345678-5"}
	renter := rentalflow.ContractParty{Role: "arrendatario", LegalName: "Arrendatario Sintético", RUT: "87654321-4"}
	preview := rentalflow.ContractPreview{
		State: "preview", QuoteID: quote.ID, SpaceID: quote.SpaceID, SpaceTitle: quote.SpaceTitle,
		RateUnit: quote.RateUnit, UnitPriceCLP: quote.UnitPriceCLP, Units: quote.Units, SubtotalCLP: quote.SubtotalCLP,
		Currency: quote.Currency, StartAt: quote.StartAt, EndAt: quote.EndAt, TimeZone: quote.TimeZone,
		Parties: []rentalflow.ContractParty{owner, renter}, GeneratedAt: createdAt,
	}
	cases := []struct {
		name   string
		schema string
		dto    any
	}{
		{name: "PublicDocumentVerification", schema: "PublicDocumentVerification", dto: publiccatalog.VerificationSummary{IdentityVerified: true}},
		{name: "RentalInterval", schema: "RentalInterval", dto: interval},
		{name: "PublicRentalAvailability", schema: "PublicRentalAvailability", dto: availability},
		{name: "RentalAvailabilityEnvelope", schema: "RentalAvailabilityEnvelope", dto: struct {
			Data rentalflow.Availability `json:"data"`
		}{Data: availability}},
		{name: "CreateRentalQuoteRequest", schema: "CreateRentalQuoteRequest", dto: rentalflow.QuoteRequest{SpaceID: availability.SpaceID, StartAt: startAt.Format(time.RFC3339), EndAt: endAt.Format(time.RFC3339)}},
		{name: "RentalQuote", schema: "RentalQuote", dto: quote},
		{name: "RentalQuoteEnvelope", schema: "RentalQuoteEnvelope", dto: struct {
			Data rentalflow.Quote `json:"data"`
		}{Data: quote}},
		{name: "RentalContractPartyPreview/owner", schema: "RentalContractPartyPreview", dto: owner},
		{name: "RentalContractPartyPreview/renter", schema: "RentalContractPartyPreview", dto: renter},
		{name: "RentalContractPreview", schema: "RentalContractPreview", dto: preview},
		{name: "RentalContractPreviewEnvelope", schema: "RentalContractPreviewEnvelope", dto: struct {
			Data rentalflow.ContractPreview `json:"data"`
		}{Data: preview}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.dto)
			if err != nil {
				t.Fatalf("marshal runtime DTO: %v", err)
			}
			var runtimeProperties map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &runtimeProperties); err != nil {
				t.Fatalf("decode runtime DTO fields: %v", err)
			}
			schema := nodeAt(t, schemas, tc.schema)
			schemaProperties := nodeAt(t, schema, "properties")
			if schemaProperties.Kind != yaml.MappingNode || len(schemaProperties.Content) != len(runtimeProperties)*2 {
				t.Fatalf("runtime DTO fields=%v, OpenAPI %s properties=%v", runtimeProperties, tc.name, schemaProperties.Content)
			}
			required := nodeAt(t, schema, "required")
			if required.Kind != yaml.SequenceNode || len(required.Content) != len(runtimeProperties) {
				t.Fatalf("runtime %s emits %d fields, OpenAPI required=%v", tc.name, len(runtimeProperties), required.Content)
			}
			for key := range runtimeProperties {
				found := false
				for _, requiredKey := range required.Content {
					if requiredKey.Value == key {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("runtime %s emits %q but OpenAPI does not require it", tc.name, key)
				}
			}
			for i := 0; i < len(schemaProperties.Content); i += 2 {
				key := schemaProperties.Content[i].Value
				runtimeValue, ok := runtimeProperties[key]
				if !ok {
					t.Fatalf("OpenAPI %s declares %q absent from runtime JSON %s", tc.name, key, encoded)
				}
				property := schemaProperties.Content[i+1]
				if _, isReference := optionalMappingValue(property, "$ref"); isReference {
					continue
				}
				assertRuntimeEnumAndConst(t, tc.schema, key, property, runtimeValue)
				assertRuntimeArrayBounds(t, tc.schema, key, property, runtimeValue)
				assertRuntimeScalarConstraints(t, tc.schema, key, property, runtimeValue)
				wantType := scalarAt(t, property, "type")
				if gotType := runtimeJSONType(t, runtimeValue); gotType != wantType {
					t.Fatalf("runtime %s.%s JSON type=%q, OpenAPI type=%q", tc.name, key, gotType, wantType)
				}
				if format, ok := optionalMappingValue(property, "format"); ok {
					assertRuntimeFormat(t, tc.name, key, format.Value, runtimeValue)
				}
			}
			for key := range runtimeProperties {
				if !hasMappingKey(schemaProperties, key) {
					t.Fatalf("runtime %s JSON emits %q absent from OpenAPI schema", tc.name, key)
				}
			}
		})
	}
}

func assertRuntimeScalarConstraints(t *testing.T, schema, property string, schemaProperty *yaml.Node, raw json.RawMessage) {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode runtime %s.%s for constraint check: %s (%v)", schema, property, raw, err)
	}
	switch typed := value.(type) {
	case string:
		length := utf8.RuneCountInString(typed)
		for _, bound := range []struct {
			key   string
			isMin bool
		}{
			{key: "minLength", isMin: true},
			{key: "maxLength"},
		} {
			constraint, ok := optionalMappingValue(schemaProperty, bound.key)
			if !ok {
				continue
			}
			limit, err := strconv.Atoi(constraint.Value)
			if err != nil {
				t.Fatalf("OpenAPI %s.%s %s=%q is not an integer", schema, property, bound.key, constraint.Value)
			}
			if (bound.isMin && length < limit) || (!bound.isMin && length > limit) {
				t.Fatalf("runtime %s.%s has %d runes, violates OpenAPI %s=%d", schema, property, length, bound.key, limit)
			}
		}
		if pattern, ok := optionalMappingValue(schemaProperty, "pattern"); ok {
			compiled, err := regexp.Compile(pattern.Value)
			if err != nil {
				t.Fatalf("OpenAPI %s.%s pattern %q is unsupported by Go regexp: %v", schema, property, pattern.Value, err)
			}
			if !compiled.MatchString(typed) {
				t.Fatalf("runtime %s.%s=%q does not match OpenAPI pattern %q", schema, property, typed, pattern.Value)
			}
		}
	case float64:
		for _, bound := range []struct {
			key   string
			isMin bool
		}{
			{key: "minimum", isMin: true},
			{key: "maximum"},
		} {
			constraint, ok := optionalMappingValue(schemaProperty, bound.key)
			if !ok {
				continue
			}
			limit, err := strconv.ParseFloat(constraint.Value, 64)
			if err != nil {
				t.Fatalf("OpenAPI %s.%s %s=%q is not numeric", schema, property, bound.key, constraint.Value)
			}
			if (bound.isMin && typed < limit) || (!bound.isMin && typed > limit) {
				t.Fatalf("runtime %s.%s=%v violates OpenAPI %s=%v", schema, property, typed, bound.key, limit)
			}
		}
	}
}

func assertRuntimeEnumAndConst(t *testing.T, schema, property string, schemaProperty *yaml.Node, raw json.RawMessage) {
	t.Helper()
	var actual string
	if err := json.Unmarshal(raw, &actual); err != nil {
		if _, hasConst := optionalMappingValue(schemaProperty, "const"); hasConst {
			t.Fatalf("runtime %s.%s const value is not a string: %s (%v)", schema, property, raw, err)
		}
		if _, hasEnum := optionalMappingValue(schemaProperty, "enum"); hasEnum {
			t.Fatalf("runtime %s.%s enum value is not a string: %s (%v)", schema, property, raw, err)
		}
		return
	}
	if expected, ok := optionalMappingValue(schemaProperty, "const"); ok && actual != expected.Value {
		t.Fatalf("runtime %s.%s=%q, OpenAPI const=%q", schema, property, actual, expected.Value)
	}
	if enum, ok := optionalMappingValue(schemaProperty, "enum"); ok {
		matched := false
		for _, candidate := range enum.Content {
			if candidate.Value == actual {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("runtime %s.%s=%q is absent from OpenAPI enum", schema, property, actual)
		}
	}
}

func assertRuntimeArrayBounds(t *testing.T, schema, property string, schemaProperty *yaml.Node, raw json.RawMessage) {
	t.Helper()
	for _, bound := range []struct {
		key   string
		isMin bool
	}{
		{key: "minItems", isMin: true},
		{key: "maxItems"},
	} {
		constraint, ok := optionalMappingValue(schemaProperty, bound.key)
		if !ok {
			continue
		}
		limit, err := strconv.Atoi(constraint.Value)
		if err != nil {
			t.Fatalf("OpenAPI %s.%s %s=%q is not an integer", schema, property, bound.key, constraint.Value)
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatalf("runtime %s.%s is not an array: %s (%v)", schema, property, raw, err)
		}
		if (bound.isMin && len(items) < limit) || (!bound.isMin && len(items) > limit) {
			t.Fatalf("runtime %s.%s has %d items, violates OpenAPI %s=%d", schema, property, len(items), bound.key, limit)
		}
	}
}

func assertRuntimeFormat(t *testing.T, schema, property, format string, raw json.RawMessage) {
	t.Helper()
	switch format {
	case "uuid":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("runtime %s.%s formatted value is not a string: %s (%v)", schema, property, raw, err)
		}
		if !rentalflow.ValidUUID(value) {
			t.Fatalf("runtime %s.%s=%q does not satisfy OpenAPI uuid format", schema, property, value)
		}
	case "date-time":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("runtime %s.%s formatted value is not a string: %s (%v)", schema, property, raw, err)
		}
		if !rentalflow.ValidTimestamp(value) {
			t.Fatalf("runtime %s.%s=%q does not satisfy OpenAPI date-time format", schema, property, value)
		}
	case "int64":
		var value int64
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("runtime %s.%s formatted value is not int64: %s (%v)", schema, property, raw, err)
		}
	default:
		t.Fatalf("unhandled OpenAPI format %q on %s.%s", format, schema, property)
	}
}

func runtimeJSONType(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode runtime JSON value %s: %v", raw, err)
	}
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64:
		if math.Trunc(typed) == typed {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		t.Fatalf("unsupported decoded JSON value type %T", value)
		return ""
	}
}

func TestPublishedRentalErrorResponsesDeclareNoStore(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	root := document.Content[0]
	paths := nodeAt(t, root, "paths")
	components := nodeAt(t, root, "components")
	componentResponses := nodeAt(t, components, "responses")
	componentHeaders := nodeAt(t, components, "headers")
	operations := []struct {
		path   string
		method string
		codes  []string
	}{
		{path: "/spaces/{spaceID}/rental-availability", method: "get", codes: []string{"400", "403", "404", "405", "422", "500", "503"}},
		{path: "/rental-quotes", method: "post", codes: []string{"400", "401", "403", "404", "405", "409", "415", "422", "500", "503"}},
		{path: "/rental-quotes/{quoteID}/contract-preview", method: "post", codes: []string{"400", "401", "403", "404", "405", "409", "422", "500", "503"}},
	}
	for _, operation := range operations {
		responses := nodeAt(t, nodeAt(t, nodeAt(t, paths, operation.path), operation.method), "responses")
		for _, code := range operation.codes {
			response := nodeAt(t, responses, code)
			if ref, ok := optionalMappingValue(response, "$ref"); ok {
				const prefix = "#/components/responses/"
				if !strings.HasPrefix(ref.Value, prefix) {
					t.Fatalf("%s %s response %s has unsupported ref %q", operation.method, operation.path, code, ref.Value)
				}
				response = nodeAt(t, componentResponses, strings.TrimPrefix(ref.Value, prefix))
			}
			headers := nodeAt(t, response, "headers")
			if code == "405" {
				_ = nodeAt(t, headers, "Allow")
			}
			cacheControl := nodeAt(t, headers, "Cache-Control")
			if ref, ok := optionalMappingValue(cacheControl, "$ref"); ok {
				const prefix = "#/components/headers/"
				if !strings.HasPrefix(ref.Value, prefix) {
					t.Fatalf("%s %s response %s has unsupported Cache-Control ref %q", operation.method, operation.path, code, ref.Value)
				}
				cacheControl = nodeAt(t, componentHeaders, strings.TrimPrefix(ref.Value, prefix))
			}
			if got := scalarAt(t, nodeAt(t, cacheControl, "schema"), "const"); got != "no-store" {
				t.Fatalf("%s %s response %s Cache-Control const=%q, want no-store", operation.method, operation.path, code, got)
			}
			if code == "401" {
				challenge := nodeAt(t, headers, "WWW-Authenticate")
				if ref, ok := optionalMappingValue(challenge, "$ref"); ok {
					const prefix = "#/components/headers/"
					if !strings.HasPrefix(ref.Value, prefix) {
						t.Fatalf("%s %s response 401 has unsupported WWW-Authenticate ref %q", operation.method, operation.path, ref.Value)
					}
					challenge = nodeAt(t, componentHeaders, strings.TrimPrefix(ref.Value, prefix))
				}
				if got := scalarAt(t, challenge, "example"); got != "Bearer" {
					t.Fatalf("%s %s response 401 challenge example=%q, want Bearer", operation.method, operation.path, got)
				}
			}
		}
	}
}

func nodeAt(t *testing.T, node *yaml.Node, key string) *yaml.Node {
	t.Helper()
	if node.Kind != yaml.MappingNode {
		t.Fatalf("expected mapping while reading %q", key)
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	t.Fatalf("missing required OpenAPI key %q", key)
	return nil
}

func scalarAt(t *testing.T, node *yaml.Node, key string) string {
	t.Helper()
	value := nodeAt(t, node, key)
	if value.Kind != yaml.ScalarNode || strings.TrimSpace(value.Value) == "" {
		t.Fatalf("OpenAPI key %q must be a non-empty scalar", key)
	}
	return value.Value
}

func hasMappingKey(node *yaml.Node, key string) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

func assertSchemaPropertyKeys(t *testing.T, schemas *yaml.Node, schemaName string, expected ...string) {
	t.Helper()
	properties := nodeAt(t, nodeAt(t, schemas, schemaName), "properties")
	if properties.Kind != yaml.MappingNode || len(properties.Content) != len(expected)*2 {
		t.Fatalf("OpenAPI schema %s property keys=%v, want exactly %v", schemaName, properties.Content, expected)
	}
	seen := make(map[string]struct{}, len(expected))
	for i := 0; i < len(properties.Content); i += 2 {
		key := properties.Content[i].Value
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("OpenAPI schema %s repeats property %q", schemaName, key)
		}
		seen[key] = struct{}{}
	}
	for _, key := range expected {
		if _, ok := seen[key]; !ok {
			t.Fatalf("OpenAPI schema %s missing expected property %q", schemaName, key)
		}
	}
}

func assertSchemaRequiredKeys(t *testing.T, schemas *yaml.Node, schemaName string, expected ...string) {
	t.Helper()
	required := nodeAt(t, nodeAt(t, schemas, schemaName), "required")
	if required.Kind != yaml.SequenceNode || len(required.Content) != len(expected) {
		t.Fatalf("OpenAPI schema %s required=%v, want exactly %v", schemaName, required.Content, expected)
	}
	seen := make(map[string]struct{}, len(expected))
	for _, item := range required.Content {
		key := item.Value
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("OpenAPI schema %s repeats required property %q", schemaName, key)
		}
		seen[key] = struct{}{}
	}
	for _, key := range expected {
		if _, ok := seen[key]; !ok {
			t.Fatalf("OpenAPI schema %s missing required property %q", schemaName, key)
		}
	}
}

func optionalMappingValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	if node.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1], true
		}
	}
	return nil, false
}
