import { afterAll, expect, test } from "bun:test";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const suiteRoot = dirname(fileURLToPath(import.meta.url));
const temporaryDirectory = mkdtempSync(resolve(tmpdir(), "canvas-catalog-audit-"));
const manifest = Bun.YAML.parse(readFileSync(resolve(suiteRoot, "contract/manifest.yaml"), "utf8"));
const wireRegistry = JSON.parse(readFileSync(resolve(suiteRoot, "contract/schemas/bindings.v1.json"), "utf8"));
const normalizePath = (path: string) => path.replace(/\{[^}]+\}/g, "{}");
function wireEvidence(method: string, path: string) {
  const binding = wireRegistry.bindings.find(binding => binding.method === method && normalizePath(binding.path) === normalizePath(path));
  return { version: wireRegistry.version, schema: binding?.schema ?? "", mode: binding?.binary ? "binary" : "json", valid: true };
}
afterAll(() => rmSync(temporaryDirectory, { recursive: true, force: true }));

function checkManifest(name: string, mutate: (copy: typeof manifest) => void) {
  const copy = structuredClone(manifest);
  mutate(copy);
  const filename = resolve(temporaryDirectory, `${name}.yaml`);
  writeFileSync(filename, Bun.YAML.stringify(copy));
  return Bun.spawnSync([process.execPath, resolve(suiteRoot, "check-catalog.ts"), `--manifest=${filename}`]);
}

test("catalog audit accepts the actual registration and scenario index", () => {
  const result = checkManifest("valid", () => {});
  expect(result.exitCode).toBe(0);
  const report = JSON.parse(result.stdout.toString());
  expect(report.direct_canvas_routes).toBe(14);
  expect(report.scenario_executed).toBeNull();
  expect(report.contract_approval).toBe("pending");
  expect(report.wire_schema_bound_routes).toBe(70);
  expect(report.wire_schema_version).toBe("v1-observed");
  expect(report.wire_validated_requests).toBeNull();
});

test("catalog audit rejects unversioned or falsely approved wire metadata", () => {
  const result = checkManifest("wire-metadata", copy => { copy.suite.wire_schema.approval = "approved"; });
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("manifest and wire registry metadata disagree");
});

test("new Required routes must gain an explicit wire binding", () => {
  const result = checkManifest("missing-wire-binding", copy => {
    const operation = copy.operations.find(operation => copy.profiles[operation.profile].status === "DEFERRED");
    copy.profiles[operation.profile].operations = copy.profiles[operation.profile].operations.filter(identity => identity !== operation.id);
    operation.profile = "core";
    operation.scenario = copy.operations[0].scenario;
    copy.profiles.core.operations.push(operation.id);
  });
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("Required route lacks wire binding");
});

test("catalog audit rejects duplicate operations", () => {
  const result = checkManifest("duplicate", copy => copy.operations.push(copy.operations[0]));
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("duplicate operation");
});

test("Required profiles may explicitly close remaining paths without approving the contract", () => {
  const result = checkManifest("explicitly-closed-paths", copy => { copy.profiles.core.remaining_paths = []; });
  expect(result.exitCode).toBe(0);
  const report = JSON.parse(result.stdout.toString());
  expect(report.remaining_paths.core).toEqual([]);
  expect(report.contract_approval).toBe("pending");
  expect(report.operation_executed).toBeNull();
});

test("Required path declarations cannot be missing, malformed, blank or duplicated", () => {
  for (const value of [undefined, "still-missing", [""], ["  "], [17], ["same-path", "same-path"]]) {
    const result = checkManifest(`invalid-path-declaration-${JSON.stringify(value)}`, copy => { copy.profiles.core.remaining_paths = value as never; });
    expect(result.exitCode).toBe(1);
    expect(result.stderr.toString()).toContain("missing/invalid explicit remaining paths");
  }
});

test("catalog audit rejects invented endpoints", () => {
  const result = checkManifest("unregistered", copy => { copy.operations[0].path = "/api/canvas-projects/invented-route"; });
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("unregistered method/path");
});

