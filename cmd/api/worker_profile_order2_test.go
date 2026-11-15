package main

import "testing"

func TestLaunchSpacesOrder2ProfileSkipsAllRegisteredWorkers(t *testing.T) {
	launched := 0
	workers := []namedWorker{{"alpha", func() { launched++ }}, {"beta", func() { launched++ }}}
	if err := launchWorkers(runtimeProfileName("local-spaces-order2"), workers); err != nil {
		t.Fatal(err)
	}
	if launched != 0 {
		t.Fatalf("launched %d workers", launched)
	}
}
