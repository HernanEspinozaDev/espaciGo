package main

import (
	"flag"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCloudOwnerInvocationBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*connectionSettings, map[string]string)
	}{
		{"wrong host", func(s *connectionSettings, _ map[string]string) {
			s.databaseURL = strings.Replace(s.databaseURL, "127.0.0.1", "localhost", 1)
		}},
		{"wrong port", func(s *connectionSettings, _ map[string]string) {
			s.databaseURL = strings.Replace(s.databaseURL, ":15432", ":15434", 1)
		}},
		{"wrong database", func(s *connectionSettings, _ map[string]string) {
			s.databaseURL = strings.Replace(s.databaseURL, cloudOwnerConfirmation, "espacigo_f5_gate_20261101", 1)
		}},
		{"wrong login", func(s *connectionSettings, _ map[string]string) {
			s.databaseURL = strings.Replace(s.databaseURL, "postgres@", "espacigo_api_login@", 1)
		}},
		{"embedded password", func(s *connectionSettings, _ map[string]string) {
			s.databaseURL = strings.Replace(s.databaseURL, "postgres@", "postgres:synthetic@", 1)
		}},
		{"no tls", func(s *connectionSettings, _ map[string]string) {
			s.databaseURL = strings.Replace(s.databaseURL, "sslmode=require", "sslmode=disable", 1)
		}},
		{"set role", func(s *connectionSettings, _ map[string]string) { s.setRole = "espacigo_runtime" }},
		{"connector mode", func(s *connectionSettings, _ map[string]string) { s.mode = "cloudsql" }},
		{"relative passfile", func(_ *connectionSettings, e map[string]string) { e["PGPASSFILE"] = "relative.pgpass" }},
		{"missing passfile", func(_ *connectionSettings, e map[string]string) { e["PGPASSFILE"] += ".absent" }},
		{"public passfile", func(_ *connectionSettings, e map[string]string) {
			if err := os.Chmod(e["PGPASSFILE"], 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"public directory", func(_ *connectionSettings, e map[string]string) {
			if err := os.Chmod(filepath.Dir(e["PGPASSFILE"]), 0755); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings, env := cloudOwnerTestSettings(t)
			tc.mutate(&settings, env)
			if err := validateCloudOwnerInvocation(true, false, false, false, cloudOwnerConfirmation, settings, func(k string) string { return env[k] }); err == nil {
				t.Fatal("accepted unsafe invocation")
			}
		})
	}
	settings, env := cloudOwnerTestSettings(t)
	getenv := func(k string) string { return env[k] }
	if err := validateCloudOwnerInvocation(true, false, false, false, cloudOwnerConfirmation, settings, getenv); err != nil {
		t.Fatal(err)
	}
	for _, mode := range [][3]bool{{true, false, false}, {false, true, false}, {false, false, true}} {
		if err := validateCloudOwnerInvocation(true, mode[0], mode[1], mode[2], cloudOwnerConfirmation, settings, getenv); err == nil {
			t.Fatal("accepted conflicting mode")
		}
	}
	if err := validateCloudOwnerInvocation(true, false, false, false, "wrong", settings, getenv); err == nil {
		t.Fatal("accepted wrong confirmation")
	}
}

func TestCloudOwnerCLIRejectsBeforeConnection(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, env := cloudOwnerTestSettings(t)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-test.run=^TestCloudOwnerCLIHelper$")
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "PG") || strings.HasPrefix(item, "DATABASE_") || strings.HasPrefix(item, "CLOUD_OWNER_TEST_") {
			continue
		}
		command.Env = append(command.Env, item)
	}
	command.Env = append(command.Env, "CLOUD_OWNER_TEST_HELPER=1", "PGPASSFILE="+env["PGPASSFILE"], "DATABASE_URL=postgresql://postgres@"+listener.Addr().String()+"/"+cloudOwnerConfirmation+"?sslmode=require")
	output, err := command.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
		t.Fatalf("expected validation exit 2; got %v: %s", err, output)
	}
	if !strings.Contains(string(output), "exact loopback staging target") {
		t.Fatalf("wrong validation failure: %s", output)
	}
	if err := listener.SetDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	connection, err := listener.Accept()
	if err == nil {
		connection.Close()
		t.Fatal("CLI connected before rejecting the invalid target")
	}
	if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatal(err)
	}
}

func TestCloudOwnerCLIHelper(t *testing.T) {
	if os.Getenv("CLOUD_OWNER_TEST_HELPER") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("cloud-owner-helper", flag.ExitOnError)
	os.Args = []string{"cloud-owner-helper", "-cloud-owner-migration", "-confirm-cloud-owner-pre81", cloudOwnerConfirmation}
	main()
	os.Exit(0)
}
