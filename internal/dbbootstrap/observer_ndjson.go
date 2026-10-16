package dbbootstrap

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

const observerMaxRecordBytes = 32 * 1024

type observerManifest struct {
	RecordType     string         `json:"record_type"`
	SchemaVersion  int            `json:"schema_version"`
	ReportSHA256   string         `json:"report_sha256"`
	ReportBytes    int            `json:"report_bytes"`
	RecordCount    int            `json:"record_count"`
	MaxRecordBytes int            `json:"max_record_bytes"`
	Passed         bool           `json:"passed"`
	SectionCounts  map[string]int `json:"section_counts"`
	SectionTotals  map[string]int `json:"section_totals"`
}

type observerSectionRecord struct {
	RecordType   string            `json:"record_type"`
	ReportSHA256 string            `json:"report_sha256"`
	Section      string            `json:"section"`
	Index        int               `json:"index"`
	Total        int               `json:"total"`
	ItemOffset   int               `json:"item_offset"`
	IsArray      bool              `json:"is_array"`
	Items        []json.RawMessage `json:"items"`
}

type observerSectionAccumulator struct {
	IsArray  bool
	IndexSet bool
	Total    int
	Chunks   map[int]observerSectionRecord
}

// WriteObserverNDJSON emits an integrity-checkable report as bounded NDJSON
// records. The full canonical report is hashed before it is split; a consumer
// must use ReadObserverNDJSON to detect missing, duplicate, reordered, or
// corrupted chunks.
func WriteObserverNDJSON(w io.Writer, report ScratchObservation) error {
	return WriteIntegrityNDJSON(w, report, report.Passed())
}

// WriteIntegrityNDJSON emits a fixed JSON report as bounded, integrity-checkable
// NDJSON records. The supplied pass state is included in the manifest.
func WriteIntegrityNDJSON(w io.Writer, report any, passed bool) error {
	if w == nil {
		return errors.New("observer output writer is required")
	}
	canonical, fields, err := canonicalReportFields(report)
	if err != nil {
		return errors.New("could not encode canonical observer report")
	}
	digest := sha256.Sum256(canonical)
	hexDigest := hex.EncodeToString(digest[:])
	sectionNames := make([]string, 0, len(fields))
	for name := range fields {
		sectionNames = append(sectionNames, name)
	}
	sort.Strings(sectionNames)
	counts := make(map[string]int, len(fields))
	totals := make(map[string]int, len(fields))
	sectionRecords := make([]observerSectionRecord, 0)
	for _, name := range sectionNames {
		raw := bytes.TrimSpace(fields[name])
		isArray := len(raw) > 0 && raw[0] == '['
		items := []json.RawMessage{append(json.RawMessage(nil), raw...)}
		if isArray {
			if err := json.Unmarshal(raw, &items); err != nil {
				return errors.New("observer report contains an invalid array section")
			}
		}
		counts[name] = len(items)
		chunks, err := splitObserverItems(name, hexDigest, isArray, items)
		if err != nil {
			return err
		}
		totals[name] = len(chunks)
		sectionRecords = append(sectionRecords, chunks...)
	}
	for i := range sectionRecords {
		sectionRecords[i].Total = totals[sectionRecords[i].Section]
	}
	manifest := observerManifest{
		RecordType: "manifest", SchemaVersion: 1, ReportSHA256: hexDigest,
		ReportBytes: len(canonical), RecordCount: len(sectionRecords) + 1,
		MaxRecordBytes: observerMaxRecordBytes, Passed: passed,
		SectionCounts: counts, SectionTotals: totals,
	}
	buffer := bufio.NewWriter(w)
	if err := writeObserverRecord(buffer, manifest); err != nil {
		return err
	}
	for _, record := range sectionRecords {
		if err := writeObserverRecord(buffer, record); err != nil {
			return err
		}
	}
	return buffer.Flush()
}

