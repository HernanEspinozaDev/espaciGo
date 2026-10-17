import unittest

from gcp03_job_evidence import validate


class JobEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.execution = {"metadata": {"name": "projects/p/locations/l/jobs/j/executions/current-execution"},
                          "status": {"succeededCount": 1, "conditions": [{"type": "Completed", "status": "True"}]}}

    def log(self, phase, payload, stream="stdout", execution_name="current-execution"):
        labels = {"container_name": phase}
        if execution_name is not None:
            labels["run.googleapis.com/execution_name"] = execution_name
        return {"logName": f"projects/p/logs/run.googleapis.com%2F{stream}", "labels": labels,
                "jsonPayload": {"phase": phase, "run_id": "g03r-1016", **payload}}

    def success(self, phase="retire", **extra):
        formats = {
            "retire": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "retire-diagnose": "espacigo-gcp03-retirement-diagnosis-v1",
        }
        payload = {"format": formats[phase], **extra}
        if phase == "retire":
            payload.setdefault("commit_outcome", "committed")
        else:
            payload.setdefault("retirement_diagnosis", {"read_only_transaction": True})
        return self.log(phase, payload)

    def test_cloud_run_success_requires_matching_phase_payload(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstdout", "labels": {"container_name": "retire", "run.googleapis.com/execution_name": "current-execution"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "phase": "retire", "run_id": "g03r-1016", "commit_outcome": "committed",
        }}]
        ok, result = validate(self.execution, logs, "retire", "g03r-1016")
        self.assertTrue(ok)
        self.assertEqual(result["result"], "success")

    def test_cloud_run_completed_does_not_hide_application_failure(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstderr", "labels": {"container_name": "retire", "run.googleapis.com/execution_name": "current-execution"}, "jsonPayload": {
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
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstdout", "labels": {"container_name": "retire-diagnose", "run.googleapis.com/execution_name": "current-execution"}, "jsonPayload": {
            "format": "espacigo-gcp03-retirement-diagnosis-v1", "phase": "retire-diagnose",
            "run_id": "g03r-1016", "retirement_diagnosis": {"read_only_transaction": True},
        }}]
        self.assertTrue(validate(self.execution, logs, "retire-diagnose", "g03r-1016")[0])
        logs[0]["jsonPayload"]["retirement_diagnosis"]["read_only_transaction"] = False
        self.assertEqual(validate(self.execution, logs, "retire-diagnose", "g03r-1016")[1]["result"], "read_only_diagnosis_not_confirmed")

    def test_missing_or_mismatched_phase_is_rejected(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstdout", "labels": {"container_name": "retire", "run.googleapis.com/execution_name": "current-execution"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "phase": "retire", "run_id": "another-run", "commit_outcome": "committed",
        }}]
        self.assertEqual(validate(self.execution, logs, "retire", "g03r-1016")[1]["result"], "application_success_evidence_missing_or_ambiguous")

    def test_execution_failure_is_rejected_even_with_success_payload(self):
        execution = {"status": {"succeededCount": 0, "conditions": [{"type": "Completed", "status": "False"}]}}
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstdout", "labels": {"container_name": "retire", "run.googleapis.com/execution_name": "current-execution"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "phase": "retire", "run_id": "g03r-1016", "commit_outcome": "committed",
        }}]
        self.assertFalse(validate(execution, logs, "retire", "g03r-1016")[0])

    def test_success_payload_on_stderr_is_rejected(self):
        logs = [{"logName": "projects/p/logs/run.googleapis.com%2Fstderr", "labels": {"container_name": "retire", "run.googleapis.com/execution_name": "current-execution"}, "jsonPayload": {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1",
            "phase": "retire", "run_id": "g03r-1016", "commit_outcome": "committed",
        }}]
        self.assertEqual(validate(self.execution, logs, "retire", "g03r-1016")[1]["result"], "application_success_evidence_missing_or_ambiguous")

    def test_execution_identity_accepts_matching_short_and_full_resource_names(self):
        full = "projects/p/locations/l/jobs/j/executions/current-execution"
        self.assertTrue(validate(self.execution, [self.success()], "retire", "g03r-1016")[0])
        self.assertTrue(validate(self.execution, [self.log("retire", {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1", "commit_outcome": "committed",
        }, execution_name=full)], "retire", "g03r-1016")[0])
        other_resource = self.log("retire", {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1", "commit_outcome": "committed",
        }, execution_name="projects/other/locations/l/jobs/j/executions/current-execution")
        self.assertEqual(validate(self.execution, [other_resource], "retire", "g03r-1016")[1]["result"],
                         "application_success_evidence_missing_or_ambiguous")

    def test_missing_or_invalid_execution_identity_is_rejected(self):
        logs = [self.success()]
        for execution in (
            {"status": self.execution["status"]},
            {"metadata": {"name": "projects/p/locations/l/jobs/j/executions/current-execution/extra"}, "status": self.execution["status"]},
            {"metadata": {"name": "current-execution"}, "name": "older-execution", "status": self.execution["status"]},
        ):
            self.assertEqual(validate(execution, logs, "retire", "g03r-1016")[1]["result"],
                             "execution_identity_missing_or_invalid")

    def test_other_execution_success_and_failure_do_not_contaminate_current(self):
        old_success = self.log("retire", {
            "format": "espacigo-gcp03-managed-synthetic-restore-evidence-v1", "commit_outcome": "committed",
        }, execution_name="projects/p/locations/l/jobs/j/executions/older-execution")
        old_failure = self.log("retire", {"error_code": "PHASE_FAILED", "application_exit_code": 1},
                               stream="stderr", execution_name="older-execution")
        self.assertEqual(validate(self.execution, [old_success], "retire", "g03r-1016")[1]["result"],
                         "application_success_evidence_missing_or_ambiguous")
        ok, result = validate(self.execution, [old_failure, self.success()], "retire", "g03r-1016")
        self.assertTrue(ok)
        self.assertEqual(result["result"], "success")

    def test_diagnosis_from_older_execution_cannot_satisfy_current_execution(self):
        old_diagnosis = self.log("retire-diagnose", {
            "format": "espacigo-gcp03-retirement-diagnosis-v1",
            "retirement_diagnosis": {"read_only_transaction": True},
        }, execution_name="older-execution")
        ok, result = validate(self.execution, [old_diagnosis], "retire-diagnose", "g03r-1016")
        self.assertFalse(ok)
        self.assertEqual(result["result"], "application_success_evidence_missing_or_ambiguous")

    def test_relevant_log_without_execution_identity_is_rejected(self):
        log = self.success()
        del log["labels"]["run.googleapis.com/execution_name"]
        self.assertEqual(validate(self.execution, [log], "retire", "g03r-1016")[1]["result"],
                         "relevant_log_execution_identity_missing_or_invalid")
        ok, result = validate(self.execution, [self.success(), log], "retire", "g03r-1016")
        self.assertFalse(ok)
        self.assertEqual(result["result"], "relevant_log_execution_identity_missing_or_invalid")

    def test_duplicate_success_or_failure_plus_success_is_not_accepted(self):
        one = self.success()
        two = self.success()
        self.assertEqual(validate(self.execution, [one, two], "retire", "g03r-1016")[1]["result"],
                         "application_success_evidence_missing_or_ambiguous")
        failure = self.log("retire", {"error_code": "PHASE_FAILED", "application_exit_code": 1}, stream="stderr")
        ok, result = validate(self.execution, [self.success(), failure], "retire", "g03r-1016")
        self.assertFalse(ok)
        self.assertEqual(result["result"], "application_phase_failed_despite_cloud_run_success")

    def test_unknown_success_format_prefix_is_rejected(self):
        log = self.log("retire", {"format": "espacigo-gcp03-new-format-v1", "commit_outcome": "committed"})
        self.assertEqual(validate(self.execution, [log], "retire", "g03r-1016")[1]["result"],
                         "application_success_evidence_missing_or_ambiguous")


if __name__ == "__main__":
    unittest.main()
