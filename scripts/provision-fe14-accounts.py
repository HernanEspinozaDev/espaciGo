#!/usr/bin/env python3
"""Create and verify private FE-14 accounts against the disposable stack."""
import json
import os
import pathlib
import re
import secrets
import sys
import time
import urllib.error
import urllib.request
import uuid


def request(base, method, path, body=None):
    payload = None if body is None else json.dumps(body).encode()
    headers = {'Content-Type': 'application/json'}
    request = urllib.request.Request(base + path, data=payload, method=method, headers=headers)
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            data = response.read()
            return response.status, json.loads(data) if data else None
    except urllib.error.HTTPError as error:
        error.read()
        raise RuntimeError('synthetic account setup failed with HTTP status') from None


def mail_for(mailpit, email):
    until = time.monotonic() + 20
    while time.monotonic() < until:
        _, listing = request(mailpit, 'GET', '/api/v1/messages')
        for message in listing.get('messages') or []:
            if not any((recipient.get('Address') or '').lower() == email.lower() for recipient in message.get('To') or []):
                continue
            _, full = request(mailpit, 'GET', '/api/v1/message/' + message['ID'])
            body = full.get('Text') or ''
            token_id = re.search(r'^Token ID: ([0-9a-f-]+)', body, re.M)
            token = re.search(r'^Token: ([A-Za-z0-9_-]+)', body, re.M)
            if token_id and token:
                return token_id.group(1), token.group(1)
        time.sleep(.15)
    raise RuntimeError('synthetic verification message did not arrive in Mailpit')


def main():
    if len(sys.argv) != 5:
        raise SystemExit('usage: provision-fe14-accounts.py API MAILPIT RUN_ID OUTPUT')
    api, mailpit, run_id, output = sys.argv[1:]
    if not re.fullmatch(r'[0-9a-f]{12}', run_id):
        raise SystemExit('invalid private run identifier')
    _, terms = request(api, 'GET', '/api/v1/auth/terms')
    ids = [item['id'] for item in terms['items'] if item['type'] == 'terminos']
    ids += [item['id'] for item in terms['items'] if item['type'] == 'privacidad']
    if not ids or not all(item.get('synthetic') for item in terms['items']):
        raise RuntimeError('Go did not provide synthetic terms')

    result = {}
    for role in ('renter', 'host', 'admin', 'globalAdmin', 'thirdParty'):
        email = f'fe14-{run_id}-{role.lower()}@ejemplo.invalid'
        password = 'F14!' + secrets.token_urlsafe(24)
        status, _ = request(api, 'POST', '/api/v1/auth/register', {
            'email': email,
            'password': password,
            'terms_version_ids': ids,
        })
        if status != 201:
            raise RuntimeError('Go did not register a synthetic account')
        token_id, token = mail_for(mailpit, email)
        status, _ = request(api, 'POST', '/api/v1/auth/verification', {'token_id': token_id, 'token': token})
        if status != 204:
            raise RuntimeError('Go did not verify a synthetic account')
        label = {'renter': 'arrendatario', 'host': 'arrendador', 'admin': 'administrador', 'globalAdmin': 'administrador_global', 'thirdParty': 'arrendatario'}[role]
        result[role] = {'email': email, 'password': password, 'roleLabel': label}

    target = pathlib.Path(output)
    target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w', encoding='utf-8') as stream:
        json.dump(result, stream)
    os.chmod(target, 0o600)
    print('PASS Go registered and verified five disposable synthetic accounts; credentials are stored in a mode-0600 temporary file')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # The exception text is deliberately controlled so no mail content or credentials are printed.
        message = str(error)
        safe = message if message in {
            'synthetic account setup failed with HTTP status',
            'synthetic verification message did not arrive in Mailpit',
            'Go did not provide synthetic terms',
            'Go did not register a synthetic account',
            'Go did not verify a synthetic account',
        } else 'synthetic account setup failed without exposing response content'
        print('ERROR ' + safe, file=sys.stderr)
        raise SystemExit(1)