func splitObserverItems(section, digest string, isArray bool, items []json.RawMessage) ([]observerSectionRecord, error) {
	if len(items) == 0 {
		return []observerSectionRecord{{RecordType: "section", ReportSHA256: digest, Section: section, Index: 1, ItemOffset: 0, IsArray: isArray, Items: []json.RawMessage{}}}, nil
	}
	groups := make([][]json.RawMessage, 0, 1)
	current := make([]json.RawMessage, 0)
	offset := 0
	for _, item := range items {
		candidate := append(append([]json.RawMessage(nil), current...), item)
		record := observerSectionRecord{RecordType: "section", ReportSHA256: digest, Section: section, Index: len(groups) + 1, ItemOffset: offset, IsArray: isArray, Items: candidate}
		line, err := json.Marshal(record)
		if err != nil {
			return nil, errors.New("could not encode observer section")
		}
		if len(line) <= observerMaxRecordBytes {
			current = candidate
			continue
		}
		if len(current) == 0 {
			return nil, fmt.Errorf("observer item in section %q exceeds the per-record limit", section)
		}
		groups = append(groups, current)
		offset += len(current)
		current = []json.RawMessage{item}
		record.Items = current
		record.Index = len(groups) + 1
		record.ItemOffset = offset
		line, err = json.Marshal(record)
		if err != nil || len(line) > observerMaxRecordBytes {
			return nil, fmt.Errorf("observer item in section %q exceeds the per-record limit", section)
		}
	}
	groups = append(groups, current)
	records := make([]observerSectionRecord, 0, len(groups))
	offset = 0
	for i, group := range groups {
		records = append(records, observerSectionRecord{
			RecordType: "section", ReportSHA256: digest, Section: section,
			Index: i + 1, Total: len(groups), ItemOffset: offset,
			IsArray: isArray, Items: group,
		})
		offset += len(group)
	}
	return records, nil
}

func writeObserverRecord(w io.Writer, record any) error {
	line, err := json.Marshal(record)
	if err != nil {
		return errors.New("could not encode observer NDJSON record")
	}
	if len(line) > observerMaxRecordBytes {
		return errors.New("observer NDJSON record exceeded its fixed size limit")
	}
	if _, err := w.Write(append(line, '\n')); err != nil {
		return errors.New("could not write observer NDJSON record")
	}
	return nil
}

// ReadObserverNDJSON validates and reconstructs a typed scratch observation.
func ReadObserverNDJSON(r io.Reader) (ScratchObservation, error) {
	var report ScratchObservation
	passed, err := ReadIntegrityNDJSON(r, &report)
	if err != nil {
		return ScratchObservation{}, err
	}
	if report.Passed() != passed {
		return ScratchObservation{}, errors.New("observer NDJSON pass state did not match the reconstructed observation")
	}
	return report, nil
}

