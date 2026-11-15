package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type nativeFixtureAuthFake struct {
	mailbox     *nativeFixtureMailbox
	calls       *[]string
	verifyErr   error
	registerErr error
}

func (f nativeFixtureAuthFake) Register(context.Context, identity.RegisterInput) (string, error) {
	*f.calls = append(*f.calls, "register")
	f.mailbox.delivery = identity.VerificationDelivery{TokenID: "synthetic-token-id", Token: identity.Secret("synthetic-token")}
	return "11111111-1111-4111-8111-111111111111", f.registerErr
}
func (f nativeFixtureAuthFake) VerifyEmail(context.Context, string, identity.Secret, string) error {
	*f.calls = append(*f.calls, "verify")
	return f.verifyErr
}
func TestNativeFixturePersistsRegistrationBeforeEmailVerification(t *testing.T) {
	for _, failed := range []bool{false, true} {
		var calls []string
		mailbox := &nativeFixtureMailbox{}
		flow := nativeFixtureAuthFake{mailbox: mailbox, calls: &calls}
		if failed {
			flow.verifyErr = errors.New("verification unavailable")
		}
		err := registerNativeFixture(context.Background(), flow, mailbox, identity.RegisterInput{}, func(id string, verified bool) error {
			if id == "" {
				t.Fatal("fixture identity omitted")
			}
			if verified {
				calls = append(calls, "persist-verified")
			} else {
				calls = append(calls, "persist-pending")
			}
			return nil
		})
		want := []string{"register", "persist-pending", "verify"}
		if !failed {
			want = append(want, "persist-verified")
		}
		if (err != nil) != failed || !reflect.DeepEqual(calls, want) {
			t.Fatalf("failed=%v calls=%v want=%v error=%v", failed, calls, want, err)
		}
	}
}

func TestNativeFixturePreservesIDOnPostCommitDeliveryError(t *testing.T) {
	var calls []string
	mailbox := &nativeFixtureMailbox{}
	flow := nativeFixtureAuthFake{mailbox: mailbox, calls: &calls, registerErr: identity.ErrDelivery}
	pending := 0
	err := registerNativeFixture(context.Background(), flow, mailbox, identity.RegisterInput{}, func(id string, verified bool) error {
		if id == "" || verified {
			t.Fatal("post-commit error did not preserve pending identity")
		}
		pending++
		return nil
	})
	if !errors.Is(err, identity.ErrDelivery) || pending != 1 || !reflect.DeepEqual(calls, []string{"register"}) {
		t.Fatal("delivery failure lost identity or attempted another auth operation")
	}
}

func TestNativeFixtureResumeDoesNotAdoptOrRecreateAccounts(t *testing.T) {
	const ownID = "11111111-1111-4111-8111-111111111111"
	const otherID = "22222222-2222-4222-8222-222222222222"
	for _, tc := range []struct {
		name     string
		receipt  nativeFixtureReceipt
		storedID string
		allowed  bool
	}{
		{"new", nativeFixtureReceipt{}, "", true},
		{"known", nativeFixtureReceipt{AccountID: ownID}, ownID, true},
		{"missing known", nativeFixtureReceipt{AccountID: ownID}, "", false},
		{"mismatch", nativeFixtureReceipt{AccountID: ownID}, otherID, false},
		{"unattempted collision", nativeFixtureReceipt{}, ownID, false},
		{"committed before receipt", nativeFixtureReceipt{RegistrationAttempted: true}, ownID, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateNativeFixtureResume(tc.receipt, tc.storedID) == nil; got != tc.allowed {
				t.Fatalf("allowed=%v want=%v", got, tc.allowed)
			}
		})
	}
}

func TestNativeFixtureReceiptBasenameAllowsOnlyTwoSlots(t *testing.T) {
	for _, tc := range []struct {
		slot, want string
		wantErr    bool
	}{
		{"", "fixture.json", false}, {"1", "fixture.json", false}, {"2", "fixture-2.json", false},
		{"3", "", true}, {"../fixture.json", "", true}, {"/tmp/x", "", true},
	} {
		got, err := nativeFixtureReceiptBasename(tc.slot)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Fatalf("slot %q got %q err=%v", tc.slot, got, err)
		}
	}
}
