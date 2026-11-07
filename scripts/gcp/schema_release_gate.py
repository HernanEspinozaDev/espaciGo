"""Reuse the existing IMG-01 checks for Job-produced catalog evidence."""
import importlib.util
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
SPEC=importlib.util.spec_from_file_location('img01_job_schema_gate',ROOT/'scripts/verify_img01_schema_release_evidence.py')
MODULE=importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def validate_catalog(evidence,now=None):
    return MODULE.validate_payload(evidence,ROOT/'db/migrations',ROOT/'db/query/img01_schema_release_evidence.sql',now)