test("catalog audit rejects missing profile assignments and evidence", () => {
  const result = checkManifest("missing-mapping", copy => {
    copy.profiles.core.operations.shift();
    copy.operations[0].evidence = ["backend/nonexistent-evidence.go"];
  });
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("operation absent from its profile");
  expect(result.stderr.toString()).toContain("missing evidence");
});

test("catalog audit rejects unimplemented scenarios", () => {
  const result = checkManifest("scenario", copy => { copy.operations[0].scenario = "canvas.nonexistent-scenario"; });
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("unimplemented scenario");
});

test("decision ledger cannot reference missing operations or evidence", () => {
  const result = checkManifest("decision-evidence", copy => {
    copy.behavior_decisions[0].operations = ["canvas.nonexistent-operation"];
    copy.behavior_decisions[0].evidence = ["tests/canvas-contract/nonexistent-test.go"];
  });
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("decision references unknown operation");
  expect(result.stderr.toString()).toContain("missing decision evidence");
});

test("pending decisions cannot silently approve Go behavior", () => {
  const result = checkManifest("decision-unapproved", copy => {
    copy.behavior_decisions[0].classification = "KNOWN_BUG";
  });
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("pending decision cannot silently approve Go behavior");
});

test("approved decisions require an approver and resolved classification", () => {
  const result = checkManifest("decision-invalid-approval", copy => {
    copy.behavior_decisions[0].approval = "approved";
  });
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("approved decision lacks an approver or resolved classification");
});

test("a passing suite without the mapped scenarios is not runtime coverage", () => {
  const filename = resolve(temporaryDirectory, "incomplete-results.jsonl");
  writeFileSync(filename, [
    { Action: "output", Test: "TestCanvasContract", Output: "implementation=go database=postgres-18 profile=core run_id=fixture" },
    { Action: "pass", Test: "TestCanvasContract" },
  ].map(event => JSON.stringify(event)).join("\n"));
  const result = Bun.spawnSync([process.execPath, resolve(suiteRoot, "check-catalog.ts"), `--results=${filename}`]);
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("result missing scenario");
});

type ResultEvent = { Action: string; Test?: string; Output?: string; Package?: string };

function fileVariantEvent(status: number, mode: string, schema: string, requestVariant?: string): ResultEvent {
  return {
    Action: "output", Test: "TestCanvasContract/canvas.public-resource.access-description",
    Output: `CANVAS_CONTRACT_HTTP ${JSON.stringify({ method: "GET", path: "/api/public/resources/fixture/file", status, body_complete: true, transport_failure: false, request_variant: requestVariant, wire: { version: wireRegistry.version, schema, mode, valid: true } })}\n`,
  };
}

test("explicit request variants permit only their bound description and error schema", () => {
  const events = resultFixture("public-resource");
  events.push(fileVariantEvent(200, "access-description", "resource.v1.schema.json#/$defs/access-single", "access-description"));
  events.push(fileVariantEvent(403, "json", "common.v1.schema.json#/$defs/error-envelope", "access-description"));
  expect(checkResults("file-description-valid", events).exitCode).toBe(0);
});

test("description evidence cannot be hidden as binary or invented from Content-Type", () => {
  for (const [mode, schema, variant] of [
    ["binary", "", "access-description"],
    ["json", "common.v1.schema.json", "access-description"],
    ["access-description", "resource.v1.schema.json#/$defs/access-single", undefined],
    ["access-description", "resource.v1.schema.json#/$defs/access-single", "signature=secret"],
  ]) {
    const events = resultFixture("public-resource");
    events.push(fileVariantEvent(200, mode!, schema!, variant));
    expect(checkResults(`file-description-invalid-${mode}-${variant === undefined}`, events).exitCode).toBe(1);
  }
});

