package main

import (
	"encoding/json"
	"flag"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestValidateV84AuthorityInvocationBoundaries(t *testing.T) {
	validURL := "postgres://postgres@localhost:5432/espacigo_dms_stage_20261020?sslmode=disable"
	settings := connectionSettings{
		databaseURL: validURL, passwordFile: v84AuthorityPassFile, mode: "cloudsql",
		instance: v84AuthorityInstance, privateIP: true,
	}
	env := func(string) string { return "" }
	sha := strings.Repeat("a", 40)
	cases := []struct {
		name, mode, baseline, confirmation string
		settings                           connectionSettings
		source                             string
		wantErr                            bool
	}{
		{"inspect", "inspect", "", v84AuthorityDatabase, settings, sha, false},
		{"prepare", "prepare", "", v84AuthorityDatabase, settings, sha, false},
		{"finalize", "finalize", strings.Repeat("b", 64), v84AuthorityDatabase, settings, sha, false},
		{"bad mode", "apply", "", v84AuthorityDatabase, settings, sha, true},
		{"missing confirmation", "inspect", "", "", settings, sha, true},
		{"bad release sha", "inspect", "", v84AuthorityDatabase, settings, "short", true},
		{"finalize missing baseline", "finalize", "", v84AuthorityDatabase, settings, sha, true},
		{"baseline on inspect", "inspect", strings.Repeat("b", 64), v84AuthorityDatabase, settings, sha, true},
		{"non-private", "inspect", "", v84AuthorityDatabase, func() connectionSettings { s := settings; s.privateIP = false; return s }(), sha, true},
		{"set role", "inspect", "", v84AuthorityDatabase, func() connectionSettings { s := settings; s.setRole = "postgres"; return s }(), sha, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateV84AuthorityInvocation(tc.mode, tc.baseline, tc.confirmation, tc.settings, tc.source, env)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateV84AuthorityRejectsURLCredentialAndOptions(t *testing.T) {
	base := "postgres://postgres@localhost:5432/espacigo_dms_stage_20261020?sslmode=disable"
	for _, raw := range []string{
		"postgres://postgres:secret@localhost:5432/espacigo_dms_stage_20261020?sslmode=disable",
		base + "&options=-crole=postgres",
	} {
		t.Run(raw, func(t *testing.T) {
			settings := connectionSettings{databaseURL: raw, passwordFile: v84AuthorityPassFile, mode: "cloudsql", instance: v84AuthorityInstance, privateIP: true}
			err := validateV84AuthorityInvocation("inspect", "", v84AuthorityDatabase, settings, strings.Repeat("a", 40), func(string) string { return "" })
			if err == nil {
				t.Fatal("expected URL to be rejected")
			}
		})
	}
}

func TestV84AuthorityURLHasPinnedUserDatabase(t *testing.T) {
	u, err := url.Parse("postgres://postgres@localhost:5432/espacigo_dms_stage_20261020?sslmode=disable")
	if err != nil || u.User.Username() != "postgres" || strings.TrimPrefix(u.Path, "/") != v84AuthorityDatabase {
		t.Fatal("test URL malformed")
	}
}

func TestV84AuthorityCLIRejectsBeforeConnection(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-v84-authority-baseline", strings.Repeat("a", 64)},
		{"-confirm-v84-authority", v84AuthorityDatabase},
		{"-v84-authority-mode="},
		{"-v84-authority-mode=inspect", "-plan-only=false", "-confirm-v84-authority", v84AuthorityDatabase},
		{"-v84-authority-mode=inspect", "-confirm-v84-authority", v84AuthorityDatabase, "positional"},
	} {
		encoded, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(binary, "-test.run=^TestV84AuthorityCLIHelper$")
		for _, item := range os.Environ() {
			if strings.HasPrefix(item, "PG") || strings.HasPrefix(item, "DATABASE_") || strings.HasPrefix(item, "V84_CLI_") {
				continue
			}
			command.Env = append(command.Env, item)
		}
		command.Env = append(command.Env, "V84_CLI_HELPER=1", "V84_CLI_ARGS="+string(encoded),
			"DATABASE_URL=postgres://postgres@"+listener.Addr().String()+"/"+v84AuthorityDatabase+"?sslmode=disable")
		output, err := command.CombinedOutput()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
			t.Fatalf("expected preconnection exit2 for %v: %v: %s", args, err, output)
		}
	}
	if err := listener.SetDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	connection, err := listener.Accept()
	if err == nil {
		connection.Close()
		t.Fatal("V84 CLI connected before rejecting conflicting flags")
	}
	if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatal(err)
	}
}

func TestV84AuthorityCLIHelper(t *testing.T) {
	if os.Getenv("V84_CLI_HELPER") != "1" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("V84_CLI_ARGS")), &args); err != nil {
		t.Fatal(err)
	}
	flag.CommandLine = flag.NewFlagSet("v84-authority-helper", flag.ExitOnError)
	os.Args = append([]string{"v84-authority-helper"}, args...)
	releaseSourceSHA = strings.Repeat("a", 40)
	main()
	os.Exit(0)
}
