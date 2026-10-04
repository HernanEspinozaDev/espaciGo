package main

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestDatabaseNameNotFoundPassesIsolationCheck(t *testing.T) {
	err := checkDatabaseIsolation(context.Background(), func(context.Context, string) ([]string, error) {
		return nil, &net.DNSError{Err: "no such host", Name: "database", IsNotFound: true}
	})
	if err != nil {
		t.Fatalf("expected isolated database name to pass, got %v", err)
	}
}

func TestResolvedDatabaseNameFailsIsolationCheck(t *testing.T) {
	err := checkDatabaseIsolation(context.Background(), func(context.Context, string) ([]string, error) {
		return []string{"172.30.0.2"}, nil
	})
	if err == nil {
		t.Fatal("database name resolution unexpectedly passed isolation check")
	}
}

func TestLookupMetadataReportsOnlyOutcomeAndAddressCount(t *testing.T) {
	got := lookupMetadata(context.Background(), func(context.Context, string) ([]string, error) {
		return []string{"192.0.2.10"}, nil
	})
	if got != "lookup=success addresses=1" {
		t.Fatalf("metadata = %q", got)
	}
}

func TestLookupMetadataClassifiesNotFoundWithoutRawError(t *testing.T) {
	got := lookupMetadata(context.Background(), func(context.Context, string) ([]string, error) {
		return nil, &net.DNSError{Err: "synthetic DNS detail", Name: "database", IsNotFound: true}
	})
	if got != "lookup=error not_found=true timeout=false temporary=false" {
		t.Fatalf("metadata = %q", got)
	}
}

func TestLookupMetadataClassifiesTemporaryFailureWithoutRawError(t *testing.T) {
	got := lookupMetadata(context.Background(), func(context.Context, string) ([]string, error) {
		return nil, &net.DNSError{Err: "synthetic DNS detail", Name: "database", IsTemporary: true, IsTimeout: true}
	})
	if got != "lookup=error not_found=false timeout=true temporary=true" {
		t.Fatalf("metadata = %q", got)
	}
}

func TestTCPProbeReachesListenerWithinTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = connection.Close()
		}
	}()

	connected, timedOut, err := tcpProbe(context.Background(), listener.Addr().String(), time.Second)
	if err != nil || !connected || timedOut {
		t.Fatalf("tcpProbe = connected:%t timeout:%t err:%v", connected, timedOut, err)
	}
}

func TestTCPProbeRejectsHostnameTargets(t *testing.T) {
	connected, timedOut, err := tcpProbe(context.Background(), "database:5432", time.Second)
	if err == nil || connected || timedOut {
		t.Fatalf("tcpProbe accepted a hostname target: connected:%t timeout:%t err:%v", connected, timedOut, err)
	}
}

func TestTCPProbeRejectsNonNumericPorts(t *testing.T) {
	connected, timedOut, err := tcpProbe(context.Background(), "127.0.0.1:not-a-port", time.Second)
	if err == nil || connected || timedOut {
		t.Fatalf("tcpProbe accepted an invalid port: connected:%t timeout:%t err:%v", connected, timedOut, err)
	}
}