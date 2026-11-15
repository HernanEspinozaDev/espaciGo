"""Offline contract tests; no provider, database, or migration execution."""
import copy
import hashlib
import unittest
from pathlib import Path
from typing import Any
from unittest.mock import patch

import migration_execution as executions
from release_jobs import execute_job, request
from migration_release_manifest import FORMAT, IMAGES, REGISTRY, TARGET, check_plan, validate_plan_contract
from test_migration_execution import BUILD, UID, CATALOG_FIXTURE, fixture as execution_fixture, verify

ROOT = Path(__file__).resolve().parents[2]


def safe_role(login=False):
    return dict(can_login=login, inherit=False, superuser=False, create_role=False,
                create_db=False, replication=False, bypass_rls=False)


def base_authority():
    return {
        'session_user': 'espacigo_migrator_login', 'current_user': 'espacigo_schema_owner',
        'current_schema': 'public', 'session_role': safe_role(True), 'schema_owner_role': safe_role(),
        'public_schema_owner': 'pg_database_owner', 'public_usage': True, 'public_create': True,
        'usuario_owner': 'postgres', 'usuario_id_references': True,
        'ledger_owner': 'postgres', 'ledger_select': True, 'ledger_insert': True,
        'memberships': [
            dict(role='espacigo_schema_owner', member='espacigo_migrator_login', admin=False, inherit=False, set=True),
            dict(role='espacigo_document_verification_fn', member='espacigo_schema_owner', admin=False, inherit=False, set=True),
        ],
        'catalog': {'schemas': ['pg_catalog', 'public'], 'helper_role': safe_role(),
                    'database_owner': 'postgres', 'database_create': False,
                    'acl': [dict(object='public', column='', owner='pg_database_owner',
                                 grantee='PUBLIC', grantor='pg_database_owner', privilege='USAGE', grantable=False)]},
    }


def tail_authority():
    return {
        'session_user': 'espacigo_migrator_login', 'current_user': 'espacigo_schema_owner',
        'current_schema': 'public', 'session_role': safe_role(True), 'public_create': True,
        'schema_owner_exists': True, 'schema_owner_role': safe_role(),
        'runtime_exists': True, 'runtime_role': safe_role(),
        'privacy_cleanup_worker_exists': True, 'privacy_cleanup_worker_role': safe_role(),
        'case_owner': 'espacigo_schema_owner', 'declaration_owner': 'espacigo_schema_owner',
        'event_owner': 'espacigo_schema_owner', 'purge_owner': 'espacigo_schema_owner',
        'purge_security_definer': True, 'purge_search_path': ['search_path=pg_catalog'],
        'purge_non_owner_acl_count': 1, 'purge_worker_execute': True,
        'purge_worker_grantor_owner': True, 'purge_worker_grantable': False,
        'runtime_purge_execute': False, 'runtime_case_select': True,
        'runtime_case_expiry_update': True, 'runtime_event_insert': True,
    }


def pending_fixture(version, with_identity=True):
    rows = CATALOG_FIXTURE.MODULE.migration_inventory(ROOT / 'db/migrations')[:version]
    inventory = {'format': 'espacigo.migrations.inventory.v1', 'migrations': rows,
                 'inventory_sha256': hashlib.sha256(''.join(f"{r['checksum']}  {r['name']}\n" for r in rows).encode()).hexdigest()}
    manifest = {'format': FORMAT, 'source_sha': 'a' * 40, 'target': TARGET.copy(),
                'images': {n: REGISTRY + '/' + r + '@sha256:' + 'b' * 64 for n, r in IMAGES.items()},
                'migration_inventory': inventory}
    plan: dict[str, Any] = dict(database=TARGET['database'], server_version='18.7', read_only=True,
                history_exists=True, applied=rows[:-1], pending=rows[-1:])
    if with_identity:
        plan.update(session_user='espacigo_migrator_login', current_user='espacigo_schema_owner')
    return manifest, plan


