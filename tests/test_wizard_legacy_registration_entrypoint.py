import importlib.util
import os
from pathlib import Path
import subprocess
import sys


LEGACY = Path(__file__).resolve().parents[1] / "frontend/scripts/wizard-improvements-register-account.py"
CANONICAL = Path(__file__).resolve().parents[1] / "scripts/wizard-improvements-register-account.py"


def test_legacy_cli_fails_closed_without_policy_or_network_access():
    env = os.environ.copy()
    env.update({"HTTP_PROXY": "http://127.0.0.1:1", "HTTPS_PROXY": "http://127.0.0.1:1", "ALL_PROXY": "http://127.0.0.1:1"})
    result = subprocess.run(
        [sys.executable, str(LEGACY), "--confirm-trial", "unexpected"],
        capture_output=True,
        text=True,
        env=env,
        timeout=5,
        check=False,
    )
    assert result.returncode == 2
    assert str(CANONICAL) in result.stderr
    assert "ERROR" not in result.stderr
    assert "private trial" not in result.stderr
    assert result.stdout == ""


def test_legacy_import_has_no_registration_exports_or_side_effects():
    spec = importlib.util.spec_from_file_location("legacy_registration", LEGACY)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    assert module.main
    assert not hasattr(module, "register_account")
    assert not hasattr(module, "request")
    assert not hasattr(module, "policy_allows")
