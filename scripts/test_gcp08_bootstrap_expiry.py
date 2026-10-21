from datetime import datetime, timedelta, timezone
from pathlib import Path
import unittest

from verify_gcp08_bootstrap_expiry import validate_expiration


ROOT = Path(__file__).resolve().parents[1]
NOW = datetime(2026, 10, 21, 12, 0, tzinfo=timezone.utc)


class Gcp08BootstrapExpiryTests(unittest.TestCase):
    def test_accepts_future_expiration_up_to_24_hours(self):
        expires = (NOW + timedelta(hours=24)).isoformat().replace("+00:00", "Z")
        self.assertTrue(validate_expiration(expires, NOW))

    def test_rejects_past_and_more_than_24_hour_expirations(self):
        past = (NOW - timedelta(seconds=1)).isoformat()
        too_far = (NOW + timedelta(hours=24, seconds=1)).isoformat()
        self.assertFalse(validate_expiration(past, NOW))
        self.assertFalse(validate_expiration(too_far, NOW))

    def test_requires_timezone_aware_rfc3339_timestamp(self):
        self.assertFalse(validate_expiration("2026-10-21T13:00:00", NOW))
        self.assertFalse(validate_expiration("not-a-timestamp", NOW))

    def test_terraform_apply_rechecks_the_expiration_before_creating_grants(self):
        source = (ROOT / "infra/terraform/environments/testprod/gcp08-bootstrap/main.tf").read_text(encoding="utf-8")
        self.assertIn("precondition {", source)
        self.assertIn("timecmp(var.bootstrap_expires_at, timestamp()) > 0", source)
        self.assertIn('timeadd(timestamp(), "24h")', source)
        self.assertIn("terraform_data.bootstrap_expiry_guard", source)


if __name__ == "__main__":
    unittest.main()