test("description requests cannot claim conditional or redirect response coverage", () => {
  for (const [status, mode] of [[206, "binary"], [304, "not-modified"], [307, "redirect"]] as const) {
    const events = resultFixture("public-resource");
    events.push(fileVariantEvent(status, mode, "", "access-description"));
    const result = checkResults(`file-description-status-${status}`, events);
    expect(result.exitCode).toBe(1);
    expect(result.stderr.toString()).toContain("incorrect access description status/mode");
  }
});

test("only an explicit file redirect binding permits 307 wire evidence", () => {
  const events = resultFixture("public-resource");
  events.push(fileVariantEvent(307, "redirect", ""));
  expect(checkResults("file-redirect-valid", events).exitCode).toBe(0);
  const invalid = resultFixture("core");
  invalid.push({ Action: "output", Test: "TestCanvasContract/canvas.list", Output: `CANVAS_CONTRACT_HTTP ${JSON.stringify({ method: "GET", path: "/api/canvas-projects", status: 307, body_complete: true, transport_failure: false, wire: { version: wireRegistry.version, schema: "", mode: "redirect", valid: true } })}\n` });
  expect(checkResults("file-redirect-unbound", invalid).exitCode).toBe(1);
});

function resultFixture(profileName: string): ResultEvent[] {
  const operations = manifest.operations.filter(operation => operation.profile === profileName);
  const scenarios = [...new Set(operations.flatMap(operation => operation.scenario.split(",").map(value => value.trim())))];
  return [
    { Action: "output", Test: "TestCanvasContract", Output: `implementation=go database=postgres-18 profile=${profileName} run_id=fixture` },
    ...responseLossResultEvents(profileName),
    ...operations.flatMap(operation => operation.scenario.split(",").map(scenario => ({ Action: "output", Test: `TestCanvasContract/${scenario.trim()}`, Output: `CANVAS_CONTRACT_HTTP ${JSON.stringify({ method: operation.method, path: operation.path.replace(/\{[^}]+\}/g, "fixture-id"), status: 200, body_complete: true, transport_failure: false, wire: wireEvidence(operation.method, operation.path) })}\n` }))),
    ...faultResultEvents(profileName),
    ...scenarios.map(scenario => ({ Action: "pass", Test: `TestCanvasContract/${scenario}` })),
    { Action: "pass", Test: "TestCanvasContract" },
  ];
}

function faultResultEvents(profileName: string): ResultEvent[] {
  return manifest.controlled_faults.filter(fault => fault.profile === profileName && fault.kind === "request-body-half-close").flatMap(fault => Array.from({ length: fault.requests_per_scenario }, () => ({
    Action: "output", Test: `TestCanvasContract/${fault.scenario}`,
    Output: `CANVAS_CONTRACT_HTTP ${JSON.stringify({ method: fault.method, path: fault.path.replace(/\{[^}]+\}/g, "fixture-id"), status: fault.expected_status, body_complete: true, transport_failure: false, request_fault: fault.id, declared_body_bytes: fault.declared_body_bytes, sent_body_bytes: fault.sent_body_bytes, wire: { version: wireRegistry.version, mode: "json", schema: "common.v1.schema.json#/$defs/error-envelope", valid: true } })}\n`,
  })));
}

function responseLossResultEvents(profileName: string): ResultEvent[] {
  const faults = manifest.controlled_faults.filter(fault => fault.profile === profileName && fault.kind === "response-loss-after-commit");
  if (!faults.length) return [];
  return [
    ...faults.map(fault => ({
      Action: "output", Test: `TestCanvasContract/${fault.scenario}`,
      Output: `CANVAS_CONTRACT_HTTP ${JSON.stringify({ method: fault.method, path: fault.path.replace(/\{[^}]+\}/g, "fixture-id"), status: 0, body_complete: false, transport_failure: true, request_fault: fault.id, response_loss: { target_requests: 1, target_status: 200, target_body_complete: true, target_body_bytes: 200, request_body_bytes: 0, request_sha256: "a".repeat(64), client_bytes_sent: 0, connection_closed: true, client_eof: true, target_wire: wireEvidence(fault.method, fault.path) } })}\n`,
    })),
    { Action: "output", Test: `TestCanvasContract/${faults[0].scenario}`, Output: "CANVAS_CONTRACT_RESTART mode=crash generation=4\n" },
  ];
}

