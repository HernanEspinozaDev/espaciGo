package main

import (
	"errors"
	"net"
	"path/filepath"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
)

type runtimeProfileName string

const (
	runtimeProfileStandard   runtimeProfileName = "standard"
	runtimeProfileWorkerFree runtimeProfileName = "local-http-worker-free"
)

func runtimeProfile(getenv func(string) string) (runtimeProfileName, error) {
	p := strings.TrimSpace(getenv("API_RUNTIME_PROFILE"))
	if p == "" {
		p = string(runtimeProfileStandard)
	}
	switch runtimeProfileName(p) {
	case runtimeProfileStandard:
		return runtimeProfileStandard, nil
	case runtimeProfileWorkerFree:
		if getenv("LOCAL_AUTH_PROTOTYPE") != "1" {
			return "", errors.New("local-http-worker-free requires LOCAL_AUTH_PROTOTYPE=1")
		}
		if getenv("APP_PROFILE") != "" {
			return "", errors.New("local-http-worker-free cannot use APP_PROFILE")
		}
		httpAddr := getenv("HTTP_ADDR")
		httpHost, _, err := net.SplitHostPort(httpAddr)
		if err != nil || strings.TrimSpace(httpAddr) != httpAddr {
			return "", errors.New("local-http-worker-free requires an explicit loopback HTTP_ADDR")
		}
		hostIP := net.ParseIP(strings.Trim(httpHost, "[]"))
		if strings.ToLower(httpHost) != "localhost" && (hostIP == nil || !hostIP.IsLoopback()) {
			return "", errors.New("local-http-worker-free requires a loopback HTTP_ADDR")
		}
		if strings.TrimSpace(getenv("DATABASE_CONNECTION_MODE")) != "direct" {
			return "", errors.New("local-http-worker-free requires direct database connection mode")
		}
		host := strings.ToLower(strings.TrimSpace(getenv("DATABASE_HOST")))
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", errors.New("local-http-worker-free requires a loopback database host")
		}
		b := getenv("BLOB_STORE_BACKEND")
		if b != "" && b != "filesystem" {
			return "", errors.New("local-http-worker-free requires filesystem blob storage")
		}
		replayBackend := strings.TrimSpace(getenv("LOCAL_PRIVACY_REPLAY_BACKEND"))
		if replayBackend != "" && replayBackend != "filesystem" {
			return "", errors.New("local-http-worker-free requires filesystem privacy replay storage")
		}
		root := strings.TrimSpace(getenv("M03_EVIDENCE_DIR"))
		if root == "" || !filepath.IsAbs(root) {
			return "", errors.New("local-http-worker-free requires an absolute M03_EVIDENCE_DIR")
		}
		if _, err := evidencefs.New(root); err != nil {
			return "", errors.New("local-http-worker-free requires an existing private evidence directory")
		}
		return runtimeProfileWorkerFree, nil
	default:
		return "", errors.New("API_RUNTIME_PROFILE must be standard or local-http-worker-free")
	}
}

type namedWorker struct {
	name   string
	launch func()
}

func validateWorkerRegistry(profile runtimeProfileName, workers []namedWorker) error {
	if profile != runtimeProfileStandard && profile != runtimeProfileWorkerFree {
		return errors.New("invalid runtime profile for background workers")
	}
	for _, worker := range workers {
		if worker.name == "" || worker.launch == nil {
			return errors.New("invalid background worker registration")
		}
	}
	return nil
}

func launchWorkers(profile runtimeProfileName, workers []namedWorker) error {
	if err := validateWorkerRegistry(profile, workers); err != nil {
		return err
	}
	if profile == runtimeProfileWorkerFree {
		return nil
	}
	for _, worker := range workers {
		worker.launch()
	}
	return nil
}