class AuthorityWireCompatibilityTests(unittest.TestCase):
    def test_exact_identity_pending_names_require_authority(self):
        for version in (84, 85, 86, 87, 88, 95, 96):
            with self.subTest(version=version):
                manifest, plan = pending_fixture(version)
                with self.assertRaises(ValueError):
                    check_plan(manifest, plan)

    def test_legacy_and_identified_plans_accept_complete_authority(self):
        for with_identity in (False, True):
            manifest, plan = pending_fixture(84, with_identity)
            plan['ddl_authority'] = base_authority()
            self.assertEqual(check_plan(manifest, plan)['pending'], 1)

    def test_per_version_privileges_match_go_guards(self):
        for version in (86, 87, 88):
            manifest, plan = pending_fixture(version)
            authority = base_authority()
            authority['usuario_id_references'] = False
            if version == 87:
                authority['public_create'] = False
            plan['ddl_authority'] = authority
            self.assertEqual(check_plan(manifest, plan)['pending'], 1)
            authority['ledger_insert'] = False
            with self.assertRaises(ValueError):
                check_plan(manifest, plan)

    def test_tail_v96_does_not_require_v95_worker(self):
        for version in (95, 96):
            manifest, plan = pending_fixture(version)
            plan['identity_ddl_authority'] = tail_authority()
            self.assertEqual(check_plan(manifest, plan)['pending'], 1)
            plan['identity_ddl_authority']['privacy_cleanup_worker_exists'] = False
            if version == 95:
                with self.assertRaises(ValueError):
                    check_plan(manifest, plan)
            else:
                self.assertEqual(check_plan(manifest, plan)['pending'], 1)

    def test_v96_still_requires_create_as_immutable_sql_specifies(self):
        manifest, plan = pending_fixture(96)
        plan['identity_ddl_authority'] = tail_authority()
        self.assertEqual(check_plan(manifest, plan)['pending'], 1)
        plan['identity_ddl_authority']['public_create'] = False
        with self.assertRaisesRegex(ValueError, 'CREATE'):
            check_plan(manifest, plan)

    def test_plan_contract_rejects_malformed_rows_before_authority_selection(self):
        _, plan = pending_fixture(97)
        validate_plan_contract(plan)
        for malformed in (None, {'version': 97, 'name': None, 'checksum': 'b'*64},
                          {'version': True, 'name': 'V000097__probe.sql', 'checksum': 'b'*64},
                          {'version': 97, 'name': 'V000097__probe.sql', 'checksum': 'bad'},
                          {'version': 97, 'name': 'V000097__probe.sql', 'checksum': 'b'*64, 'unknown': True}):
            invalid = copy.deepcopy(plan)
            invalid['pending'] = [malformed]
            with self.assertRaisesRegex(ValueError, 'migration.*row'):
                validate_plan_contract(invalid)

    def test_partial_unknown_wrong_identity_and_null_evidence_reject(self):
        for mutate in (
            lambda p: p.update(unexpected=True),
            lambda p: p.pop('current_user'),
            lambda p: p.update(current_user='postgres'),
            lambda p: p.update(ddl_authority=None),
            lambda p: p['ddl_authority'].update(unknown=True),
            lambda p: p['ddl_authority'].pop('ledger_insert'),
            lambda p: p['ddl_authority'].update(public_create=1),
        ):
            manifest, plan = pending_fixture(84)
            plan['ddl_authority'] = base_authority()
            mutate(plan)
            with self.assertRaises(ValueError):
                check_plan(manifest, plan)

    def test_application_evidence_must_match_reviewed_preflight(self):
        for version, field, authority in (
            (84, 'ddl_authority', base_authority()),
            (95, 'identity_ddl_authority', tail_authority()),
        ):
            _, plan = pending_fixture(version)
            plan[field] = authority
            report = {field: copy.deepcopy(authority)}
            executions.check_execution_authority(plan, report)
            with self.assertRaises(ValueError):
                executions.check_execution_authority(plan, {})
            report[field]['public_create'] = False
            with self.assertRaises(ValueError):
                executions.check_execution_authority(plan, report)
        _, plan = pending_fixture(97)
        with self.assertRaises(ValueError):
            executions.check_execution_authority(plan, {'ddl_authority': base_authority()})

    def test_full_execution_validator_preserves_tail_evidence_and_rejects_drift(self):
        manifest, before, execution, logs, name = execution_fixture()
        rows = manifest['migration_inventory']['migrations']
        before['applied'] = rows[:94]
        before['pending'] = rows[94:]
        before['identity_ddl_authority'] = tail_authority()
        proposed = executions.proposal(manifest, before, BUILD, '1')
        execution['spec']['template']['spec']['containers'][0]['args'] = proposed['args']
        report = logs[0]['jsonPayload']
        report['contract'] = proposed['contract']
        report['before'] = copy.deepcopy(before)
        report['applied'] = [row['version'] for row in rows[94:]]
        report['identity_ddl_authority'] = tail_authority()
        self.assertEqual(verify(manifest, before, execution, logs, name)['applied'], [95, 96, 97])
        report['identity_ddl_authority']['public_create'] = False
        with self.assertRaisesRegex(ValueError, 'apply-time authority'):
            verify(manifest, before, execution, logs, name)


