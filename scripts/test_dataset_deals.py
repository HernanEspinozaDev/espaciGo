import importlib.util
import json
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from types import SimpleNamespace


SCRIPT = Path(__file__).with_name("dataset-deals.py")
SPEC = importlib.util.spec_from_file_location("dataset_deals", SCRIPT)
deals = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(deals)


def synthetic_sources(root: Path):
    dataset = root / "dataset"
    assignments_path = dataset / "para_la_ia/asignaciones/escenarios_planificados.jsonl"
    listings_path = dataset / "para_la_ia/asignaciones/publicaciones_planificadas.jsonl"
    assignments_path.parent.mkdir(parents=True)
    assignments = []
    listings = []
    users = {}
    listing_refs = {}
    for index in range(1200):
        if index < len(deals.PILOT_ROWS):
            code, category, unit = deals.PILOT_ROWS[index]
        else:
            code, category, unit = "ciclo_cerrado", "oficina", "hora"
        ref = f"T{index:04d}"
        listing_ref = f"L{index:04d}"
        host_ref, renter_ref = f"H{index:04d}", f"R{index:04d}"
        users[host_ref] = f"host-id-{index}"
        users[renter_ref] = f"renter-id-{index}"
        listing_refs[listing_ref] = {"id": f"space-id-{index}"}
        assignments.append({
            "scenario_ref": ref, "scenario_code": code, "category_code": category,
            "rate_unit": unit, "host_ref": host_ref, "renter_ref": renter_ref,
            "listing_ref": listing_ref,
        })
        listings.append({
            "listing_ref": listing_ref, "intended_visibility": "publica",
            "rate_unit": unit, "time_zone": "America/Santiago",
        })
    assignments_path.write_text("".join(json.dumps(row) + "\n" for row in assignments))
    listings_path.write_text("".join(json.dumps(row) + "\n" for row in listings))
    source_path = root / ".local/testing/accounts.json"
    source_path.parent.mkdir(parents=True)
    source_path.write_text(json.dumps({
        "dataset_id": "espacigo-1000-v1", "users": users,
        "listings": listing_refs,
    }))
    source_path.chmod(0o600)
    return dataset, source_path, assignments


class FakeReservationAPI:
    def __init__(self):
        self.quotes = 0
        self.reservation_calls = []
        self.reservations = {}
        self.fail_after_commit = True

    def request(self, method, path, token="", body=None, key="", expect=(200,)):
        if path == deals.BASE + "/quotes":
            self.quotes += 1
            return {"data": {"id": "stable-quote-id"}}
        if path == deals.BASE + "/reservations":
            self.reservation_calls.append((key, dict(body)))
            prior = self.reservations.get(key)
            if prior is None:
                prior = {"id": "single-reservation-id", "start_at": "2030-01-02T10:00:00Z",
                         "end_at": "2030-01-02T11:00:00Z", "time_zone": "America/Santiago"}
                self.reservations[key] = prior
            elif body != {"quote_id": "stable-quote-id"}:
                raise AssertionError("idempotent retry changed its request payload")
            if self.fail_after_commit:
                self.fail_after_commit = False
                raise RuntimeError("simulated response loss after reservation commit")
            return {"data": prior}
        raise AssertionError(f"unexpected API request {method} {path}")


class DatasetDealsTests(unittest.TestCase):
    def test_import_selection_keeps_all_1200_rows_and_pilot_is_balanced(self):
        with tempfile.TemporaryDirectory() as temp:
            dataset, source, assignments = synthetic_sources(Path(temp))
            state, all_rows, listing_map = deals.read_sources(dataset, source)
            self.assertEqual(len(all_rows), 1200)
            self.assertEqual(len(listing_map), 1200)
            self.assertEqual(len(deals.select_scenarios("import", all_rows)), 1200)
            pilot = deals.select_scenarios("pilot", all_rows)
            self.assertEqual(len(pilot), 10)
            self.assertEqual({row["scenario_code"] for row in pilot}, {
                "ciclo_cerrado", "pendiente_pago", "cancelacion_con_devolucion",
                "rechazo_anfitrion", "reclamo_abierto",
            })
            self.assertEqual({row["category_code"] for row in pilot}, {
                "oficina", "bodega", "estacionamiento", "sala_multiproposito",
                "local_flexible", "quincho", "parcela_eventos", "stand",
            })
            self.assertEqual({row["rate_unit"] for row in pilot}, {"hora", "dia", "mes"})
            self.assertEqual(len(assignments), 1200)

    def test_reservation_retry_reuses_checkpointed_quote_and_idempotency_payload(self):
        with tempfile.TemporaryDirectory() as temp:
            repo = Path(temp)
            state_path = repo / ".local/testing/deals.json"
            state = {"scenarios": {"T0001": {}}}
            runner = deals.Runner.__new__(deals.Runner)
            runner.api = FakeReservationAPI()
            runner.args = SimpleNamespace(repo=str(repo), state_file=str(state_path))
            runner.state = state
            runner.sessions = {}
            runner.interval = lambda _row, _listing, _space_id, _renter: (
                datetime(2030, 1, 2, 10, tzinfo=timezone.utc),
                datetime(2030, 1, 2, 11, tzinfo=timezone.utc),
            )
            row = {"scenario_ref": "T0001"}
            record = state["scenarios"]["T0001"]
            with self.assertRaisesRegex(RuntimeError, "response loss"):
                runner.create_reservation_from_intent(row, record, {}, "space-id", "renter-token")
            self.assertTrue(state_path.is_file())
            self.assertEqual(state_path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(record["reservation_intent"]["quote_id"], "stable-quote-id")
            reservation = runner.create_reservation_from_intent(row, record, {}, "space-id", "renter-token")
            self.assertEqual(reservation["id"], "single-reservation-id")
            self.assertEqual(runner.api.quotes, 1)
            self.assertEqual(len(runner.api.reservations), 1)
            self.assertEqual(len(runner.api.reservation_calls), 2)
            self.assertEqual(runner.api.reservation_calls[0], runner.api.reservation_calls[1])


if __name__ == "__main__":
    unittest.main()
