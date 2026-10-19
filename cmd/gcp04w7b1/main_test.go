package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w7b1"
)

func TestDefaultPlanMakesNoCloudCallAndIsOff(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run(nil, &out, &stderr); code != 0 {
		t.Fatalf("plan exit=%d stderr=%s", code, stderr.String())
	}
	var p gcp04w7b1.Plan
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Mode != "default_off_preview" || p.Mutations != "none" || p.UserRequest.Name != gcp04w7b1.Bootstrap || len(p.UserRequest.Roles) != 1 || p.UserRequest.Roles[0] != gcp04w7b1.AdminRole {
		t.Fatalf("unsafe offline plan %#v", p)
	}
}

func TestCreateAndCleanupRequireExactStageSwitchBeforeADC(t *testing.T) {
	for _, args := range [][]string{{"--mode=create"}, {"--mode=cleanup"}, {"--mode=probe"}, {"--mode=unknown"}} {
		var out, stderr bytes.Buffer
		code := run(args, &out, &stderr)
		if code != 2 || strings.Contains(stderr.String(), "Cloud API client") {
			t.Fatalf("mode reached cloud setup without gates: args=%v code=%d err=%s", args, code, stderr.String())
		}
	}
}

func TestPlanRejectsReleaseAndCredentialFlags(t *testing.T) {
	for _, args := range [][]string{{"--mode=plan", "--release-file=x"}, {"--mode=plan", "--password-file=x"}, {"--mode=plan", "--execute-b1-create"}} {
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code != 2 {
			t.Fatalf("plan mode accepted operational flag %v", args)
		}
	}
}