class PlanLogFixture:
    """Provider substitute only; the production log selection/validator run."""
    def __init__(self, report):
        self.report = report
        self.calls = []
        self.name = 'espacigo-db-p-' + BUILD.replace('-', '')
        self.execution = self.name + '-abc12'
        self.task = {'synthetic': 'same-task'}

    def json(self, *args):
        self.calls.append(args)
        if args[:3] == ('run', 'jobs', 'execute'):
            return {'metadata': {'name': self.execution}}
        if args[:4] == ('run', 'jobs', 'executions', 'describe'):
            return {'metadata': {'namespace': '447373903153', 'labels': {'run.googleapis.com/jobUid': UID}},
                    'spec': {'template': {'spec': self.task}},
                    'status': {'conditions': [{'type': 'Completed', 'status': 'True'}], 'succeededCount': 1}}
        if args[:2] == ('logging', 'read'):
            return [{'jsonPayload': self.report,
                     'resource': {'labels': {'project_id': 'espacigo-testprod', 'location': 'southamerica-west1', 'job_name': self.name}},
                     'labels': {'run.googleapis.com/execution_name': self.execution}}]
        raise AssertionError(('unexpected operation', args))

    def job(self):
        return {'metadata': {'name': self.name, 'uid': UID},
                'spec': {'template': {'spec': {'template': {'spec': self.task}}}}}


class AuthorityJobLogReaderTests(unittest.TestCase):
    def execute_report(self, manifest, report):
        cloud = PlanLogFixture(report)
        with patch('release_jobs.time.sleep', side_effect=AssertionError('plan-shaped record was discarded')):
            try:
                return execute_job(cloud, cloud.job(), request(manifest, BUILD, 'plan'), lambda *args: None)
            finally:
                self.assertEqual(sum(call[:2] == ('logging', 'read') for call in cloud.calls), 1)
                self.assertEqual(sum(call[:3] == ('run', 'jobs', 'execute') for call in cloud.calls), 1)
                self.assertFalse(any('backups' in call or 'deploy' in call for call in cloud.calls))

    def test_real_reader_passes_complete_authority_without_stripping_fields(self):
        for version, field, authority in ((84, 'ddl_authority', base_authority()), (95, 'identity_ddl_authority', tail_authority())):
            manifest, report = pending_fixture(version)
            report[field] = authority
            self.assertEqual(self.execute_report(manifest, report), report)

    def test_real_reader_surfaces_malformed_authority_to_strict_validation(self):
        for mutate in (lambda p: p.update(unexpected=True), lambda p: p.pop('identity_ddl_authority'),
                       lambda p: p['identity_ddl_authority'].update(purge_non_owner_acl_count=True)):
            manifest, report = pending_fixture(95)
            report['identity_ddl_authority'] = tail_authority()
            mutate(report)
            with self.assertRaises(ValueError):
                self.execute_report(manifest, report)


if __name__ == '__main__':
    unittest.main()