test("response loss counts incomplete client requests without inventing complete wire coverage", () => {
  const result = checkResults("response-loss-valid", resultFixture("restart-recovery"));
  expect(result.exitCode).toBe(0);
  const report = JSON.parse(result.stdout.toString());
  expect(report.controlled_response_loss_requests).toBe(3);
  expect(report.controlled_fault_requests).toBe(3);
  expect(report.incomplete_requests).toBe(3);
  expect(report.unclassified_incomplete_requests).toBe(0);
  expect(report.requests_observed).toBe(report.wire_validated_requests);
});

test("response loss requires one complete target acknowledgement and exact isolated failure witness", () => {
  for (const [name, mutate] of [
    ["missing-witness", (fact: Record<string, any>) => { delete fact.response_loss; }],
    ["duplicate-target", (fact: Record<string, any>) => { fact.response_loss.target_requests = 2; }],
    ["target-error", (fact: Record<string, any>) => { fact.response_loss.target_status = 500; }],
    ["target-incomplete", (fact: Record<string, any>) => { fact.response_loss.target_body_complete = false; }],
    ["missing-bytes", (fact: Record<string, any>) => { fact.response_loss.target_body_bytes = 0; }],
    ["missing-fingerprint", (fact: Record<string, any>) => { fact.response_loss.request_sha256 = "secret-prompt"; }],
    ["wrong-wire", (fact: Record<string, any>) => { fact.response_loss.target_wire.schema = "wrong"; }],
    ["wire-invalid", (fact: Record<string, any>) => { fact.response_loss.target_wire.valid = false; }],
    ["unexpected-client-wire", (fact: Record<string, any>) => { fact.wire = wireEvidence(fact.method, fact.path); }],
    ["client-received-byte", (fact: Record<string, any>) => { fact.response_loss.client_bytes_sent = 1; }],
    ["no-closure", (fact: Record<string, any>) => { fact.response_loss.connection_closed = false; }],
    ["non-eof", (fact: Record<string, any>) => { fact.response_loss.client_eof = false; }],
    ["claimed-client-success", (fact: Record<string, any>) => { fact.status = 200; fact.body_complete = true; fact.transport_failure = false; }],
  ] as const) {
    const events = resultFixture("restart-recovery");
    const event = events.find(event => event.Output?.includes('"response_loss"'))!;
    const fact = JSON.parse(event.Output!.slice("CANVAS_CONTRACT_HTTP ".length));
    mutate(fact);
    event.Output = `CANVAS_CONTRACT_HTTP ${JSON.stringify(fact)}\n`;
    const result = checkResults(`invalid-response-loss-${name}`, events);
    expect(result.exitCode).toBe(1);
    expect(result.stderr.toString()).toContain("response loss lacks one complete target 200");
  }
});

test("response loss cannot excuse an undeclared route, another failure or missing crash/recovery evidence", () => {
  for (const [name, mutate] of [
    ["fault-id", (fact: Record<string, any>) => { fact.request_fault = "accept-all-network-errors"; }],
    ["route", (fact: Record<string, any>) => { fact.path = "/api/tasks"; }],
    ["method", (fact: Record<string, any>) => { fact.method = "DELETE"; }],
    ["label-missing", (fact: Record<string, any>) => { delete fact.request_fault; }],
  ] as const) {
    const events = resultFixture("restart-recovery");
    const event = events.find(event => event.Output?.includes('"response_loss"'))!;
    const fact = JSON.parse(event.Output!.slice("CANVAS_CONTRACT_HTTP ".length));
    mutate(fact);
    event.Output = `CANVAS_CONTRACT_HTTP ${JSON.stringify(fact)}\n`;
    expect(checkResults(`unclassified-response-loss-${name}`, events).exitCode).toBe(1);
  }
  const noCrash = resultFixture("restart-recovery").filter(event => !event.Output?.startsWith("CANVAS_CONTRACT_RESTART"));
  expect(checkResults("response-loss-no-crash", noCrash).stderr.toString()).toContain("response loss lacks subsequent owned-target crash");
  const noRecovery = resultFixture("restart-recovery").filter(event => !(event.Test === "TestCanvasContract/canvas.recovery.response-loss-and-restart" && event.Output?.includes('"method":"GET"') && event.Output?.includes('"path":"/api/resources/storage-usage"')));
  expect(checkResults("response-loss-no-recovery", noRecovery).stderr.toString()).toContain("response loss lacks subsequent successful recovery");
  const reordered = resultFixture("restart-recovery");
  const restart = reordered.splice(reordered.findIndex(event => event.Output?.startsWith("CANVAS_CONTRACT_RESTART")), 1)[0];
  reordered.push(restart);
  expect(checkResults("response-loss-recovery-before-crash", reordered).exitCode).toBe(1);
  expect(checkManifest("response-loss-weakened-status", copy => { copy.controlled_faults.find(fault => fault.id === "document-response-loss").target_status = 500; }).exitCode).toBe(1);
});

