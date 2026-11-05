package dbbootstrap

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

//go:embed testdata/runtime_acl_reference.json.gz
var runtimeACLReferenceFS embed.FS

//go:embed semantic_acl_snapshot_query.sql
var runtimeACLSemanticQuery string

type runtimeACLSemanticSnapshot struct {
	Database          string            `json:"database"`
	ServerVersion     int               `json:"server_version_num"`
	ACLInventory      []string          `json:"acl_inventory"`
	OwnerInventory    []string          `json:"owner_inventory"`
	Owners            []json.RawMessage `json:"owners"`
	ACLs              []json.RawMessage `json:"acls"`
	InitialPrivileges []json.RawMessage `json:"initial_privileges"`
	ACLStates         []json.RawMessage `json:"acl_states"`
	DefaultACLObjects []json.RawMessage `json:"default_acl_objects"`
	DefaultACLs       []json.RawMessage `json:"default_acls"`
}

// Decode the exact catalog contract, including empty-but-present arrays. Never
// discard unknown/duplicate root fields or accept null/absent inventories.
func decodeRuntimeACLSemanticSnapshot(payload []byte) (runtimeACLSemanticSnapshot, error) {
	var snapshot runtimeACLSemanticSnapshot
	decoder := json.NewDecoder(bytes.NewReader(payload))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return snapshot, fmt.Errorf("semantic ACL snapshot must be an object")
	}
	fields := map[string]any{
		"database": &snapshot.Database, "server_version_num": &snapshot.ServerVersion,
		"acl_inventory": &snapshot.ACLInventory, "owner_inventory": &snapshot.OwnerInventory,
		"owners": &snapshot.Owners, "acls": &snapshot.ACLs,
		"initial_privileges": &snapshot.InitialPrivileges, "acl_states": &snapshot.ACLStates,
		"default_acl_objects": &snapshot.DefaultACLObjects, "default_acls": &snapshot.DefaultACLs,
	}
	seen := make(map[string]bool, len(fields))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return snapshot, err
		}
		name, ok := token.(string)
		destination, known := fields[name]
		if !ok || !known || seen[name] {
			return snapshot, fmt.Errorf("unknown or duplicate semantic ACL field")
		}
		seen[name] = true
		if err := decoder.Decode(destination); err != nil {
			return snapshot, err
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return snapshot, fmt.Errorf("incomplete semantic ACL object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return snapshot, fmt.Errorf("trailing semantic ACL content")
	}
	if len(seen) != len(fields) || snapshot.Database == "" || snapshot.ServerVersion <= 0 ||
		snapshot.ACLInventory == nil || snapshot.OwnerInventory == nil || snapshot.Owners == nil ||
		snapshot.ACLs == nil || snapshot.InitialPrivileges == nil || snapshot.ACLStates == nil ||
		snapshot.DefaultACLObjects == nil || snapshot.DefaultACLs == nil {
		return snapshot, fmt.Errorf("missing or null required semantic ACL field")
	}
	return snapshot, nil
}

func loadRuntimeACLSemanticReference() (runtimeACLSemanticSnapshot, error) {
	var snapshot runtimeACLSemanticSnapshot
	data, err := runtimeACLReferenceFS.Open("testdata/runtime_acl_reference.json.gz")
	if err != nil {
		return snapshot, err
	}
	defer data.Close()
	reader, err := gzip.NewReader(data)
	if err != nil {
		return snapshot, err
	}
	defer reader.Close()
	payload, err := io.ReadAll(reader)
	if err != nil {
		return snapshot, err
	}
	// Pinned independent, schema-only PG18.6 reference; never learned from
	// the runtime target. No account rows or credentials are in this catalog.
	if fmt.Sprintf("%x", sha256.Sum256(payload)) != "2998f3291069050be0ef96f183d51a035a7b072110135355769f87f485829f4c" {
		return snapshot, fmt.Errorf("semantic ACL independent reference checksum mismatch")
	}
	snapshot, err = decodeRuntimeACLSemanticSnapshot(payload)
	if err != nil {
		return snapshot, fmt.Errorf("decode semantic ACL reference: %w", err)
	}
	if snapshot.Database != runtimeACLF5DatabaseName || snapshot.ServerVersion != runtimeACLF5BaselineServer || len(snapshot.ACLInventory) != 14 || len(snapshot.OwnerInventory) != 22 || len(snapshot.ACLs) != 21678 || len(snapshot.Owners) != 9201 || len(snapshot.InitialPrivileges) != 1513 || len(snapshot.ACLStates) != 12406 {
		return snapshot, fmt.Errorf("semantic ACL reference has unexpected identity or inventory")
	}
	return snapshot, nil
}

func canonicalRuntimeACLJSON(raw json.RawMessage) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	value = sortRuntimeACLValue(value)
	return json.Marshal(value)
}

func sortRuntimeACLValue(value any) any {
	switch v := value.(type) {
	case []any:
		keys := make([]struct {
			value any
			key   []byte
		}, len(v))
		for i := range v {
			v[i] = sortRuntimeACLValue(v[i])
			keys[i].value = v[i]
			keys[i].key, _ = json.Marshal(v[i])
		}
		sort.Slice(keys, func(i, j int) bool {
			return bytes.Compare(keys[i].key, keys[j].key) < 0
		})
		for i := range keys {
			v[i] = keys[i].value
		}
		return v
	case map[string]any:
		for k, x := range v {
			v[k] = sortRuntimeACLValue(x)
		}
		return v
	default:
		return value
	}
}

func compareRuntimeACLSemanticSnapshots(reference, observed runtimeACLSemanticSnapshot) error {
	if reference.Database != observed.Database || reference.ServerVersion != observed.ServerVersion {
		return fmt.Errorf("database/engine identity differs")
	}
	pairs := []struct {
		name string
		a, b any
	}{
		{"ACL inventory", reference.ACLInventory, observed.ACLInventory}, {"owner inventory", reference.OwnerInventory, observed.OwnerInventory},
		{"owners", reference.Owners, observed.Owners}, {"effective ACLs", reference.ACLs, observed.ACLs}, {"initial privileges", reference.InitialPrivileges, observed.InitialPrivileges},
		{"ACL raw states", reference.ACLStates, observed.ACLStates}, {"default ACL objects", reference.DefaultACLObjects, observed.DefaultACLObjects}, {"default ACLs", reference.DefaultACLs, observed.DefaultACLs},
	}
	for _, pair := range pairs {
		a, _ := json.Marshal(pair.a)
		b, _ := json.Marshal(pair.b)
		ca, err := canonicalRuntimeACLJSON(a)
		if err != nil {
			return err
		}
		cb, err := canonicalRuntimeACLJSON(b)
		if err != nil {
			return err
		}
		if !bytes.Equal(ca, cb) {
			return fmt.Errorf("%s differ", pair.name)
		}
	}
	return nil
}

func verifyRuntimeACLSemanticBaseline(ctx context.Context, conn ScratchPermissionDB) error {
	reference, err := loadRuntimeACLSemanticReference()
	if err != nil {
		return err
	}
	var raw []byte
	if err := conn.QueryRow(ctx, runtimeACLSemanticQuery).Scan(&raw); err != nil {
		return fmt.Errorf("could not read semantic ACL catalog: %w", err)
	}
	observed, err := decodeRuntimeACLSemanticSnapshot(raw)
	if err != nil {
		return fmt.Errorf("malformed semantic ACL catalog: %w", err)
	}
	if err := compareRuntimeACLSemanticSnapshots(reference, observed); err != nil {
		return fmt.Errorf("semantic ACL baseline mismatch: %w", err)
	}
	return nil
}
