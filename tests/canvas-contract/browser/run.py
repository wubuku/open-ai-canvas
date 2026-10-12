import argparse
import hashlib
import json
import os
import re
import secrets
import struct
import sys
import time
import zlib
from contextlib import contextmanager
from datetime import datetime, timezone
from importlib.metadata import version
from pathlib import Path
from urllib.parse import urlsplit

from playwright.sync_api import expect, sync_playwright
from contracts import canonical_path, fixture_http, network_policy, redact_error, validate_report


@contextmanager
def isolated_browser():
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True, executable_path=os.environ.get("CHROME_BIN") or None)
        try:
            yield playwright, browser
        finally:
            for current in browser.contexts:
                current.unroute_all(behavior="wait")
                current.close()
            browser.close()


def image_fixture():
    def chunk(kind, content):
        return struct.pack(">I", len(content)) + kind + content + struct.pack(">I", zlib.crc32(kind + content))

    pixels = b"\x00" + b"\x40\x80\xc0\xff" * 3
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 3, 2, 8, 6, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(pixels * 2)) + chunk(b"IEND", b"")


def successful_data(response):
    assert response.status == 200, f"API status {response.status} at {canonical_path(response.url)}"
    value = response.json()
    assert value.get("code") == 0 and "data" in value and isinstance(value.get("msg"), str), "invalid success envelope"
    return value["data"]


@contextmanager
def authenticated_api(playwright, origin, username, password):
    request = playwright.request.new_context(base_url=origin)
    try:
        successful_data(fixture_http(request, "POST", "/api/auth/login", data={"username": username, "password": password}))
        session = successful_data(fixture_http(request, "GET", "/api/auth/session"))
        assert session["user"]["username"] == username and session["user"]["id"], "audit API session has the wrong identity"
        yield request
    finally:
        request.dispose()


def wait_for_data(request, path, predicate, timeout=30):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        data = successful_data(fixture_http(request, "GET", path))
        if predicate(data):
            return data
        time.sleep(0.25)
    raise AssertionError(f"persisted postcondition timed out at {canonical_path(path)}")


