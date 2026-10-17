import unittest

from gcp03_job_evidence import validate


class JobEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.execution = {"status": {"succeededCount": 1, "conditions": [{"type": "Completed", "status": "True"}]}}

    def test_cloud_run_success_requires_matching_phase_payload(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstdout", "labels": {"container_name": "retire"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "phase": "retire", "run_id": "g03r-1016", "commit_outcome": "committed",
        }}]
        ok, result = validate(self.execution, logs, "retire", "g03r-1016")
        self.assertTrue(ok)
        self.assertEqual(result["result"], "success")

    def test_cloud_run_completed_does_not_hide_application_failure(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstderr", "labels": {"container_name": "retire"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-failure-v1",
            "phase": "retire", "run_id": "g03r-1016", "error_code": "PHASE_FAILED",
            "substep": "audit_acl_precheck", "application_exit_code": 1, "private": "must-not-be-echoed",
        }}]
        ok, result = validate(self.execution, logs, "retire", "g03r-1016")
        self.assertFalse(ok)
        self.assertEqual(result["result"], "application_phase_failed_despite_cloud_run_success")
        self.assertEqual(result["application_exit_code"], 1)
        self.assertNotIn("private", str(result))

    def test_successful_diagnosis_requires_read_only_attestation(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstdout", "labels": {"container_name": "retire-diagnose"}, "jsonPayload": {
            "format": "espacigo-gcp03-retirement-diagnosis-v1", "phase": "retire-diagnose",
            "run_id": "g03r-1016", "retirement_diagnosis": {"read_only_transaction": True},
        }}]
        self.assertTrue(validate(self.execution, logs, "retire-diagnose", "g03r-1016")[0])
        logs[0]["jsonPayload"]["retirement_diagnosis"]["read_only_transaction"] = False
        self.assertEqual(validate(self.execution, logs, "retire-diagnose", "g03r-1016")[1]["result"], "read_only_diagnosis_not_confirmed")

    def test_missing_or_mismatched_phase_is_rejected(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstdout", "labels": {"container_name": "retire"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "phase": "retire", "run_id": "another-run", "commit_outcome": "committed",
        }}]
        self.assertEqual(validate(self.execution, logs, "retire", "g03r-1016")[1]["result"], "application_success_evidence_missing_or_ambiguous")

    def test_execution_failure_is_rejected_even_with_success_payload(self):
        execution = {"status": {"succeededCount": 0, "conditions": [{"type": "Completed", "status": "False"}]}}
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstdout", "labels": {"container_name": "retire"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "phase": "retire", "run_id": "g03r-1016", "commit_outcome": "committed",
        }}]
        self.assertFalse(validate(execution, logs, "retire", "g03r-1016")[0])

    def test_success_payload_on_stderr_is_rejected(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstderr", "labels": {"container_name": "retire"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "phase": "retire", "run_id": "g03r-1016", "commit_outcome": "committed",
        }}]
        self.assertEqual(validate(self.execution, logs, "retire", "g03r-1016")[1]["result"], "application_success_evidence_missing_or_ambiguous")


if __name__ == "__main__":
    unittest.main()
