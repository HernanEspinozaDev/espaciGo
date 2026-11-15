package main

import (
	"reflect"
	"testing"
)

func exactNativeIdentityCleanupAuthorityReport() nativeIdentityCleanupAuthority {
	return nativeIdentityCleanupAuthority{
		AuditDelete: true,
		AuditSelect: map[string]bool{
			"actor_id":       true,
			"recurso_tipo":   true,
			"recurso_id":     true,
			"accion":         true,
			"correlacion_id": true,
		},
		ClaimsDelete: true,
		OutboxDelete: true,
		AuditACL: []nativeIdentityCleanupACL{
			{Object: "public.evento_auditoria_local", Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "DELETE"},
			{Object: "public.evento_auditoria_local", Column: "actor_id", Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "SELECT"},
			{Object: "public.evento_auditoria_local", Column: "recurso_tipo", Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "SELECT"},
			{Object: "public.evento_auditoria_local", Column: "recurso_id", Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "SELECT"},
			{Object: "public.evento_auditoria_local", Column: "accion", Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "SELECT"},
			{Object: "public.evento_auditoria_local", Column: "correlacion_id", Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "SELECT"},
		},
		RoleFlags:   expectedNativeIdentityCleanupRoles(),
		Memberships: expectedNativeIdentityCleanupMemberships(),
	}
}

func TestNativeIdentityCleanupAuthorityReportsExactMissingPrivileges(t *testing.T) {
	report := exactNativeIdentityCleanupAuthorityReport()
	report.AuditDelete = false
	report.PublicCreate = true
	report.AuditSelect["actor_id"] = false
	report.AuditSelect["recurso_id"] = false
	report.AuditSelect["correlacion_id"] = false
	report.ClaimsDelete = false
	report.OutboxDelete = false
	report.AuditACL = nil
	want := []string{
		"DELETE public.evento_auditoria_local",
		"SELECT public.evento_auditoria_local.actor_id",
		"SELECT public.evento_auditoria_local.recurso_id",
		"SELECT public.evento_auditoria_local.correlacion_id",
		"DELETE public.identity_rut_claim",
		"DELETE public.outbox_evento_local",
		"required audit privilege is available through an unapproved grant path: SELECT public.evento_auditoria_local.recurso_tipo",
		"required audit privilege is available through an unapproved grant path: SELECT public.evento_auditoria_local.accion",
		"unexpected CREATE on public schema",
	}
	if got := nativeIdentityCleanupAuthorityIssues(report); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup authority issues = %#v, want %#v", got, want)
	}
}

func TestNativeIdentityCleanupAuthorityAcceptsOnlyExactRequiredSet(t *testing.T) {
	if got := nativeIdentityCleanupAuthorityIssues(exactNativeIdentityCleanupAuthorityReport()); len(got) != 0 {
		t.Fatalf("complete cleanup authority reported issues: %#v", got)
	}
}

func TestNativeIdentityCleanupAuthorityRejectsUnexpectedAuditPrivilege(t *testing.T) {
	report := exactNativeIdentityCleanupAuthorityReport()
	report.AuditACL = append(report.AuditACL, nativeIdentityCleanupACL{
		Object: "public.evento_auditoria_local", Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "UPDATE",
	})
	want := []string{"unexpected audit ACL: espacigo_schema_owner UPDATE on public.evento_auditoria_local"}
	if got := nativeIdentityCleanupAuthorityIssues(report); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup authority issues = %#v, want %#v", got, want)
	}
}

func TestNativeIdentityCleanupAuthorityRejectsPublicAuditGrant(t *testing.T) {
	report := exactNativeIdentityCleanupAuthorityReport()
	report.AuditACL = append(report.AuditACL, nativeIdentityCleanupACL{
		Object: "public.evento_auditoria_local", Column: "actor_id", Grantee: "PUBLIC", Grantor: "postgres", Privilege: "SELECT",
	})
	want := []string{"unexpected audit ACL: PUBLIC SELECT on public.evento_auditoria_local.actor_id"}
	if got := nativeIdentityCleanupAuthorityIssues(report); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup authority issues = %#v, want %#v", got, want)
	}
}

func TestNativeIdentityCleanupAuthorityRejectsReachableRoleAuditGrant(t *testing.T) {
	report := exactNativeIdentityCleanupAuthorityReport()
	report.AuditACL = append(report.AuditACL, nativeIdentityCleanupACL{
		Object: "public.evento_auditoria_local", Grantee: "espacigo_document_verification_fn", Grantor: "postgres", Privilege: "DELETE",
	})
	want := []string{"unexpected audit ACL: espacigo_document_verification_fn DELETE on public.evento_auditoria_local"}
	if got := nativeIdentityCleanupAuthorityIssues(report); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup authority issues = %#v, want %#v", got, want)
	}
}
