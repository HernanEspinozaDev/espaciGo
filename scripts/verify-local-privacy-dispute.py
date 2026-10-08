#!/usr/bin/env python3
"""Prepare three local synthetic actors and verify the LOCAL-PRIV-01A2 flow.

Credentials are generated once and kept in a mode-0600 file outside the repo.
The script uses only the loopback API/Mailpit and the local database container.
"""
from __future__ import annotations

import datetime as dt
import json
import os
import pathlib
import re
import secrets
import string
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


ROOT = pathlib.Path(__file__).resolve().parent.parent
API = os.environ.get("ESPACIGO_LOCAL_API", "http://127.0.0.1:8080")
MAILPIT = os.environ.get("ESPACIGO_LOCAL_MAILPIT", "http://127.0.0.1:8025")
for base in (API, MAILPIT):
    if urllib.parse.urlparse(base).hostname not in {"localhost", "127.0.0.1"}:
        raise SystemExit("This verifier only accepts loopback API and Mailpit URLs")

state_root = pathlib.Path(os.environ.get("XDG_STATE_HOME", pathlib.Path.home() / ".local/state"))
state_dir = state_root / "espacigo"
state_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
state_dir.chmod(0o700)
state_path = state_dir / "local-priv-01a2-test-accounts.json"


def save_state(state: dict) -> None:
    temp = state_path.with_suffix(".tmp")
    fd = os.open(temp, os.O_CREAT | os.O_TRUNC | os.O_WRONLY, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as stream:
        json.dump(state, stream, indent=2)
        stream.write("\n")
    os.chmod(temp, 0o600)
    os.replace(temp, state_path)
    os.chmod(state_path, 0o600)


def request(method: str, url: str, data=None, token: str | None = None, headers=None):
    request_headers = {"Content-Type": "application/json"}
    if token:
        request_headers["Authorization"] = "Bearer " + token
    request_headers.update(headers or {})
    payload = None if data is None else json.dumps(data).encode()
    req = urllib.request.Request(url, data=payload, method=method, headers=request_headers)
    try:
        response = urllib.request.urlopen(req, timeout=12)
    except urllib.error.HTTPError as error:
        response = error
    raw = response.read()
    try:
        body = json.loads(raw) if raw else None
    except json.JSONDecodeError:
        body = raw.decode("utf-8", "replace")
    return response.status, body


def api(method: str, path: str, expected: int, data=None, token=None, headers=None):
    status, body = request(method, API + path, data, token, headers)
    if status != expected:
        # Do not print response bodies: registration/error paths can contain identifiers.
        raise RuntimeError(f"{method} {path}: HTTP {status}; expected {expected}")
    return body


def mail_ids() -> set[str]:
    status, listing = request("GET", MAILPIT + "/api/v1/messages")
    if status != 200:
        raise RuntimeError("Local Mailpit is not available")
    return {item.get("ID", "") for item in listing.get("messages", [])}


def find_mail(email: str, excluded_ids: set[str], purpose: str, timeout: float) -> dict | None:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        status, listing = request("GET", MAILPIT + "/api/v1/messages")
        if status == 200:
            for item in listing.get("messages", []):
                if item.get("ID") in excluded_ids:
                    continue
                recipients = item.get("To") or []
                if any((recipient.get("Address") or "").lower() == email.lower() for recipient in recipients):
                    status, message = request("GET", MAILPIT + "/api/v1/message/" + item["ID"])
                    subject = (message.get("Subject") or "").lower() if status == 200 else ""
                    if status == 200 and purpose in subject:
                        return message
        time.sleep(0.2)
    return None


def wait_mail(email: str, excluded_ids: set[str], purpose: str) -> dict:
    message = find_mail(email, excluded_ids, purpose, 15)
    if message:
        return message
    raise RuntimeError("Timed out waiting for the local verification message")


def consume_mail_token(email: str, excluded_ids: set[str], purpose: str, endpoint: str, payload_builder) -> None:
    message = wait_mail(email, excluded_ids, purpose)
    text = message.get("Text", "")
    token_id = re.search(r"^Token ID: ([0-9a-f-]+)", text, re.M)
    token = re.search(r"^Token: ([A-Za-z0-9_-]+)", text, re.M)
    if not token_id or not token:
        raise RuntimeError("Local verification email did not contain the expected activation fields")
    api("POST", endpoint, 204, payload_builder(token_id[1], token[1]))


def password_for_test() -> str:
    alphabet = string.ascii_letters + string.digits
    return "LocalTest#7" + "".join(secrets.choice(alphabet) for _ in range(24))


def new_state() -> dict:
    suffix = uuid.uuid4().hex[:12]
    actors = {}
    for kind, preference in (("host", "ofrecer"), ("renter", "arrendar"), ("admin", "arrendar")):
        actors[kind] = {
            "email": f"local-priv189-{suffix}-{kind}@example.test",
            "password": password_for_test(),
            "preference": preference,
        }
    return {"version": 1, "actors": actors, "fixture": {}}


def login(email: str, password: str):
    status, body = request("POST", API + "/api/v1/auth/login", {"email": email, "password": password})
    if status == 200:
        return body
    return None


def account_status(email: str):
    if not re.fullmatch(r"[a-z0-9-]+@example\.test", email):
        raise RuntimeError("Synthetic account email is invalid")
    row = psql("SELECT id::text || '|' || estado FROM public.usuario WHERE correo_normalizado='" + email + "';")
    if not row:
        return None
    pieces = row.split("|", 1)
    if len(pieces) != 2:
        raise RuntimeError("Could not inspect synthetic account state")
    return pieces[0], pieces[1]


def ensure_account(state: dict, kind: str, terms_ids: list[str]) -> dict:
    actor = state["actors"][kind]
    existing = login(actor["email"], actor["password"])
    if existing:
        actor["id"] = existing["account_id"]
        save_state(state)
        return existing
    current = account_status(actor["email"])
    newly_registered = False
    if current is None:
        before = mail_ids()
        status, registered = request("POST", API + "/api/v1/auth/register", {
            "email": actor["email"], "password": actor["password"],
            "use_preference": actor["preference"], "terms_version_ids": terms_ids,
        })
        if status == 201:
            actor["id"] = registered["account_id"]
            save_state(state)
            current = (actor["id"], "correo_pendiente")
            newly_registered = True
        elif status == 409:
            current = account_status(actor["email"])
        else:
            raise RuntimeError(f"Could not register the {kind} synthetic account (HTTP {status})")
    if current is None:
        raise RuntimeError("Synthetic account could not be recovered after registration")
    actor["id"] = current[0]
    save_state(state)

    if current[1] == "correo_pendiente":
        before = set() if newly_registered else mail_ids()
        if not newly_registered:
            status, _ = request("POST", API + "/api/v1/auth/verification/reissue", {"email": actor["email"]})
            if status != 204:
                raise RuntimeError(f"Could not reissue local verification for {kind} (HTTP {status})")
        consume_mail_token(actor["email"], before, "verifica tu correo", "/api/v1/auth/verification",
                           lambda token_id, token: {"token_id": token_id, "token": token})
        result = login(actor["email"], actor["password"])
        if result:
            actor["id"] = result["account_id"]
            save_state(state)
            return result

    # An active synthetic account with a lost/stale local password is recovered
    # only through the local email flow. Save the replacement before requesting
    # mail so an interrupted invocation can safely retry it.
    if not actor.get("recovery_pending"):
        actor["password"] = password_for_test()
        actor["recovery_excluded_ids"] = sorted(mail_ids())
        actor["recovery_pending"] = True
        save_state(state)
    before = set(actor.get("recovery_excluded_ids", []))
    recovery_mail = find_mail(actor["email"], before, "recupera tu clave", 0.3)
    if recovery_mail is None:
        status, _ = request("POST", API + "/api/v1/auth/password/recovery", {"email": actor["email"]})
        if status != 202:
            raise RuntimeError(f"Could not start local credential recovery for {kind} (HTTP {status})")
    consume_mail_token(actor["email"], before, "recupera tu clave", "/api/v1/auth/password/recovery/consume",
                       lambda token_id, token: {"token_id": token_id, "token": token,
                                               "new_password": actor["password"],
                                               "confirm_password": actor["password"]})
    result = login(actor["email"], actor["password"])
    if not result:
        raise RuntimeError(f"Recovered {kind} synthetic account could not log in")
    actor["id"] = result["account_id"]
    actor.pop("recovery_pending", None)
    actor.pop("recovery_excluded_ids", None)
    save_state(state)
    return result


def compose_env() -> dict:
    env = os.environ.copy()
    uid = os.getuid()
    env["LOCAL_UID"] = str(uid)
    env["LOCAL_GID"] = str(os.getgid())
    data_home = pathlib.Path(env.get("XDG_DATA_HOME", pathlib.Path.home() / ".local/share"))
    env["LOCAL_M03_EVIDENCE_DIR"] = str(data_home / "espacigo/m03-evidence")
    return env


def psql(sql: str) -> str:
    command = [
        "docker", "compose", "--project-directory", str(ROOT), "-f", str(ROOT / "compose.yaml"),
        "exec", "-T", "database", "psql", "--no-psqlrc", "--quiet", "--set=ON_ERROR_STOP=1",
        "--tuples-only", "--no-align",
        "--username", "espacigo_admin", "--dbname", "espacigo_local",
    ]
    result = subprocess.run(command, input=sql, text=True, capture_output=True, env=compose_env())
    if result.returncode:
        # Database diagnostics can include SQL values. Keep output private.
        raise RuntimeError("Local synthetic data preparation failed; inspect the database container health")
    return result.stdout.strip()


def seed_test_data(state: dict) -> None:
    fixture = state["fixture"]
    if not fixture:
        fixture["space_id"] = str(uuid.uuid4())
    # One new booking per execution makes the verifier reusable after the
    # previous run canceled its booking. Closed synthetic history is kept.
    if fixture.get("completed") or not fixture.get("reservation_id"):
        space_id = fixture["space_id"]
        fixture.clear()
        fixture["space_id"] = space_id
        fixture.update({
            "quote_id": str(uuid.uuid4()),
            "reservation_id": str(uuid.uuid4()),
            "occupancy_id": str(uuid.uuid4()),
            "transition_id": str(uuid.uuid4()),
            "completed": False,
        })
    save_state(state)
    host = state["actors"]["host"]
    renter = state["actors"]["renter"]
    admin = state["actors"]["admin"]
    values = {"host_id": host["id"], "renter_id": renter["id"], "admin_id": admin["id"]}
    sql = f"""
BEGIN;
INSERT INTO public.rol_usuario(usuario_id,rol)
VALUES ('{values['host_id']}'::uuid,'arrendador'),
       ('{values['admin_id']}'::uuid,'administrador')
ON CONFLICT (usuario_id,rol) DO NOTHING;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM public.usuario WHERE id='{values['host_id']}'::uuid AND estado='activo')
     OR NOT EXISTS (SELECT 1 FROM public.usuario WHERE id='{values['renter_id']}'::uuid AND estado='activo')
     OR NOT EXISTS (SELECT 1 FROM public.usuario WHERE id='{values['admin_id']}'::uuid AND estado='activo') THEN
    RAISE EXCEPTION 'synthetic actors must be active';
  END IF;
  IF '{values['host_id']}'='{values['renter_id']}' OR '{values['host_id']}'='{values['admin_id']}' OR '{values['renter_id']}'='{values['admin_id']}' THEN
    RAISE EXCEPTION 'synthetic actors must be independent';
  END IF;
END $$;
INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
  reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
VALUES ('{fixture['space_id']}'::uuid,'{values['host_id']}'::uuid,'oficina','LOCAL-PRIV-01A2 ensayo',
  repeat('Fixture sintético local para comprobar el bloqueador de disputa. ',4),20,2,'Ensayo sintético controlado',
  'hora',12000,'Dirección sintética local','borrador','America/Santiago')
ON CONFLICT (id) DO NOTHING;
INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp,moneda,creada_en)
VALUES ('{fixture['space_id']}'::uuid,1,'hora',12000,'CLP',now())
ON CONFLICT (espacio_id,version) DO NOTHING;
INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores,actualizado_en)
SELECT '{fixture['space_id']}'::uuid,'oficina',1,'{{}}'::jsonb,now()
WHERE EXISTS (SELECT 1 FROM public.categoria_perfil_atributos WHERE categoria_codigo='oficina' AND version=1)
ON CONFLICT (espacio_id) DO NOTHING;
INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada)
VALUES ('{fixture['space_id']}'::uuid,'{values['host_id']}'::uuid,'{values['renter_id']}'::uuid,true)
ON CONFLICT (espacio_id) DO NOTHING;
WITH clock AS (SELECT now() AS instant)
INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,
  precio_unitario_clp,categoria_codigo,perfil_version,perfil_valores_snapshot,moneda,unidades,subtotal_clp,
  inicio,termino,zona_horaria,creada_en,vence_en,condiciones_snapshot,politica_cancelacion_version)
SELECT '{fixture['quote_id']}'::uuid,'{fixture['space_id']}'::uuid,'{values['host_id']}'::uuid,'{values['renter_id']}'::uuid,
  1,'hora',12000,'oficina',1,'{{}}'::jsonb,'CLP',2,24000,instant+interval '48 hours',instant+interval '50 hours',
  'America/Santiago',instant,instant+interval '15 minutes','Condiciones sintéticas locales','local_flexible_v1' FROM clock
ON CONFLICT (id) DO NOTHING;
WITH clock AS (SELECT now() AS instant)
INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,expira_en,creada_en)
SELECT '{fixture['occupancy_id']}'::uuid,'{fixture['space_id']}'::uuid,'{fixture['reservation_id']}'::uuid,
  tstzrange(instant+interval '48 hours',instant+interval '50 hours','[)'),'retencion',true,instant+interval '15 minutes',instant
FROM clock ON CONFLICT (id) DO NOTHING;
WITH clock AS (SELECT now() AS instant)
INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,
  huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,
  zona_horaria,pago_vence_en,creada_en,actualizada_en,condiciones_snapshot,politica_cancelacion_version)
SELECT '{fixture['reservation_id']}'::uuid,'{fixture['quote_id']}'::uuid,'{fixture['space_id']}'::uuid,
  '{values['host_id']}'::uuid,'{values['renter_id']}'::uuid,'local-priv189-reservation-' || '{fixture['reservation_id']}',decode(repeat('19',32),'hex'),
  '{fixture['occupancy_id']}'::uuid,'pendiente_de_pago',12000,2,24000,'hora','CLP',instant+interval '48 hours',
  instant+interval '50 hours','America/Santiago',instant+interval '15 minutes',instant,instant,
  'Condiciones sintéticas locales','local_flexible_v1' FROM clock ON CONFLICT (id) DO NOTHING;
INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,estado_anterior,estado_nuevo,actor_id,motivo,creada_en,secuencia)
VALUES ('{fixture['transition_id']}'::uuid,'{fixture['reservation_id']}'::uuid,NULL,'pendiente_de_pago',
  '{values['renter_id']}'::uuid,'solicitud_sintetica_privacidad',now(),1) ON CONFLICT (id) DO NOTHING;
COMMIT;
"""
    psql(sql)
    save_state(state)


def ensure_key(fixture: dict, name: str) -> str:
    if name not in fixture:
        fixture[name] = "local-priv189-" + name.replace("_", "-") + "-" + uuid.uuid4().hex
    return fixture[name]


def list_right_requests(token: str):
    return api("GET", "/api/v1/rights-requests", 200, token=token).get("items", [])


def interrupt_if_requested(phase: str) -> None:
    if os.environ.get("LOCAL_PRIV189_TEST_INTERRUPT_AFTER") == phase:
        raise SystemExit(f"INTERRUPTED after {phase} commit (test hook)")


def request_created_in_run(item: dict, started_at: str) -> bool:
    try:
        created = dt.datetime.fromisoformat(item["created_at"].replace("Z", "+00:00"))
        started = dt.datetime.fromisoformat(started_at.replace("Z", "+00:00"))
        return item.get("kind") == "supresion" and created >= started - dt.timedelta(seconds=2)
    except (KeyError, ValueError, TypeError):
        return False


def ensure_right_request(state: dict, fixture: dict, participant: str, token: str) -> str:
    requests = fixture.setdefault("suppression_requests", {})
    if requests.get(participant):
        return requests[participant]
    for item in list_right_requests(token):
        if request_created_in_run(item, fixture["run_started_at"]):
            requests[participant] = item["id"]
            save_state(state)
            return item["id"]
    # The rights request endpoint has no idempotency header; the account-scoped
    # list above is the recovery index if the process stops after commit.
    save_state(state)
    created = api("POST", "/api/v1/rights-requests", 202, {"type": "supresion"}, token=token)
    interrupt_if_requested("rights_request_" + participant)
    requests[participant] = created["id"]
    save_state(state)
    return created["id"]


def main() -> None:
    api("GET", "/health/ready", 200)
    mail_status, _ = request("GET", MAILPIT + "/api/v1/messages")
    if mail_status != 200:
        raise RuntimeError("Local Mailpit is not available")
    v24_present = psql("SELECT EXISTS (SELECT 1 FROM public.schema_migrations WHERE version=24);")
    if v24_present != "t":
        raise RuntimeError("Expected incremental local schema V24 to be applied")
    if state_path.exists():
        if state_path.stat().st_mode & 0o077:
            raise RuntimeError("Credential file permissions are too broad; chmod 600 it before reuse")
        state = json.loads(state_path.read_text(encoding="utf-8"))
        if state.get("version") != 1:
            raise RuntimeError("Unsupported local credential state version")
    else:
        state = new_state()
        save_state(state)

    terms = api("GET", "/api/v1/auth/terms", 200)
    terms_ids = [item["id"] for item in terms["items"] if item["type"] in {"terminos", "privacidad"}]
    if len(terms_ids) < 2:
        raise RuntimeError("Expected synthetic terms and privacy versions were not available")
    logins = {kind: ensure_account(state, kind, terms_ids) for kind in ("host", "renter", "admin")}
    if len({state["actors"][kind]["id"] for kind in ("host", "renter", "admin")}) != 3:
        raise RuntimeError("Actor identity separation check failed")

    fixture = state["fixture"]
    seed_test_data(state)
    fixture = state["fixture"]
    # Refresh sessions after bootstrap assigned the separated local roles.
    for kind in ("host", "renter", "admin"):
        actor = state["actors"][kind]
        logins[kind] = login(actor["email"], actor["password"])
        if not logins[kind]:
            raise RuntimeError(f"The {kind} synthetic account could not log in after local role assignment")
    host_roles = set(logins["host"].get("roles", []))
    renter_roles = set(logins["renter"].get("roles", []))
    admin_roles = set(logins["admin"].get("roles", []))
    if "arrendador" not in host_roles or "administrador" in host_roles:
        raise RuntimeError("Host role is missing or accidentally elevated")
    if renter_roles != {"arrendatario"}:
        raise RuntimeError("Renter account does not have its independent renter-only role")
    if "administrador" not in admin_roles or "arrendador" in admin_roles:
        raise RuntimeError("Independent administrator role is missing or mixed with host role")
    host_token, renter_token, admin_token = (logins[k]["access_token"] for k in ("host", "renter", "admin"))
    reservation_id = fixture["reservation_id"]
    dispute_path = f"/api/v1/local/booking-trial/reservations/{reservation_id}/disputes"
    reservation_path = f"/api/v1/local/booking-trial/reservations/{reservation_id}"
    host_detail = api("GET", reservation_path, 200, token=host_token)
    renter_detail = api("GET", reservation_path, 200, token=renter_token)
    if host_detail.get("data", {}).get("id") != reservation_id or renter_detail.get("data", {}).get("id") != reservation_id:
        raise RuntimeError("Reservation detail did not resolve for both participants")

    # Opening uses a persisted key, so a rerun after API commit replays the same
    # operation instead of creating a second dispute.
    before_host_close = api("GET", dispute_path, 200, token=renter_token)
    dispute_key = ensure_key(fixture, "dispute_open_key")
    save_state(state)
    if before_host_close.get("items"):
        opened = before_host_close["items"][0]
        fixture["dispute_id"] = opened["id"]
    else:
        denied = request("POST", API + dispute_path, {"reason_code": "ensayo_privacidad"}, token=renter_token,
                         headers={"Idempotency-Key": ensure_key(fixture, "renter_denied_key")})
        if denied[0] != 404:
            raise RuntimeError("A renter other than the host opened the dispute")
        status, opened = request("POST", API + dispute_path, {"reason_code": "ensayo_privacidad"}, token=host_token,
                                 headers={"Idempotency-Key": dispute_key})
        if status not in (200, 201):
            raise RuntimeError(f"Could not open synthetic dispute (HTTP {status})")
        # A controlled interruption exercises the exact crash window after the
        # server commits but before this client persists the returned identifier.
        interrupt_if_requested("dispute_open")
        fixture["dispute_id"] = opened["id"]
        save_state(state)
    dispute_id = fixture["dispute_id"]
    if opened.get("state") not in ("abierta", "cerrada") or opened.get("opened_by") != state["actors"]["host"]["id"]:
        # The participant list may not include opened_by; the canonical lookup is
        # the host idempotent replay if the dispute was discovered by the renter.
        status, opened = request("POST", API + dispute_path, {"reason_code": "ensayo_privacidad"}, token=host_token,
                                 headers={"Idempotency-Key": dispute_key})
        if status not in (200, 201) or opened.get("opened_by") != state["actors"]["host"]["id"]:
            raise RuntimeError("Synthetic host dispute did not resolve through its persisted idempotency key")
    if not fixture.get("run_started_at"):
        fixture["run_started_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
        save_state(state)
    # A new booking receives its own rights-request window; write it before any
    # request so a restart can rediscover a committed request via the API.
    if fixture.get("rights_requests_for_reservation") != reservation_id:
        fixture["run_started_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
        fixture["suppression_requests"] = {}
        fixture["review_keys"] = {}
        fixture["rights_requests_for_reservation"] = reservation_id
        save_state(state)
    host_request = ensure_right_request(state, fixture, "host", host_token)
    renter_request = ensure_right_request(state, fixture, "renter", renter_token)
    requests = {"host": host_request, "renter": renter_request}
    fixture["dispute_id"] = dispute_id
    save_state(state)

    def review(request_id: str, phase: str, participant: str):
        keymap = fixture.setdefault("review_keys", {}).setdefault(phase, {})
        key = keymap.get(participant) or ensure_key(fixture, f"review_{phase}_{participant}")
        keymap[participant] = key
        save_state(state)
        return api("POST", f"/api/v1/privacy/suppression-requests/{request_id}/review", 200,
                   token=admin_token, headers={"Idempotency-Key": key})

    expected_pending = ["matriz_retencion_historicos_incompleta"]
    renter_view = api("GET", dispute_path, 200, token=renter_token)
    if len(renter_view.get("items", [])) != 1 or renter_view["items"][0].get("id") != dispute_id:
        raise RuntimeError("Renter could not read the host's dispute")
    for participant in ("host", "renter"):
        status, _ = request("POST", API + f"/api/v1/admin/disputes/{dispute_id}/close", {
            "reason_code": "ensayo_finalizado"}, token=logins[participant]["access_token"])
        if status != 403:
            raise RuntimeError("Non-admin participant could close the dispute")
    for participant, request_id in requests.items():
        result = review(request_id, "open", participant)
        if result.get("obligations_detected") != ["reserva_activa", "disputa_abierta"] or result.get("pending_checks") != expected_pending:
            raise RuntimeError(f"Open dispute blocker mismatch for {participant}")

    queue = api("GET", "/api/v1/admin/disputes", 200, token=admin_token)
    if not any(item.get("id") == dispute_id for item in queue.get("items", [])) and renter_view["items"][0].get("state") == "abierta":
        raise RuntimeError("Independent administrator could not see the open dispute queue")
    dispute_state = renter_view["items"][0].get("state")
    if dispute_state == "abierta":
        close_key = ensure_key(fixture, "dispute_close_key")
        save_state(state)
        closed_status, _ = request("POST", API + f"/api/v1/admin/disputes/{dispute_id}/close",
                                   {"reason_code": "ensayo_finalizado"}, token=admin_token,
                                   headers={"Idempotency-Key": close_key})
        if closed_status != 200:
            # Close endpoint is not idempotent; inspect persisted state before
            # reporting failure in case the prior run committed before stopping.
            refreshed = api("GET", dispute_path, 200, token=renter_token).get("items", [])
            if not refreshed or refreshed[0].get("state") != "cerrada":
                raise RuntimeError(f"Administrator could not close dispute (HTTP {closed_status})")
    renter_closed_view = api("GET", dispute_path, 200, token=renter_token)
    if len(renter_closed_view.get("items", [])) != 1 or renter_closed_view["items"][0].get("state") != "cerrada":
        raise RuntimeError("Renter did not see the closed dispute")
    for participant, request_id in requests.items():
        result = review(request_id, "closed", participant)
        if result.get("obligations_detected") != ["reserva_activa"] or result.get("pending_checks") != expected_pending:
            raise RuntimeError(f"Closing dispute changed more than its blocker for {participant}")

    detail_after_close = api("GET", reservation_path, 200, token=renter_token)
    if detail_after_close.get("data", {}).get("state") == "pendiente_de_pago":
        cancel_key = ensure_key(fixture, "cancel_key")
        save_state(state)
        status, cancel = request("POST", API + reservation_path + "/cancel", {"reason": "Fin del ensayo local"},
                                 token=renter_token, headers={"Idempotency-Key": cancel_key})
        if status != 200:
            refreshed = api("GET", reservation_path, 200, token=renter_token).get("data", {})
            if refreshed.get("state") != "cancelada_arrendatario":
                raise RuntimeError(f"Renter could not cancel synthetic pending booking (HTTP {status})")
        elif cancel.get("data", {}).get("reservation", {}).get("state") != "cancelada_arrendatario":
            raise RuntimeError("Renter reservation action did not cancel the synthetic pending booking")
    canceled_detail = api("GET", reservation_path, 200, token=host_token)
    if canceled_detail.get("data", {}).get("state") != "cancelada_arrendatario":
        raise RuntimeError("Host could not see the renter's confirmed reservation action")
    for participant, request_id in requests.items():
        result = review(request_id, "canceled", participant)
        if result.get("obligations_detected") != [] or result.get("pending_checks") != expected_pending:
            raise RuntimeError(f"Terminal reservation retained an unexpected blocker for {participant}")

    fixture["completed"] = True
    save_state(state)
    print("PASS schema V24 row exists (independent of any later migration version); API and Mailpit healthy")
    print("PASS three independent verified synthetic actors and separated permissions")
    print("PASS dispute open/read/admin close; recovery keys and per-step state permit interruption resume")
    print("PASS both participants detect disputa_abierta; closure removes only that blocker")
    print("PASS reservation detail/action remains usable; renter cancels and host sees updated state")
    print("PASS after cancellation, no obligation remains; historical retention check remains pending")
    print("Synthetic account credentials remain outside repository with mode 0600:", state_path)
    print("Synthetic fixture reservation:", reservation_id)
    print("Dispute:", dispute_id)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # Keep credentials, tokens and database payloads out of terminal logs.
        if isinstance(error, RuntimeError):
            print("FAIL", str(error))
        else:
            print("FAIL local privacy dispute verification")
        raise SystemExit(1)
