// Command dataset-loader validates the private EspaciGo synthetic dataset before
// a future domain-backed import. It intentionally performs no database writes.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var categories = map[string]bool{
	"bodega": true, "estacionamiento": true, "local_flexible": true, "oficina": true,
	"parcela_eventos": true, "quincho": true, "sala_multiproposito": true, "stand": true,
}

type manifest struct {
	DatasetID string `json:"dataset_id"`
	Status    string `json:"status"`
	Counts    struct {
		Users     int `json:"users"`
		Hosts     int `json:"hosts"`
		Renters   int `json:"renters"`
		Admins    int `json:"admins"`
		Listings  int `json:"listings"`
		Public    int `json:"public_listings"`
		Draft     int `json:"draft_listings"`
		Hidden    int `json:"hidden_listings"`
		Scenarios int `json:"scenarios"`
	} `json:"counts"`
}

type userAssignment struct {
	Ref          string   `json:"user_ref"`
	Roles        []string `json:"roles"`
	ListingCount int      `json:"planned_listing_count"`
}

type listingAssignment struct {
	Ref        string `json:"listing_ref"`
	Owner      string `json:"owner_ref"`
	Category   string `json:"category_code"`
	Visibility string `json:"intended_visibility"`
}

type scenarioAssignment struct {
	Ref      string `json:"scenario_ref"`
	Host     string `json:"host_ref"`
	Renter   string `json:"renter_ref"`
	Listing  string `json:"listing_ref"`
	Category string `json:"category_code"`
	Code     string `json:"scenario_code"`
}

func main() {
	datasetDir := flag.String("dataset-dir", "", "private dataset directory (for example .local/datasets/espacigo-1000)")
	flag.Parse()
	if strings.TrimSpace(*datasetDir) == "" || flag.NArg() != 0 {
		fatal("usage: dataset-loader --dataset-dir PATH")
	}
	summary, err := validate(filepath.Clean(*datasetDir))
	if err != nil {
		fatal("dataset preflight failed: " + err.Error())
	}
	encoded, _ := json.Marshal(summary)
	fmt.Printf("preflight passed (no database writes): %s\n", encoded)
}

type summary struct {
	DatasetID string         `json:"dataset_id"`
	Assigned  map[string]int `json:"assigned"`
	Visible   map[string]int `json:"visibility"`
	Scenarios map[string]int `json:"scenarios"`
}

