package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/media"
)

func TestHTTPServerTimeoutsCoverBoundedMultipartUpload(t *testing.T) {
	server := newHTTPServer(":0", http.NotFoundHandler())
	uploadLimit := media.MaxInputBytes + (1 << 20)
	if server.ReadTimeout < 2*time.Minute {
		t.Fatalf("read timeout %s is too short for %d-byte multipart body", server.ReadTimeout, uploadLimit)
	}
	minimumBytesPerSecond := float64(uploadLimit) / server.ReadTimeout.Seconds()
	if minimumBytesPerSecond > 100*1024 {
		t.Fatalf("read timeout requires upload rate %.0f B/s", minimumBytesPerSecond)
	}
	if server.WriteTimeout < server.ReadTimeout+2*time.Minute {
		t.Fatalf("write timeout %s leaves less than two minutes after body read %s", server.WriteTimeout, server.ReadTimeout)
	}
}
