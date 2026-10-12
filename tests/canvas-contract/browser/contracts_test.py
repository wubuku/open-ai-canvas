import copy
import json
import unittest
from pathlib import Path

from contracts import canonical_path, fixture_http, network_policy, redact_error, validate_report


class BrowserContractTests(unittest.TestCase):
    def setUp(self):
        self.catalog = json.loads(Path(__file__).with_name("scenarios.json").read_text())

    def valid_report(self):
        facts = []
        scenarios = []
        for item in self.catalog["scenarios"]:
            for route in item["required_ui_routes"]:
                method, path = route.split(" ", 1)
                facts.append({"scenario": item["id"], "method": method, "path": path, "status": 200, "source": "real-page-network-response-headers"})
            scenarios.append({"id": item["id"], "passed": True, "ui_routes": sorted(item["required_ui_routes"])})
        for identity, method, path, status in ((self.catalog["scenarios"][0]["id"], "POST", "/api/auth/login", 401), ("browser.share.anonymous-read-and-revocation", "GET", "/api/public/canvas-shares/{token}", 404), ("browser.auth.same-context-switch-and-peer-reload", "GET", "/api/canvas-projects/{id}", 404)):
            facts.append({"scenario": identity, "method": method, "path": path, "status": status, "source": "real-page-network-response-headers"})
        return {"version": self.catalog["version"], "approval": "pending", "target": "java", "scenarios": scenarios, "http": facts, "blocked_decorations": [], "remaining_paths": self.catalog["remaining_paths"], "passed": True}

    def test_canonical_paths_preserve_literals_and_remove_secrets(self):
        for path in ("/api/resources/storage-usage", "/api/resources/import", "/api/resources/uploads", "/api/resources/access", "/api/assets/batch", "/api/assets/batch-delete"):
            self.assertEqual(canonical_path(path), path)
        for address, expected in (("https://target/api/public/canvas-shares/secret/resources/fixture/file?signature=private", "/api/public/canvas-shares/{token}/resources/{id}/file"), ("/api/public/resources/resource-id/file?token=secret", "/api/public/resources/{id}/file"), ("/api/resources/resource-id/file?token=secret", "/api/resources/{id}/file"), ("/api/canvas-projects/canvas-id/share", "/api/canvas-projects/{id}/share"), ("/api/assets/asset-id", "/api/assets/{id}")):
            self.assertEqual(canonical_path(address), expected)

    def test_origin_guard_never_allows_decoration_egress(self):
        decoration = self.catalog["blocked_decorations"][0]
        self.assertEqual(network_policy(decoration["url"], "media", "http://127.0.0.1:18184", self.catalog["blocked_decorations"]), ("blocked-decoration", decoration["id"]))
        for address, resource_type in ((decoration["url"], "fetch"), (decoration["url"] + "?secret=1", "media"), ("https://127.0.0.1:18184/api/auth/session", "fetch"), ("http://user:pass@127.0.0.1:18184/api/auth/session", "fetch"), ("https://foreign.invalid/api/resources", "fetch")):
            self.assertEqual(network_policy(address, resource_type, "http://127.0.0.1:18184", self.catalog["blocked_decorations"])[0], "blocked-external")
        self.assertEqual(network_policy("http://127.0.0.1:18184/api/auth/session", "fetch", "http://127.0.0.1:18184", [])[0], "target")

    def test_error_redaction(self):
        output = redact_error("passLong pass /share/canvas/share-secret /api/public/canvas-shares/api-secret https://target/api?a=key-secret", ("pass", "passLong", ""))
        for secret in ("pass", "share-secret", "api-secret", "key-secret"):
            self.assertNotIn(secret, output)
        self.assertEqual(len(redact_error("x" * 2000, [])), 1500)

    def test_fixture_never_follows_redirects(self):
        class RequestRecorder:
            def fetch(self, path, **options):
                return path, options

        path, options = fixture_http(RequestRecorder(), "POST", "/api/auth/login", data={"fixture": True})
        self.assertEqual(path, "/api/auth/login")
        self.assertEqual(options, {"method": "POST", "max_redirects": 0, "data": {"fixture": True}})
        for invalid in ("https://foreign.invalid/api/auth/login", "//foreign.invalid/api/auth/login", "/login", "/api/auth/login#fragment"):
            with self.assertRaises(AssertionError):
                fixture_http(RequestRecorder(), "POST", invalid)

    def test_full_report_passes_without_go_dependency(self):
        validate_report(self.catalog, self.valid_report())

    def test_missing_duplicate_failed_unknown_scenarios_are_rejected(self):
        for mutation in (lambda report: report["scenarios"].pop(), lambda report: report["scenarios"].append(report["scenarios"][0]), lambda report: report["scenarios"][0].update(passed=False), lambda report: report.update(target="unassigned"), lambda report: report.update(remaining_paths=[]), lambda report: report.update(passed=False)):
            report = self.valid_report()
            mutation(report)
            with self.assertRaises(AssertionError):
                validate_report(self.catalog, report)

    def test_negative_only_routes_and_fake_network_evidence_are_rejected(self):
        for field, value in (("status", 500), ("source", "fixture-request"), ("path", "/api/auth/login?password=secret"), ("scenario", "undeclared")):
            report = self.valid_report()
            report["http"][0][field] = value
            with self.assertRaises(AssertionError):
                validate_report(self.catalog, report)

    def test_missing_rejection_witnesses_are_rejected(self):
        report = self.valid_report()
        for status in (401, 404):
            altered = copy.deepcopy(report)
            altered["http"] = [item for item in altered["http"] if item["status"] != status]
            with self.assertRaises(AssertionError):
                validate_report(self.catalog, altered)

    def test_share_and_peer_denials_are_not_interchangeable(self):
        for scenario in ("browser.share.anonymous-read-and-revocation", "browser.auth.same-context-switch-and-peer-reload"):
            report = self.valid_report()
            report["http"] = [item for item in report["http"] if not (item["scenario"] == scenario and item["status"] == 404)]
            with self.assertRaises(AssertionError):
                validate_report(self.catalog, report)


if __name__ == "__main__":
    unittest.main()
