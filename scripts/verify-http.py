#!/usr/bin/env python3
import json
import os
import urllib.error
import urllib.request

ANGULAR_ORIGIN = os.environ.get("ANGULAR_ORIGIN", "http://127.0.0.1:4200")
API_ORIGIN = os.environ.get("API_ORIGIN", "http://127.0.0.1:8080")
API_BASE = os.environ.get("EXPECTED_API_BASE", f"{API_ORIGIN}/api/v1")


def request(url: str, *, origin: str | None = None, method: str = "GET"):
    headers = {"Origin": origin} if origin is not None else {}
    req = urllib.request.Request(url, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=5) as response:
            return response.status, response.headers, response.read()
    except urllib.error.HTTPError as response:
        return response.code, response.headers, response.read()


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"FAIL: {message}")
    print(f"PASS: {message}")


status, _, body = request(f"{API_ORIGIN}/health/live")
require(status == 200 and json.loads(body) == {"status": "live"}, "backend liveness responds without database details")

status, headers, body = request(f"{API_ORIGIN}/health/ready", origin=ANGULAR_ORIGIN)
require(status == 200 and json.loads(body) == {"status": "ready"}, "backend readiness confirms PostgreSQL with a generic response")
require(headers.get("Access-Control-Allow-Origin") == ANGULAR_ORIGIN, "readiness permits the Angular origin through CORS")

status, headers, _ = request(
    f"{API_ORIGIN}/health/ready",
    origin=ANGULAR_ORIGIN,
    method="OPTIONS",
)
require(status == 204 and headers.get("Access-Control-Allow-Origin") == ANGULAR_ORIGIN, "readiness handles Angular-origin CORS preflight")

status, headers, body = request(f"{ANGULAR_ORIGIN}/")
require(status == 200 and "text/html" in headers.get("Content-Type", "") and b"EspaciGo" in body, "Angular serves the application shell")

status, headers, body = request(f"{ANGULAR_ORIGIN}/healthz")
require(status == 200 and json.loads(body) == {"status": "ok"}, "Angular process health endpoint responds")

status, headers, body = request(f"{ANGULAR_ORIGIN}/config.json")
config = json.loads(body)
require(
    status == 200 and config == {"apiBaseUrl": API_BASE, "siteOrigin": ANGULAR_ORIGIN},
    "Angular runtime config exposes only the expected public API URL and site origin",
)

status, headers, body = request(f"{ANGULAR_ORIGIN}/acceso")
require(status == 200 and "text/html" in headers.get("Content-Type", "") and b"EspaciGo" in body, "Angular SSR serves the identity route shell")

status, headers, body = request(f"{API_ORIGIN}/health/ready", origin=ANGULAR_ORIGIN)
require(status == 200 and json.loads(body) == {"status": "ready"}, "Angular browser origin can reach configured Go readiness")
