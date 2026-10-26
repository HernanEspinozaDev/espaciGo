#!/usr/bin/env python3
"""Offline regression tests for the web-only release boundary."""
import importlib.util
import io
import tarfile
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

class WebReleaseTests(unittest.TestCase):
    def test_web_release_is_explicitly_guarded_and_full_gate_is_preserved(self):
        text = (ROOT / 'cloudbuild.yaml').read_text()
        self.assertIn('_DEPLOY_SCOPE: web', text)
        self.assertIn('scripts/gcp/rollout_web.py', text)
        self.assertIn('verify_img01_schema_release_evidence.py', text)
        self.assertIn('_APP_SCHEMA_EVIDENCE_GENERATION', text)

    def test_backend_guard_rejects_changed_runtime_sources(self):
        module_path = ROOT / 'scripts/gcp/verify_web_backend.py'
        self.assertTrue(module_path.is_file(), 'backend parity guard is missing')
        spec = importlib.util.spec_from_file_location('guard', module_path)
        assert spec and spec.loader
        guard = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(guard)
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'go.mod').write_text('module original')
            archive = root / 'source.tgz'
            with tarfile.open(archive, 'w:gz') as tar:
                body = b'module original'
                member = tarfile.TarInfo('go.mod'); member.size = len(body)
                tar.addfile(member, io.BytesIO(body))
            self.assertEqual(guard.compare_backend(root, archive), 1)
            (root / 'go.mod').write_text('module changed')
            with self.assertRaisesRegex(guard.GuardError, 'full rollout'):
                guard.compare_backend(root, archive)

    def test_web_deploy_plan_never_updates_api_or_auth_policy(self):
        module_path = ROOT / 'scripts/gcp/rollout_web.py'
        self.assertTrue(module_path.is_file(), 'web rollout executor is missing')
        spec = importlib.util.spec_from_file_location('rollout', module_path)
        assert spec and spec.loader
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        image = 'southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/espacigo-web@sha256:' + 'a' * 64
        args = module.deploy_args('espacigo-test-private', image, 'b' * 40, '12345678-1234-1234-1234-123456789abc', 'https://espacigo-test-api-test.a.run.app')
        self.assertIn('--no-traffic', args)
        self.assertNotIn('--allow-unauthenticated', args)
        self.assertNotIn('--no-allow-unauthenticated', args)
        self.assertFalse(any(a.startswith('--set-env-vars') or a.startswith('--set-secrets') for a in args))
        with self.assertRaises(ValueError):
            module.deploy_args('espacigo-test-api', image, 'b' * 40, '12345678-1234-1234-1234-123456789abc', 'https://espacigo-test-api-test.a.run.app')

    def test_archive_rejects_traversal_symlinks_duplicates_and_missing_files(self):
        import verify_web_backend as guard
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'go.mod').write_text('module safe')
            archive = root / 'source.tgz'
            for names, symlink in [(['../go.mod'], False), (['go.mod'], True), (['go.mod', 'go.mod'], False), (['internal/new.go', 'go.mod'], False)]:
                with self.subTest(names=names, symlink=symlink):
                    with tarfile.open(archive, 'w:gz') as tar:
                        for name in names:
                            member = tarfile.TarInfo(name)
                            if symlink:
                                member.type = tarfile.SYMTYPE; member.linkname = '/etc/passwd'
                                tar.addfile(member)
                            else:
                                body = b'module safe'; member.size = len(body)
                                tar.addfile(member, io.BytesIO(body))
                    with self.assertRaises(guard.GuardError):
                        guard.compare_backend(root, archive)

    def test_slsa_source_requires_exact_digest_project_generation_and_hash(self):
        import copy
        import verify_web_backend as guard
        digest = 'a' * 64
        source_uri = f'gs://{guard.BUCKET}/source/test.tgz#123'
        statement = {'predicateType': 'https://slsa.dev/provenance/v1', 'subject': [{'digest': {'sha256': digest}}], 'predicate': {'runDetails': {'metadata': {'invocationId': f'https://cloudbuild.googleapis.com/v1/projects/{guard.PROJECT}/locations/{guard.REGION}/builds/12345678-1234-1234-1234-123456789abc'}}, 'buildDefinition': {'externalParameters': {'gcsPath': {'bucket': guard.BUCKET, 'object': 'source/test.tgz', 'generation': 123}}, 'resolvedDependencies': [{'uri': source_uri, 'digest': {'sha256': 'b' * 64}}]}}}
        report = {'image_summary': {'digest': 'sha256:' + digest}, 'provenance_summary': {'provenance': [{'build': {'inTotoSlsaProvenanceV1': statement}}]}}
        image = f'{guard.REGISTRY}/espacigo-api@sha256:{digest}'
        self.assertEqual(guard.source_evidence(report, image)['uri'], source_uri)
        for mutation in ('digest', 'project', 'generation', 'hash', 'duplicate'):
            bad = copy.deepcopy(report)
            data = bad['provenance_summary']['provenance'][0]['build']['inTotoSlsaProvenanceV1']['predicate']
            if mutation == 'digest': bad['image_summary']['digest'] = 'sha256:' + 'c' * 64
            if mutation == 'project': data['runDetails']['metadata']['invocationId'] = data['runDetails']['metadata']['invocationId'].replace('espacigo-testprod', 'wrong-project')
            if mutation == 'generation': data['buildDefinition']['externalParameters']['gcsPath']['generation'] = ''
            if mutation == 'hash': data['buildDefinition']['resolvedDependencies'] = []
            if mutation == 'duplicate': bad['provenance_summary']['provenance'] *= 2
            with self.subTest(mutation=mutation), self.assertRaises(guard.GuardError):
                guard.source_evidence(bad, image)

    def test_full_release_still_fails_before_gcloud_when_evidence_is_missing(self):
        import subprocess
        import yaml
        from importlib.util import spec_from_file_location, module_from_spec
        spec = spec_from_file_location('transition', ROOT / 'scripts/validate_cloud_build_transition.py')
        assert spec and spec.loader
        transition = module_from_spec(spec); spec.loader.exec_module(transition)
        config = yaml.safe_load((ROOT / 'cloudbuild.yaml').read_text())
        step = next(s for s in config['steps'] if s['id'] == 'acquire-and-validate-deploy-lock')
        for scope, expected in [('web', 0), ('full', 1), ('bogus', 1)]:
            values = dict(config['substitutions'], COMMIT_SHA='a' * 40, BUILD_ID='12345678-1234-1234-1234-123456789abc', PROJECT_ID='espacigo-testprod')
            values['_DEPLOY_SCOPE'] = scope
            body = transition.expand_cloud_build_substitutions(step['args'][-1], values)
            result = subprocess.run(['/bin/bash', '-ceu', body], capture_output=True, text=True, env={'PATH': '/nonexistent'})
            self.assertEqual(result.returncode, expected, (scope, result.stderr))

    def test_revision_must_match_digest_labels_and_readiness_before_promotion(self):
        import rollout_web as rollout
        image = 'registry@sha256:' + 'a' * 64
        revision = {'status': {'imageDigest': image, 'conditions': [{'type': 'Ready', 'status': 'True'}]}, 'metadata': {'labels': {'source-commit': 'b' * 40, 'cloud-build-id': 'test-build'}}}
        rollout.ready_revision(revision, image, 'b' * 40, 'test-build')
        with self.assertRaises(rollout.GuardError):
            rollout.ready_revision(revision, image, 'c' * 40, 'test-build')
        revision['status']['conditions'][0]['status'] = 'False'
        with self.assertRaises(rollout.GuardError):
            rollout.ready_revision(revision, image, 'b' * 40, 'test-build')

    def test_parent_build_identity_rejects_wrong_scope_sha_and_trigger(self):
        import copy
        import rollout_web as rollout
        sha = 'a' * 40
        build_id = '12345678-1234-1234-1234-123456789abc'
        parent = {'id': build_id, 'buildTriggerId': rollout.TRIGGER, 'serviceAccount': rollout.SA, 'status': 'WORKING', 'createTime': '2026-10-26T00:00:00Z', 'substitutions': {'COMMIT_SHA': sha, '_DEPLOY_SCOPE': 'web', 'REPO_FULL_NAME': 'HernanEspinozaDev/espaciGo'}}
        self.assertGreater(rollout.parent_identity(parent, sha, build_id), 0)
        for bad_key in ('scope', 'sha', 'trigger', 'account', 'status'):
            bad = copy.deepcopy(parent)
            if bad_key == 'scope': bad['substitutions']['_DEPLOY_SCOPE'] = 'full'
            if bad_key == 'sha': bad['substitutions']['COMMIT_SHA'] = 'b' * 40
            if bad_key == 'trigger': bad['buildTriggerId'] = 'wrong'
            if bad_key == 'account': bad['serviceAccount'] = 'default-compute'
            if bad_key == 'status': bad['status'] = 'FAILURE'
            with self.subTest(key=bad_key), self.assertRaises(rollout.GuardError):
                rollout.parent_identity(bad, sha, build_id)

    def test_private_iap_must_preexist_and_must_not_be_public(self):
        import rollout_web as rollout
        class FakeCloud:
            public = False
            iap = True
            def json(self, *args):
                if 'get-iam-policy' in args:
                    return {'bindings': [{'role': 'roles/run.invoker', 'members': ['allUsers'] if self.public else ['serviceAccount:service-447373903153@gcp-sa-iap.iam.gserviceaccount.com']}]}
                return {'metadata': {'annotations': {'run.googleapis.com/iap-enabled': 'true' if self.iap else 'false'}}}
        cloud = FakeCloud()
        rollout.verify_private(cloud)
        cloud.public = True
        with self.assertRaises(rollout.GuardError): rollout.verify_private(cloud)
        cloud.public = False; cloud.iap = False
        with self.assertRaises(rollout.GuardError): rollout.verify_private(cloud)

if __name__ == '__main__':
    unittest.main()
