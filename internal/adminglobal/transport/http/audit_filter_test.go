package http

import (
	"net/url"
	"testing"
	"time"
)

func TestParseAuditFilterIncludesScopedMetadataAndPinnedSnapshot(t *testing.T) {
	from := "2026-10-15T00:00:00Z"
	until := "2026-10-16T00:00:00Z"
	cutoff := "2026-10-15T12:34:56.123456Z"
	actor := "00000000-0000-4000-8000-000000000001"
	resource := "00000000-0000-4000-8000-000000000002"
	correlation := "00000000-0000-4000-8000-000000000003"
	query := url.Values{
		"from": {from}, "until": {until}, "actor_id": {actor}, "target_account_id": {resource},
		"resource_type": {"cuenta_global"}, "resource_id": {resource}, "action": {"admin.global.accounts.read"},
		"result": {"rechazo"}, "reason_code": {"cuenta_no_encontrada"}, "correlation_id": {correlation},
		"snapshot_cutoff": {cutoff}, "page_size": {"3"}, "cursor": {"opaque"},
	}
	filter, valid := parseAuditFilter(query)
	if !valid {
		t.Fatal("valid scoped audit filter rejected")
	}
	if filter.ActorID != actor || filter.TargetID != resource || filter.ResourceID != resource || filter.ReasonCode != "cuenta_no_encontrada" || filter.CorrelationID != correlation || filter.Cursor != "opaque" || filter.PageSize != 3 || filter.SnapshotCutoff == nil {
		t.Fatalf("parsed filter lost fields: %+v", filter)
	}
	wantCutoff, _ := time.Parse(time.RFC3339Nano, cutoff)
	if !filter.SnapshotCutoff.Equal(wantCutoff) {
		t.Fatalf("snapshot cutoff %s, want %s", filter.SnapshotCutoff, wantCutoff)
	}

	query.Add("reason_code", "other")
	if _, valid := parseAuditFilter(query); valid {
		t.Fatal("duplicate filter value accepted")
	}
	query.Del("reason_code")
	query.Set("unknown", "value")
	if _, valid := parseAuditFilter(query); valid {
		t.Fatal("unknown filter accepted")
	}
}
