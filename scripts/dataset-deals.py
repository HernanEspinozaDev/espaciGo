#!/usr/bin/env python3
"""Run the EspaciGo synthetic deal scenarios through the local Go API only."""

from __future__ import annotations

import argparse
from collections import Counter
from datetime import date, datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode, urlparse
from urllib.request import HTTPRedirectHandler, Request, build_opener


BASE = "/api/v1/local/booking-trial"
MAX_REPLY = 8 * 1024 * 1024
PILOT_ROWS = [
    ("ciclo_cerrado", "oficina", "hora"), ("ciclo_cerrado", "bodega", "dia"),
    ("pendiente_pago", "estacionamiento", "mes"), ("pendiente_pago", "sala_multiproposito", "hora"),
    ("cancelacion_con_devolucion", "local_flexible", "dia"), ("cancelacion_con_devolucion", "quincho", "mes"),
    ("rechazo_anfitrion", "parcela_eventos", "hora"), ("rechazo_anfitrion", "stand", "dia"),
    ("reclamo_abierto", "oficina", "mes"), ("reclamo_abierto", "bodega", "hora"),
]


class SafeHTTPError(Exception):
    def __init__(self, path: str, status: int, code: str = "") -> None:
        self.path, self.status, self.code = path.split("?", 1)[0], status, code
        super().__init__(f"{self.path}: HTTP {status}" + (f" ({code})" if code else ""))


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class API:
    def __init__(self, base: str) -> None:
        self.base = base.rstrip("/")
        self.opener = build_opener(NoRedirect())

    def request(self, method: str, path: str, token: str = "", body=None, key: str = "", expect=(200,)):
        raw = None if body is None else json.dumps(body, ensure_ascii=False).encode()
        headers = {"Accept": "application/json"}
        if raw is not None:
            headers["Content-Type"] = "application/json"
        if token:
            headers["Authorization"] = "Bearer " + token
        if key:
            headers["Idempotency-Key"] = key
        request = Request(self.base + path, data=raw, headers=headers, method=method)
        try:
            with self.opener.open(request, timeout=30) as response:
                payload = response.read(MAX_REPLY + 1)
                if len(payload) > MAX_REPLY:
                    raise SafeHTTPError(path, response.status, "response_too_large")
                if response.status not in expect:
                    raise SafeHTTPError(path, response.status)
                return decode(payload)
        except HTTPError as error:
            code = ""
            try:
                decoded = json.loads(error.read(MAX_REPLY))
                code = decoded.get("error", {}).get("code", "")
            except Exception:
                pass
            raise SafeHTTPError(path, error.code, code) from None
        except (URLError, TimeoutError, OSError) as error:
            raise RuntimeError(f"API request failed at {path.split('?', 1)[0]}") from None


def decode(raw: bytes):
    if not raw:
        return {}
    try:
        return json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError):
        raise RuntimeError("API returned invalid JSON") from None


def data(value, key: str | None = None):
    if not isinstance(value, dict) or not isinstance(value.get("data"), dict):
        raise RuntimeError("API response omitted its data object")
    result = value["data"]
    if key is not None:
        if key not in result:
            raise RuntimeError("API response omitted a required domain field")
        return result[key]
    return result


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def private_json(path: Path) -> dict:
    if path.is_symlink():
        raise RuntimeError("private file cannot be a symlink")
    info = path.stat()
    if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600:
        raise RuntimeError("private file must be a regular file with mode 0600")
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        raise RuntimeError("private JSON file is invalid") from None


