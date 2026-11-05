package dbbootstrap

import (
	"encoding/json"
	"testing"
)

func TestSemanticACLSnapshotDecodeRequiresExactCompleteShape(t *testing.T) {
	valid := `{"database":"espacigo_f5_gate_20261101","server_version_num":180006,"acl_inventory":[],"owner_inventory":[],"owners":[],"acls":[],"initial_privileges":[],"acl_states":[],"default_acl_objects":[],"default_acls":[]}`
	if _, err := decodeRuntimeACLSemanticSnapshot([]byte(valid)); err != nil {
		t.Fatal("valid complete snapshot rejected", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(valid), &fields); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"unknown field":    valid[:len(valid)-1] + `,"unverified_acl_class":[]}`,
		"trailing JSON":    valid + ` {}`,
		"trailing garbage": valid + ` invalid`,
		"null root":        `null`,
		"array root":       `[]`,
		"duplicate field":  valid[:len(valid)-1] + `,"default_acls":[]}`,
	}
	for name := range fields {
		missing := map[string]json.RawMessage{}
		null := map[string]json.RawMessage{}
		for k, v := range fields {
			null[k] = v
			if k != name {
				missing[k] = v
			}
		}
		null[name] = json.RawMessage(`null`)
		a, err := json.Marshal(missing)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(null)
		if err != nil {
			t.Fatal(err)
		}
		cases["missing "+name] = string(a)
		cases["null "+name] = string(b)

	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeRuntimeACLSemanticSnapshot([]byte(payload)); err == nil {
				t.Fatal("incomplete/ambiguous snapshot accepted")
			}
		})
	}
}
