package inspection

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunPinnedExecutableUsesOpenedInodeAfterPathReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool")
	original := filepath.Join(dir, "original")
	replacement := filepath.Join(dir, "replacement")
	for name, body := range map[string]string{
		original:    "#!/bin/sh\nprintf original\n",
		replacement: "#!/bin/sh\nprintf replacement\n",
	} {
		if err := os.WriteFile(name, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(original, path); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	err = runPinnedExecutable(context.Background(), f, nil, nil, nil, &out, &stderr)
	if err != nil {
		t.Fatalf("run pinned executable: %v", err)
	}
	if out.String() != "original" || stderr.Len() != 0 {
		t.Fatalf("executed replacement pathname or wrote stderr: stdout=%q stderr=%q", out.String(), stderr.String())
	}
}
