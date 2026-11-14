package credentials_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
)

const failEntropyReaderEnv = "ESPACIGO_CREDENTIALS_TEST_FAIL_ENTROPY"

type failingEntropyReader struct {
	partialReturned bool
}

func (r *failingEntropyReader) Read(buffer []byte) (int, error) {
	if !r.partialReturned {
		r.partialReturned = true
		buffer[0] = 0xa5
		return 1, nil
	}
	return 0, errors.New("synthetic entropy failure after partial read")
}

func TestGeneratorReturnsErrorsWhenEntropyReaderFails(t *testing.T) {
	if os.Getenv(failEntropyReaderEnv) == "1" {
		rand.Reader = &failingEntropyReader{}
		generator := credentials.Generator{}
		if _, err := generator.ID(); err == nil {
			fmt.Fprintln(os.Stderr, "ID returned no entropy error")
			os.Exit(2)
		}
		rand.Reader = &failingEntropyReader{}
		if _, err := generator.Token(); err == nil {
			fmt.Fprintln(os.Stderr, "Token returned no entropy error")
			os.Exit(3)
		}
		fmt.Fprintln(os.Stdout, "synthetic entropy errors returned")
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestGeneratorReturnsErrorsWhenEntropyReaderFails$")
	cmd.Env = append(os.Environ(), failEntropyReaderEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "synthetic entropy errors returned") {
		t.Fatalf("entropy failure subprocess: err=%v output=%s", err, output)
	}
}

type chunkedEntropyReader struct {
	bytesRead int
}

func (r *chunkedEntropyReader) Read(buffer []byte) (int, error) {
	n := len(buffer)
	if n > 3 {
		n = 3
	}
	for i := 0; i < n; i++ {
		buffer[i] = 0xc7
	}
	r.bytesRead += n
	return n, nil
}

// This test replaces the process-wide crypto/rand.Reader; keep it non-parallel.
func TestGeneratorCompletesShortEntropyReads(t *testing.T) {
	previousReader := rand.Reader
	reader := &chunkedEntropyReader{}
	rand.Reader = reader
	t.Cleanup(func() { rand.Reader = previousReader })

	generator := credentials.Generator{}
	id, err := generator.ID()
	if err != nil || id == "" {
		t.Fatalf("ID=%q err=%v; want a generated identifier", id, err)
	}
	token, err := generator.Token()
	if err != nil || token == "" {
		t.Fatalf("token empty=%t err=%v; want a generated token", token == "", err)
	}
	if reader.bytesRead != 48 {
		t.Fatalf("entropy bytes read=%d, want 16 for ID plus 32 for token", reader.bytesRead)
	}
}
