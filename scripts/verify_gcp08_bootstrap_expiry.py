#!/usr/bin/env python3
"""Check a proposed temporary GCP-08 IAM expiration before Terraform planning."""
import argparse
from datetime import datetime, timedelta, timezone
import sys


def validate_expiration(value, now=None):
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except (AttributeError, TypeError, ValueError):
        return False
    if parsed.tzinfo is None:
        return False
    current = now or datetime.now(timezone.utc)
    expires = parsed.astimezone(timezone.utc)
    return current < expires <= current + timedelta(hours=24)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--expires-at", required=True, help="RFC3339 timestamp in the next 24 hours")
    args = parser.parse_args()
    if not validate_expiration(args.expires_at):
        print("Bootstrap expiration must be RFC3339, in the future, and no more than 24 hours away.", file=sys.stderr)
        return 1
    print("Bootstrap expiration accepted (at most 24 hours).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
