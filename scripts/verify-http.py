#!/usr/bin/env python3
import json
import urllib.error
import urllib.request

MOCK_ORIGIN = "http://127.0.0.1:8081"
API_ORIGIN = "http://127.0.0.1:8080"


def request(url: str, *, origin: str | None = None, method: str = "GET", headers: dict[str, str] | None = None):
    request_headers = dict(headers or {})
    if origin is not None:
        request_headers["Origin"] = origin
    req = urllib.request.Request(url, headers=request_headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=4) as response:
            return response.status, response.headers, response.read()
    except urllib.error.HTTPError as response:
        return response.code, response.headers, response.read()


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"FAIL: {message}")
    print(f"PASS: {message}")


status, headers, body = request(f"{API_ORIGIN}/health/live")
require(status == 200 and json.loads(body) == {"status": "live"}, "backend liveness responds without database details")

status, headers, body = request(f"{API_ORIGIN}/health/ready", origin=MOCK_ORIGIN)
ready = json.loads(body)
require(status == 200 and ready == {"status": "ready"}, "backend readiness confirms PostgreSQL with a generic response")
require(headers.get("Access-Control-Allow-Origin") == MOCK_ORIGIN, "readiness permits the mock origin through CORS")

status, headers, body = request(
    f"{API_ORIGIN}/health/ready",
    origin=MOCK_ORIGIN,
    method="OPTIONS",
    headers={"Access-Control-Request-Method": "GET"},
)
require(status == 204 and headers.get("Access-Control-Allow-Origin") == MOCK_ORIGIN, "readiness handles mock-origin CORS preflight")

status, headers, body = request(f"{MOCK_ORIGIN}/")
require(status == 200 and "text/html" in headers.get("Content-Type", "") and b"EspaciGo" in body, "mock serves its static HTML page")

status, headers, body = request(f"{MOCK_ORIGIN}/config.json")
config = json.loads(body)
require(status == 200 and set(config) == {"apiReadyURL"}, "mock config exposes only the public readiness URL")

status, headers, body = request(f"{MOCK_ORIGIN}/app.js")
require(status == 200 and "javascript" in headers.get("Content-Type", "").lower() and b"/config.json" in body, "mock serves the compiled TypeScript module")

status, headers, body = request(f"{MOCK_ORIGIN}/styles.css")
require(status == 200 and "text/css" in headers.get("Content-Type", ""), "mock serves its static CSS")

status, headers, body = request(f"{config['apiReadyURL']}", origin=MOCK_ORIGIN)
require(status == 200 and json.loads(body) == {"status": "ready"}, "configured mock API URL is reachable over HTTP/CORS")
