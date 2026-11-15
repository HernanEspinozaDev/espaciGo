package main

import (
	"reflect"
	"testing"
)

func expectedNativeIdentityCleanupRoles() []nativeIdentityCleanupRoleFlags {
	return []nativeIdentityCleanupRoleFlags{
		{Name: "espacigo_migrator_login", CanLogin: true},
		{Name: "espacigo_document_verification_fn"},
		{Name: "espacigo_schema_owner"},
	}
}

func expectedNativeIdentityCleanupMemberships() []nativeIdentityCleanupMembership {
	return []nativeIdentityCleanupMembership{
		{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login", Grantor: "postgres", Set: true},
		{Role: "espacigo_document_verification_fn", Member: "espacigo_schema_owner", Grantor: "postgres", Set: true},
	}
}

func TestNativeIdentityCleanupRoleContractAcceptsExactSourceContract(t *testing.T) {
	if got := nativeIdentityCleanupRoleIssues(expectedNativeIdentityCleanupRoles(), expectedNativeIdentityCleanupMemberships()); len(got) != 0 {
		t.Fatalf("exact cleanup role contract reported issues: %#v", got)
	}
}

func TestNativeIdentityCleanupRoleContractRejectsInheritedMembership(t *testing.T) {
	memberships := expectedNativeIdentityCleanupMemberships()
	memberships = append(memberships, nativeIdentityCleanupMembership{
		Role: "extra_privilege_role", Member: "espacigo_schema_owner", Grantor: "postgres", Inherit: true, Set: true,
	})
	roles := append(expectedNativeIdentityCleanupRoles(), nativeIdentityCleanupRoleFlags{Name: "extra_privilege_role"})
	want := []string{
		"unsafe cleanup role attributes: extra_privilege_role",
		"unexpected cleanup role membership: role=extra_privilege_role member=espacigo_schema_owner",
	}
	if got := nativeIdentityCleanupRoleIssues(roles, memberships); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup role issues = %#v, want %#v", got, want)
	}
}

func TestNativeIdentityCleanupRoleContractRejectsInheritAttribute(t *testing.T) {
	roles := expectedNativeIdentityCleanupRoles()
	roles[2].Inherit = true
	want := []string{"unsafe cleanup role attributes: espacigo_schema_owner"}
	if got := nativeIdentityCleanupRoleIssues(roles, expectedNativeIdentityCleanupMemberships()); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup role issues = %#v, want %#v", got, want)
	}
}

func TestNativeIdentityCleanupRoleReachabilityIncludesTransitiveSetAndInheritEdges(t *testing.T) {
	memberships := append(expectedNativeIdentityCleanupMemberships(),
		nativeIdentityCleanupMembership{Role: "espacigo_review_bridge", Member: "espacigo_document_verification_fn", Grantor: "postgres", Set: true},
		nativeIdentityCleanupMembership{Role: "espacigo_audit_reader", Member: "espacigo_review_bridge", Grantor: "postgres", Inherit: true},
		nativeIdentityCleanupMembership{Role: "unrelated_role", Member: "unrelated_login", Grantor: "postgres", Set: true},
	)
	want := []string{"espacigo_audit_reader", "espacigo_document_verification_fn", "espacigo_review_bridge", "espacigo_schema_owner"}
	if got := nativeIdentityCleanupReachableRoleNames(memberships); !reflect.DeepEqual(got, want) {
		t.Fatalf("reachable cleanup roles = %#v, want %#v", got, want)
	}
	wantAuthorityRoles := []string{"espacigo_audit_reader", "espacigo_document_verification_fn", "espacigo_migrator_login", "espacigo_review_bridge", "espacigo_schema_owner"}
	if got := nativeIdentityCleanupAuthorityRoleNames(memberships); !reflect.DeepEqual(got, wantAuthorityRoles) {
		t.Fatalf("cleanup authority role names = %#v, want %#v", got, wantAuthorityRoles)
	}
}

func TestNativeIdentityCleanupRoleContractRejectsTransitiveSetRolePath(t *testing.T) {
	memberships := append(expectedNativeIdentityCleanupMemberships(),
		nativeIdentityCleanupMembership{Role: "espacigo_review_bridge", Member: "espacigo_document_verification_fn", Grantor: "postgres", Set: true},
		nativeIdentityCleanupMembership{Role: "espacigo_audit_reader", Member: "espacigo_review_bridge", Grantor: "postgres", Inherit: true},
	)
	roles := append(expectedNativeIdentityCleanupRoles(),
		nativeIdentityCleanupRoleFlags{Name: "espacigo_review_bridge"},
		nativeIdentityCleanupRoleFlags{Name: "espacigo_audit_reader"},
	)
	want := []string{
		"unsafe cleanup role attributes: espacigo_review_bridge",
		"unsafe cleanup role attributes: espacigo_audit_reader",
		"unexpected cleanup role membership: role=espacigo_review_bridge member=espacigo_document_verification_fn",
		"unexpected cleanup role membership: role=espacigo_audit_reader member=espacigo_review_bridge",
	}
	if got := nativeIdentityCleanupRoleIssues(roles, memberships); !reflect.DeepEqual(got, want) {
		t.Fatalf("cleanup role issues = %#v, want %#v", got, want)
	}
}
