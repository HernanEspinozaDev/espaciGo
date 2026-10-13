#!/usr/bin/env bash
# Local account selection and effective identity verification; never prints tokens.
set -euo pipefail
role="${1:-status}"
root="$(git rev-parse --show-toplevel)"
cd "$root"

case "$role" in
  development|review)
    if [[ "$role" == development ]]; then
      account=HernanMEC
    else
      account=HernanEspinozaDev
    fi
    gh auth switch --hostname github.com --user "$account"
    login="$(gh api user --jq .login)"
    account_id="$(gh api user --jq .id)"
    [[ "$login" == "$account" ]] || { echo 'La cuenta efectiva no coincide.' >&2; exit 1; }
    git config --local user.name "$login"
    git config --local user.email "$account_id+$login@users.noreply.github.com"
    git config --local --replace-all credential.https://github.com.helper ''
    git config --local --add credential.https://github.com.helper '!gh auth git-credential'
    git config --local core.hooksPath .githooks
    ;;
  check-development|status) ;;
  *) echo 'Uso: scripts/git-role.sh development|review|check-development|status' >&2; exit 2 ;;
esac

expected=''
[[ "$role" == development || "$role" == check-development ]] && expected=HernanMEC
[[ "$role" == review ]] && expected=HernanEspinozaDev
python3 - "$expected" <<'PY'
import json, subprocess, sys, urllib.request

def run(*args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs).strip()

try:
    cli = json.loads(run('gh', 'api', 'user'))['login']
    response = run('git', 'credential', 'fill', input='protocol=https\nhost=github.com\n\n', stderr=subprocess.DEVNULL)
    credentials = dict(line.split('=', 1) for line in response.splitlines() if '=' in line)
    request = urllib.request.Request('https://api.github.com/user', headers={
        'Authorization': 'Bearer ' + credentials['password'],
        'Accept': 'application/vnd.github+json', 'User-Agent': 'espacigo-role-check',
    })
    with urllib.request.urlopen(request, timeout=15) as result:
        git_login = json.load(result)['login']
    author = run('git', 'config', 'user.name')
    email = run('git', 'config', 'user.email')
    print(f'gh: {cli}; Git: {git_login}; autor: {author}')
    expected = sys.argv[1]
    if expected and (cli != expected or git_login != expected or author != expected or not email.endswith('+' + expected + '@users.noreply.github.com')):
        raise RuntimeError('identity mismatch')
except Exception:
    print('No se pudo verificar la identidad requerida. Ejecuta scripts/git-role.sh development o review; no publiques con otra cuenta.', file=sys.stderr)
    sys.exit(1)
PY
