package dbbootstrap

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

const observerMaxRecordBytes = 32 * 1024
const integrityChunkBytes = 12 * 1024

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

type integrityManifest struct {
	RecordType     string `json:"record_type"`
	SchemaVersion  int    `json:"schema_version"`
	ReportSHA256   string `json:"report_sha256"`
	ReportBytes    int    `json:"report_bytes"`
	RecordCount    int    `json:"record_count"`
	MaxRecordBytes int    `json:"max_record_bytes"`
	ChunkBytes     int    `json:"chunk_bytes"`
	Passed         bool   `json:"passed"`
}

type integrityPayloadRecord struct {
	RecordType   string `json:"record_type"`
	ReportSHA256 string `json:"report_sha256"`
	Index        int    `json:"index"`
	Total        int    `json:"total"`
	ByteOffset   int    `json:"byte_offset"`
	Data         string `json:"data"`
}

// WriteObserverNDJSON emits an integrity-checkable report as bounded NDJSON
// records. The full canonical report is hashed before it is split; a consumer
// must use ReadObserverNDJSON to detect missing, duplicate, reordered, or
// corrupted chunks.
func WriteObserverNDJSON(w io.Writer, report ScratchObservation) error {
	if w == nil {
		return errors.New("observer output writer is required")
	}
	canonical, fields, err := canonicalObserverFields(report)
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
		MaxRecordBytes: observerMaxRecordBytes, Passed: report.Passed(),
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

// WriteIntegrityNDJSON emits the complete canonical JSON payload in bounded
// byte chunks. Unlike the observer's legacy sectioned format, it also splits
// large nested objects such as a retired verification's embedded observation.
func WriteIntegrityNDJSON(w io.Writer, report any, passed bool) error {
	if w == nil || report == nil {
		return errors.New("integrity report and output writer are required")
	}
	canonical, err := json.Marshal(report)
	if err != nil || len(canonical) == 0 {
		return errors.New("could not encode canonical integrity report")
	}
	digest := sha256.Sum256(canonical)
	hexDigest := hex.EncodeToString(digest[:])
	total := (len(canonical) + integrityChunkBytes - 1) / integrityChunkBytes
	manifest := integrityManifest{
		RecordType: "manifest", SchemaVersion: 2, ReportSHA256: hexDigest,
		ReportBytes: len(canonical), RecordCount: total + 1,
		MaxRecordBytes: observerMaxRecordBytes, ChunkBytes: integrityChunkBytes,
		Passed: passed,
	}
	buffer := bufio.NewWriter(w)
	if err := writeObserverRecord(buffer, manifest); err != nil {
		return err
	}
	for index, offset := 1, 0; offset < len(canonical); index++ {
		end := offset + integrityChunkBytes
		if end > len(canonical) {
			end = len(canonical)
		}
		record := integrityPayloadRecord{
			RecordType: "payload", ReportSHA256: hexDigest, Index: index,
			Total: total, ByteOffset: offset,
			Data: base64.RawStdEncoding.EncodeToString(canonical[offset:end]),
		}
		if err := writeObserverRecord(buffer, record); err != nil {
			return err
		}
		offset = end
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

// ReadObserverNDJSON validates the manifest, section sequence, bounded line
// sizes and complete report hash before reconstructing the typed observation.
func ReadObserverNDJSON(r io.Reader) (ScratchObservation, error) {
	if r == nil {
		return ScratchObservation{}, errors.New("observer NDJSON reader is required")
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, observerMaxRecordBytes), observerMaxRecordBytes+1)
	if !scanner.Scan() {
		return ScratchObservation{}, errors.New("observer NDJSON manifest is missing")
	}
	if len(scanner.Bytes()) > observerMaxRecordBytes {
		return ScratchObservation{}, errors.New("observer NDJSON manifest exceeds the size limit")
	}
	var manifest observerManifest
	if err := json.Unmarshal(scanner.Bytes(), &manifest); err != nil || manifest.RecordType != "manifest" || manifest.SchemaVersion != 1 || manifest.MaxRecordBytes != observerMaxRecordBytes || manifest.RecordCount < 1 || manifest.ReportBytes < 1 || len(manifest.ReportSHA256) != 64 {
		return ScratchObservation{}, errors.New("observer NDJSON manifest is invalid")
	}
	sections := make(map[string]*observerSectionAccumulator, len(manifest.SectionCounts))
	for name, count := range manifest.SectionCounts {
		if name == "" || count < 0 || manifest.SectionTotals[name] < 1 {
			return ScratchObservation{}, errors.New("observer NDJSON manifest has invalid section counts")
		}
		sections[name] = &observerSectionAccumulator{Total: manifest.SectionTotals[name], Chunks: make(map[int]observerSectionRecord)}
	}
	if len(sections) == 0 || len(manifest.SectionTotals) != len(sections) {
		return ScratchObservation{}, errors.New("observer NDJSON manifest section inventory is invalid")
	}
	recordCount := 1
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(line) > observerMaxRecordBytes {
			return ScratchObservation{}, errors.New("observer NDJSON section exceeds the size limit")
		}
		var record observerSectionRecord
		if err := json.Unmarshal(line, &record); err != nil || record.RecordType != "section" || record.ReportSHA256 != manifest.ReportSHA256 || record.Index < 1 || record.Total < 1 || record.Index > record.Total || record.ItemOffset < 0 {
			return ScratchObservation{}, errors.New("observer NDJSON section record is invalid")
		}
		accumulator, ok := sections[record.Section]
		if !ok || accumulator.Total != record.Total {
			return ScratchObservation{}, errors.New("observer NDJSON section is not in the manifest")
		}
		if _, exists := accumulator.Chunks[record.Index]; exists {
			return ScratchObservation{}, errors.New("observer NDJSON contains a duplicate section chunk")
		}
		if accumulator.IndexSet && accumulator.IsArray != record.IsArray {
			return ScratchObservation{}, errors.New("observer NDJSON section type changed between chunks")
		}
		accumulator.IsArray = record.IsArray
		accumulator.IndexSet = true
		accumulator.Chunks[record.Index] = record
		recordCount++
	}
	if err := scanner.Err(); err != nil {
		return ScratchObservation{}, errors.New("observer NDJSON input could not be read completely")
	}
	if recordCount != manifest.RecordCount {
		return ScratchObservation{}, errors.New("observer NDJSON record count is incomplete")
	}
	fields := make(map[string]json.RawMessage, len(sections))
	for name, accumulator := range sections {
		if len(accumulator.Chunks) != accumulator.Total {
			return ScratchObservation{}, errors.New("observer NDJSON section chunks are incomplete")
		}
		items := make([]json.RawMessage, 0, manifest.SectionCounts[name])
		for i := 1; i <= accumulator.Total; i++ {
			chunk, ok := accumulator.Chunks[i]
			if !ok || chunk.ItemOffset != len(items) {
				return ScratchObservation{}, errors.New("observer NDJSON section order or offset is incomplete")
			}
			items = append(items, chunk.Items...)
		}
		if len(items) != manifest.SectionCounts[name] {
			return ScratchObservation{}, errors.New("observer NDJSON section item count does not match the manifest")
		}
		if accumulator.IsArray {
			value, err := json.Marshal(items)
			if err != nil {
				return ScratchObservation{}, errors.New("observer NDJSON array section could not be reconstructed")
			}
			fields[name] = value
		} else {
			if len(items) != 1 {
				return ScratchObservation{}, errors.New("observer NDJSON scalar section has an invalid item count")
			}
			fields[name] = items[0]
		}
	}
	canonical, err := json.Marshal(fields)
	if err != nil {
		return ScratchObservation{}, errors.New("observer NDJSON report could not be reconstructed")
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != manifest.ReportSHA256 || len(canonical) != manifest.ReportBytes {
		return ScratchObservation{}, errors.New("observer NDJSON report hash or length did not match")
	}
	var report ScratchObservation
	if err := json.Unmarshal(canonical, &report); err != nil {
		return ScratchObservation{}, errors.New("observer NDJSON report has an invalid typed payload")
	}
	if report.Passed() != manifest.Passed {
		return ScratchObservation{}, errors.New("observer NDJSON pass state did not match the manifest")
	}
	return report, nil
}

// ReadIntegrityNDJSON reconstructs a report emitted by WriteIntegrityNDJSON
// and rejects incomplete, reordered, duplicated, oversized, or corrupted data.
func ReadIntegrityNDJSON(r io.Reader, target any) (bool, error) {
	if r == nil || target == nil {
		return false, errors.New("integrity report reader and target are required")
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, observerMaxRecordBytes), observerMaxRecordBytes+1)
	if !scanner.Scan() || len(scanner.Bytes()) > observerMaxRecordBytes {
		return false, errors.New("integrity report manifest is missing or oversized")
	}
	var manifest integrityManifest
	if err := json.Unmarshal(scanner.Bytes(), &manifest); err != nil || manifest.RecordType != "manifest" ||
		manifest.SchemaVersion != 2 || manifest.ReportBytes < 1 || manifest.RecordCount < 2 ||
		manifest.MaxRecordBytes != observerMaxRecordBytes || manifest.ChunkBytes != integrityChunkBytes || len(manifest.ReportSHA256) != 64 {
		return false, errors.New("integrity report manifest is invalid")
	}
	chunkCount := manifest.RecordCount - 1
	var payload bytes.Buffer
	for index := 1; scanner.Scan(); index++ {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(line) > observerMaxRecordBytes {
			return false, errors.New("integrity report record exceeds the size limit")
		}
		var record integrityPayloadRecord
		if err := json.Unmarshal(line, &record); err != nil || record.RecordType != "payload" ||
			record.ReportSHA256 != manifest.ReportSHA256 || record.Index != index || record.Total != chunkCount ||
			record.ByteOffset != payload.Len() {
			return false, errors.New("integrity report payload sequence is invalid")
		}
		chunk, err := base64.RawStdEncoding.DecodeString(record.Data)
		if err != nil || len(chunk) == 0 || len(chunk) > integrityChunkBytes || (index < chunkCount && len(chunk) != integrityChunkBytes) {
			return false, errors.New("integrity report payload chunk is invalid")
		}
		_, _ = payload.Write(chunk)
	}
	if err := scanner.Err(); err != nil {
		return false, errors.New("integrity report could not be read completely")
	}
	if payload.Len() != manifest.ReportBytes {
		return false, errors.New("integrity report payload length does not match manifest")
	}
	digest := sha256.Sum256(payload.Bytes())
	if hex.EncodeToString(digest[:]) != manifest.ReportSHA256 {
		return false, errors.New("integrity report payload hash did not match")
	}
	if err := json.Unmarshal(payload.Bytes(), target); err != nil {
		return false, errors.New("integrity report payload has an invalid typed report")
	}
	switch status := target.(type) {
	case interface{ Passed() bool }:
		if status.Passed() != manifest.Passed {
			return false, errors.New("integrity report pass state does not match payload")
		}
	case interface{ IntegrityPassed() bool }:
		if status.IntegrityPassed() != manifest.Passed {
			return false, errors.New("integrity report pass state does not match payload")
		}
	}
	return manifest.Passed, nil
}

func canonicalObserverFields(report ScratchObservation) ([]byte, map[string]json.RawMessage, error) {
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
