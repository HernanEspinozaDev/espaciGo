package rentalflow

import (
	"errors"
	"testing"
	"time"
)

func TestRentalFlowServicesRejectMissingDependencies(t *testing.T) {
	clock := func() time.Time { return time.Date(2030, time.May, 10, 12, 0, 0, 0, time.UTC) }
	cases := []struct {
		name string
		new  func() error
	}{
		{name: "availability catalog", new: func() error { _, err := NewAvailabilityService(nil, &calendarStub{}, clock); return err }},
		{name: "availability calendar", new: func() error { _, err := NewAvailabilityService(&catalogStub{}, nil, clock); return err }},
		{name: "availability clock", new: func() error { _, err := NewAvailabilityService(&catalogStub{}, &calendarStub{}, nil); return err }},
		{name: "quote catalog", new: func() error {
			_, err := NewQuoteService(nil, &calendarStub{}, &quoteRepositoryStub{}, clock)
			return err
		}},
		{name: "quote calendar", new: func() error {
			_, err := NewQuoteService(&catalogStub{}, nil, &quoteRepositoryStub{}, clock)
			return err
		}},
		{name: "quote repository", new: func() error { _, err := NewQuoteService(&catalogStub{}, &calendarStub{}, nil, clock); return err }},
		{name: "quote clock", new: func() error {
			_, err := NewQuoteService(&catalogStub{}, &calendarStub{}, &quoteRepositoryStub{}, nil)
			return err
		}},
		{name: "preview catalog", new: func() error {
			_, err := NewContractPreviewService(nil, &calendarStub{}, &previewQuoteRepository{}, &identityGateStub{}, &contractIdentityReaderStub{}, clock)
			return err
		}},
		{name: "preview calendar", new: func() error {
			_, err := NewContractPreviewService(&catalogStub{}, nil, &previewQuoteRepository{}, &identityGateStub{}, &contractIdentityReaderStub{}, clock)
			return err
		}},
		{name: "preview quote repository", new: func() error {
			_, err := NewContractPreviewService(&catalogStub{}, &calendarStub{}, nil, &identityGateStub{}, &contractIdentityReaderStub{}, clock)
			return err
		}},
		{name: "preview identity gate", new: func() error {
			_, err := NewContractPreviewService(&catalogStub{}, &calendarStub{}, &previewQuoteRepository{}, nil, &contractIdentityReaderStub{}, clock)
			return err
		}},
		{name: "preview legal identity reader", new: func() error {
			_, err := NewContractPreviewService(&catalogStub{}, &calendarStub{}, &previewQuoteRepository{}, &identityGateStub{}, nil, clock)
			return err
		}},
		{name: "preview clock", new: func() error {
			_, err := NewContractPreviewService(&catalogStub{}, &calendarStub{}, &previewQuoteRepository{}, &identityGateStub{}, &contractIdentityReaderStub{}, nil)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.new(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("constructor error=%v, want ErrInvalid", err)
			}
		})
	}
}
