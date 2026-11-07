import json
import io
from http.client import HTTPMessage
import unittest
from urllib.error import HTTPError
from unittest.mock import patch

import recovery_observation_v2 as observation


class Response:
    def __init__(self, payload, status=200):
        self.payload = payload.encode() if isinstance(payload, str) else payload
        self.status = status
    def read(self, size=-1):
        return self.payload if size < 0 else self.payload[:size]
    def __enter__(self): return self
    def __exit__(self, *args): pass
    def getcode(self): return self.status


class ObservationTests(unittest.TestCase):
    def test_other_url_fails_before_token_or_http(self):
        with patch.object(observation, 'urlopen', return_value=Response('{}')) as opener:
            with self.assertRaises(observation.ObservationError):
                observation.HistoricalExecutionReader()._request('https://untrusted.example.test/')
            self.sdk_run.assert_not_called()
            opener.assert_not_called()

    def test_real_redirect_handler_does_not_forward_bearer(self):
        request = observation.urllib.request.Request(observation.BASE, headers={'Authorization': 'Bearer synthetic'})
        self.assertIsNone(observation._NoRedirect().redirect_request(request, io.BytesIO(), 302, 'redirect', HTTPMessage(), 'https://other.example.test/'))

    def setUp(self):
        self.sdk_run = patch.object(observation.subprocess, 'run').start()
        self.addCleanup(patch.stopall)
        self.sdk_run.return_value.returncode = 0
        self.sdk_run.return_value.stdout = 'token-secret\n'
        self.sdk_run.return_value.stderr = ''

    def opener(self, response):
        calls = []
        def open_(request, timeout):
            calls.append((request, timeout))
            return response
        return calls, open_

    def test_list_uses_exact_scoped_v2_get_and_account(self):
        calls, op = self.opener(Response('{"executions":[]}'))
        with patch.object(observation, 'urlopen', op):
            result = observation.HistoricalExecutionReader(account='operator@example.test').list()
        req, _ = calls[0]
        self.assertEqual(result, {'executions': []})
        self.assertEqual(req.method, 'GET')
        self.assertIn('/projects/espacigo-testprod/locations/southamerica-west1/jobs/espacigo-db-r-2dc002a95831471780646432b98d3e01/executions?', req.full_url)
        self.assertNotIn("'-'", req.full_url)
        self.assertIn('pageSize=2', req.full_url)
        self.assertIn('fields=', req.full_url)
        self.assertEqual(req.get_header('Authorization'), 'Bearer token-secret')
        self.assertIn('--account=operator@example.test', self.sdk_run.call_args.args[0])
        self.assertFalse(any('impersonat' in x for x in self.sdk_run.call_args.args[0]))

    def test_get_uses_fixed_execution(self):
        calls, op = self.opener(Response('{"name":"execution"}'))
        with patch.object(observation, 'urlopen', op):
            self.assertEqual(observation.HistoricalExecutionReader().get(), {'name': 'execution'})
        self.assertIn('/executions/espacigo-db-r-2dc002a95831471780646432b98d3e01-v62zb?', calls[0][0].full_url)

    def test_missing_token_is_sanitized(self):
        self.sdk_run.return_value.stdout = ' '
        with self.assertRaises(observation.ObservationError) as err:
            observation.HistoricalExecutionReader().list()
        self.assertNotIn('token-secret', str(err.exception))

    def test_http_error_sanitizes_secret(self):
        with patch.object(observation, 'urlopen', side_effect=HTTPError('https://safe', 403, 'token-secret', HTTPMessage(), None)):
            with self.assertRaises(observation.ObservationError) as err: observation.HistoricalExecutionReader().get()
        self.assertNotIn('token-secret', str(err.exception))

    def test_redirect_is_rejected(self):
        with patch.object(observation, 'urlopen', side_effect=observation.urllib.error.HTTPError('https://evil.test', 302, 'redirect', HTTPMessage(), None)):
            with self.assertRaises(observation.ObservationError): observation.HistoricalExecutionReader().get()

    def test_oversize_and_bad_json_rejected(self):
        for body in (b'x' * (1024 * 1024 + 1), b'{bad'):
            with self.subTest(size=len(body)), patch.object(observation, 'urlopen', return_value=Response(body)):
                with self.assertRaises(observation.ObservationError): observation.HistoricalExecutionReader().get()

    def test_api_documentation_is_scoped_parent(self):
        self.assertIn('https://docs.cloud.google.com/run/docs/reference/rest/v2/projects.locations.jobs.executions/list', observation.__doc__ or '')


if __name__ == '__main__': unittest.main()
