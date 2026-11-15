// rentaldemo serves an isolated, reset-on-restart, in-memory rental demo.
// It intentionally imports no database, provider or production API startup.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentaldemo"
)

func validateAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	n, e := strconv.Atoi(port)
	if err != nil || host != "127.0.0.1" || e != nil || n < 1 || n > 65535 {
		return errors.New("demo requires numeric IPv4 loopback and an explicit port")
	}
	return nil
}
func run(args []string) error {
	flags := flag.NewFlagSet("rentaldemo", flag.ContinueOnError)
	addr := flags.String("addr", "127.0.0.1:18117", "isolated loopback listener")
	fixed := flags.String("now", "", "optional RFC3339 synthetic clock; not a production clock")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected demo arguments")
	}
	if err := validateAddress(*addr); err != nil {
		return err
	}
	now := time.Now
	if *fixed != "" {
		value, err := time.Parse(time.RFC3339Nano, *fixed)
		if err != nil || value.IsZero() {
			return errors.New("invalid synthetic demo clock")
		}
		now = func() time.Time { return value }
	}
	store, err := rentaldemo.NewStore(now)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: *addr, Handler: rentaldemo.NewHandler(store, now), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Print("Isolated rental demo: in-memory state; no KYC, legal contract or real payment; resets on restart")
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Print("Rental demo could not run; check local address or synthetic configuration")
		os.Exit(1)
	}
}
