package documentcodec

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const boundaryOwner = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const boundarySpace = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const boundaryCase = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
const boundaryFile = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
const boundaryOther = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"

func TestCodecRejectsEveryCrossScopeDimension(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	base, err := SpaceScope(boundaryOwner, boundarySpace, boundaryCase, boundaryFile)
	if err != nil {
		t.Fatal(err)
	}
	scopes := map[string]Scope{}
	scopes["owner"], _ = SpaceScope(boundaryOther, boundarySpace, boundaryCase, boundaryFile)
	scopes["space"], _ = SpaceScope(boundaryOwner, boundaryOther, boundaryCase, boundaryFile)
	scopes["case"], _ = SpaceScope(boundaryOwner, boundarySpace, boundaryOther, boundaryFile)
	scopes["file"], _ = SpaceScope(boundaryOwner, boundarySpace, boundaryCase, boundaryOther)
	scopes["purpose"], _ = IdentityScope(boundaryOwner, boundaryCase, boundaryFile)
	sealed, err := Seal(key, base, []byte("synthetic-private-document"))
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range scopes {
		t.Run(name, func(t *testing.T) {
			out, err := Open(key, s, sealed)
			if !errors.Is(err, ErrCorrupt) || out != nil {
				t.Fatalf("cross-scope accepted: bytes=%d err=%v", len(out), err)
			}
		})
	}
}

func TestCodecCanonicalScopesOnly(t *testing.T) {
	for _, id := range []string{"", "00000000-0000-0000-0000-000000000000", strings.ToUpper(boundaryOwner), "../escape", " " + boundaryOwner, boundaryOwner + "/child"} {
		if _, err := IdentityScope(id, boundaryCase, boundaryFile); !errors.Is(err, ErrInvalid) {
			t.Errorf("invalid owner accepted")
		}
		if _, err := SpaceScope(boundaryOwner, id, boundaryCase, boundaryFile); !errors.Is(err, ErrInvalid) {
			t.Errorf("invalid space accepted")
		}
	}
	base, _ := SpaceScope(boundaryOwner, boundarySpace, boundaryCase, boundaryFile)
	aad, err := KeyForScope(base)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := KeyForScope(ScopeFromAAD(aad)); err != nil || got != aad {
		t.Fatal("canonical AAD failed roundtrip")
	}
	for _, bad := range []string{"../" + aad, "/" + aad, aad + "/", strings.Replace(aad, "/spaces/", "//spaces/", 1), strings.Replace(aad, ".bin", ".bin.bin", 1), "https://invalid.example/" + aad} {
		if _, err := KeyForScope(ScopeFromAAD(bad)); !errors.Is(err, ErrInvalid) {
			t.Error("noncanonical AAD accepted")
		}
	}
	key := bytes.Repeat([]byte{1}, 32)
	if out, err := Seal(key, Scope{}, []byte("x")); !errors.Is(err, ErrInvalid) || out != nil {
		t.Fatal("zero scope seal accepted")
	}
	if out, err := Open(key, Scope{}, []byte("x")); !errors.Is(err, ErrInvalid) || out != nil {
		t.Fatal("zero scope open accepted")
	}
}

func TestCodecExactLimitsKeysAndNonce(t *testing.T) {
	scope, _ := IdentityScope(boundaryOwner, boundaryCase, boundaryFile)
	key := bytes.Repeat([]byte{3}, 32)
	for _, n := range []int{0, MaxDocumentBytes + 1} {
		if out, err := Seal(key, scope, make([]byte, n)); !errors.Is(err, ErrInvalid) || out != nil {
			t.Fatalf("seal size %d accepted", n)
		}
	}
	plain := bytes.Repeat([]byte{5}, MaxDocumentBytes)
	first, err := Seal(key, scope, plain)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Open(key, scope, first)
	if err != nil || !bytes.Equal(out, plain) {
		t.Fatal("exact maximum failed")
	}
	second, err := Seal(key, scope, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(first, []byte("ESPACIGODOC1")) || bytes.Equal(first, second) {
		t.Fatal("legacy magic or nonce independence broken")
	}
	if out, err := Open(bytes.Repeat([]byte{4}, 32), scope, first); !errors.Is(err, ErrCorrupt) || out != nil {
		t.Fatal("foreign key accepted")
	}
	for _, n := range []int{0, 16, 24, 31, 33} {
		if out, err := Open(make([]byte, n), scope, first); !errors.Is(err, ErrInvalid) || out != nil {
			t.Fatalf("invalid key length %d accepted", n)
		}
	}
	for _, data := range [][]byte{nil, {}, []byte("ESPACIGODOC1"), make([]byte, MaxDocumentBytes+128)} {
		if out, err := Open(key, scope, data); !errors.Is(err, ErrCorrupt) || out != nil {
			t.Fatal("invalid envelope accepted")
		}
	}
}