test("controlled request-body faults require complete wire, framing and exact scenario counts", () => {
  const result = checkResults("fault-valid", resultFixture("resource-integrity"));
  expect(result.exitCode).toBe(0);
  expect(JSON.parse(result.stdout.toString()).controlled_fault_requests).toBe(2);
  const events = resultFixture("resource-integrity");
  events.splice(events.findIndex(event => event.Output?.includes('"request_fault"')), 1);
  const missing = checkResults("fault-missing", events);
  expect(missing.exitCode).toBe(1);
  expect(missing.stderr.toString()).toContain("controlled fault request count differs");
});

test("controlled faults cannot be moved to an undeclared scenario, operation or identity", () => {
  for (const [name, change] of [
    ["identity", (fact: Record<string, unknown>) => { fact.request_fault = "undeclared-reset"; }],
    ["route", (fact: Record<string, unknown>) => { fact.path = "/api/resources/fixture-id/file"; }],
    ["method", (fact: Record<string, unknown>) => { fact.method = "GET"; }],
  ] as const) {
    const events = resultFixture("resource-integrity");
    const event = events.find(event => event.Output?.includes('"request_fault"'))!;
    const fact = JSON.parse(event.Output!.slice("CANVAS_CONTRACT_HTTP ".length));
    change(fact);
    event.Output = `CANVAS_CONTRACT_HTTP ${JSON.stringify(fact)}\n`;
    const result = checkResults(`fault-${name}`, events);
    expect(result.exitCode).toBe(1);
    expect(result.stderr.toString()).toContain("controlled fault outside declared scenario/operation");
  }
  const events = resultFixture("resource-integrity");
  events.find(event => event.Output?.includes('"request_fault"'))!.Test = "TestCanvasContract/canvas.resource.metadata-and-access";
  expect(checkResults("fault-wrong-scenario", events).exitCode).toBe(1);
});

test("controlled fault classification never excuses transport failure or incomplete responses", () => {
  for (const [name, change] of [
    ["transport", (fact: Record<string, unknown>) => { fact.status = 0; fact.body_complete = false; fact.transport_failure = true; }],
    ["truncation", (fact: Record<string, unknown>) => { fact.body_complete = false; }],
    ["declared-bytes", (fact: Record<string, unknown>) => { fact.declared_body_bytes = 14; }],
    ["sent-bytes", (fact: Record<string, unknown>) => { fact.sent_body_bytes = 7; }],
    ["unexpected-success", (fact: Record<string, unknown>) => { fact.status = 200; }],
  ] as const) {
    const events = resultFixture("resource-integrity");
    const event = events.find(event => event.Output?.includes('"request_fault"'))!;
    const fact = JSON.parse(event.Output!.slice("CANVAS_CONTRACT_HTTP ".length));
    change(fact);
    event.Output = `CANVAS_CONTRACT_HTTP ${JSON.stringify(fact)}\n`;
    const result = checkResults(`fault-invalid-${name}`, events);
    expect(result.exitCode).toBe(1);
    expect(result.stderr.toString()).toContain("controlled fault lacks exact framing/complete failure response");
    if (name === "transport" || name === "truncation") expect(result.stderr.toString()).toContain("unclassified incomplete HTTP request");
  }
});

