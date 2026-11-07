"""Read-only observation of one historical Cloud Run execution.

The V2 executions.list API scopes run.executions.list to the named parent
(job), unlike the legacy namespace-scoped list API:
https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.jobs.executions/list
"""
from __future__ import annotations

import json
import subprocess
import urllib.error
import urllib.parse
import urllib.request

from verify_web_backend import PROJECT, REGION

JOB_ID = 'espacigo-db-r-2dc002a95831471780646432b98d3e01'
EXECUTION_ID = JOB_ID + '-v62zb'
BASE = f'https://run.googleapis.com/v2/projects/{PROJECT}/locations/{REGION}/jobs/{JOB_ID}'
LIST_FIELDS = 'executions(name,uid,job,conditions,failedCount,runningCount,succeededCount),nextPageToken'
GET_FIELDS = 'name,uid,job,conditions,failedCount,runningCount,succeededCount'
MAX_RESPONSE = 1024 * 1024
TIMEOUT = 20
LIST_URL = BASE + '/executions?' + urllib.parse.urlencode({'pageSize': '2', 'fields': LIST_FIELDS})
GET_URL = BASE + '/executions/' + EXECUTION_ID + '?' + urllib.parse.urlencode({'fields': GET_FIELDS})


class ObservationError(ValueError):
    """Sanitized failure while reading historical execution metadata."""


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


_opener = urllib.request.build_opener(_NoRedirect)
urlopen = _opener.open


class HistoricalExecutionReader:
    def __init__(self, account=None):
        self.account = account

    def _request(self, url):
        if url not in (LIST_URL, GET_URL):
            raise ObservationError('observation URL is outside the exact historical target')
        try:
            args = ['gcloud', 'auth', 'print-access-token']
            if self.account:
                args.append('--account=' + self.account)
            proc = subprocess.run(args, text=True, capture_output=True, timeout=TIMEOUT, check=False)
            token = proc.stdout.strip() if proc.returncode == 0 else ''
            if not token:
                raise ObservationError('authentication token unavailable')
            request = urllib.request.Request(url, headers={'Authorization': 'Bearer ' + token}, method='GET')
            with urlopen(request, timeout=TIMEOUT) as response:
                body = response.read(MAX_RESPONSE + 1)
            if len(body) > MAX_RESPONSE:
                raise ObservationError('observation response exceeds size limit')
            try:
                result = json.loads(body)
            except (ValueError, UnicodeDecodeError):
                raise ObservationError('observation response is invalid JSON') from None
            if not isinstance(result, dict):
                raise ObservationError('observation response has invalid shape')
            return result
        except ObservationError:
            raise
        except Exception:
            # Never include subprocess/network exceptions; they may contain credentials or payloads.
            raise ObservationError('read-only observation request failed') from None

    def list(self):
        return self._request(LIST_URL)

    def get(self):
        return self._request(GET_URL)
