package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type lookupHost func(context.Context, string) ([]string, error)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "connect" {
		result := databaseConnectMetadata(os.Args[2])
		fmt.Println(result)
		if result != "connect=success" {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "diagnose" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fmt.Println(lookupMetadata(ctx, net.DefaultResolver.LookupHost))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := checkDatabaseIsolation(ctx, net.DefaultResolver.LookupHost); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS: database name is not resolvable from the mock network")
}

func databaseConnectMetadata(targetIP string) string {
	ip := net.ParseIP(targetIP)
	if ip == nil {
		return "connect=invalid_target"
	}
	target := net.JoinHostPort(ip.String(), "5432")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	connected, timedOut, err := tcpProbe(ctx, target, 2*time.Second)
	if err != nil {
		return "connect=probe_error"
	}
	if connected {
		return "connect=success"
	}
	return fmt.Sprintf("connect=failed timeout=%t", timedOut)
}

func tcpProbe(ctx context.Context, target string, timeout time.Duration) (bool, bool, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil || net.ParseIP(host) == nil {
		return false, false, errors.New("TCP probe requires an IP address and port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return false, false, errors.New("TCP probe requires a valid port")
	}
	dialer := net.Dialer{Timeout: timeout}
	connection, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		var networkError net.Error
		return false, errors.As(err, &networkError) && networkError.Timeout(), nil
	}
	_ = connection.Close()
	return true, false, nil
}

func lookupMetadata(ctx context.Context, lookup lookupHost) string {
	addresses, err := lookup(ctx, "database")
	if err == nil {
		return fmt.Sprintf("lookup=success addresses=%d", len(addresses))
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Sprintf("lookup=error not_found=%t timeout=%t temporary=%t", dnsErr.IsNotFound, dnsErr.IsTimeout, dnsErr.IsTemporary)
	}
	return "lookup=error dns_error=false"
}

func checkDatabaseIsolation(ctx context.Context, lookup lookupHost) error {
	addresses, err := lookup(ctx, "database")
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return nil
		}
		if errors.As(err, &dnsErr) {
			return errors.New("DNS lookup inconclusive from the mock network")
		}
		return errors.New("DNS lookup returned an unexpected error")
	}
	if len(addresses) == 0 {
		return errors.New("DNS lookup returned no addresses without a not-found error")
	}
	return errors.New("database name resolved from the mock network")
}
