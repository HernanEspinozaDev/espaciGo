package identitydeclaration

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// This is a bounded SQL source contract, not a database/concurrency test.
// The same production predicate is exercised in the native rollback rehearsal.
func TestSessionReadySQLKeepsTerminalAndBoundSessionsImmutable(t *testing.T) {
	data, err := os.ReadFile("provider_attempt.go")
	if err != nil {
		t.Fatal(err)
	}
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "provider_attempt.go", data, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "MarkAttemptSessionReady" {
			body = string(data[fs.Position(fn.Pos()).Offset:fs.Position(fn.End()).Offset])
		}
	}
	if body == "" {
		t.Fatal("session ready method missing")
	}
	for _, predicate := range []string{"AND status = 'CREATED'", "AND declaration_id IS NOT NULL", "AND provider_session_id IS NULL", "AND retention_at > $3", "GREATEST(updated_at, $3)"} {
		if !strings.Contains(body, predicate) {
			t.Errorf("session ready source contract missing %q", predicate)
		}
	}
}