def main():
    parser = argparse.ArgumentParser(description="Real frontend/browser contract against an already-running Go, Java or mixed HTTP target")
    parser.add_argument("--web-origin", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--report", required=True)
    parser.add_argument("--implementation", choices=("go", "java", "mixed"), required=True)
    parser.add_argument("--isolated-fixture", action="store_true")
    args = parser.parse_args()
    origin = urlsplit(args.web_origin)
    assert origin.scheme in ("http", "https") and origin.netloc and origin.path in ("", "/") and not origin.username and not origin.query and not origin.fragment, "web origin must not contain credentials or an API path"
    assert args.isolated_fixture, "browser writes require an explicitly isolated target"
    admin_name = os.environ.get("CANVAS_CONTRACT_BROWSER_ADMIN_USERNAME", "")
    admin_password = os.environ.get("CANVAS_CONTRACT_BROWSER_ADMIN_PASSWORD", "")
    assert admin_name and admin_password, "isolated HTTP test-admin credentials are required"
    root = Path(__file__).resolve().parent
    catalog = json.loads((root / "scenarios.json").read_text())
    report = {"version": catalog["version"], "approval": "pending", "target": args.implementation, "run_id": args.run_id, "scenarios": [], "http": [], "blocked_decorations": [], "remaining_paths": catalog["remaining_paths"], "passed": False}
    stage = "fixture-setup"
    blocked_external = []
    browser_errors = []
    password = "CanvasBrowser-" + secrets.token_hex(12) + "!"
    digest = hashlib.sha256(args.run_id.encode()).hexdigest()[:12]
    owner_name, foreign_name = "browser_a_" + digest, "browser_b_" + digest
    try:
        with isolated_browser() as (playwright, browser):
            report["runtime"] = {"python": sys.version.split()[0], "playwright": version("playwright"), "chromium": browser.version}
            contexts = []

            def context():
                current = browser.new_context(base_url=args.web_origin, viewport={"width": 1440, "height": 1000}, permissions=["clipboard-read", "clipboard-write"])
                contexts.append(current)

                def guard(route):
                    action, identity = network_policy(route.request.url, route.request.resource_type, args.web_origin, catalog["blocked_decorations"])
                    if action in ("local", "target"):
                        route.continue_()
                    else:
                        if action == "blocked-decoration":
                            report["blocked_decorations"].append({"scenario": stage, "id": identity})
                        else:
                            blocked_external.append(identity)
                        route.abort()

                current.route("**/*", guard)
                return current

            def page_for(current):
                page = current.new_page()
                page.set_default_timeout(30000)

                def record(response):
                    if urlsplit(response.url).path.startswith("/api/"):
                        report["http"].append({"scenario": stage, "method": response.request.method, "path": canonical_path(response.url), "status": response.status, "source": "real-page-network-response-headers"})

                page.on("response", record)
                page.on("pageerror", lambda error: browser_errors.append(type(error).__name__))
                return page

            with authenticated_api(playwright, args.web_origin, admin_name, admin_password) as admin:
                for name in (owner_name, foreign_name):
                    successful_data(fixture_http(admin, "POST", "/api/admin/users", data={"username": name, "password": password, "displayName": "Browser contract fixture", "role": "user", "status": "active"}))
            owner, foreign, anonymous = context(), context(), context()
            owner_page, foreign_page, public_page = page_for(owner), page_for(foreign), page_for(anonymous)

            def visit(page, path):
                page.goto(args.web_origin + path)
                page.wait_for_load_state("networkidle")

            def login(page, name, supplied_password):
                visit(page, "/login?next=%2Fcanvas")
                page.get_by_role("textbox", name="用户名 / 邮箱", exact=True).fill(name)
                page.get_by_placeholder("请输入密码", exact=True).fill(supplied_password)
                page.get_by_role("button", name="登录", exact=True).click()

            def finish(identity):
                for current in contexts:
                    for page in current.pages:
                        if page.url != "about:blank":
                            page.wait_for_load_state("networkidle")
                required = next(item["required_ui_routes"] for item in catalog["scenarios"] if item["id"] == identity)
                observed = {item["method"] + " " + item["path"] for item in report["http"] if item["scenario"] == identity}
                successful = {item["method"] + " " + item["path"] for item in report["http"] if item["scenario"] == identity and 200 <= item["status"] < 300}
                assert set(required).issubset(successful), "scenario lacks successful required real-page HTTP evidence"
                assert not browser_errors, "real page raised an uncaught exception"
                assert not blocked_external, "page attempted undeclared external origins: " + ", ".join(sorted(set(blocked_external)))
                report["scenarios"].append({"id": identity, "passed": True, "ui_routes": sorted(observed)})
                print("PASS " + identity, flush=True)

            stage = "browser.auth.login-failure-and-owner-session"
            with owner_page.expect_response(lambda response: urlsplit(response.url).path == "/api/auth/login" and response.request.method == "POST") as denied_login:
                login(owner_page, owner_name, password + "wrong")
            assert denied_login.value.status == 401, "wrong password was not denied"
            expect(owner_page.get_by_role("button", name="登录", exact=True)).to_be_visible()
            assert any(item["method"] == "POST" and item["path"] == "/api/auth/login" and item["status"] == 401 for item in report["http"] if item["scenario"] == stage), "wrong password was not denied"
            owner_page.get_by_placeholder("请输入密码", exact=True).fill(password)
            owner_page.get_by_role("button", name="登录", exact=True).click()
            owner_page.wait_for_url("**/canvas")
            expect(owner_page.get_by_role("heading", name="我的画布", exact=True)).to_be_visible()
            finish(stage)

            stage = "browser.canvas.create-rename-reopen"
            owner_page.get_by_role("button", name="新建画布", exact=True).first.click()
            owner_page.wait_for_url(re.compile(r".*/canvas/[^/?]+$"))
            canvas_id = urlsplit(owner_page.url).path.rsplit("/", 1)[1]
            title = "Browser contract " + digest
            owner_page.get_by_role("button", name="重命名画布", exact=True).click()
            owner_page.get_by_role("textbox", name="画布名称", exact=True).fill(title)
            owner_page.get_by_role("textbox", name="画布名称", exact=True).press("Enter")
            stored = wait_for_data(owner.request, "/api/canvas-projects/" + canvas_id, lambda data: data["project"]["title"] == title)["project"]
            fresh_page = page_for(owner)
            visit(fresh_page, "/canvas/" + canvas_id)
            expect(fresh_page.get_by_role("button", name=title, exact=True)).to_be_visible()
            owner_page.close()
            owner_page = fresh_page
            finish(stage)

            stage = "browser.canvas.text-edit-media-and-topology"
            successful_data(fixture_http(owner.request, "POST", "/api/resources", multipart={"kind": "image", "file": {"name": "browser-image.png", "mimeType": "image/png", "buffer": image_fixture()}}))
            resources = successful_data(fixture_http(owner.request, "GET", "/api/resources"))["resources"]
            assert len(resources) == 1, "browser fixture resource identity is ambiguous"
            resource_id = resources[0]["id"]
            timestamp = datetime.now(timezone.utc).isoformat()
            successful_data(fixture_http(owner.request, "PUT", "/api/assets/" + resource_id, data={"asset": {"id": resource_id, "kind": "image", "title": "Browser image", "coverUrl": "/api/resources/" + resource_id + "/file", "tags": [], "createdAt": timestamp, "updatedAt": timestamp, "data": {"dataUrl": "", "storageKey": "resource:" + resource_id, "width": 3, "height": 2, "bytes": resources[0]["size"], "mimeType": "image/png"}}}))
            text_id, image_id = "browser-text", "browser-image"
            stored["nodes"] = [
                {"id": text_id, "type": "text", "title": "Browser text node", "position": {"x": 160, "y": 120}, "width": 420, "height": 260, "metadata": {"content": "Initial browser text"}},
                {"id": image_id, "type": "image", "title": "Browser image node", "position": {"x": 660, "y": 120}, "width": 240, "height": 160, "metadata": {"content": "/api/resources/" + resource_id + "/file", "storageKey": "resource:" + resource_id, "assetId": resource_id, "naturalWidth": 3, "naturalHeight": 2, "mimeType": "image/png"}},
            ]
            stored["connections"] = [{"id": "browser-edge", "fromNodeId": text_id, "toNodeId": image_id}]
            successful_data(fixture_http(owner.request, "PUT", "/api/canvas-projects/" + canvas_id, data={"project": stored}))
            seeded = successful_data(fixture_http(owner.request, "GET", "/api/canvas-projects/" + canvas_id))["project"]
            assert [node["id"] for node in seeded["nodes"]] == [text_id, image_id], "HTTP seed did not persist fixture nodes"
            owner_page.close()
            owner = context()
            owner_page = page_for(owner)
            login(owner_page, owner_name, password)
            owner_page.wait_for_url("**/canvas")
            visit(owner_page, "/canvas/" + canvas_id)
            text_node = owner_page.locator('[data-node-id="browser-text"]')
            expect(text_node).to_be_visible()
            text_editor_button = text_node.get_by_role("button", name="放大编辑文本", exact=True)
            text_editor_button.focus()
            text_editor_button.press("Enter")
            editor = owner_page.locator('[contenteditable="true"][aria-label="文本节点富文本编辑区"]')
            editor.fill("Persisted browser edited text " + digest)
            owner_page.get_by_role("textbox", name="文本节点标题", exact=True).fill("Edited browser text node")
            owner_page.get_by_role("button", name="保存", exact=True).click()
            owner_page.get_by_role("button", name="关闭文本编辑器", exact=True).click()
            owner_page.get_by_role("button", name="打开画布菜单", exact=True).click()
            owner_page.get_by_role("menuitem", name=re.compile(r"^保存")).click()
            edited = wait_for_data(owner.request, "/api/canvas-projects/" + canvas_id, lambda data: data["project"]["nodes"][0]["title"] == "Edited browser text node")["project"]
            assert "Persisted browser edited text " + digest in edited["nodes"][0]["metadata"]["content"], "text editor did not persist its actual body"
            assert edited["connections"] == stored["connections"], "text edit changed unrelated topology"
            assert edited["nodes"][1]["metadata"]["storageKey"] == "resource:" + resource_id, "text edit changed unrelated resource identity"
            visit(owner_page, "/canvas/" + canvas_id)
            expect(owner_page.locator('[data-node-id="browser-text"]')).to_contain_text("Persisted browser edited text " + digest)
            media = owner_page.locator('[data-node-id="browser-image"] img').first
            expect(media).to_be_visible()
            assert media.evaluate("image => image.complete && image.naturalWidth === 3 && image.naturalHeight === 2"), "rendered resource image is incomplete or has wrong natural dimensions"
            finish(stage)

            stage = "browser.share.anonymous-read-and-revocation"
            owner_page.get_by_role("button", name="分享画布", exact=True).click()
            owner_page.get_by_role("button", name="创建并复制链接", exact=True).click()
            share_input = owner_page.locator('input[readonly]').first
            expect(share_input).to_have_value(re.compile(r".*/share/canvas/.+"))
            share_path = urlsplit(share_input.input_value()).path
            visit(public_page, share_path)
            expect(public_page.get_by_text(title, exact=True)).to_be_visible()
            expect(public_page.locator('[data-node-id="browser-text"]')).to_contain_text("Persisted browser edited text " + digest)
            assert successful_data(fixture_http(owner.request, "GET", "/api/canvas-projects/" + canvas_id))["project"] == edited, "anonymous share viewing changed the owner's persisted document"
            owner_page.get_by_role("button", name="停止分享", exact=True).click()
            owner_page.get_by_role("dialog").filter(has_text="停止公开分享？").get_by_role("button", name="停止分享", exact=True).click()
            expect(owner_page.get_by_role("button", name="创建并复制链接", exact=True)).to_be_visible()
            visit(public_page, share_path)
            expect(public_page.locator('[data-node-id="browser-text"]')).to_have_count(0)
            assert any(item["method"] == "GET" and item["path"] == "/api/public/canvas-shares/{token}" and item["status"] == 404 for item in report["http"] if item["scenario"] == stage), "revoked share did not deny anonymous HTTP read"
            finish(stage)

            stage = "browser.auth.foreign-library-isolation"
            login(foreign_page, foreign_name, password)
            foreign_page.wait_for_url("**/canvas")
            expect(foreign_page.get_by_role("heading", name="我的画布", exact=True)).to_be_visible()
            expect(foreign_page.get_by_text(title, exact=True)).to_have_count(0)
            foreign_listing = successful_data(fixture_http(foreign.request, "GET", "/api/canvas-projects"))["projects"] or []
            assert all(item["id"] != canvas_id for item in foreign_listing), "foreign library exposed owner canvas identity"
            denied = fixture_http(foreign.request, "GET", "/api/canvas-projects/" + canvas_id)
            assert denied.status == 404 and denied.json().get("code") != 0, "foreign direct HTTP read did not deny access"
            finish(stage)

            stage = "browser.auth.same-context-switch-and-peer-reload"
            peer_page = page_for(owner)
            visit(peer_page, "/canvas/" + canvas_id)
            expect(peer_page.locator('[data-node-id="browser-text"]')).to_contain_text("Persisted browser edited text " + digest)
            expect(peer_page.locator('[data-node-id="browser-image"] img').first).to_be_visible()
            visit(owner_page, "/canvas")
            expect(owner_page.get_by_text(title, exact=True)).to_be_visible()
            owner_page.get_by_role("button", name="打开账户菜单", exact=True).click()
            with owner_page.expect_response(lambda response: urlsplit(response.url).path == "/api/auth/logout" and response.request.method == "POST") as logout_response:
                owner_page.get_by_role("button", name="退出登录", exact=True).click()
            assert logout_response.value.status == 200, "same-context UI logout failed"
            owner_page.wait_for_url(re.compile(r".*/login(?:\?.*)?$"))
            login(owner_page, foreign_name, password)
            owner_page.wait_for_url("**/canvas")
            expect(owner_page.get_by_role("heading", name="我的画布", exact=True)).to_be_visible()
            owner_page.wait_for_load_state("networkidle")
            expect(owner_page.get_by_text(title, exact=True)).to_have_count(0)
            switched = successful_data(fixture_http(owner.request, "GET", "/api/auth/session"))
            assert switched["user"]["username"] == foreign_name, "same-context session did not adopt the foreign identity"
            denied = fixture_http(owner.request, "GET", "/api/canvas-projects/" + canvas_id)
            assert denied.status == 404 and denied.json().get("code") != 0, "switched account retained owner HTTP authority"
            with peer_page.expect_response(lambda response: urlsplit(response.url).path == "/api/canvas-projects/" + canvas_id and response.request.method == "GET") as peer_read:
                visit(peer_page, "/canvas/" + canvas_id)
            assert peer_read.value.status == 404, "peer reload did not adopt the switched account"
            expect(peer_page.locator('[data-node-id="browser-text"]')).to_have_count(0)
            expect(peer_page.locator('[data-node-id="browser-image"]')).to_have_count(0)
            expect(peer_page.get_by_text(title, exact=True)).to_have_count(0)
            expect(peer_page.get_by_text("Persisted browser edited text " + digest, exact=True)).to_have_count(0)
            with authenticated_api(playwright, args.web_origin, owner_name, password) as owner_audit:
                owner_unchanged = successful_data(fixture_http(owner_audit, "GET", "/api/canvas-projects/" + canvas_id))["project"]
                assert owner_unchanged == edited, "account switch or foreign peer reload changed owner's persisted document"
            finish(stage)

            assert len(report["scenarios"]) == len(catalog["scenarios"]), "browser scenario catalog was not fully executed"
            report["passed"] = True
            validate_report(catalog, report)
    except Exception as error:
        report["passed"] = False
        report["failure"] = {"scenario": stage, "type": type(error).__name__}
        detail = redact_error(str(error), (admin_name, admin_password, owner_name, foreign_name, password))
        report["failure"]["detail"] = detail[:1500]
        print("FAIL " + stage + " (" + type(error).__name__ + "): " + detail[:800], file=sys.stderr, flush=True)
    finally:
        destination = Path(args.report)
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    sys.exit(main())
