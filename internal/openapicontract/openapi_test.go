package openapicontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var httpMethods = map[string]struct{}{
	"get": {}, "post": {}, "put": {}, "patch": {}, "delete": {}, "head": {}, "options": {}, "trace": {},
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
