#!/usr/bin/env python3
"""Domain-backed EspaciGo account and listing importer for the private fixture kit.

All writes go through the existing HTTP API. The tool never writes PostgreSQL
rows directly and keeps IDs/checkpoints only in a mode-0600 file.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import html
import json
import os
import re
import stat
import subprocess
import sys
import tempfile
import time
from collections import Counter, defaultdict
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs, quote, urlencode, urlparse
from urllib.request import HTTPRedirectHandler, Request, build_opener


ROLE_LANDLORD = "arrendador"
ROLE_ADMIN = "administrador"
MAX_REPLY = 8 * 1024 * 1024
API_CURSORS: dict[tuple[str, ...], int] = {}


class SafeHTTPError(Exception):
    def __init__(self, endpoint: str, status: int, code: str = "") -> None:
        self.endpoint = endpoint
        self.status = status
        self.code = code
        super().__init__(f"{endpoint}: HTTP {status}" + (f" ({code})" if code else ""))


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class API:
    def __init__(self, bases: list[str], token: str = "") -> None:
        self.bases = bases
        self.token = token
        self.opener = build_opener(NoRedirect())

    def request(self, method: str, path: str, body=None, key: str = "", expect=(200,)):
        raw = None if body is None else json.dumps(body, ensure_ascii=False).encode()
        last_error = None
        for offset in range(len(self.bases)):
            cursor_key = tuple(self.bases)
            start = API_CURSORS.get(cursor_key, 0)
            base = self.bases[(start + offset) % len(self.bases)]
            API_CURSORS[cursor_key] = (start + offset + 1) % len(self.bases)
            headers = {"Accept": "application/json"}
            if raw is not None:
                headers["Content-Type"] = "application/json"
            if self.token:
                headers["Authorization"] = "Bearer " + self.token
            if key:
                headers["Idempotency-Key"] = key
            request = Request(base + path, data=raw, headers=headers, method=method)
            try:
                with self.opener.open(request, timeout=30) as response:
                    data = response.read(MAX_REPLY + 1)
                    if len(data) > MAX_REPLY:
                        raise SafeHTTPError(path.split("?", 1)[0], response.status, "response_too_large")
                    if response.status not in expect:
                        raise SafeHTTPError(path.split("?", 1)[0], response.status)
                    if response.status == 204 or not data:
                        return {}
                    try:
                        return json.loads(data)
                    except (UnicodeDecodeError, json.JSONDecodeError):
                        return data.decode("utf-8", "replace")
            except HTTPError as error:
                code = ""
                try:
                    payload = json.loads(error.read(MAX_REPLY))
                    code = payload.get("error", {}).get("code", "")
                except Exception:
                    pass
                if error.code == 429:
                    last_error = SafeHTTPError(path, error.code, code)
                    continue
                raise SafeHTTPError(path.split("?", 1)[0], error.code, code) from None
            except (URLError, TimeoutError, OSError) as error:
                # A dropped connection may follow a committed write. Never replay
                # an operation against another API process without an idempotency key.
                raise SafeHTTPError(path.split("?", 1)[0], 0, "connection_failed") from None
        if last_error:
            raise last_error
        raise SafeHTTPError(path, 0, "no_api_endpoint")

    def request_bytes(self, path: str, token: str = ""):
        """Fetch binary content without decoding it as JSON or text."""
        cursor_key = tuple(self.bases)
        start = API_CURSORS.get(cursor_key, 0)
        last_status = 0
        last_headers = {}
        for offset in range(len(self.bases)):
            base = self.bases[(start + offset) % len(self.bases)]
            API_CURSORS[cursor_key] = (start + offset + 1) % len(self.bases)
            headers = {"Accept": "image/png"}
            if token:
                headers["Authorization"] = "Bearer " + token
            request = Request(base + path, headers=headers, method="GET")
            try:
                with self.opener.open(request, timeout=30) as response:
                    return response.status, dict(response.headers.items()), response.read(MAX_REPLY + 1)
            except HTTPError as error:
                last_status = error.code
                last_headers = dict(error.headers.items()) if error.headers else {}
                if error.code not in (404, 503):
                    raise SafeHTTPError(path.split("?", 1)[0], error.code) from None
            except (URLError, TimeoutError, OSError):
                raise SafeHTTPError(path.split("?", 1)[0], 0, "connection_failed") from None
        return last_status, last_headers, b""


class Mailpit:
    def __init__(self, base: str) -> None:
        self.base = base.rstrip("/")
        self.opener = build_opener(NoRedirect())

    def verification_token(self, email: str) -> tuple[str, str]:
        url = self.base + "/view/latest.txt?" + urlencode({"query": "to:" + email})
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            try:
                with self.opener.open(url, timeout=10) as response:
                    message = response.read(2 * 1024 * 1024).decode("utf-8", "replace")
                # The local identity mail uses a plaintext mock form rather
                # than a clickable URL. Extract only its labeled credentials.
                token_id_match = re.search(r"(?mi)^Token ID:\s*([^\s]+)", message)
                token_match = re.search(r"(?mi)^Token:\s*([^\s]+)", message)
                if token_id_match and token_match:
                    return token_id_match.group(1), token_match.group(1)
                for candidate in re.findall(r"https?://[^\s<>\"']+", message):
                    candidate = html.unescape(candidate).rstrip(".,);]")
                    query = parse_qs(urlparse(candidate).query)
                    token_id = query.get("token_id", query.get("tokenId", [""]))[0]
                    token = query.get("token", [""])[0]
                    if token_id and token:
                        return token_id, token
                raise RuntimeError("verification email format not recognized")
            except HTTPError as error:
                if error.code != 404:
                    raise RuntimeError("local mailbox lookup failed") from None
            except (URLError, TimeoutError, OSError):
                pass
            time.sleep(0.5)
        raise RuntimeError("verification email did not arrive in the local mailbox")


def private_json(path: Path) -> dict:
    info = path.stat()
    if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600:
        raise RuntimeError("private account policy must be a regular file with mode 0600")
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict) or not value.get("owner_test_email") or not value.get("test_password"):
        raise RuntimeError("private account policy is missing required fields")
    return value


def ensure_private_parent(path: Path) -> None:
    absolute = path.absolute()
    if ".local" not in absolute.parts or "testing" not in absolute.parts:
        raise RuntimeError("checkpoint path must stay under a private .local/testing directory")
    parent = absolute.parent
    parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(parent, 0o700)
    if parent.is_symlink() or stat.S_IMODE(parent.stat().st_mode) != 0o700:
        raise RuntimeError("checkpoint directory must be a private non-symlink directory with mode 0700")


def atomic_checkpoint(path: Path, value: dict) -> None:
    encoded = (json.dumps(value, sort_keys=True, ensure_ascii=False, indent=2) + "\n").encode()
    fd, temporary = tempfile.mkstemp(prefix=".dataset-import-", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "wb") as output:
            output.write(encoded)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        os.chmod(path, 0o600)
        dir_fd = os.open(path.parent, os.O_RDONLY | getattr(os, "O_DIRECTORY", 0))
        try:
            os.fsync(dir_fd)
        finally:
            os.close(dir_fd)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def read_jsonl(path: Path) -> list[dict]:
    values = []
    with path.open(encoding="utf-8") as source:
        for line in source:
            if line.strip():
                value = json.loads(line)
                if not isinstance(value, dict):
                    raise RuntimeError("dataset JSONL contains a non-object row")
                values.append(value)
    return values


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def preflight(repo: Path, dataset: Path) -> None:
    result = subprocess.run(
        ["go", "run", "./cmd/dataset-loader", "--dataset-dir", str(dataset)],
        cwd=repo,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    if result.returncode:
        raise RuntimeError("dataset structural preflight failed; no import actions started")


def local_url(value: str, pilot: bool, allow_shared_api: bool = False) -> str:
    parsed = urlparse(value)
    if parsed.scheme != "http" or parsed.username or parsed.password or parsed.path not in ("", "/"):
        raise RuntimeError("API and Mailpit URLs must be plain local HTTP origins")
    if parsed.hostname not in ("127.0.0.1", "localhost", "::1"):
        raise RuntimeError("only loopback APIs are accepted by this local importer")
    port = parsed.port or (80 if parsed.scheme == "http" else 443)
    if pilot and port < 10000:
        raise RuntimeError("pilot API ports must be dedicated high ports, separate from the persistent stack")
    if port == 8080 and not allow_shared_api:
        raise RuntimeError("the shared persistent API port is not accepted")
    return value.rstrip("/")


def load_sources(dataset: Path):
    assignment_dir = dataset / "para_la_ia" / "asignaciones"
    users = read_jsonl(assignment_dir / "usuarios_planificados.jsonl")
    user_content = {row["user_ref"]: row for row in read_jsonl(dataset / "salida_validada" / "usuarios.jsonl")}
    listings = read_jsonl(assignment_dir / "publicaciones_planificadas.jsonl")
    scenarios = read_jsonl(assignment_dir / "escenarios_planificados.jsonl")
    listing_content = {row["listing_ref"]: row for row in read_jsonl(dataset / "salida_validada" / "publicaciones.jsonl")}
    if len(user_content) != 1000 or len(listing_content) != 1200:
        raise RuntimeError("generated content does not match the expected reference counts")
    for user in users:
        if user["user_ref"] not in user_content:
            raise RuntimeError("a planned account is missing generated profile content")
    for listing in listings:
        if listing["listing_ref"] not in listing_content:
            raise RuntimeError("a planned publication is missing generated content")
    return users, user_content, listings, listing_content, scenarios


def choose_users(users: list[dict], mode: str, listing_assignments: list[dict], scenarios: list[dict] | None = None) -> list[dict]:
    if mode == "import":
        return sorted(users, key=lambda row: row["user_ref"])
    if mode == "deal-pilot":
        if scenarios is None:
            raise RuntimeError("deal pilot requires scenario assignments")
        requested = [
            ("ciclo_cerrado", "oficina", "hora"), ("ciclo_cerrado", "bodega", "dia"),
            ("pendiente_pago", "estacionamiento", "mes"), ("pendiente_pago", "sala_multiproposito", "hora"),
            ("cancelacion_con_devolucion", "local_flexible", "dia"), ("cancelacion_con_devolucion", "quincho", "mes"),
            ("rechazo_anfitrion", "parcela_eventos", "hora"), ("rechazo_anfitrion", "stand", "dia"),
            ("reclamo_abierto", "oficina", "mes"), ("reclamo_abierto", "bodega", "hora"),
        ]
        chosen_scenarios = []
        chosen_hosts: set[str] = set()
        chosen_renters: set[str] = set()
        used_listings: set[str] = set()
        for scenario_code, category, rate_unit in requested:
            candidates = [
                row for row in scenarios
                if row.get("scenario_code") == scenario_code
                and row.get("category_code") == category
                and row.get("rate_unit") == rate_unit
                and row.get("listing_ref") not in used_listings
            ]
            if not candidates:
                raise RuntimeError("deal pilot scenario selection is incomplete")
            choice = max(candidates, key=lambda row: (
                -(int(row["host_ref"] not in chosen_hosts) + int(row["renter_ref"] not in chosen_renters)),
                row["scenario_ref"],
            ))
            chosen_scenarios.append(choice)
            used_listings.add(choice["listing_ref"])
            chosen_hosts.add(choice["host_ref"])
            chosen_renters.add(choice["renter_ref"])
        refs = chosen_hosts | chosen_renters
        selected = [row for row in users if row["user_ref"] in refs]
        if len(selected) > 20 or len(chosen_hosts) > 10 or len(chosen_renters) > 10:
            raise RuntimeError("deal pilot account selection exceeds its isolated scope")
        if len(chosen_scenarios) != 10 or {row["category_code"] for row in chosen_scenarios} != {
            "bodega", "estacionamiento", "local_flexible", "oficina", "parcela_eventos",
            "quincho", "sala_multiproposito", "stand",
        }:
            raise RuntimeError("deal pilot must cover five scenarios and all eight categories")
        if Counter(row["rate_unit"] for row in chosen_scenarios) != Counter({"hora": 4, "dia": 3, "mes": 3}):
            raise RuntimeError("deal pilot must cover all three rate units")
        return sorted(selected, key=lambda row: row["user_ref"])
    by_role = defaultdict(list)
    for user in users:
        by_role[user["roles"][0]].append(user)
    selected = []
    hosts = sorted(by_role[ROLE_LANDLORD], key=lambda x: x["user_ref"])
    by_owner: dict[str, list[dict]] = defaultdict(list)
    for listing in listing_assignments:
        by_owner[listing["owner_ref"]].append(listing)
    required = {"publica", "oculta", "borrador", "hora", "dia", "mes"}
    chosen_hosts = []
    remaining = set(required)
    candidates = list(hosts)
    while candidates and len(chosen_hosts) < 6:
        best = max(
            candidates,
            key=lambda host: (
                len({row["intended_visibility"] for row in by_owner[host["user_ref"]]} & remaining)
                + len({row["rate_unit"] for row in by_owner[host["user_ref"]]} & remaining),
                -int(host["user_ref"][1:]),
            ),
        )
        chosen_hosts.append(best)
        covered = {row["intended_visibility"] for row in by_owner[best["user_ref"]]} | {row["rate_unit"] for row in by_owner[best["user_ref"]]}
        remaining -= covered
        candidates.remove(best)
    if remaining:
        raise RuntimeError("the pilot host sample does not cover each publication visibility and rate unit")
    selected.extend(chosen_hosts)
    selected.extend(sorted(by_role["arrendatario"], key=lambda x: x["user_ref"])[:12])
    selected.extend(sorted(by_role[ROLE_ADMIN], key=lambda x: x["user_ref"])[:2])
    if len(selected) != 20:
        raise RuntimeError("the 20-account pilot selection is incomplete")
    return sorted(selected, key=lambda row: row["user_ref"])


def generated_email(user_ref: str) -> str:
    suffix = user_ref.lower()
    if not re.fullmatch(r"u[0-9]{4}", suffix):
        raise RuntimeError("unexpected external account reference format")
    return suffix + "@dataset.invalid"


def object_data(value, key: str):
    if not isinstance(value, dict) or key not in value:
        raise RuntimeError("API response did not contain the expected resource")
    return value[key]


class Importer:
    def __init__(self, api_bases, mailpit, policy, state, dataset_id):
        self.api = API(api_bases)
        self.mailpit = Mailpit(mailpit)
        self.email = policy["owner_test_email"]
        self.password = policy["test_password"]
        self.state = state
        self.dataset_id = dataset_id
        self.admin_token = ""
        self.sessions: dict[str, dict] = {}
        self.admin_created = 0.0
        self.terms_ids = []

    def login(self, email: str, password: str, require_global=False) -> str:
        response = API(self.api.bases).request(
            "POST", "/api/v1/auth/login", {"email": email, "password": password}, expect=(200,)
        )
        token = object_data(response, "access_token")
        if not isinstance(token, str) or not token:
            raise RuntimeError("login response did not include an access token")
        # A token exists even if the role/capability check below fails, so it
        # must already be owned by this run's cleanup registry.
        self.sessions[token] = {"email": email, "active": True, "created": time.monotonic()}
        if require_global:
            roles = response.get("roles", [])
            caps = response.get("capabilities", [])
            if "administrador_global" not in roles or "legacy.administration" not in caps:
                raise RuntimeError("the private operator account is not an active global administrator")
        return token

    def admin(self) -> API:
        # GADM account-role updates require authentication within the last 10 minutes.
        if self.admin_token and time.monotonic() - self.admin_created < 8 * 60:
            return API(self.api.bases, self.admin_token)
        if self.admin_token:
            self.close_session(self.admin_token)
        self.admin_token = self.login(self.email, self.password, require_global=True)
        self.admin_created = time.monotonic()
        return API(self.api.bases, self.admin_token)

    def close_session(self, token: str) -> None:
        session = self.sessions.get(token)
        if not session or not session["active"]:
            return
        API(self.api.bases, token).request("POST", "/api/v1/auth/logout", {}, expect=(204,))
        session["active"] = False
        if token == self.admin_token:
            self.admin_token = ""

    def cleanup_sessions(self) -> int:
        failures = 0
        for token, session in list(self.sessions.items()):
            if session["active"]:
                try:
                    self.close_session(token)
                except (SafeHTTPError, OSError, RuntimeError):
                    failures += 1
        return failures

    def register_or_recover(self, user: dict, profile: dict) -> str:
        ref = user["user_ref"]
        email = generated_email(ref)
        registered_now = False
        if ref in self.state["users"]:
            account_id = self.state["users"][ref]
        else:
            attempts = self.state.setdefault("registration_attempts", {})
            attempt = attempts.get(ref)
            recovering_attempt = attempt is not None
            if attempt is None:
                if self.find_account(email) is not None:
                    raise RuntimeError("registration stopped: deterministic email already belongs to an account")
                attempt = {"email": email, "baseline_absent": True,
                           "checked_at": datetime.now(timezone.utc).isoformat()}
                attempts[ref] = attempt
            if attempt.get("email") != email or attempt.get("baseline_absent") is not True:
                raise RuntimeError("registration provenance checkpoint is invalid")
            self.state.setdefault("pending_users", {})[ref] = {"attempt": attempt["checked_at"]}
            self.save()
            preference = "ofrecer" if user["roles"][0] == ROLE_LANDLORD else "arrendar"
            try:
                response = API(self.api.bases).request(
                    "POST",
                    "/api/v1/auth/register",
                    {"email": email, "password": self.password, "use_preference": preference, "terms_version_ids": self.terms_ids},
                    expect=(201, 202),
                )
                account_id = object_data(response, "account_id")
                registered_now = True
            except SafeHTTPError as error:
                if error.status != 409:
                    raise
                if not recovering_attempt:
                    raise RuntimeError("registration conflict occurred after the baseline; refusing to adopt a concurrent account")
                account = self.find_account(email)
                if account is None or not account.get("created_at") or datetime.fromisoformat(account["created_at"].replace("Z", "+00:00")) < datetime.fromisoformat(attempt["checked_at"]):
                    raise RuntimeError("registration conflict lacks matching post-baseline provenance")
                account_id = account["account_id"]
            self.state["users"][ref] = account_id
            self.state["pending_users"].pop(ref, None)
            self.save()

        # A crash can happen after registration but before verification. Reissue
        # only if login confirms that the account is still pending.
        try:
            token = self.login(email, self.password)
            user_api = API(self.api.bases, token)
        except SafeHTTPError as error:
            if error.status != 401 and not (error.status == 403 and error.code == "email_unverified"):
                raise
            if not registered_now:
                API(self.api.bases).request(
                    "POST", "/api/v1/auth/verification/reissue", {"email": email}, expect=(204,)
                )
            token_id, verify_token = self.mailpit.verification_token(email)
            API(self.api.bases).request(
                "POST", "/api/v1/auth/verification", {"token_id": token_id, "token": verify_token}, expect=(204,)
            )
            user_api = API(self.api.bases, self.login(email, self.password))
        try:
            current_profile = user_api.request("GET", "/api/v1/profile")
            profile_matches = current_profile.get("display_name") == profile["display_name"] and current_profile.get("phone") in (None, "")
        except SafeHTTPError as error:
            if error.status != 404:
                raise
            profile_matches = False
        if not profile_matches:
            user_api.request("PUT", "/api/v1/profile", {"display_name": profile["display_name"], "phone": ""}, expect=(200,))
        role = user["roles"][0]
        if role == ROLE_LANDLORD:
            admin_api = self.admin()
            admin_api.request(
                "PUT",
                f"/api/v1/admin/global/accounts/{quote(account_id, safe='')}/roles/{quote(role, safe='')}",
                {"granted": True, "reason": "revision_administrativa"},
                expect=(200,),
            )
        elif role == ROLE_ADMIN:
            admin_api = self.admin()
            admin_api.request(
                "PUT",
                f"/api/v1/admin/global/accounts/{quote(account_id, safe='')}/legacy-administrator",
                {"granted": True, "reason": "aprovisionamiento_sintetico"},
                expect=(200,),
            )
        if role == ROLE_LANDLORD:
            self.ensure_kyc(user_api, ref)
        self.close_session(user_api.token)
        return account_id

    def find_account(self, email: str) -> dict | None:
        admin = self.admin()
        query = urlencode({"email_prefix": email, "page_size": 10})
        response = admin.request("GET", "/api/v1/admin/global/accounts?" + query)
        data = response.get("data", {})
        accounts = data.get("items", []) if isinstance(data, dict) else []
        matches = [row for row in accounts if row.get("email", "").lower() == email.lower()]
        if not matches:
            return None
        if len(matches) != 1:
            raise RuntimeError("registration conflict could not be reconciled to one account")
        return matches[0]

    def ensure_kyc(self, user_api: API, ref: str) -> None:
        if ref not in self.state["verifications"]:
            response = user_api.request(
                "POST", "/api/v1/verifications", {"type": "kyc"}, key=self.key("kyc", ref), expect=(201,)
            )
            self.state["verifications"][ref] = object_data(response, "id")
            self.save()
        case_id = self.state["verifications"][ref]
        current = user_api.request("GET", "/api/v1/verifications/" + quote(case_id, safe=""))
        if current.get("state") == "aprobada":
            return
        admin = self.admin()
        admin.request(
            "POST", "/api/v1/admin/verifications/" + quote(case_id, safe="") + "/review",
            {"decision": "aprobada"}, expect=(200,),
        )
        confirmed = user_api.request("GET", "/api/v1/verifications/" + quote(case_id, safe=""))
        if confirmed.get("state") != "aprobada":
            raise RuntimeError("synthetic KYC review did not produce the expected domain state")

    def ensure_spaces(self, host_ref: str, assignments: list[dict], content_by_ref: dict) -> None:
        account_id = self.state["users"].get(host_ref)
        if not account_id:
            raise RuntimeError("listing owner has no checkpointed account mapping")
        email = generated_email(host_ref)
        owner = API(self.api.bases, self.login(email, self.password))
        existing_response = owner.request("GET", "/api/v1/spaces")
        existing = existing_response.get("items", [])
        remaining = list(existing)
        prepared = []
        for assignment in sorted(assignments, key=lambda row: row["listing_ref"]):
            ref = assignment["listing_ref"]
            photo_id = ""
            if ref in self.state["listings"]:
                value = self.state["listings"][ref]
                space_id = value["id"]
                photo_id = value.get("photo_id", "")
            else:
                source = content_by_ref[ref]
                body = {
                    "category_code": assignment["category_code"],
                    "title": source["title"], "description": source["description"],
                    "area_m2": source["area_m2"], "capacity": source["capacity"],
                    "usage_rules": source["usage_rules"], "rate_unit": assignment["rate_unit"],
                    # Generated JSON stores CLP as a decimal string for exactness;
                    # the domain API accepts an integer CLP amount.
                    "base_price_clp": int(source["base_price_clp"]), "address": source["address"],
                    "region_code": assignment["region_code"], "comuna_code": assignment["comuna_code"],
                    "attribute_schema_version": assignment["attribute_schema_version"],
                    "attributes": source["attributes"],
                }
                match = self.find_existing_space(remaining, body)
                if match:
                    remaining.remove(match)
                    space_id = match["id"]
                else:
                    created = owner.request("POST", "/api/v1/spaces", body, expect=(201,))
                    space_id = object_data(created, "id")
                self.state["listings"][ref] = {"id": space_id}
                self.save()
            timezone = assignment["time_zone"]
            current = owner.request("GET", f"/api/v1/spaces/{quote(space_id, safe='')}")
            if current.get("time_zone") != timezone:
                owner.request("PUT", f"/api/v1/spaces/{quote(space_id, safe='')}/availability", {"time_zone": timezone}, expect=(200,))
            if assignment["rate_unit"] == "hora":
                admin = self.admin()
                schedule_path = f"/api/v1/admin/global/publications/{quote(space_id, safe='')}/schedule"
                desired_days = [
                    {"weekday": weekday, "periods": ([{"open": "09:00", "close": "18:00"}] if weekday <= 5 else [])}
                    for weekday in range(1, 8)
                ]
                current_schedule = admin.request("GET", schedule_path + "?" + urlencode({"owner_id": account_id})).get("data", {})
                if current_schedule.get("enabled") is not True or current_schedule.get("days") != desired_days or current_schedule.get("time_zone") != timezone:
                    page = admin.request("GET", "/api/v1/admin/global/publications?" + urlencode({"owner_id": account_id, "page_size": 50}))
                    publication_page = page.get("data", {})
                    publication = next((item for item in publication_page.get("items", []) if item.get("publication_id") == space_id), None)
                    if publication is None or not publication.get("updated_at"):
                        raise RuntimeError("administrator publication version was not available for schedule update")
                    admin.request(
                        "PUT", schedule_path,
                        {"owner_id": account_id, "expected_updated_at": publication["updated_at"], "reason": "revision_administrativa", "enabled": True, "days": desired_days},
                        expect=(200,),
                    )
            gallery = owner.request("GET", f"/api/v1/spaces/{quote(space_id, safe='')}/gallery").get("items", [])
            if not any(item.get("id") == photo_id for item in gallery):
                photo_id = next((item.get("id", "") for item in gallery if item.get("fixture_code") == "synthetic-png-v1"), "")
                if not photo_id:
                    photo = owner.request("POST", f"/api/v1/spaces/{quote(space_id, safe='')}/gallery", key=self.key("photo", ref), expect=(200, 201))
                    photo_id = object_data(photo, "photo")["id"]
                self.state["listings"][ref]["photo_id"] = photo_id
                self.save()

            desired = {"publica": "activa", "oculta": "oculta", "borrador": "borrador"}[assignment["intended_visibility"]]
            current = owner.request("GET", f"/api/v1/spaces/{quote(space_id, safe='')}")
            if desired == "oculta" and current.get("state") == "borrador":
                # The domain only allows hiding a listing after it has been
                # published. Follow both audited owner-domain transitions.
                owner.request("PUT", f"/api/v1/spaces/{quote(space_id, safe='')}/publication", {"state": "activa"}, expect=(200,))
                current["state"] = "activa"
            if current.get("state") != desired and desired != "borrador":
                owner.request("PUT", f"/api/v1/spaces/{quote(space_id, safe='')}/publication", {"state": desired}, expect=(200,))
            if desired == "activa" and photo_id:
                photo_id = self.ensure_photo_content(owner, space_id, ref, photo_id, gallery, public=True)
                self.state["listings"][ref]["photo_id"] = photo_id
                self.save()
            elif photo_id:
                photo_id = self.ensure_photo_content(owner, space_id, ref, photo_id, gallery, public=False)
                self.state["listings"][ref]["photo_id"] = photo_id
                self.save()
            confirmed = owner.request("GET", f"/api/v1/spaces/{quote(space_id, safe='')}")
            if confirmed.get("state") != desired or confirmed.get("time_zone") != assignment["time_zone"]:
                raise RuntimeError("listing domain state did not match the planned visibility or time zone")
            prepared.append(ref)
            self.state["listings"][ref] = {"id": space_id, "photo_id": photo_id}
            self.save()
        self.close_session(owner.token)

    def ensure_photo_content(self, owner: API, space_id: str, ref: str, photo_id: str,
                             gallery: list[dict], public: bool) -> str:
        """Ensure a checkpointed synthetic photo has durable bytes on every API instance.

        Dedicated API processes must share their configured evidence store. A
        missing private blob is repaired by creating a new domain photo with a
        stable idempotency key; metadata is never written outside the API.
        """
        path = f"/api/v1/spaces/{quote(space_id, safe='')}/gallery"
        # Refresh after any create/recovery earlier in this run so newly added
        # metadata is included without trusting a stale pre-write page.
        items = owner.request("GET", path).get("items", [])
        current = next((item for item in items if item.get("id") == photo_id), None)
        if current is None:
            raise RuntimeError("checkpoint photo is absent from the owner gallery")

        def content_is_valid(candidate_id: str, metadata: dict) -> bool:
            content_path = path + f"/{quote(candidate_id, safe='')}/content"
            for base in owner.bases:
                status, headers, blob = API([base]).request_bytes(content_path, owner.token)
                # The gallery service maps an absent private file to 503 after
                # loading valid gallery metadata. Treat it as missing content
                # here so the idempotent replacement path can repair the
                # metadata/file mismatch; a failed replacement write still
                # fails the subsequent readability check.
                if status == 503:
                    return False
                if status != 200:
                    return False
                expected = metadata.get("sha256")
                mime = headers.get("Content-Type", "").split(";", 1)[0].lower()
                if (mime != "image/png" or len(blob) > MAX_REPLY or not blob.startswith(b"\x89PNG\r\n\x1a\n")
                        or not expected or hashlib.sha256(blob).hexdigest() != expected
                        or metadata.get("size_bytes") != len(blob)):
                    return False
            return True

        previous_id = photo_id
        if not content_is_valid(photo_id, current):
            state = self.state["listings"][ref]
            replacement_id = state.get("replacement_photo_id", "")
            replacement = next((item for item in items if item.get("id") == replacement_id), None)
            if replacement is None or not content_is_valid(replacement_id, replacement):
                created = owner.request("POST", path, key=self.key("photo-recovery", ref), expect=(200, 201))
                replacement = object_data(created, "photo")
                replacement_id = replacement.get("id", "")
                if not replacement_id or not content_is_valid(replacement_id, replacement):
                    raise RuntimeError("new synthetic gallery photo is not readable from every API instance")
                state["replacement_photo_id"] = replacement_id
                state["retire_photo_id"] = photo_id
                self.save()
                items.append(replacement)
            photo_id = replacement_id

        if public:
            public_photo = next((item for item in items if item.get("id") == photo_id), None)
            if public_photo is None or public_photo.get("public_in_catalog") is not True:
                owner.request(
                    "PUT", path + f"/{quote(photo_id, safe='')}/publication",
                    {"public_in_catalog": True}, expect=(200,),
                )
            cover = owner.request("PUT", f"/api/v1/spaces/{quote(space_id, safe='')}/cover",
                                  {"photo_id": photo_id}, expect=(200,))
            if cover.get("photo_id") != photo_id or cover.get("selected") is not True:
                raise RuntimeError("active listing did not confirm its synthetic cover")
            public_path = f"/api/v1/public/spaces/{quote(space_id, safe='')}/images/{quote(photo_id, safe='')}"
            selected = next(item for item in items if item.get("id") == photo_id)
            for base in owner.bases:
                status, headers, blob = API([base]).request_bytes(public_path)
                mime = headers.get("Content-Type", "").split(";", 1)[0].lower()
                if (status != 200 or mime != "image/png" or not blob.startswith(b"\x89PNG\r\n\x1a\n")
                        or hashlib.sha256(blob).hexdigest() != selected.get("sha256")):
                    raise RuntimeError("public synthetic cover is not readable from every API instance")
        else:
            private_photo = next((item for item in items if item.get("id") == photo_id), None)
            if private_photo is not None and private_photo.get("public_in_catalog") is True:
                owner.request("PUT", path + f"/{quote(photo_id, safe='')}/publication",
                              {"public_in_catalog": False}, expect=(200,))

        if photo_id != previous_id:
            # Save the replacement mapping before retiring the broken fixture;
            # a retry can resume through the same idempotency key.
            self.state["listings"][ref]["photo_id"] = photo_id
            self.save()
        state = self.state["listings"][ref]
        retired_id = state.get("retire_photo_id", "")
        if retired_id and retired_id != photo_id:
            refreshed = owner.request("GET", path).get("items", [])
            if any(item.get("id") == retired_id for item in refreshed):
                owner.request("DELETE", path + f"/{quote(retired_id, safe='')}", expect=(200,))
            state.pop("retire_photo_id", None)
            state.pop("replacement_photo_id", None)
            self.save()
        return photo_id

    def repair_photos(self, assignments: list[dict]) -> tuple[int, int]:
        grouped = defaultdict(list)
        for row in assignments:
            grouped[row["owner_ref"]].append(row)
        repaired = active_covers = 0
        for host_ref in sorted(grouped):
            if not self.state["users"].get(host_ref):
                raise RuntimeError("active listing owner is missing from the private checkpoint")
            owner = API(self.api.bases, self.login(generated_email(host_ref), self.password))
            for assignment in sorted(grouped[host_ref], key=lambda row: row["listing_ref"]):
                ref = assignment["listing_ref"]
                listing = self.state["listings"].get(ref, {})
                space_id, photo_id = listing.get("id"), listing.get("photo_id")
                if not space_id or not photo_id:
                    raise RuntimeError("active listing is missing its checkpointed API references")
                gallery = owner.request("GET", f"/api/v1/spaces/{quote(space_id, safe='')}/gallery").get("items", [])
                if not any(item.get("id") == photo_id for item in gallery):
                    raise RuntimeError("checkpoint photo is absent from the owner gallery")
                is_public = assignment["intended_visibility"] == "publica"
                new_id = self.ensure_photo_content(owner, space_id, ref, photo_id, gallery, public=is_public)
                listing["photo_id"] = new_id
                self.save()
                repaired += 1
                active_covers += int(is_public)
            self.close_session(owner.token)
        return repaired, active_covers

    @staticmethod
    def find_existing_space(candidates: list[dict], body: dict):
        for item in candidates:
            if all(item.get(key) == body.get(key) for key in (
                "category_code", "title", "description", "area_m2", "capacity", "usage_rules",
                "rate_unit", "base_price_clp", "address", "region_code", "comuna_code",
                "attribute_schema_version", "attributes",
            )):
                return item
        return None

    @staticmethod
    def key(action: str, ref: str) -> str:
        return f"dataset-1000-v1:{action}:{ref}"

    def save(self) -> None:
        atomic_checkpoint(self.state_path, self.state)

    @property
    def state_path(self) -> Path:
        return self._state_path


def verify_target(mode: str, urls: list[str], args) -> None:
    for value in urls:
        local_url(value, pilot=(mode in ("pilot", "deal-pilot")), allow_shared_api=(mode == "repair-images"))
    if mode == "import":
        if len(urls) < 3:
            raise RuntimeError("persistent account import requires three dedicated API processes to stay within the per-IP verification quota")
        if args.confirm_target != "espacigo_local" or not args.backup_manifest:
            raise RuntimeError("persistent mode requires --confirm-target espacigo_local and a private backup manifest")
        manifest_path = Path(args.backup_manifest)
        if not manifest_path.is_file() or stat.S_IMODE(manifest_path.stat().st_mode) != 0o600:
            raise RuntimeError("backup manifest must be a private regular file with mode 0600")
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        backup = manifest_path.parent / manifest.get("backup_file", "")
        if not backup.is_file() or sha256(backup) != manifest.get("backup_sha256"):
            raise RuntimeError("backup dump is missing or its hash does not match the private manifest")
    if mode == "repair-images":
        if len(urls) != 1 or args.confirm_target != "espacigo_local" or not args.backup_manifest:
            raise RuntimeError("image repair requires one confirmed persistent API and the verified backup manifest")
        manifest_path = Path(args.backup_manifest)
        if not manifest_path.is_file() or stat.S_IMODE(manifest_path.stat().st_mode) != 0o600:
            raise RuntimeError("backup manifest must be a private regular file with mode 0600")
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        backup = manifest_path.parent / manifest.get("backup_file", "")
        if not backup.is_file() or sha256(backup) != manifest.get("backup_sha256"):
            raise RuntimeError("backup dump is missing or its hash does not match the private manifest")


def main() -> int:
    parser = argparse.ArgumentParser(description="Domain-backed EspaciGo synthetic dataset loader")
    parser.add_argument("--mode", choices=("pilot", "deal-pilot", "import", "repair-images"), required=True)
    parser.add_argument("--repo", required=True, help="repository root containing cmd/dataset-loader")
    parser.add_argument("--dataset-dir", required=True)
    parser.add_argument("--api-base", action="append", required=True, help="dedicated loopback API origin; may be repeated")
    parser.add_argument("--mailpit-base", required=True)
    parser.add_argument("--policy-file", required=True, help="private 0600 local account policy")
    parser.add_argument("--state-file", required=True, help="private mapping/checkpoint path under .local/testing")
    parser.add_argument("--confirm-target", default="")
    parser.add_argument("--backup-manifest", default="")
    args = parser.parse_args()

    importer = None
    try:
        repo = Path(args.repo).resolve(strict=True)
        dataset = Path(args.dataset_dir).resolve(strict=True)
        policy_path = Path(args.policy_file).resolve(strict=True)
        state_path = Path(args.state_file).absolute()
        isolated_pilot = args.mode in ("pilot", "deal-pilot")
        bases = [local_url(value, pilot=isolated_pilot, allow_shared_api=args.mode == "repair-images") for value in args.api_base]
        mailpit = local_url(args.mailpit_base, pilot=isolated_pilot)
        verify_target(args.mode, bases, args)
        preflight(repo, dataset)
        policy = private_json(policy_path)
        ensure_private_parent(state_path)

        users, user_content, listing_assignments, listing_content, scenarios = load_sources(dataset)
        selected = choose_users(users, "import" if args.mode == "repair-images" else args.mode, listing_assignments, scenarios)
        selected_refs = {user["user_ref"] for user in selected}
        selected_listings = [row for row in listing_assignments if row["owner_ref"] in selected_refs]
        user_counts = Counter(user["roles"][0] for user in selected)
        hashes = {
            "manifest": sha256(dataset / "manifiesto.json"),
            "users": sha256(dataset / "salida_validada" / "usuarios.jsonl"),
            "user_assignments": sha256(dataset / "para_la_ia" / "asignaciones" / "usuarios_planificados.jsonl"),
            "listings": sha256(dataset / "salida_validada" / "publicaciones.jsonl"),
            "listing_assignments": sha256(dataset / "para_la_ia" / "asignaciones" / "publicaciones_planificadas.jsonl"),
        }
        if args.mode == "deal-pilot":
            hashes["scenario_assignments"] = sha256(dataset / "para_la_ia" / "asignaciones" / "escenarios_planificados.jsonl")
        if state_path.exists():
            if state_path.is_symlink():
                raise RuntimeError("checkpoint must not be a symlink")
            if stat.S_IMODE(state_path.stat().st_mode) != 0o600:
                raise RuntimeError("existing checkpoint must have mode 0600")
            state = json.loads(state_path.read_text(encoding="utf-8"))
            if state.get("dataset_id") != "espacigo-1000-v1" or state.get("source_hashes") != hashes:
                raise RuntimeError("checkpoint belongs to a different dataset content hash")
        else:
            state = {"format_version": 1, "dataset_id": "espacigo-1000-v1", "source_hashes": hashes,
                     "users": {}, "pending_users": {}, "registration_attempts": {}, "verifications": {}, "listings": {}}
        importer = Importer(bases, mailpit, policy, state, "espacigo-1000-v1")
        importer._state_path = state_path
        terms_response = API(bases).request("GET", "/api/v1/auth/terms")
        active_terms = terms_response.get("items", [])
        importer.terms_ids = [item.get("id") for item in active_terms if item.get("id")]
        if not importer.terms_ids or not any(item.get("type") == "terminos" for item in active_terms):
            raise RuntimeError("active synthetic terms were not available from the identity service")
        importer.api.request("GET", "/health/ready")
        if args.mode != "repair-images":
            importer.admin()
        importer.save()

        if args.mode == "repair-images":
            repaired, active_covers = importer.repair_photos(listing_assignments)
            print(json.dumps({"mode": args.mode, "photos_repaired_or_verified": repaired,
                              "active_covers_verified": active_covers,
                              "note": "domain API operations only; no PostgreSQL row mutations"}, sort_keys=True))
            return 0

        # Register accounts and apply role/KYC transitions through the existing APIs.
        for user in selected:
            importer.register_or_recover(user, user_content[user["user_ref"]])

        grouped = defaultdict(list)
        for row in selected_listings:
            grouped[row["owner_ref"]].append(row)
        for host_ref in sorted(grouped):
            importer.ensure_spaces(host_ref, grouped[host_ref], listing_content)

        result = {
            "mode": args.mode,
            "dataset_id": "espacigo-1000-v1",
            "accounts_checkpointed": len(selected),
            "roles": dict(user_counts),
            "listings_checkpointed": len(selected_listings),
            "note": "account/listing operations through Go APIs; deal-pilot accounts are a scenario-balanced input for dataset-deals.py",
        }
        print(json.dumps(result, ensure_ascii=False, sort_keys=True))
        return 0
    except (RuntimeError, SafeHTTPError, OSError, ValueError, KeyError, json.JSONDecodeError) as error:
        # Never include remote response bodies, account data, token, or exception repr.
        if isinstance(error, SafeHTTPError):
            print(f"dataset import stopped at {error.endpoint}: HTTP {error.status} ({error.code})", file=sys.stderr)
        else:
            print(f"dataset import stopped: {str(error)}", file=sys.stderr)
        return 2
    finally:
        if importer is not None:
            failures = importer.cleanup_sessions()
            if failures:
                print(f"dataset import session cleanup incomplete: {failures} session(s)", file=sys.stderr)
                raise SystemExit(2)


if __name__ == "__main__":
    raise SystemExit(main())
