package clamav

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

// TestLocalScannerSyntheticFiles is opt-in and must only read the authorized
// synthetic clean/EICAR samples and the explicitly selected local scanner.
func TestLocalScannerSyntheticFiles(t *testing.T) {
	address := os.Getenv("ESPACIGO_CLAMAV_TEST_ADDRESS")
	sampleDir := os.Getenv("ESPACIGO_CLAMAV_TEST_SAMPLE_DIR")
	if address == "" || sampleDir == "" {
		t.Skip("set ESPACIGO_CLAMAV_TEST_ADDRESS and ESPACIGO_CLAMAV_TEST_SAMPLE_DIR for the authorized local scanner")
	}
	a, err := New(Config{
		Address: address, Timeout: 5 * time.Second, MaxBytes: 10 << 20,
		MaxSignatureAge: 48 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		file  string
		state dv.FileState
	}{
		{name: "clean", file: "synthetic-clean.txt", state: dv.FileClean},
		{name: "EICAR", file: "eicar.com.txt", state: dv.FileBlocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(sampleDir, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			state, err := a.Scan(context.Background(), content)
			if err != nil || state != tc.state {
				t.Fatalf("Scan(%s) = %q, %v; want %q and no error", tc.file, state, err, tc.state)
			}
			t.Logf("synthetic sample %s -> %s", tc.file, state)
		})
	}
}