test("controlled faults still require their error schema and cannot hide framing labels", () => {
  for (const mode of ["missing-wire", "wrong-wire", "no-fault-id"]) {
    const events = resultFixture("resource-integrity");
    const event = events.find(event => event.Output?.includes('"request_fault"'))!;
    const fact = JSON.parse(event.Output!.slice("CANVAS_CONTRACT_HTTP ".length));
    if (mode === "missing-wire") delete fact.wire;
    if (mode === "wrong-wire") fact.wire.mode = "binary";
    if (mode === "no-fault-id") delete fact.request_fault;
    event.Output = `CANVAS_CONTRACT_HTTP ${JSON.stringify(fact)}\n`;
    expect(checkResults(`fault-${mode}`, events).exitCode).toBe(1);
  }
});

test("controlled fault registry cannot declare a broad failure or omit executable evidence", () => {
  for (const [name, change] of [
    ["wrong-status", (fault: Record<string, unknown>) => { fault.expected_status = 0; }],
    ["outside-route", (fault: Record<string, unknown>) => { fault.path = "/api/canvas-projects"; }],
    ["complete-body", (fault: Record<string, unknown>) => { fault.sent_body_bytes = fault.declared_body_bytes; }],
    ["missing-evidence", (fault: Record<string, unknown>) => { fault.evidence = []; }],
  ] as const) {
    expect(checkManifest(`fault-registry-${name}`, copy => { change(copy.controlled_faults[0]); }).exitCode).toBe(1);
  }
});

function checkResults(name: string, events: ReturnType<typeof resultFixture>) {
  const filename = resolve(temporaryDirectory, `${name}.jsonl`);
  writeFileSync(filename, events.map(event => JSON.stringify(event)).join("\n"));
  return Bun.spawnSync([process.execPath, resolve(suiteRoot, "check-catalog.ts"), `--results=${filename}`]);
}

test("passing scenario names alone do not prove HTTP route touches", () => {
  const result = checkResults("untraced", resultFixture("core").filter(event => !event.Output?.startsWith("CANVAS_CONTRACT_HTTP")));
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("result missing HTTP request evidence");
});

test("actual route evidence distinguishes setup touches from implemented operations", () => {
  const events = resultFixture("core");
  events.push({ Action: "output", Test: "TestCanvasContract", Output: `CANVAS_CONTRACT_HTTP ${JSON.stringify({ method: "POST", path: "/api/projects", status: 200, body_complete: true, transport_failure: false, wire: wireEvidence("POST", "/api/projects") })}\n` });
  const result = checkResults("actual-touches", events);
  expect(result.exitCode).toBe(0);
  const report = JSON.parse(result.stdout.toString());
  expect(report.method_paths_mapped_executed).toBe(7);
  expect(report.method_paths_exercised).toBe(8);
  expect(report.operation_executed).toBe(13);
});

test("mapped operations need matching method/path evidence in mapped scenarios", () => {
  const events = resultFixture("core");
  events.forEach(event => {
    if (event.Output?.includes('"method":"DELETE"')) event.Output = event.Output.replace('"method":"DELETE"', '"method":"GET"');
  });
  const result = checkResults("incorrect-method", events);
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("mapped operation lacks HTTP evidence");
});

test("every mapped scenario needs its own matching operation request", () => {
  const events = resultFixture("core").filter(event => !(event.Test === "TestCanvasContract/canvas.concurrent-cas" && event.Output?.includes('"method":"PUT"')));
  const result = checkResults("missing-one-scenario-request", events);
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("mapped operation lacks HTTP evidence in its scenario: canvas.update: canvas.concurrent-cas");
});

