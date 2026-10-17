package gcp03restore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestManifestBindsRunAndExactArtifacts(t *testing.T) {
	rawTOC := []byte("; pg_restore list\n1; 1259 10 TABLE public example postgres\n2; 0 0 EXTENSION - postgis\n")
	allow := []byte("; reviewed allowlist\n1; 1259 10 TABLE public example postgres\n")
	dump := []byte("synthetic custom-format dump")
	manifest := Manifest{
		Format: ManifestFormat, RunID: "restore-2026", Database: "espacigo_scratch",
		DumpObject: "runs/restore-2026/synthetic-v1-v61.dump", DumpBytes: int64(len(dump)), DumpSHA256: sum(dump),
		TOCObject: "runs/restore-2026/dump.toc", TOCSHA256: sum(rawTOC),
		AllowlistObject: "runs/restore-2026/allowlist.toc", AllowlistSHA256: sum(allow),
		MigrationCount: 61, MigrationSHA256: sum([]byte("immutable V1-V61 inventory")),
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	m, err := DecodeManifest(data, sum(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyDump(dump, m); err != nil {
		t.Fatal(err)
	}
	if err := VerifyTOC(rawTOC, allow, m); err != nil {
		t.Fatal(err)
	}
}

func TestManifestRejectsUnreviewedBytesAndOutOfRunObjects(t *testing.T) {
	manifest := Manifest{Format: ManifestFormat, RunID: "restore-2026", Database: "espacigo_scratch", DumpObject: "runs/other-run/synthetic-v1-v61.dump", DumpBytes: 1, DumpSHA256: sum([]byte("x")), TOCObject: "runs/restore-2026/dump.toc", TOCSHA256: sum([]byte("toc")), AllowlistObject: "runs/restore-2026/allowlist.toc", AllowlistSHA256: sum([]byte("entry")), MigrationCount: 61, MigrationSHA256: sum([]byte("migrations"))}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeManifest(data, sum(data)); err == nil {
		t.Fatal("accepted object outside run prefix")
	}
	manifest.DumpObject = "runs/restore-2026/synthetic-v1-v61.dump"
	data, _ = json.Marshal(manifest)
	m, err := DecodeManifest(data, sum(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyDump([]byte("y"), m); err == nil {
		t.Fatal("accepted dump whose bytes differ from pinned hash")
	}
}

func TestAllowlistMustBeSubsetOfExactDumpTOC(t *testing.T) {
	raw := []byte("1; 1259 10 TABLE public example postgres\n")
	bad := []byte("9; 1259 99 TABLE public other postgres\n")
	m := Manifest{TOCSHA256: sum(raw), AllowlistSHA256: sum(bad)}
	if err := VerifyTOC(raw, bad, m); err == nil {
		t.Fatal("accepted TOC entry absent from dump")
	}
}

func TestSameTOCEntriesIgnoresOnlyEnvironmentDependentComments(t *testing.T) {
	sealed := []byte("; Archive created at 2026-10-16 17:53:07 -03\n; dbname: fixture\n1; 0 10 TABLE public sample owner\n")
	generated := []byte("; Archive created at 2026-10-16 16:53:07 UTC\n; dbname: fixture\n1; 0 10 TABLE public sample owner\n")
	if !SameTOCEntries(sealed, generated) {
		t.Fatal("timezone-dependent header prevented matching identical TOC entries")
	}
	changed := []byte("; Archive created at another time\n; dbname: fixture\n1; 0 10 TABLE public changed owner\n")
	if SameTOCEntries(sealed, changed) {
		t.Fatal("accepted a changed TOC entry")
	}
	if SameTOCEntries(sealed, []byte("; header only\n")) {
		t.Fatal("accepted a missing TOC entry")
	}
}

func TestAllowlistCannotRestorePreinstalledExtensionObjects(t *testing.T) {
	raw := []byte("2; 3079 21312 EXTENSION - postgis\n4929; 0 21631 TABLE DATA public spatial_ref_sys postgres\n")
	for _, selected := range []string{
		"2; 3079 21312 EXTENSION - postgis\n",
		"4929; 0 21631 TABLE DATA public spatial_ref_sys postgres\n",
	} {
		allow := []byte(selected)
		m := Manifest{TOCSHA256: sum(raw), AllowlistSHA256: sum(allow)}
		if err := VerifyTOC(raw, allow, m); err == nil {
			t.Fatalf("accepted preinstalled extension TOC entry %q", selected)
		}
	}
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
