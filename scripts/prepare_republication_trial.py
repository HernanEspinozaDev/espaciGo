#!/usr/bin/env python3
"""Prepare only synthetic end-of-rental fixtures in the authorized local trial.

Does not provision/change accounts, execute payment/signature/guarantee flows,
change ACLs or migrations, or replace the existing gallery. Account material and
identifiers stay in the private test directory. The native test must execute
check-out, owner reception and republication through the real API/interface.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import uuid

ROOT = Path(__file__).resolve().parents[1]
CONTAINER = 'espacigo-f5-gate-db-20261101'
DATABASE = 'espacigo_f5_gate_20261101'
IMAGE = 'sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'


def uid(value):
    if not isinstance(value, str) or str(uuid.UUID(value)) != value:
        raise ValueError('Invalid private fixture identifier')
    return value


def sql(text):
    result = subprocess.run([
        'docker', 'exec', '-i', '-u', 'postgres', CONTAINER,
        'psql', '-X', '-w', '-At', '--single-transaction',
        '-v', 'ON_ERROR_STOP=1', '-d', DATABASE,
    ], input=text, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError('Synthetic fixture SQL failed; no transaction committed')
    return [line for line in result.stdout.splitlines() if line.startswith('{')]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--suffix', required=True)
    parser.add_argument('--inventory', type=Path, required=True)
    parser.add_argument('--confirm', required=True, choices=['local-synthetic-republication-only'])
    args = parser.parse_args()
    if not re.fullmatch(r'\d{8}-[a-z0-9]{1,8}', args.suffix):
        raise ValueError('Invalid synthetic fixture suffix')
    private = ROOT / '.local/testing'
    policy = json.loads((private / 'test-account-policy.json').read_text())
    if any(policy['database'].get(k) != v for k, v in {'host': '127.0.0.1', 'port': 15434, 'name': DATABASE}.items()):
        raise ValueError('Authorized database policy mismatch')
    if 'republication_authorization_20261104' not in policy:
        raise ValueError('Explicit local republication authorization is required')
    inventory = args.inventory
    if not inventory.is_absolute() or not inventory.resolve().is_relative_to(private.resolve()):
        raise ValueError('Account inventory must remain under the authorized private root')
    accounts = json.loads(inventory.read_text())
    target = accounts['target']
    if any(target.get(k) != v for k, v in {'host': '127.0.0.1', 'port': 15434, 'database': DATABASE}.items()):
        raise ValueError('Account inventory target mismatch')
    owner = next(a for a in accounts['accounts'] if a['slot'] == 'owner')
    other = next(a for a in accounts['accounts'] if a['slot'] != 'owner')
    owner_id, renter_id = uid(owner['account_id']), uid(other['account_id'])
    if owner_id == renter_id:
        raise ValueError('Two existing distinct synthetic accounts are required')
    demo = private / 'space-gallery-demo'
    output = demo / f'republication-fixture-{args.suffix}.json'
    if output.exists():
        raise ValueError('Fixture selector already used; do not replay setup as fresh acceptance')
    review = json.loads((demo / 'review-target.json').read_text())
    route = review['route']
    prefix = '/anfitrion/espacios/'
    if not route.startswith(prefix) or not route.endswith('/editar'):
        raise ValueError('Invalid retained gallery review route')
    space = uid(route[len(prefix):-len('/editar')])
    inspection = subprocess.check_output([
        'docker', 'inspect', CONTAINER, '--format',
        '{{.State.Running}}|{{.Image}}|{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}',
    ], text=True).strip()
    if inspection != f'true|{IMAGE}|172.28.0.2':
        raise ValueError('Authorized PostgreSQL container boundary changed')
    status = json.loads(sql(f"""SET TRANSACTION READ ONLY;
SELECT jsonb_build_object('database',current_database(),'engine',current_setting('server_version_num'),
'accounts',(SELECT count(*) FROM public.usuario),'ledger',(SELECT count(*) FROM public.schema_migrations),
'owned',EXISTS(SELECT 1 FROM public.espacio WHERE id='{space}' AND propietario_id='{owner_id}' AND estado='activa'),
'photos',(SELECT count(*) FROM public.space_media sm JOIN public.media_asset a ON a.id=sm.media_id WHERE sm.espacio_id='{space}' AND sm.retired_at IS NULL AND a.estado='ready'),
'prior_fixture_reservations',(SELECT count(*) FROM public.reserva_ensayo_local WHERE espacio_id='{space}' AND condiciones_snapshot='Ensayo local de devolución; no acredita reserva, pago ni firma.'));""")[0])
    prior_fixture_reservations = status.pop('prior_fixture_reservations')
    if not isinstance(prior_fixture_reservations, int) or prior_fixture_reservations % 2:
        raise ValueError('Unexpected retained synthetic fixture inventory')
    future_day = 3 + 7 * (prior_fixture_reservations // 2)
    manual_day = future_day + 2
    if status != {'database': DATABASE, 'engine': '180006', 'accounts': 2, 'ledger': 83, 'owned': True, 'photos': 30}:
        raise ValueError('Retained gallery/target preflight failed')
    fixture = {'database': DATABASE, 'host': '127.0.0.1', 'port': 15434,
               'selector': args.suffix, 'route': route, 'space_id': space,
               'reservation_id': str(uuid.uuid4()), 'occupation_id': str(uuid.uuid4()),
               'quote_id': str(uuid.uuid4()), 'contract_id': str(uuid.uuid4()),
               'future_reservation_id': str(uuid.uuid4()), 'future_occupation_id': str(uuid.uuid4()),
               'future_quote_id': str(uuid.uuid4()), 'manual_block_id': str(uuid.uuid4()),
               'initial_state': 'en_curso', 'contract_state': 'generado',
               'notice': 'Synthetic end-of-rental fixture only; no payment, signature, guarantee or legal issuance proved.'}
    main_id, occ, quote = fixture['reservation_id'], fixture['occupation_id'], fixture['quote_id']
    future, future_occ, future_quote = fixture['future_reservation_id'], fixture['future_occupation_id'], fixture['future_quote_id']
    setup = f"SELECT id FROM public.espacio WHERE id='{space}' AND propietario_id='{owner_id}' FOR UPDATE;\n"
    for reservation, occupation, quotation, start, end, state in [
        (main_id, occ, quote, "now()-interval '2 hours'", "now()-interval '1 hour'", 'en_curso'),
        (future, future_occ, future_quote, f"now()+interval '{future_day} days'", f"now()+interval '{future_day} days 1 hour'", 'lista_para_checkin'),
    ]:
        setup += f"""
INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,
precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en,condiciones_snapshot,categoria_codigo,perfil_version,perfil_valores_snapshot)
SELECT '{quotation}',e.id,'{owner_id}','{renter_id}',t.version,'hora',9000,'CLP',1,9000,{start},{end},e.zona_horaria,
now(),now()+interval '15 minutes','Ensayo local de devolución; no acredita reserva, pago ni firma.',e.categoria_codigo,ec.perfil_version,ec.valores
FROM public.espacio e JOIN public.espacio_caracteristicas ec ON ec.espacio_id=e.id
JOIN LATERAL (SELECT version FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)t ON true WHERE e.id='{space}';
INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,
huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,pago_vence_en,creada_en,actualizada_en,condiciones_snapshot)
SELECT '{reservation}',q.id,q.espacio_id,q.anfitrion_id,q.arrendatario_id,'return-fixture-{reservation}',decode(repeat('00',32),'hex'),'{occupation}','{state}',
q.precio_unitario_clp,q.unidades,q.subtotal_clp,q.modalidad,q.moneda,q.inicio,q.termino,q.zona_horaria,q.vence_en,now(),now(),q.condiciones_snapshot
FROM public.cotizacion_reserva_ensayo q WHERE q.id='{quotation}';
INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo)
SELECT '{occupation}',espacio_id,id,tstzrange(inicio,termino,'[)'),'reserva',true FROM public.reserva_ensayo_local WHERE id='{reservation}';
"""
    fixture_contract = json.dumps({'synthetic_fixture': 'local-return-republication-v1', 'reservation_id': main_id,
                                   'currency': 'CLP', 'conditions_snapshot': fixture['notice']}, sort_keys=True)
    contract_hash = hashlib.sha256(fixture_contract.encode()).hexdigest()
    fixture['contract_sha256'] = contract_hash
    setup += f"""
INSERT INTO public.ocupacion(id,espacio_id,intervalo,tipo,motivo)
VALUES('{fixture['manual_block_id']}','{space}',tstzrange(now()+interval '{manual_day} days',now()+interval '{manual_day} days 1 hour','[)'),
'bloqueo_manual','Ensayo local: bloqueo ajeno al arriendo terminado, debe conservarse');
INSERT INTO public.contrato_ensayo_local(id,reserva_id,version,estado,snapshot,documento_id,sha256,creada_en,actualizada_en,pdf_retirado_en)
VALUES('{fixture['contract_id']}','{main_id}',1,'generado','{fixture_contract.replace(chr(39),chr(39)*2)}'::jsonb,NULL,
decode('{contract_hash}','hex'),now(),now(),now());
SELECT jsonb_build_object('reservation_rows',(SELECT count(*) FROM public.reserva_ensayo_local WHERE id IN ('{main_id}','{future}')),
'own_occupations',(SELECT count(*) FROM public.ocupacion WHERE id IN ('{occ}','{future_occ}','{fixture['manual_block_id']}') AND activo),
'contract_rows',(SELECT count(*) FROM public.contrato_ensayo_local WHERE id='{fixture['contract_id']}'));
"""
    result = json.loads(sql(setup)[0])
    if result != {'reservation_rows': 2, 'own_occupations': 3, 'contract_rows': 1}:
        raise RuntimeError('Fixture postcheck failed')
    # Read back the exact target after the committed setup, before recording it.
    verified = json.loads(sql(f"""SET TRANSACTION READ ONLY;
SELECT jsonb_build_object('state',(SELECT estado FROM public.reserva_ensayo_local WHERE id='{main_id}'),
'active',(SELECT activo FROM public.ocupacion WHERE id='{occ}'),
'contract_hash',(SELECT encode(sha256,'hex') FROM public.contrato_ensayo_local WHERE id='{fixture['contract_id']}'));""")[0])
    if verified != {'state': 'en_curso', 'active': True, 'contract_hash': contract_hash}:
        raise RuntimeError('Committed fixture readback failed')
    output.write_text(json.dumps(fixture, indent=2)+'\n')
    os.chmod(output, 0o600)
    print('SYNTHETIC_RETURN_FIXTURE_PREPARED_AND_READ_BACK')
    print('ACCOUNTS_UNCHANGED_2 LEDGER_UNCHANGED_83 GALLERY_PRESERVED_30')


if __name__ == '__main__':
    main()