test("literal resource routes take precedence over resource IDs", () => {
  const result = checkResults("literal-route", resultFixture("resource-integrity"));
  expect(result.exitCode).toBe(0);
  const report = JSON.parse(result.stdout.toString());
  expect(report.observed_routes.some(item => item.route === "GET /api/resources/storage-usage")).toBe(true);
});

test("route touches without valid versioned wire evidence cannot pass", () => {
  for (const mutation of ["missing", "invalid", "wrong-version", "wrong-schema", "wrong-mode"]) {
    const events = resultFixture("core");
    const event = events.find(event => event.Output?.includes("CANVAS_CONTRACT_HTTP "))!;
    const fact = JSON.parse(event.Output!.split("CANVAS_CONTRACT_HTTP ")[1]);
    if (mutation === "missing") delete fact.wire;
    if (mutation === "invalid") fact.wire.valid = false;
    if (mutation === "wrong-version") fact.wire.version = "unversioned";
    if (mutation === "wrong-schema") fact.wire.schema = "common.v1.schema.json";
    if (mutation === "wrong-mode") fact.wire.mode = "unbound-raw";
    event.Output = `CANVAS_CONTRACT_HTTP ${JSON.stringify(fact)}\n`;
    const result = checkResults(`wire-${mutation}`, events);
    expect(result.exitCode).toBe(1);
    expect(result.stderr.toString()).toContain("wire");
  }
});

test("binary route touches cannot masquerade as JSON validation", () => {
  const events = resultFixture("public-resource");
  const event = events.find(event => event.Output?.includes("CANVAS_CONTRACT_HTTP "))!;
  const fact = JSON.parse(event.Output!.split("CANVAS_CONTRACT_HTTP ")[1]);
  fact.wire.mode = "json";
  fact.wire.schema = "common.v1.schema.json";
  event.Output = `CANVAS_CONTRACT_HTTP ${JSON.stringify(fact)}\n`;
  const result = checkResults("binary-wire-mode", events);
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("binary route mislabeled as JSON wire coverage");
});

test("passing scenarios cannot hide unclassified incomplete requests", () => {
  const events = resultFixture("core");
  events.push({ Action: "output", Test: "TestCanvasContract/canvas.list", Output: `CANVAS_CONTRACT_HTTP ${JSON.stringify({ method: "GET", path: "/api/canvas-projects", status: 0, body_complete: false, transport_failure: true })}\n` });
  const result = checkResults("unclassified-incomplete", events);
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("unclassified incomplete HTTP request");
});

test("incomplete requests cannot bypass route catalog validation", () => {
  const events = resultFixture("core");
  events.push({ Action: "output", Test: "TestCanvasContract/canvas.list", Output: `CANVAS_CONTRACT_HTTP ${JSON.stringify({ method: "GET", path: "/api/canvas-projects/fixture-id/unregistered", status: 0, body_complete: false, transport_failure: true })}\n` });
  const result = checkResults("uncataloged-incomplete", events);
  expect(result.exitCode).toBe(1);
  expect(result.stderr.toString()).toContain("uncataloged HTTP request");
});

test("auxiliary command packages without tests are not skipped contract scenarios", () => {
  const events = resultFixture("core");
  events.push({ Action: "output", Package: "canvas-contract/cmd/target-control", Output: "? canvas-contract/cmd/target-control [no test files]\n" });
  events.push({ Action: "skip", Package: "canvas-contract/cmd/target-control" });
  expect(checkResults("command-without-tests", events).exitCode).toBe(0);
});

test("named scenario skips and unexplained package skips still fail the baseline", () => {
  for (const skipped of [
    { Action: "skip", Package: "canvas-contract", Test: "TestCanvasContract/canvas.list" },
    { Action: "skip", Package: "canvas-contract" },
  ]) {
    const events = resultFixture("core");
    events.push(skipped);
    const result = checkResults(`real-skip-${skipped.Test ? "scenario" : "package"}`, events);
    expect(result.exitCode).toBe(1);
    expect(result.stderr.toString()).toContain("failed/skipped execution");
  }
});
