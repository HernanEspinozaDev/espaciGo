package publiccatalog

import (
	"context"
	"strings"
	"testing"
)

func TestListRejectsMalformedLegacyAttributeCodesBeforeRepository(t *testing.T) {
	profile := []AttributeFilter{{Code: "wifi", Type: "boolean", ProfileVersion: 1}}
	validRepo := &discoveryRepository{categoryActive: true, filters: profile}
	validService := newDiscoveryService(t, validRepo)
	if _, err := validService.List(context.Background(), Filters{CategoryCode: "oficina", Attributes: map[string]bool{"wifi": true}}); err != nil {
		t.Fatalf("valid control filter rejected: %v", err)
	}
	if validRepo.categoryCalls != 1 || validRepo.filterCalls != 1 || validRepo.listCalls != 1 {
		t.Fatalf("valid control calls category=%d profile=%d list=%d, want 1/1/1", validRepo.categoryCalls, validRepo.filterCalls, validRepo.listCalls)
	}

	cases := []struct {
		name string
		code string
	}{
		{name: "empty", code: ""},
		{name: "uppercase", code: "Wifi"},
		{name: "hyphen", code: "wi-fi"},
		{name: "non-ASCII", code: "wifiñ"},
		{name: "too long", code: strings.Repeat("a", 61)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &discoveryRepository{categoryActive: true, filters: profile}
			service := newDiscoveryService(t, repo)
			_, err := service.List(context.Background(), Filters{CategoryCode: "oficina", Attributes: map[string]bool{tc.code: true}})
			if err != ErrInvalid || repo.categoryCalls != 0 || repo.filterCalls != 0 || repo.listCalls != 0 {
				t.Fatalf("code=%q error=%v calls category=%d profile=%d list=%d", tc.code, err, repo.categoryCalls, repo.filterCalls, repo.listCalls)
			}
		})
	}
}