def checkpoint(path: Path, value: dict) -> None:
    if ".local" not in path.parts or "testing" not in path.parts:
        raise RuntimeError("deal checkpoint must stay under .local/testing")
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(path.parent, 0o700)
    if path.parent.is_symlink() or stat.S_IMODE(path.parent.stat().st_mode) != 0o700:
        raise RuntimeError("checkpoint directory must be private and non-symlink")
    fd, temporary = tempfile.mkstemp(prefix=".dataset-deals-", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            json.dump(value, stream, ensure_ascii=False, indent=2, sort_keys=True)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        os.chmod(path, 0o600)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def read_jsonl(path: Path) -> list[dict]:
    result = []
    with path.open(encoding="utf-8") as stream:
        for line in stream:
            if line.strip():
                value = json.loads(line)
                if not isinstance(value, dict):
                    raise RuntimeError("scenario assignment JSONL is invalid")
                result.append(value)
    return result


def choose_pilot_rows(rows: list[dict]) -> list[dict]:
    chosen, hosts, renters, listings = [], set(), set(), set()
    for code, category, unit in PILOT_ROWS:
        candidates = [r for r in rows if r["scenario_code"] == code and r["category_code"] == category and r["rate_unit"] == unit and r["listing_ref"] not in listings]
        if not candidates:
            raise RuntimeError("the approved ten-scenario pilot cannot be selected from the source manifest")
        row = max(candidates, key=lambda r: (-(int(r["host_ref"] not in hosts) + int(r["renter_ref"] not in renters)), r["scenario_ref"]))
        chosen.append(row)
        hosts.add(row["host_ref"])
        renters.add(row["renter_ref"])
        listings.add(row["listing_ref"])
    if len(chosen) != 10 or len({r["scenario_code"] for r in chosen}) != 5 or len({r["category_code"] for r in chosen}) != 8 or Counter(r["rate_unit"] for r in chosen) != Counter({"hora": 4, "dia": 3, "mes": 3}):
        raise RuntimeError("pilot selection must cover ten rows, five scenarios, eight categories, and all rate units")
    return chosen


def select_scenarios(mode: str, assignments: list[dict]) -> list[dict]:
    if mode == "pilot":
        return choose_pilot_rows(assignments)
    if mode == "import":
        if len(assignments) != 1200:
            raise RuntimeError("persistent scenario import requires all 1,200 manifest rows")
        return assignments
    raise RuntimeError("unknown scenario selection mode")


def read_sources(dataset: Path, source_state: Path):
    if source_state.is_symlink():
        raise RuntimeError("source checkpoint cannot be a symlink")
    if stat.S_IMODE(source_state.stat().st_mode) != 0o600:
        raise RuntimeError("source checkpoint must have mode 0600")
    state = private_json(source_state)
    if state.get("dataset_id") != "espacigo-1000-v1" or not isinstance(state.get("users"), dict) or not isinstance(state.get("listings"), dict):
        raise RuntimeError("source checkpoint is not a dataset-1000 account/listing checkpoint")
    assignments = read_jsonl(dataset / "para_la_ia/asignaciones/escenarios_planificados.jsonl")
    listings = read_jsonl(dataset / "para_la_ia/asignaciones/publicaciones_planificadas.jsonl")
    listing_map = {row["listing_ref"]: row for row in listings}
    if len(assignments) != 1200 or len(listing_map) != 1200:
        raise RuntimeError("scenario and listing manifests must contain all 1,200 assignments")
    return state, assignments, listing_map


class Runner:
    def __init__(self, api: API, args, policy: dict, source_state: dict, rows: list[dict], listing_map: dict, state: dict):
        self.api, self.args, self.policy = api, args, policy
        self.source_state, self.rows, self.listing_map, self.state = source_state, rows, listing_map, state
        self.sessions: dict[str, str] = {}

    def save(self):
        checkpoint(Path(self.args.state_file), self.state)

    def login(self, email: str, password: str, global_admin: bool = False) -> str:
        if email in self.sessions:
            return self.sessions[email]
        result = self.api.request("POST", "/api/v1/auth/login", body={"email": email, "password": password}, expect=(200,))
        token = result.get("access_token") if isinstance(result, dict) else None
        if not isinstance(token, str) or not token:
            raise RuntimeError("login response did not include an access token")
        self.sessions[email] = token
        if global_admin and ("administrador_global" not in result.get("roles", []) or "legacy.administration" not in result.get("capabilities", [])):
            raise RuntimeError("release actor is not the authorized global administrator")
        return token

    def user(self, ref: str) -> str:
        return self.login(ref.lower() + "@dataset.invalid", self.policy["test_password"])

    def admin(self) -> str:
        return self.login(self.policy["owner_test_email"], self.policy["test_password"], global_admin=True)

    def ensure_kyc(self, ref: str, token: str):
        verification_id = self.source_state.get("verifications", {}).get(ref)
        verification_id = verification_id or self.state.setdefault("verifications", {}).get(ref)
        if not verification_id:
            response = self.call("POST", "/api/v1/verifications", token, {"type": "kyc"}, key=f"dataset-deal-kyc:{ref}", expect=(201,))
            verification_id = response.get("id") if isinstance(response, dict) else None
            if not verification_id:
                raise RuntimeError("verification request did not return a case ID")
            self.state.setdefault("verifications", {})[ref] = verification_id
            self.save()
        current = self.call("GET", f"/api/v1/verifications/{quote(verification_id, safe='')}", token)
        if current.get("state") != "aprobada":
            admin = self.admin()
            self.call("POST", f"/api/v1/admin/verifications/{quote(verification_id, safe='')}/review", admin, {"decision": "aprobada"}, expect=(200,))
            current = self.call("GET", f"/api/v1/verifications/{quote(verification_id, safe='')}", token)
        if current.get("state") != "aprobada":
            raise RuntimeError("synthetic KYC review did not become approved")

    def ensure_contract_signed(self, contract_id: str, user_ref: str, token: str):
        user_id = self.source_state["users"][user_ref]
        path = f"{BASE}/contracts/{quote(contract_id, safe='')}"
        contract = data(self.call("GET", path, token))
        signature = next((item for item in contract.get("signatures", []) if item.get("signer_id") == user_id), None)
        if signature is None:
            raise RuntimeError("synthetic contract omitted the participant signature")
        if signature.get("state") == "firmada":
            return
        if signature.get("state") != "pendiente":
            raise RuntimeError("synthetic contract participant signature is not pending")
        self.call("POST", f"{path}/sign", token, expect=(200,))
        contract = data(self.call("GET", path, token))
        signature = next((item for item in contract.get("signatures", []) if item.get("signer_id") == user_id), None)
        if signature is None or signature.get("state") != "firmada":
            raise RuntimeError("synthetic contract signature did not persist")

    def call(self, method: str, path: str, token: str, body=None, key="", expect=(200,)):
        try:
            return self.api.request(method, path, token, body, key, expect)
        except SafeHTTPError as error:
            if error.status != 401:
                raise
            email = next((name for name, current in self.sessions.items() if current == token), None)
            if email is None:
                raise
            del self.sessions[email]
            replacement = self.login(email, self.policy["test_password"], global_admin=email == self.policy["owner_test_email"])
            return self.api.request(method, path, replacement, body, key, expect)

    def clock(self, token: str) -> datetime:
        value = data(self.call("GET", BASE + "/business-clock", token), "now")
        api_now = datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)
        if api_now != self.clock_helper():
            raise RuntimeError("API business clock does not match the isolated run clock")
        return api_now

    def clock_helper(self) -> datetime:
        command = [sys.executable, str(Path(self.args.repo) / "scripts/local-close-02-business-clock"), self.args.clock_run_id, "show"]
        env = os.environ.copy()
        env["LOCAL_CLOSE02_CLOCK_KIND"] = "run"
        result = subprocess.run(command, cwd=self.args.repo, env=env, capture_output=True, text=True, check=False)
        if result.returncode:
            raise RuntimeError("the run-scoped business clock is unavailable")
        try:
            value = json.loads(result.stdout)["now"]
            return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)
        except (json.JSONDecodeError, KeyError, ValueError):
            raise RuntimeError("the run-scoped business clock is invalid") from None

    def set_clock(self, value: datetime):
        if value.astimezone(timezone.utc) < self.clock_helper():
            raise RuntimeError("the isolated business clock cannot move backwards")
        command = [sys.executable, str(Path(self.args.repo) / "scripts/local-close-02-business-clock"), self.args.clock_run_id, "set", value.isoformat(timespec="seconds").replace("+00:00", "Z")]
        env = os.environ.copy()
        env["LOCAL_CLOSE02_CLOCK_KIND"] = "run"
        result = subprocess.run(command, cwd=self.args.repo, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
        if result.returncode:
            raise RuntimeError("the run-scoped business clock could not be advanced")
        if self.clock_helper() != value.astimezone(timezone.utc).replace(microsecond=0):
            raise RuntimeError("the isolated business clock did not reach the requested checkpoint")

    def interval(self, row: dict, listing: dict, space_id: str, renter: str):
        unit = row["rate_unit"]
        if unit not in ("hora", "dia", "mes") or unit != listing["rate_unit"]:
            raise RuntimeError("scenario rate unit differs from its listing")
        now = self.clock(renter)
        zone = listing["time_zone"]
        from zoneinfo import ZoneInfo
        local_now = now.astimezone(ZoneInfo(zone))
        for offset in range(1, 81):
            day = local_now.date() + timedelta(days=offset)
            path = f"{BASE}/catalog/{quote(space_id, safe='')}/availability-options?" + urlencode({"date": day.isoformat(), "duration": 1})
            result = data(self.call("GET", path, renter))
            if result.get("time_zone") != zone or result.get("rate_unit") != unit:
                raise RuntimeError("availability response does not match the listing assignment")
            options = result.get("items", [])
            if options:
                first = options[0]
                start = datetime.fromisoformat(first["start_at"].replace("Z", "+00:00"))
                end = datetime.fromisoformat(first["end_at"].replace("Z", "+00:00"))
                return start.astimezone(timezone.utc), end.astimezone(timezone.utc)
        raise RuntimeError("no compatible API availability interval was found in 80 days")

    def create_reservation_from_intent(self, row: dict, record: dict, listing: dict, space_id: str, renter: str):
        ref = row["scenario_ref"]
        intent = record.get("reservation_intent")
        if intent is None:
            start, end = self.interval(row, listing, space_id, renter)
            quote_result = self.call("POST", BASE + "/quotes", renter, {
                "space_id": space_id,
                "start_at": start.isoformat().replace("+00:00", "Z"),
                "end_at": end.isoformat().replace("+00:00", "Z"),
            }, expect=(200,))
            quote = data(quote_result)
            intent = {
                "quote_id": quote["id"],
                "idempotency_key": f"dataset-deal:{ref}:reservation",
                "payload": {"quote_id": quote["id"]},
            }
            record["reservation_intent"] = intent
            self.save()
        if intent.get("idempotency_key") != f"dataset-deal:{ref}:reservation" or intent.get("payload") != {"quote_id": intent.get("quote_id")}:
            raise RuntimeError("reservation intent checkpoint is invalid")
        try:
            result = self.call("POST", f"{BASE}/reservations", renter, intent["payload"], key=intent["idempotency_key"], expect=(201,))
        except SafeHTTPError as error:
            raise RuntimeError(f"scenario {ref} reservation rejected by API ({error.status}:{error.code})") from None
        return data(result)

    def reserve_and_pay(self, row: dict, pending: bool = False):
        ref = row["scenario_ref"]
        record = self.state["scenarios"].setdefault(ref, {"scenario_code": row["scenario_code"], "listing_ref": row["listing_ref"]})
        renter = self.user(row["renter_ref"])
        self.ensure_kyc(row["renter_ref"], renter)
        host = self.user(row["host_ref"])
        self.ensure_kyc(row["host_ref"], host)
        listing = self.listing_map[row["listing_ref"]]
        space_id = self.source_state["listings"][row["listing_ref"]]["id"]
        if not record.get("reservation_id"):
            reservation = self.create_reservation_from_intent(row, record, listing, space_id, renter)
            record.update({"reservation_id": reservation["id"], "space_id": space_id, "start_at": reservation["start_at"], "end_at": reservation["end_at"], "time_zone": reservation["time_zone"]})
            self.save()
        if pending:
            detail = data(self.call("GET", f"{BASE}/reservations/{record['reservation_id']}", renter))
            if detail.get("state") != "pendiente_de_pago":
                raise RuntimeError("pending payment scenario is not in pending state")
            record["state"] = detail["state"]
            self.save()
            return record
        reservation_id = record["reservation_id"]
        if not record.get("paid"):
            self.call("POST", f"{BASE}/reservations/{reservation_id}/payment", renter, {"outcome": "exito"}, key=f"dataset-deal:{ref}:payment", expect=(200,))
            record["paid"] = True
            self.save()
        if not record.get("guarantee_authorized"):
            self.call("POST", f"{BASE}/reservations/{reservation_id}/guarantee", renter, {"kind": "autorizacion", "amount_clp": 50000, "outcome": "exito"}, key=f"dataset-deal:{ref}:guarantee:authorization", expect=(202,))
            record["guarantee_authorized"] = True
            self.save()
        detail = data(self.call("GET", f"{BASE}/reservations/{reservation_id}", renter))
        if row["scenario_code"] == "rechazo_anfitrion":
            if detail.get("state") != "rechazada_arrendador":
                self.call("POST", f"{BASE}/reservations/{reservation_id}/decision", host, {"decision": "rechazar", "reason": "No compatible con disponibilidad sintética"}, expect=(200,))
                detail = data(self.call("GET", f"{BASE}/reservations/{reservation_id}", renter))
            if detail.get("state") != "rechazada_arrendador":
                raise RuntimeError("host rejection did not produce its expected reservation state")
            record["state"] = detail["state"]
            self.save()
            return record
        if detail.get("state") not in ("aprobada_host", "firma_parcial", "lista_para_checkin", "en_curso", "finalizada", "cerrada", "en_disputa", "cancelada_arrendatario", "cancelada_por_rechazo", "cancelada_por_firma", "rechazada_arrendador"):
            self.call("POST", f"{BASE}/reservations/{reservation_id}/decision", host, {"decision": "aprobar"}, expect=(200,))
            detail = data(self.call("GET", f"{BASE}/reservations/{reservation_id}", renter))
        if row["scenario_code"] == "cancelacion_con_devolucion":
            if detail.get("state") != "cancelada_arrendatario":
                self.call("POST", f"{BASE}/reservations/{reservation_id}/cancel", renter, {"reason": "Cancelación sintética del dataset"}, key=f"dataset-deal:{ref}:cancel", expect=(200,))
                detail = data(self.call("GET", f"{BASE}/reservations/{reservation_id}", renter))
            operation = detail.get("refund_operation_id")
            if not operation:
                raise RuntimeError("cancelled reservation omitted the durable refund operation")
            if detail.get("refund_state") != "completada":
                refund = data(self.call("POST", f"{BASE}/reservations/{reservation_id}/refund", renter, {"outcome": "exito"}, key=operation, expect=(200,)))
                if refund.get("state") != "completada":
                    raise RuntimeError("cancellation refund did not complete")
            record["state"] = detail["state"]
            record["refund_operation_id"] = operation
            self.save()
            return record
        contract_response = self.call("POST", f"{BASE}/reservations/{reservation_id}/contract", host, expect=(200,))
        contract = data(contract_response)
        record["contract_id"] = contract["id"]
        self.save()
        self.ensure_contract_signed(contract["id"], row["host_ref"], host)
        self.ensure_contract_signed(contract["id"], row["renter_ref"], renter)
        record["contract_signed"] = True
        self.save()
        return record

    def operate(self, row: dict):
        ref = row["scenario_ref"]
        record = self.state["scenarios"][ref]
        reservation_id = record["reservation_id"]
        renter, host = self.user(row["renter_ref"]), self.user(row["host_ref"])
        start = datetime.fromisoformat(record["start_at"].replace("Z", "+00:00")).astimezone(timezone.utc)
        if self.clock(renter) < start:
            self.set_clock(start)
        if not record.get("check_in"):
            self.call("POST", f"{BASE}/reservations/{reservation_id}/check-in", renter, {"comments": "Ingreso sintético del dataset"}, key=f"dataset-deal:{ref}:check-in", expect=(200, 201))
            record["check_in"] = True
            self.save()
        if not record.get("check_out"):
            self.call("POST", f"{BASE}/reservations/{reservation_id}/check-out", renter, {"comments": "Salida sintética del dataset"}, key=f"dataset-deal:{ref}:check-out", expect=(200, 201))
            record["check_out"] = True
            self.save()
        if row["scenario_code"] == "reclamo_abierto":
            if record.get("state") != "en_disputa":
                claim = data(self.call("POST", f"{BASE}/reservations/{reservation_id}/damage-claim", host, {"description": "Daño sintético reportado para el piloto"}, key=f"dataset-deal:{ref}:claim", expect=(200, 201)))
                detail = data(self.call("GET", f"{BASE}/reservations/{reservation_id}", renter))
                if claim.get("state") != "abierto" or detail.get("state") != "en_disputa":
                    raise RuntimeError("damage claim did not remain open in dispute")
                record["state"] = detail["state"]
                self.save()
            return
        if not record.get("reception"):
            self.call("POST", f"{BASE}/reservations/{reservation_id}/reception", host, {"observations": "Recepción sintética conforme"}, key=f"dataset-deal:{ref}:reception", expect=(200, 201))
            record["reception"] = True
            self.save()

    def close_cycles(self, rows: list[dict]):
        if not rows:
            return
        # A single monotonically advancing isolated clock closes the pilot's
        # check-out windows without changing the shared system clock.
        latest = max(datetime.fromisoformat(self.state["scenarios"][r["scenario_ref"]]["end_at"].replace("Z", "+00:00")).astimezone(timezone.utc) for r in rows)
        target = latest + timedelta(hours=24)
        admin = self.admin()
        now = self.clock(admin)
        if now < target:
            self.set_clock(target)
        for row in rows:
            record = self.state["scenarios"][row["scenario_ref"]]
            reservation_id = record["reservation_id"]
            host = self.user(row["host_ref"])
            self.call("POST", f"{BASE}/reservations/{reservation_id}/guarantee", admin, {"kind": "liberacion", "amount_clp": 50000, "outcome": "exito"}, key=f"dataset-deal:{row['scenario_ref']}:guarantee:release", expect=(202,))
            guarantee = data(self.call("GET", f"{BASE}/reservations/{reservation_id}/guarantee", admin))
            settlement = data(self.call("POST", f"{BASE}/reservations/{reservation_id}/settlement", host, {"outcome": "exito"}, key=f"dataset-deal:{row['scenario_ref']}:settlement", expect=(202,)))
            receipt = self.call("GET", f"{BASE}/reservations/{reservation_id}/receipt", host, expect=(200,))
            detail = data(self.call("GET", f"{BASE}/reservations/{reservation_id}", host))
            receipt_data = data(receipt)
            if detail.get("state") != "cerrada" or not detail.get("history") or settlement.get("state") != "liquidada" or guarantee.get("state") != "liberada" or receipt_data.get("reservation_id") != reservation_id:
                raise RuntimeError("closed reservation, settlement, guarantee release, or receipt failed API verification")
            record["state"] = detail["state"]
            record["history_verified"] = True
            record["settlement_verified"] = True
            record["guarantee_release_verified"] = True
            record["receipt_verified"] = True
            self.save()

    def execute(self):
        rows_by_code = Counter(row["scenario_code"] for row in self.rows)
        for row in self.rows:
            if row["scenario_code"] in ("pendiente_pago", "reclamo_abierto"):
                continue
            self.reserve_and_pay(row)
        closing = sorted((r for r in self.rows if r["scenario_code"] == "ciclo_cerrado"), key=lambda r: self.state["scenarios"].get(r["scenario_ref"], {}).get("start_at", ""))
        for row in closing:
            self.operate(row)
        self.close_cycles([r for r in self.rows if r["scenario_code"] == "ciclo_cerrado"])
        # Claims are deliberately opened after closing cycles so their persisted
        # 24-hour response window has not expired when the load completes.
        for row in self.rows:
            if row["scenario_code"] == "reclamo_abierto":
                self.reserve_and_pay(row)
                self.operate(row)
        # Unpaid holds are deliberately created last and remain pending.
        for row in self.rows:
            if row["scenario_code"] == "pendiente_pago":
                self.reserve_and_pay(row, pending=True)
        expected_states = {
            "ciclo_cerrado": "cerrada", "pendiente_pago": "pendiente_de_pago",
            "cancelacion_con_devolucion": "cancelada_arrendatario",
            "rechazo_anfitrion": "rechazada_arrendador", "reclamo_abierto": "en_disputa",
        }
        state_counts = Counter(record.get("state", "") for record in self.state["scenarios"].values())
        for code, expected_state in expected_states.items():
            wanted = rows_by_code[code]
            got = sum(record.get("scenario_code") == code and record.get("state") == expected_state for record in self.state["scenarios"].values())
            if got != wanted:
                raise RuntimeError("scenario checkpoint state totals do not match the approved distribution")
        self.state["summary"] = {"scenario_counts": dict(rows_by_code), "state_counts": dict(state_counts), "completed": len(self.state["scenarios"])}
        self.save()

    def cleanup(self):
        failures = 0
        for email, token in list(self.sessions.items()):
            try:
                self.api.request("POST", "/api/v1/auth/logout", token, body={}, expect=(204,))
            except Exception:
                failures += 1
        self.sessions.clear()
        return failures


def validate_local_origin(value: str) -> str:
    parsed = urlparse(value)
    if parsed.scheme != "http" or parsed.username or parsed.password or parsed.path not in ("", "/") or parsed.hostname not in ("127.0.0.1", "localhost") or not parsed.port or parsed.port < 10000:
        raise RuntimeError("API must be a high-port loopback HTTP origin")
    return value.rstrip("/")


def main() -> int:
    parser = argparse.ArgumentParser(description="Execute synthetic dataset deals through Go APIs")
    parser.add_argument("--mode", choices=("pilot", "import"), required=True)
    parser.add_argument("--repo", required=True)
    parser.add_argument("--dataset-dir", required=True)
    parser.add_argument("--source-state-file", required=True, help="private account/listing checkpoint from dataset-import.py")
    parser.add_argument("--state-file", required=True, help="private idempotent deal checkpoint under .local/testing")
    parser.add_argument("--api-base", required=True)
    parser.add_argument("--policy-file", required=True)
    parser.add_argument("--clock-run-id", required=True)
    parser.add_argument("--confirm-target", default="")
    parser.add_argument("--backup-manifest", default="")
    args = parser.parse_args()
    runner = None
    try:
        repo, dataset = Path(args.repo).resolve(strict=True), Path(args.dataset_dir).resolve(strict=True)
        source_path, state_path = Path(args.source_state_file).resolve(strict=True), Path(args.state_file).absolute()
        policy = private_json(Path(args.policy_file).resolve(strict=True))
        if not re.fullmatch(r"[0-9a-f]{12}", args.clock_run_id):
            raise RuntimeError("clock run ID must be 12 lowercase hexadecimal characters")
        api = API(validate_local_origin(args.api_base))
        source_state, all_rows, listing_map = read_sources(dataset, source_path)
        rows = select_scenarios(args.mode, all_rows)
        if args.mode == "import":
            if len(source_state["users"]) != 1000 or len(source_state["listings"]) != 1200:
                raise RuntimeError("persistent scenario import requires all account and listing checkpoint references")
            if any(row["host_ref"] not in source_state["users"] or row["renter_ref"] not in source_state["users"] or row["listing_ref"] not in source_state["listings"] for row in rows):
                raise RuntimeError("persistent scenario manifest is missing a checkpointed participant or listing")
        else:
            for row in rows:
                if row["host_ref"] not in source_state["users"] or row["renter_ref"] not in source_state["users"]:
                    raise RuntimeError("the source checkpoint does not contain all pilot participants")
                if row["listing_ref"] not in source_state["listings"] or listing_map[row["listing_ref"]]["intended_visibility"] != "publica":
                    raise RuntimeError("the source checkpoint does not contain the pilot public listing")
        if args.mode == "import":
            if args.confirm_target != "espacigo_local" or not args.backup_manifest:
                raise RuntimeError("persistent deal import requires explicit target and a current private backup manifest")
            manifest_path = Path(args.backup_manifest).resolve(strict=True)
            manifest = private_json(manifest_path)
            dump = manifest_path.parent / manifest.get("backup_file", "")
            if not dump.is_file() or sha256(dump) != manifest.get("backup_sha256") or manifest.get("target") != "espacigo_local":
                raise RuntimeError("persistent backup manifest does not match the confirmed target and dump")
            if len(rows) != 1200:
                raise RuntimeError("persistent import requires the complete 1,200-row scenario manifest")
        source_hash = sha256(dataset / "para_la_ia/asignaciones/escenarios_planificados.jsonl")
        source_state_hash = sha256(source_path)
        if state_path.exists():
            state = private_json(state_path)
            if state.get("source_hash") != source_hash or state.get("source_state_hash") != source_state_hash or state.get("mode") != args.mode:
                raise RuntimeError("deal checkpoint belongs to a different dataset or source mapping")
        else:
            state = {"format_version": 1, "dataset_id": "espacigo-1000-v1", "source_hash": source_hash,
                     "source_state_hash": source_state_hash, "mode": args.mode, "scenarios": {}}
        runner = Runner(api, args, policy, source_state, rows, listing_map, state)
        # The isolated API's clock must match this run's mounted clock helper.
        runner.execute()
        print(json.dumps({"mode": args.mode, "processed": len(rows), "scenario_counts": dict(Counter(r["scenario_code"] for r in rows)), "note": "Go API/fake providers only; no direct database writes"}, sort_keys=True))
        return 0
    except (RuntimeError, SafeHTTPError, OSError, ValueError, KeyError, json.JSONDecodeError) as error:
        if isinstance(error, SafeHTTPError):
            print(f"dataset deal run stopped at {error.path}: HTTP {error.status} ({error.code})", file=sys.stderr)
        else:
            print(f"dataset deal run stopped: {str(error)}", file=sys.stderr)
        return 2
    finally:
        if runner is not None:
            failures = runner.cleanup()
            if failures:
                print(f"dataset deal session cleanup incomplete: {failures} session(s)", file=sys.stderr)
                raise SystemExit(2)


if __name__ == "__main__":
    raise SystemExit(main())