func validate(root string) (summary, error) {
	var result summary
	data, err := os.ReadFile(filepath.Join(root, "manifiesto.json"))
	if err != nil {
		return result, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return result, errors.New("invalid manifest")
	}
	if m.DatasetID != "espacigo-1000-v1" || m.Counts.Users != 1000 || m.Counts.Hosts != 300 || m.Counts.Renters != 690 || m.Counts.Admins != 10 || m.Counts.Listings != 1200 || m.Counts.Public != 960 || m.Counts.Draft != 120 || m.Counts.Hidden != 120 || m.Counts.Scenarios != 1200 {
		return result, errors.New("manifest does not match the approved dataset scope")
	}
	base := filepath.Join(root, "para_la_ia", "asignaciones")
	users, err := readLines[userAssignment](filepath.Join(base, "usuarios_planificados.jsonl"), 1000)
	if err != nil {
		return result, err
	}
	listings, err := readLines[listingAssignment](filepath.Join(base, "publicaciones_planificadas.jsonl"), 1200)
	if err != nil {
		return result, err
	}
	scenarios, err := readLines[scenarioAssignment](filepath.Join(base, "escenarios_planificados.jsonl"), 1200)
	if err != nil {
		return result, err
	}
	// Also parse generated content without including any user content in output.
	if _, err := readRawLines(filepath.Join(root, "salida_validada", "usuarios.jsonl"), 1000); err != nil {
		return result, err
	}
	if _, err := readRawLines(filepath.Join(root, "salida_validada", "publicaciones.jsonl"), 1200); err != nil {
		return result, err
	}

	userRefs := make(map[string]bool, len(users))
	userRoles := make(map[string]string, len(users))
	roleCounts := map[string]int{}
	listingCounts := map[string]int{}
	for _, user := range users {
		if user.Ref == "" || userRefs[user.Ref] {
			return result, errors.New("user references must be non-empty and unique")
		}
		userRefs[user.Ref] = true
		if len(user.Roles) != 1 {
			return result, errors.New("each account must have one planned role")
		}
		role := user.Roles[0]
		if role != "arrendador" && role != "arrendatario" && role != "administrador" {
			return result, errors.New("unsupported planned account role")
		}
		roleCounts[role]++
		userRoles[user.Ref] = role
		if user.ListingCount < 0 {
			return result, errors.New("negative planned listing count")
		}
		listingCounts[user.Ref] = user.ListingCount
	}
	if roleCounts["arrendador"] != 300 || roleCounts["arrendatario"] != 690 || roleCounts["administrador"] != 10 {
		return result, errors.New("planned account role totals differ from the manifest")
	}
	listingRefs := make(map[string]listingAssignment, len(listings))
	visibility := map[string]int{}
	categoryCounts := map[string]int{}
	ownerListings := map[string]int{}
	for _, listing := range listings {
		if listing.Ref == "" || listingRefs[listing.Ref].Ref != "" {
			return result, errors.New("listing references must be non-empty and unique")
		}
		if !userRefs[listing.Owner] || !categories[listing.Category] {
			return result, errors.New("listing owner or category reference is invalid")
		}
		if listing.Visibility != "publica" && listing.Visibility != "borrador" && listing.Visibility != "oculta" {
			return result, errors.New("unsupported listing visibility")
		}
		listingRefs[listing.Ref] = listing
		visibility[listing.Visibility]++
		categoryCounts[listing.Category]++
		ownerListings[listing.Owner]++
	}
	if visibility["publica"] != 960 || visibility["borrador"] != 120 || visibility["oculta"] != 120 {
		return result, errors.New("planned visibility totals differ from the manifest")
	}
	for category := range categories {
		if categoryCounts[category] != 150 {
			return result, errors.New("each category must have 150 assigned listings")
		}
	}
	for ref, planned := range listingCounts {
		if ownerListings[ref] != planned {
			return result, errors.New("owner listing assignments do not reconcile")
		}
	}
	scenarioCounts := map[string]int{}
	scenarioRefs := make(map[string]bool, len(scenarios))
	for _, scenario := range scenarios {
		listing, ok := listingRefs[scenario.Listing]
		if !ok || !userRefs[scenario.Host] || !userRefs[scenario.Renter] || scenario.Host == scenario.Renter || userRoles[scenario.Host] != "arrendador" || userRoles[scenario.Renter] != "arrendatario" || listing.Owner != scenario.Host || listing.Visibility != "publica" || listing.Category != scenario.Category {
			return result, errors.New("scenario references must resolve to its public host listing and renter")
		}
		if scenario.Ref == "" || scenarioRefs[scenario.Ref] || scenario.Code == "" {
			return result, errors.New("scenario references must be non-empty and unique")
		}
		scenarioRefs[scenario.Ref] = true
		scenarioCounts[scenario.Code]++
	}
	wantScenarios := map[string]int{"ciclo_cerrado": 600, "pendiente_pago": 240, "cancelacion_con_devolucion": 120, "rechazo_anfitrion": 120, "reclamo_abierto": 120}
	if len(scenarioCounts) != len(wantScenarios) {
		return result, errors.New("scenario distribution is incomplete")
	}
	for name, count := range wantScenarios {
		if scenarioCounts[name] != count {
			return result, errors.New("scenario distribution differs from the agreed plan")
		}
	}
	result = summary{DatasetID: m.DatasetID, Assigned: roleCounts, Visible: visibility, Scenarios: scenarioCounts}
	return result, nil
}

func readLines[T any](path string, want int) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	values := make([]T, 0, want)
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for s.Scan() {
		var value T
		if err := json.Unmarshal(s.Bytes(), &value); err != nil {
			return nil, errors.New("invalid assignment JSONL")
		}
		values = append(values, value)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(values) != want {
		return nil, fmt.Errorf("assignment count mismatch: expected %d, got %d", want, len(values))
	}
	return values, nil
}

func readRawLines(path string, want int) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64*1024), 4*1024*1024)
	n := 0
	for s.Scan() {
		var value json.RawMessage
		if json.Unmarshal(s.Bytes(), &value) != nil || len(value) == 0 {
			return n, errors.New("generated content JSONL is invalid")
		}
		n++
	}
	if err := s.Err(); err != nil {
		return n, err
	}
	if n != want {
		return n, fmt.Errorf("generated content count mismatch: expected %d, got %d", want, n)
	}
	return n, nil
}

func fatal(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(2) }
