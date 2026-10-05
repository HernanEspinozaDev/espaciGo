#!/usr/bin/env python3
"""Local M02 HTTP check. Run only against an isolated disposable prototype stack."""
import json
import time
import urllib.error
import urllib.request
import uuid

API = "http://127.0.0.1:18080"
MAIL = "http://127.0.0.1:18025"


def call(method, url, payload=None, token=None, expected=200):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(url, data=None if payload is None else json.dumps(payload).encode(),
                                     method=method, headers=headers)
    try:
        response = urllib.request.urlopen(request, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    body = response.read()
    assert response.status == expected, f"{method} {url}: HTTP {response.status}, expected {expected}"
    return json.loads(body) if body else None


terms = call("GET", API + "/api/v1/auth/terms")
term_ids = [item["id"] for item in terms["items"] if item["type"] == "terminos"]
email = f"m02-{uuid.uuid4().hex}@ejemplo.invalid"
password = "Synthetic#123"
registration = call("POST", API + "/api/v1/auth/register",
                    {"email": email, "password": password, "terms_version_ids": term_ids}, expected=201)

deadline = time.monotonic() + 10
message = None
while time.monotonic() < deadline:
    listing = call("GET", MAIL + "/api/v1/messages")
    for item in listing.get("messages", []):
        if any(recipient["Address"].lower() == email.lower() for recipient in item.get("To", [])):
            message = call("GET", MAIL + "/api/v1/message/" + item["ID"])
            break
    if message:
        break
    time.sleep(.1)
assert message, "development mailbox did not receive registration message"
token_id, raw_token = None, None
for line in message["Text"].splitlines():
    if line.startswith("Token ID: "):
        token_id = line.removeprefix("Token ID: ")
    if line.startswith("Token: "):
        raw_token = line.removeprefix("Token: ")
assert token_id and raw_token, "verification token missing from local message"
call("POST", API + "/api/v1/auth/verification", {"token_id": token_id, "token": raw_token}, expected=204)
login = call("POST", API + "/api/v1/auth/login", {"email": email, "password": password})
token = login["access_token"]
assert login["account_id"] == registration["account_id"]

call("GET", API + "/api/v1/profile", token=token, expected=404)
call("PUT", API + "/api/v1/profile", {"display_name": "Synthetic Profile", "phone": "123456789"}, token, 200)
profile = call("GET", API + "/api/v1/profile", token=token)
assert profile["display_name"] == "Synthetic Profile" and profile["phone"] == "123456789"
call("PUT", API + "/api/v1/profile", {"display_name": "Bad phone", "phone": "12"}, token, 422)
call("GET", API + "/api/v1/rights-requests", token=token)
request = call("POST", API + "/api/v1/rights-requests", {"type": "supresion"}, token, 202)
assert request["state"] == "en_revision", "suppression must remain pending review"
listed = call("GET", API + "/api/v1/rights-requests", token=token)
assert len(listed["items"]) == 1 and listed["items"][0]["id"] == request["id"]
call("GET", API + "/api/v1/profile", token="invalid-session", expected=401)
print("PASS M02 local HTTP: own profile read/update, validation, rights request pending review")
