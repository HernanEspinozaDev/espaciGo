package main

import "testing"

func TestActionForCommandUsesSupportedCLICommands(t *testing.T) {
	for command, expected := range map[string]string{
		"bootstrap": "bootstrap",
		"recover":   "recover",
	} {
		actual, ok := actionForCommand(command)
		if !ok || actual != expected {
			t.Fatalf("actionForCommand(%q) = %q, %v; want %q, true", command, actual, ok, expected)
		}
	}
	if _, ok := actionForCommand("recuperacion"); ok {
		t.Fatal("persisted action name is not a supported CLI command")
	}
}
