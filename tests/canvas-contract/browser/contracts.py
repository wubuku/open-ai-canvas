import re
from urllib.parse import urlsplit


def canonical_path(address):
    path = urlsplit(address).path
    path = re.sub(r"(/api/public/canvas-shares/)[^/]+", r"\1{token}", path)
    if path not in ("/api/resources/storage-usage", "/api/resources/import", "/api/resources/uploads", "/api/resources/access"):
        path = re.sub(r"(/api/(?:canvas-projects|resources)/)[^/]+", r"\1{id}", path)
    path = re.sub(r"(/api/public/resources/)[^/]+", r"\1{id}", path)
    path = re.sub(r"(/api/public/canvas-shares/\{token\}/resources/)[^/]+", r"\1{id}", path)
    path = re.sub(r"(/api/assets/)(?!batch(?:-delete)?$)[^/]+", r"\1{id}", path)
    path = re.sub(r"(/api/(?:tasks|agent/runs)/)[^/]+", r"\1{id}", path)
    return path


def network_policy(address, resource_type, web_origin, decorations):
    destination = urlsplit(address)
    origin = urlsplit(web_origin)
    if destination.scheme in ("data", "blob"):
        return "local", None
    if destination.scheme == origin.scheme and destination.netloc == origin.netloc and not destination.username:
        return "target", None
    for decoration in decorations:
        if address == decoration["url"] and resource_type in decoration["resource_types"]:
            return "blocked-decoration", decoration["id"]
    return "blocked-external", destination.hostname or "invalid-origin"


def redact_error(detail, credentials):
    for secret in sorted((value for value in credentials if value), key=len, reverse=True):
        detail = detail.replace(secret, "<redacted>")
    detail = re.sub(r"(/(?:api/public/canvas-shares|share/canvas)/)[^\s/?\"']+", r"\1{token}", detail)
    detail = re.sub(r"(https?://[^\s?\"']+)\?[^\s\"']+", r"\1?<redacted>", detail)
    return detail[:1500]


def fixture_http(request, method, path, **options):
    destination = urlsplit(path)
    if not path.startswith("/api/") or destination.scheme or destination.netloc or destination.fragment:
        raise AssertionError("fixture must use a target-relative API path")
    return request.fetch(path, method=method, max_redirects=0, **options)


def validate_report(catalog, report):
    expected_ids = [item["id"] for item in catalog["scenarios"]]
    observed_ids = [item["id"] for item in report["scenarios"]]
    if len(set(expected_ids)) != len(expected_ids) or observed_ids != expected_ids:
        raise AssertionError("browser scenario execution is missing, duplicated or reordered")
    if report["version"] != catalog["version"] or report["approval"] != "pending" or report["target"] not in ("go", "java", "mixed"):
        raise AssertionError("invalid browser contract version, approval or execution target")
    if report["remaining_paths"] != catalog["remaining_paths"] or not report["passed"] or "failure" in report:
        raise AssertionError("browser report failed or hid remaining paths")
    decoration_ids = {item["id"] for item in catalog["blocked_decorations"]}
    if any(item["id"] not in decoration_ids or item["scenario"] not in expected_ids for item in report["blocked_decorations"]):
        raise AssertionError("unknown blocked browser decoration")
    for fact in report["http"]:
        if fact["scenario"] not in expected_ids or fact["source"] != "real-page-network-response-headers" or not isinstance(fact["status"], int) or not 100 <= fact["status"] <= 599:
            raise AssertionError("invalid real-page HTTP evidence")
        if not fact["path"].startswith("/api/") or fact["path"] != canonical_path(fact["path"]):
            raise AssertionError("browser evidence contains raw dynamic identity or query")
    for declaration, executed in zip(catalog["scenarios"], report["scenarios"]):
        facts = [item for item in report["http"] if item["scenario"] == declaration["id"]]
        successful = {item["method"] + " " + item["path"] for item in facts if 200 <= item["status"] < 300}
        observed = {item["method"] + " " + item["path"] for item in facts}
        if not executed["passed"] or not set(declaration["required_ui_routes"]).issubset(successful) or executed["ui_routes"] != sorted(observed):
            raise AssertionError("browser scenario lacks successful UI route evidence")
    login = [item for item in report["http"] if item["scenario"] == expected_ids[0] and item["method"] == "POST" and item["path"] == "/api/auth/login"]
    if 401 not in {item["status"] for item in login}:
        raise AssertionError("browser login lacks wrong-password rejection")
    revoked = [item for item in report["http"] if item["scenario"] == "browser.share.anonymous-read-and-revocation" and item["path"] == "/api/public/canvas-shares/{token}"]
    if 404 not in {item["status"] for item in revoked}:
        raise AssertionError("browser share lacks revoked anonymous read")
    peer = [item for item in report["http"] if item["scenario"] == "browser.auth.same-context-switch-and-peer-reload" and item["method"] == "GET" and item["path"] == "/api/canvas-projects/{id}"]
    if 404 not in {item["status"] for item in peer}:
        raise AssertionError("browser switched account lacks denied peer read")
