import copy
from datetime import datetime,timezone
import unittest
import urllib.error
import ssl
from quality_gate import BASE_PATHS,DATABASE_PATH,REPOSITORY,REPOSITORY_ID,PendingQuality,evaluate,verify_main_ref,retryable_transport

NOW=datetime(2026,11,7,0,0,tzinfo=timezone.utc)
SHA='a'*40


def runs():
    return [{'id':n,'run_attempt':1,'path':p,'head_sha':SHA,'head_branch':'main','event':'push','status':'completed','conclusion':'success','updated_at':NOW.isoformat(),'repository':{'id':REPOSITORY_ID,'full_name':REPOSITORY},'head_repository':{'id':REPOSITORY_ID,'full_name':REPOSITORY}} for n,p in enumerate(sorted(BASE_PATHS|{DATABASE_PATH}),1)]


class ReleaseQualityTests(unittest.TestCase):
    def test_requires_exact_sha_branch_event_and_all_required_workflows(self):
        self.assertEqual(len(evaluate(runs(),SHA,now=NOW)['runs']),3)
        for field,value in [('head_sha','b'*40),('head_branch','codex/work'),('event','pull_request'),('path','another-workflow.yml')]:
            fixture=runs();fixture[0][field]=value
            with self.assertRaises(PendingQuality):evaluate(fixture,SHA,now=NOW)
        with self.assertRaises(PendingQuality):evaluate([],SHA,now=NOW)

    def test_web_scope_needs_quality_but_not_a_database_workflow_for_unchanged_backend(self):
        fixture=[r for r in runs() if r['path']!=DATABASE_PATH]
        self.assertEqual(len(evaluate(fixture,SHA,False,now=NOW)['runs']),2)
        with self.assertRaises(PendingQuality):evaluate(fixture,SHA,True,now=NOW)

    def test_failure_skip_cancel_staleness_and_foreign_repo_fail_closed(self):
        for mutate in [lambda r:r.update(conclusion='failure'),lambda r:r.update(conclusion='skipped'),lambda r:r.update(conclusion='cancelled'),lambda r:r.update(updated_at='2020-01-01T00:00:00Z'),lambda r:r.update(updated_at='2030-01-01T00:00:00Z'),lambda r:r['head_repository'].update(id=42)]:
            fixture=runs();mutate(fixture[0])
            with self.assertRaises(ValueError):evaluate(fixture,SHA,now=NOW)

    def test_latest_attempt_cannot_hide_behind_an_earlier_green_run(self):
        fixture=runs();latest=copy.deepcopy(fixture[0]);latest.update(run_attempt=2,conclusion='failure')
        with self.assertRaises(ValueError):evaluate(fixture+[latest],SHA,now=NOW)
        latest.update(status='in_progress',conclusion=None)
        with self.assertRaises(PendingQuality):evaluate(fixture+[latest],SHA,now=NOW)

    def test_transport_retries_do_not_relax_authentication_tls_or_quality_failures(self):
        for code in [408,429,500,502,503,504]:self.assertTrue(retryable_transport(urllib.error.HTTPError('https://api.github.com',code,'temporary',{},None)))
        for code in [400,401,403,404]:self.assertFalse(retryable_transport(urllib.error.HTTPError('https://api.github.com',code,'refused',{},None)))
        self.assertTrue(retryable_transport(urllib.error.URLError(TimeoutError())))
        self.assertFalse(retryable_transport(urllib.error.URLError(ssl.SSLCertVerificationError())))
        self.assertFalse(retryable_transport(ValueError('quality failed')))

    def test_rejects_obsolete_candidate_even_if_its_tests_were_green(self):
        verify_main_ref({'ref':'refs/heads/main','object':{'type':'commit','sha':SHA}},SHA)
        for ref in [{'ref':'refs/heads/main','object':{'type':'commit','sha':'b'*40}},{'ref':'refs/tags/release','object':{'type':'commit','sha':SHA}}]:
            with self.assertRaises(ValueError):verify_main_ref(ref,SHA)


if __name__=='__main__':unittest.main()