func ReadIntegrityNDJSON(r io.Reader, target any) (bool, error) {
	if r == nil || target == nil {
		return false, errors.New("observer NDJSON reader is required")
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, observerMaxRecordBytes), observerMaxRecordBytes+1)
	if !scanner.Scan() {
		return false, errors.New("observer NDJSON manifest is missing")
	}
	if len(scanner.Bytes()) > observerMaxRecordBytes {
		return false, errors.New("observer NDJSON manifest exceeds the size limit")
	}
	var manifest observerManifest
	if err := json.Unmarshal(scanner.Bytes(), &manifest); err != nil || manifest.RecordType != "manifest" || manifest.SchemaVersion != 1 || manifest.MaxRecordBytes != observerMaxRecordBytes || manifest.RecordCount < 1 || manifest.ReportBytes < 1 || len(manifest.ReportSHA256) != 64 {
		return false, errors.New("observer NDJSON manifest is invalid")
	}
	sections := make(map[string]*observerSectionAccumulator, len(manifest.SectionCounts))
	for name, count := range manifest.SectionCounts {
		if name == "" || count < 0 || manifest.SectionTotals[name] < 1 {
			return false, errors.New("observer NDJSON manifest has invalid section counts")
		}
		sections[name] = &observerSectionAccumulator{Total: manifest.SectionTotals[name], Chunks: make(map[int]observerSectionRecord)}
	}
	if len(sections) == 0 || len(manifest.SectionTotals) != len(sections) {
		return false, errors.New("observer NDJSON manifest section inventory is invalid")
	}
	recordCount := 1
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(line) > observerMaxRecordBytes {
			return false, errors.New("observer NDJSON section exceeds the size limit")
		}
		var record observerSectionRecord
		if err := json.Unmarshal(line, &record); err != nil || record.RecordType != "section" || record.ReportSHA256 != manifest.ReportSHA256 || record.Index < 1 || record.Total < 1 || record.Index > record.Total || record.ItemOffset < 0 {
			return false, errors.New("observer NDJSON section record is invalid")
		}
		accumulator, ok := sections[record.Section]
		if !ok || accumulator.Total != record.Total {
			return false, errors.New("observer NDJSON section is not in the manifest")
		}
		if _, exists := accumulator.Chunks[record.Index]; exists {
			return false, errors.New("observer NDJSON contains a duplicate section chunk")
		}
		if accumulator.IndexSet && accumulator.IsArray != record.IsArray {
			return false, errors.New("observer NDJSON section type changed between chunks")
		}
		accumulator.IsArray = record.IsArray
		accumulator.IndexSet = true
		accumulator.Chunks[record.Index] = record
		recordCount++
	}
	if err := scanner.Err(); err != nil {
		return false, errors.New("observer NDJSON input could not be read completely")
	}
	if recordCount != manifest.RecordCount {
		return false, errors.New("observer NDJSON record count is incomplete")
	}
	fields := make(map[string]json.RawMessage, len(sections))
	for name, accumulator := range sections {
		if len(accumulator.Chunks) != accumulator.Total {
			return false, errors.New("observer NDJSON section chunks are incomplete")
		}
		items := make([]json.RawMessage, 0, manifest.SectionCounts[name])
		for i := 1; i <= accumulator.Total; i++ {
			chunk, ok := accumulator.Chunks[i]
			if !ok || chunk.ItemOffset != len(items) {
				return false, errors.New("observer NDJSON section order or offset is incomplete")
			}
			items = append(items, chunk.Items...)
		}
		if len(items) != manifest.SectionCounts[name] {
			return false, errors.New("observer NDJSON section item count does not match the manifest")
		}
		if accumulator.IsArray {
			value, err := json.Marshal(items)
			if err != nil {
				return false, errors.New("observer NDJSON array section could not be reconstructed")
			}
			fields[name] = value
		} else {
			if len(items) != 1 {
				return false, errors.New("observer NDJSON scalar section has an invalid item count")
			}
			fields[name] = items[0]
		}
	}
	canonical, err := json.Marshal(fields)
	if err != nil {
		return false, errors.New("observer NDJSON report could not be reconstructed")
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != manifest.ReportSHA256 || len(canonical) != manifest.ReportBytes {
		return false, errors.New("observer NDJSON report hash or length did not match")
	}
	if err := json.Unmarshal(canonical, target); err != nil {
		return false, errors.New("observer NDJSON report has an invalid typed payload")
	}
	switch status := target.(type) {
	case interface{ Passed() bool }:
		if status.Passed() != manifest.Passed {
			return false, errors.New("observer NDJSON pass state did not match the reconstructed report")
		}
	case interface{ IntegrityPassed() bool }:
		if status.IntegrityPassed() != manifest.Passed {
			return false, errors.New("observer NDJSON pass state did not match the reconstructed report")
		}
	}
	return manifest.Passed, nil
}

func canonicalObserverFields(report ScratchObservation) ([]byte, map[string]json.RawMessage, error) {
	return canonicalReportFields(report)
}

func canonicalReportFields(report any) ([]byte, map[string]json.RawMessage, error) {
	raw, err := json.Marshal(report)
	if err != nil {
		return nil, nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, nil, err
	}
	canonical, err := json.Marshal(fields)
	return canonical, fields, err
}
