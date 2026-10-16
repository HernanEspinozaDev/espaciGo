package dbbootstrap

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestObserverNDJSONChunksLargeReportsAndVerifiesReconstruction(t *testing.T) {
	report := validObserverReport()
	for i := 0; i < 5000; i++ {
		report.RelationOwners = append(report.RelationOwners, OwnedObjectObservation{
			Schema: "public", Name: fmt.Sprintf("relation_%04d_%s", i, strings.Repeat("x", 48)),
			Kind: "i", Owner: "espacigo_scratch_migrator",
		})
	}
	original, _, err := canonicalObserverFields(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(original) <= 256*1024 {
		t.Fatalf("fixture must exceed Cloud Logging's 256 KiB single-entry limit, got %d bytes", len(original))
	}

	var output bytes.Buffer
	if err := WriteObserverNDJSON(&output, report); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("large report was not split into NDJSON records: %d", len(lines))
	}
	for i, line := range lines {
		if len(line) > observerMaxRecordBytes || len(line) >= 256*1024 {
			t.Fatalf("record %d exceeds bounded size: %d", i, len(line))
		}
	}
	reconstructed, err := ReadObserverNDJSON(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatalf("reconstruct complete observer report: %v", err)
	}
	if !reflect.DeepEqual(report, reconstructed) {
		t.Fatal("reconstructed observer report differs from original")
	}

	incomplete := strings.Join(lines[:len(lines)-1], "\n") + "\n"
	if _, err := ReadObserverNDJSON(strings.NewReader(incomplete)); err == nil {
		t.Fatal("incomplete observer NDJSON was accepted")
	}

	corrupt := strings.Replace(output.String(), "relation_0000_", "relation_xxxx_", 1)
	if corrupt == output.String() {
		t.Fatal("corruption fixture was not changed")
	}
	if _, err := ReadObserverNDJSON(strings.NewReader(corrupt)); err == nil {
		t.Fatal("observer NDJSON with a mismatched report hash was accepted")
	}
}

func validObserverReport() ScratchObservation {
	return ScratchObservation{
		ServerVersionNum: 180006, ServerVersion: "18.6", PostGISFullVersion: "POSTGIS=3.6.4",
		Extensions:            []ExtensionObservation{{Name: "btree_gist", Version: "1.8"}, {Name: "postgis", Version: "3.6.4"}, {Name: "unaccent", Version: "1.1"}},
		MembershipExpectation: RoleMembershipLocalPostgres18,
		RoleInventoryPass:     true, DatabaseACLPass: true, RuntimePermissionsPass: true,
		RuntimeChecks: passingRuntimeChecks(),
		Spatial: SpatialObservation{
			GeometryValid: true, GeometryIntersects: true, GeographyDistanceMeters: 111000,
			ExpectedIndex: "public.reserva_ensayo_local_ubicacion_gist", ActualIndex: "reserva_ensayo_local_ubicacion_gist",
			IndexAccessMethod: "gist", IndexValid: true, IndexReady: true,
			IndexDefinition: "CREATE INDEX reserva_ensayo_local_ubicacion_gist ON public.reserva_ensayo_local_ubicacion_sintetica USING gist (punto)",
		},
	}
}
