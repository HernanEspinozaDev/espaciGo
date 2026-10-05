#!/usr/bin/env python3
"""Local real-HTTP/PostgreSQL/SMTP flow. Prints checks only, never credentials."""
import argparse
import json
import pathlib
import re
import subprocess
import time
import urllib.error
import urllib.request
import uuid
from urllib.parse import urlparse

import jsonschema
import yaml

parser = argparse.ArgumentParser()
parser.add_argument('--api', default='http://127.0.0.1:8080')
parser.add_argument('--mailpit', default='http://127.0.0.1:8025')
parser.add_argument('--mock', default='http://127.0.0.1:8081')
args = parser.parse_args()
for base in (args.api, args.mailpit, args.mock):
    assert urlparse(base).hostname in ('localhost', '127.0.0.1'), 'requires local prototype'
root = pathlib.Path(__file__).resolve().parent.parent
spec = yaml.safe_load((root / 'planning/openapi.yaml').read_text())


def resolve(value):
    if isinstance(value, dict):
        if '$ref' in value:
            assert value['$ref'].startswith('#/'), 'only local contract refs'
            target = spec
            for key in value['$ref'][2:].split('/'):
                target = target[key]
            return resolve(target)
        return {key: resolve(item) for key, item in value.items()}
    if isinstance(value, list):
        return [resolve(item) for item in value]
    return value


# Parse/check referenced response schemas before running the real flow.
for operations in spec['paths'].values():
    for operation in operations.values():
        for response in operation['responses'].values():
            response = resolve(response)
            for content in response.get('content', {}).values():
                jsonschema.Draft202012Validator.check_schema(content['schema'])
print('PASS OpenAPI YAML, references and JSON schemas')


def raw(method, url, data=None, headers=None):
    payload = None if data is None else json.dumps(data).encode()
    request = urllib.request.Request(url, data=payload, method=method,
                                    headers={'Content-Type': 'application/json', **(headers or {})})
    try:
        response = urllib.request.urlopen(request, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    body = response.read()
    return response.status, (json.loads(body) if body else None), response.headers


secrets = []


def api(method, path, expected, data=None, token=None, label=None):
    headers = {'Authorization': 'Bearer ' + token} if token else {}
    status, body, response_headers = raw(method, args.api + '/api/v1/auth/' + path, data, headers)
    assert status == expected, f'{label or path}: unexpected HTTP {status}, expected {expected}'
    uuid.UUID(response_headers['X-Request-ID'])
    assert response_headers['Cache-Control'] == 'no-store', 'private response cached'
    contract = resolve(spec['paths']['/auth/' + path][method.lower()]['responses'][str(status)])
    if body is not None:
        schema = contract['content']['application/json']['schema']
        try:
            jsonschema.Draft202012Validator(schema).validate(body)
        except jsonschema.ValidationError:
            raise AssertionError('response schema mismatch') from None
    if status == 401:
        assert response_headers['WWW-Authenticate'] == 'Bearer', 'missing challenge'
    print('PASS', label or path, 'HTTP', status, 'contract')
    return body


terms = api('GET', 'terms', 200)
assert all(item['synthetic'] for item in terms['items']), 'requires synthetic local terms'
term_ids = [item['id'] for item in terms['items'] if item['type'] == 'terminos']
email = f'prototype-{uuid.uuid4().hex}@ejemplo.invalid'
password = 'Synthetic#123'
api('POST', 'register', 400, {'email': email, 'role': 'administrador'}, label='unknown fields rejected')
api('POST', 'register', 422, {'email': email, 'password': 'weak', 'terms_version_ids': term_ids}, label='password policy')
api('GET', 'session', 401, label='session requires Bearer')
registration = api('POST', 'register', 201, {'email': email, 'password': password, 'terms_version_ids': term_ids})
api('POST', 'register', 409, {'email': email.upper(), 'password': password, 'terms_version_ids': term_ids}, label='canonical duplicate')
api('POST', 'login', 403, {'email': email, 'password': password}, label='unverified login denied')


def verification_mail(excluded_id=None):
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        _, listing, _ = raw('GET', args.mailpit + '/api/v1/messages')
        for item in listing['messages']:
            if any(recipient['Address'].lower() == email.lower() for recipient in (item.get('To') or [])):
                _, message, _ = raw('GET', args.mailpit + '/api/v1/message/' + item['ID'])
                text = message['Text']
                token_id = re.search(r'^Token ID: ([0-9a-f-]+)', text, re.M)
                token = re.search(r'^Token: ([A-Za-z0-9_-]+)', text, re.M)
                if token_id and token and token_id[1] != excluded_id:
                    secrets.append(token[1])
                    return token_id[1], token[1]
        time.sleep(.1)
    raise AssertionError('verification message not captured by local SMTP')


old_id, old_token = verification_mail()
api('POST', 'verification/reissue', 204, {'email': email})
new_id, new_token = verification_mail(old_id)
assert old_id != new_id, 'SMTP reissue not captured'
api('POST', 'verification', 422, {'token_id': old_id, 'token': old_token}, label='replaced token rejected')
api('POST', 'verification', 422, {'token_id': new_id, 'token': 'incorrect'}, label='incorrect verification rejected')
api('POST', 'verification', 204, {'token_id': new_id, 'token': new_token})
api('POST', 'verification', 422, {'token_id': new_id, 'token': new_token}, label='verification replay rejected')
api('POST', 'login', 401, {'email': email, 'password': 'Wrong#123'}, label='wrong password denied')
login = api('POST', 'login', 200, {'email': email.upper(), 'password': password})
secrets.append(login['access_token'])
assert login['roles'] == ['arrendatario'], 'registration granted elevated role'
session = api('GET', 'session', 200, token=login['access_token'])
assert session['account_id'] == registration['account_id'], 'session resource mismatch'
api('POST', 'logout', 204, token=login['access_token'])
api('GET', 'session', 401, token=login['access_token'], label='revoked session replay rejected')
api('POST', 'logout', 401, label='logout requires Bearer')
# Actual CORS transport, not a mocked response.
status, _, headers = raw('OPTIONS', args.api + '/api/v1/auth/login', headers={'Origin': 'http://localhost:8081', 'Access-Control-Request-Method': 'POST'})
assert status == 204 and headers['Access-Control-Allow-Origin'] == 'http://localhost:8081'
status, _, _ = raw('GET', args.api + '/api/v1/auth/session', headers={'Origin': 'http://unexpected.invalid'})
assert status == 403
print('PASS CORS allowed/preflight and forbidden origin')
for path in ('/', '/app.js', '/styles.css'):
    with urllib.request.urlopen(args.mock + path) as response:
        assert response.status == 200
print('PASS separate mock serves HTML, compiled TypeScript and CSS')
# Capture logs in memory only. Never print either logs or matching credentials.
logs = subprocess.check_output(['docker', 'compose', '--project-directory', str(root), '-f', str(root / 'compose.yaml'), 'logs', '--no-color'], env={**__import__('os').environ, 'LOCAL_UID': str(__import__('os').getuid()), 'LOCAL_GID': str(__import__('os').getgid())}, stderr=subprocess.DEVNULL, text=True)
assert all(secret not in logs for secret in secrets + [password]), 'credential found in container logs'
print('PASS actual verification/session secrets absent from stack logs')
print('PASS M01 local real DB -> backend -> HTTP -> SMTP inbox -> session -> logout')
