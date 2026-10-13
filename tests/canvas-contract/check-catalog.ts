import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

type Operation = {
  id: string;
  profile: string;
  scenario: string;
  method: string;
  path: string;
  owner: string;
  replay: string;
  implementation?: string;
  evidence: string[];
};
type BehaviorDecision = {
  id: string;
  classification: string;
  proposal: string;
  approval: string;
  approved_by?: string;
  operations: string[];
  go_observation: string;
  proposed_target: string;
  evidence: string[];
};
type Profile = {
  status: string;
  implementation: string;
  operations: string[];
  reason?: string;
  remaining_paths?: string[];
  authorization: string;
  effects: string;
  dependencies: string[];
};
type RequestEvidence = {
  method: string;
  path: string;
  status: number;
  body_complete: boolean;
  transport_failure: boolean;
  request_variant?: string;
  request_fault?: string;
  declared_body_bytes?: number;
  sent_body_bytes?: number;
  response_loss?: {
    target_requests: number; target_status: number; target_body_complete: boolean; target_body_bytes: number;
    request_body_bytes: number; request_sha256: string; client_bytes_sent: number; connection_closed: boolean; client_eof: boolean;
    target_wire: { version: string; schema: string; mode: string; valid: boolean };
  };
  wire?: { version: string; schema: string; mode: string; valid: boolean };
};
type ControlledFault = {
  id: string;
  kind: "request-body-half-close" | "response-loss-after-commit";
  profile: string;
  scenario: string;
  method: string;
  path: string;
  declared_body_bytes?: number;
  sent_body_bytes?: number;
  expected_status: number;
  target_status?: number;
  recovery_operations?: string[];
  recovery_restart?: { mode: string; generation: number };
  requests_per_scenario: number;
  evidence: string[];
};
type WireBinding = { method: string; path: string; schema?: string; binary?: boolean; stream?: boolean; not_modified?: boolean; descriptor_schema?: string; redirect?: boolean };

const suiteRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(suiteRoot, "../..");
const manifestArgument = process.argv.slice(2).find(argument => argument.startsWith("--manifest="));
const manifestFile = manifestArgument ? resolve(manifestArgument.slice("--manifest=".length)) : resolve(suiteRoot, "contract/manifest.yaml");
const manifest = Bun.YAML.parse(readFileSync(manifestFile, "utf8")) as {
  suite: { wire_schema: { version: string; registry: string; approval: string } };
  coverage_policy: { route_prefixes: string[]; direct_canvas_routes: number; direct_canvas_route_breakdown: Record<string, number> };
  profiles: Record<string, Profile>;
  operations: Operation[];
  behavior_decisions: BehaviorDecision[];
  controlled_faults?: ControlledFault[];
};
const canonical = (route: string) => route.replace(/:[A-Za-z][A-Za-z0-9]*/g, "{}").replace(/\{[^}]+\}/g, "{}");
const routeKey = (method: string, route: string) => `${method} ${canonical(route)}`;
const inScope = (route: string) => manifest.coverage_policy.route_prefixes.some(prefix => route === prefix || route.startsWith(`${prefix}/`));
const scenariosOf = (operation: Operation) => operation.scenario.split(",").map(value => value.trim());
const errors: string[] = [];
const assert = (condition: unknown, message: string) => { if (!condition) errors.push(message); };
const source = readdirSync(suiteRoot).filter(name => name.endsWith("_test.go")).sort().map(name => readFileSync(resolve(suiteRoot, name), "utf8")).join("\n");
const scenarios = new Set([...source.matchAll(/\b(?:t|test)\.Run\("([^"\n]+)"/g)].map(match => match[1]));
const scenarioIndex = readFileSync(resolve(suiteRoot, "scenarios/README.md"), "utf8");
const indexedScenarios = new Set([...scenarioIndex.matchAll(/^- `([^`]+)`$/gm)].map(match => match[1]));
for (const scenario of scenarios) assert(indexedScenarios.has(scenario), `scenario absent from README index: ${scenario}`);
for (const scenario of indexedScenarios) assert(scenarios.has(scenario), `README indexes unknown scenario: ${scenario}`);
const routes = new Set<string>();
for (const file of readdirSync(resolve(repoRoot, "backend/internal/handler")).filter(name => name.endsWith(".go") && !name.endsWith("_test.go"))) {
  const handler = readFileSync(resolve(repoRoot, "backend/internal/handler", file), "utf8");
  for (const match of handler.matchAll(/\b\w+\.(GET|POST|PUT|PATCH|DELETE)\("([^"\n]+)"\s*,/g)) {
    if (inScope(`/api${match[2]}`)) routes.add(routeKey(match[1], `/api${match[2]}`));
  }
  for (const match of handler.matchAll(/for _, (\w+) := range \[\]string\{([^}]+)\} \{\s*\w+\.(GET|POST|PUT|PATCH|DELETE)\("([^"\n]+)"\+\1,/g)) {
    for (const action of match[2].matchAll(/"([^"\n]+)"/g)) {
      if (inScope(`/api${match[4]}${action[1]}`)) routes.add(routeKey(match[3], `/api${match[4]}${action[1]}`));
    }
  }
}
const operations = new Map<string, Operation>();
const catalogRoutes = new Set<string>();
const mappedScenarios = new Set<string>();
for (const operation of manifest.operations) {
  assert(!operations.has(operation.id), `duplicate operation: ${operation.id}`);
  operations.set(operation.id, operation);
  catalogRoutes.add(routeKey(operation.method, operation.path));
  assert(routes.has(routeKey(operation.method, operation.path)), `unregistered method/path: ${operation.id}`);
  assert(operation.owner && operation.replay && operation.evidence.length, `missing contract fields: ${operation.id}`);
  for (const evidence of operation.evidence) assert(existsSync(resolve(repoRoot, evidence)), `missing evidence: ${operation.id}: ${evidence}`);
  const profile = manifest.profiles[operation.profile];
  assert(profile?.operations.includes(operation.id), `operation absent from its profile: ${operation.id}`);
  if (profile?.status === "REQUIRED") {
    for (const scenario of scenariosOf(operation)) {
      mappedScenarios.add(scenario);
      assert(scenarios.has(scenario), `unimplemented scenario: ${operation.id}: ${scenario}`);
    }
  }
}
for (const route of routes) assert(catalogRoutes.has(route), `uncataloged handler route: ${route}`);
const wireRegistry = JSON.parse(readFileSync(resolve(repoRoot, "tests/canvas-contract/contract/schemas/bindings.v1.json"), "utf8")) as { version: string; approval: string; bindings: WireBinding[] };
assert(manifest.suite.wire_schema?.version === wireRegistry.version && manifest.suite.wire_schema?.registry === "tests/canvas-contract/contract/schemas/bindings.v1.json" && manifest.suite.wire_schema?.approval === wireRegistry.approval, "manifest and wire registry metadata disagree");
const wireBindings = new Map<string, WireBinding>();
const requiredRoutes = new Set(manifest.operations.filter(operation => manifest.profiles[operation.profile]?.status === "REQUIRED").map(operation => routeKey(operation.method, operation.path)));
for (const binding of wireRegistry.bindings) {
  const route = routeKey(binding.method, binding.path);
  assert(!wireBindings.has(route), `duplicate wire binding: ${route}`);
  assert(requiredRoutes.has(route), `wire binding not assigned to a Required route: ${route}`);
  assert(Number(Boolean(binding.binary)) + Number(Boolean(binding.stream)) + Number(Boolean(binding.schema)) === 1, `wire binding must have exactly one JSON schema, binary or stream mode: ${route}`);
  assert(!binding.stream || !binding.descriptor_schema && !binding.redirect && !binding.not_modified, `stream binding cannot declare file variants: ${route}`);
  assert(binding.binary || binding.stream || !binding.descriptor_schema && !binding.redirect, `file variants require a binary binding: ${route}`);
  for (const reference of [binding.schema, binding.descriptor_schema].filter(Boolean) as string[]) {
    const [filename, fragment] = reference.split("#");
    assert(/^[a-z-]+\.v1\.schema\.json$/.test(filename), `invalid schema filename: ${route}`);
    const schemaFile = resolve(suiteRoot, "contract/schemas", filename);
    assert(existsSync(schemaFile), `missing wire schema: ${route}`);
    if (existsSync(schemaFile)) {
      const schema = JSON.parse(readFileSync(schemaFile, "utf8"));
      const definition = fragment?.split("/").slice(1).reduce((value, segment) => value?.[segment.replace(/~1/g, "/").replace(/~0/g, "~")], schema);
      assert(!fragment || definition, `missing wire schema fragment: ${route}`);
    }
  }
  wireBindings.set(route, binding);
}
for (const route of requiredRoutes) assert(wireBindings.has(route), `Required route lacks wire binding: ${route}`);
for (const scenario of scenarios) assert(mappedScenarios.has(scenario), `scenario not indexed: ${scenario}`);
for (const [name, profile] of Object.entries(manifest.profiles)) {
  assert(["REQUIRED", "DEFERRED", "OUT_OF_SCOPE"].includes(profile.status), `invalid profile status: ${name}`);
  assert(profile.authorization && profile.effects, `missing authorization/effects: ${name}`);
  for (const dependency of profile.dependencies) assert(manifest.profiles[dependency], `unknown dependency: ${name}: ${dependency}`);
  assert(new Set(profile.operations).size === profile.operations.length, `duplicate profile operation: ${name}`);
  for (const operationID of profile.operations) assert(operations.get(operationID)?.profile === name, `incorrect profile assignment: ${name}: ${operationID}`);
  if (profile.status === "REQUIRED") {
    assert(Array.isArray(profile.remaining_paths) && profile.remaining_paths.every(path => typeof path === "string" && path.trim().length > 0) && new Set(profile.remaining_paths).size === profile.remaining_paths.length, `missing/invalid explicit remaining paths: ${name}`);
  } else {
    assert(typeof profile.reason === "string" && profile.reason.trim().length > 0, `missing remaining paths/blocker: ${name}`);
  }
}
const controlledFaults = manifest.controlled_faults ?? [];
const controlledFaultIDs = new Set<string>();
for (const fault of controlledFaults) {
  const lossPaths: Record<string, string> = { "document-response-loss": "PUT /api/canvas-projects/{}", "resource-create-response-loss": "POST /api/resources", "upload-complete-response-loss": "POST /api/resources/uploads/{}/complete", "resource-import-response-loss": "POST /api/resources/import" };
  assert((fault.kind === "request-body-half-close" && fault.id === "upload-body-half-close" || fault.kind === "response-loss-after-commit" && lossPaths[fault.id] === routeKey(fault.method, fault.path)) && !controlledFaultIDs.has(fault.id), `unknown/duplicate controlled fault: ${fault.id}`);
  controlledFaultIDs.add(fault.id);
  assert(manifest.profiles[fault.profile]?.status === "REQUIRED" && scenarios.has(fault.scenario), `controlled fault lacks a Required scenario: ${fault.id}`);
  assert(manifest.operations.some(operation => operation.profile === fault.profile && scenariosOf(operation).includes(fault.scenario) && routeKey(operation.method, operation.path) === routeKey(fault.method, fault.path)), `controlled fault lacks its operation mapping: ${fault.id}`);
  if (fault.kind === "request-body-half-close") {
    assert(fault.method === "PUT" && canonical(fault.path) === "/api/resources/uploads/{}/chunks/{}" && fault.expected_status === 400 && fault.target_status === undefined && fault.recovery_operations === undefined && fault.recovery_restart === undefined, `controlled fault cannot bypass error wire or route checks: ${fault.id}`);
    assert(Number.isSafeInteger(fault.declared_body_bytes) && Number.isSafeInteger(fault.sent_body_bytes) && fault.sent_body_bytes! > 0 && fault.sent_body_bytes! < fault.declared_body_bytes!, `invalid controlled fault framing/count: ${fault.id}`);
  } else {
    assert(fault.profile === "restart-recovery" && fault.scenario === "canvas.recovery.response-loss-and-restart" && fault.expected_status === 0 && fault.target_status === 200 && fault.declared_body_bytes === undefined && fault.sent_body_bytes === undefined && fault.requests_per_scenario === 1, `invalid response-loss declaration: ${fault.id}`);
    assert(fault.recovery_restart?.mode === "crash" && fault.recovery_restart.generation === 4, `response loss lacks owned-target crash recovery: ${fault.id}`);
    assert(Array.isArray(fault.recovery_operations) && fault.recovery_operations.length > 0 && new Set(fault.recovery_operations).size === fault.recovery_operations.length, `response loss lacks recovery operations: ${fault.id}`);
    for (const identity of fault.recovery_operations ?? []) {
      const operation = operations.get(identity);
      assert(operation?.profile === fault.profile && scenariosOf(operation).includes(fault.scenario), `response loss recovery operation outside scenario: ${fault.id}: ${identity}`);
    }
  }
  assert(Number.isSafeInteger(fault.requests_per_scenario) && fault.requests_per_scenario > 0, `invalid controlled fault framing/count: ${fault.id}`);
  assert(fault.evidence?.length, `controlled fault lacks source evidence: ${fault.id}`);
  for (const path of fault.evidence ?? []) assert(existsSync(resolve(repoRoot, path)), `missing controlled fault evidence: ${path}`);
}
const directProfiles = { core_document: "core", share_public: "share-public", import: "import" };
const decisionIDs = new Set<string>();
assert(manifest.behavior_decisions?.length, "missing behavior decision ledger");
for (const decision of manifest.behavior_decisions ?? []) {
  assert(decision.id && !decisionIDs.has(decision.id), `duplicate/missing behavior decision: ${decision.id}`);
  decisionIDs.add(decision.id);
  assert(["UNKNOWN", "MUST_KEEP", "KNOWN_BUG", "APPROVED_CHANGE"].includes(decision.classification), `invalid decision classification: ${decision.id}`);
  assert(["MUST_KEEP", "KNOWN_BUG", "APPROVED_CHANGE"].includes(decision.proposal), `invalid decision proposal: ${decision.id}`);
  assert(["pending", "approved"].includes(decision.approval), `invalid decision approval: ${decision.id}`);
  if (decision.approval === "pending") assert(decision.classification === "UNKNOWN", `pending decision cannot silently approve Go behavior: ${decision.id}`);
  if (decision.approval === "approved") assert(decision.approved_by && decision.classification !== "UNKNOWN", `approved decision lacks an approver or resolved classification: ${decision.id}`);
  assert(decision.operations?.length && decision.go_observation && decision.proposed_target && decision.evidence?.length, `incomplete behavior decision: ${decision.id}`);
  for (const operation of decision.operations ?? []) assert(operations.has(operation), `decision references unknown operation: ${decision.id}: ${operation}`);
  for (const evidence of decision.evidence ?? []) assert(existsSync(resolve(repoRoot, evidence)), `missing decision evidence: ${decision.id}: ${evidence}`);
}
for (const [surface, name] of Object.entries(directProfiles)) {
  const distinctRoutes = new Set(manifest.operations.filter(operation => operation.profile === name).map(operation => routeKey(operation.method, operation.path)));
  assert(distinctRoutes.size === manifest.coverage_policy.direct_canvas_route_breakdown[surface], `incorrect direct route count: ${surface}`);
}
assert(Object.values(manifest.coverage_policy.direct_canvas_route_breakdown).reduce((total, count) => total + count, 0) === manifest.coverage_policy.direct_canvas_routes, "incorrect direct route total");

const results = new Map<string, Set<string>>();
const requestFacts: { profile: string; scenario: string; route: string; status: number }[] = [];
let wireValidatedRequests = 0;
const routeMatchers = [...catalogRoutes].map(route => {
  const separator = route.indexOf(" ");
  const method = route.slice(0, separator);
  const path = route.slice(separator + 1);
  const segments = path.split("/");
  const expression = segments.map(segment => segment === "{}" ? "[^/]+" : segment.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("/");
  return { route, method, expression: new RegExp(`^${expression}$`), specificity: segments.filter(segment => segment !== "{}").length };
}).sort((first, second) => second.specificity - first.specificity);
let resultRuns = 0;
let incompleteRequests = 0;
let unclassifiedIncompleteRequests = 0;
let controlledFaultRequests = 0;
let controlledResponseLossRequests = 0;
const executionTargets = new Set<string>();
for (const argument of process.argv.slice(2)) {
  if (argument.startsWith("--routes-log=")) {
    const log = readFileSync(resolve(argument.slice("--routes-log=".length)), "utf8");
    const registered = new Set([...log.matchAll(/\[GIN-debug\] (GET|POST|PUT|PATCH|DELETE)\s+(\S+)/g)].filter(match => inScope(match[2])).map(match => routeKey(match[1], match[2])));
    assert(registered.size > 0, "no registered routes found in service log");
    for (const route of registered) assert(catalogRoutes.has(route), `uncataloged live registration: ${route}`);
    for (const route of catalogRoutes) assert(registered.has(route), `catalog route absent from live registration: ${route}`);
  } else if (argument.startsWith("--results=")) {
    const events = readFileSync(resolve(argument.slice("--results=".length)), "utf8").split("\n").filter(line => line.trim()).map(line => JSON.parse(line));
    const metadata = events.find(event => event.Test === "TestCanvasContract" && /implementation=\S+ database=\S+ profile=\S+ run_id=\S+/.test(event.Output ?? ""));
    const profileName = metadata?.Output?.match(/profile=(\S+)/)?.[1];
    if (metadata) executionTargets.add(`${metadata.Output.match(/implementation=(\S+)/)?.[1]} / ${metadata.Output.match(/database=(\S+)/)?.[1]}`);
    assert(profileName && manifest.profiles[profileName]?.status === "REQUIRED", "result missing executable profile metadata");
    assert(events.some(event => event.Action === "pass" && event.Test === "TestCanvasContract"), `profile did not pass: ${profileName ?? "unknown"}`);
    assert(!events.some(event => event.Action === "fail" || event.Action === "skip" && (event.Test || !event.Package || !events.some(output => output.Action === "output" && output.Package === event.Package && /\[no test files\]/.test(output.Output ?? "")))), `failed/skipped execution: ${profileName ?? "unknown"}`);
    const passed = new Set<string>(events.filter(event => event.Action === "pass" && event.Test?.startsWith("TestCanvasContract/")).map(event => event.Test.split("/")[1]));
    const observed: (typeof requestFacts[number] & { eventIndex: number })[] = [];
    const lossEvents: { fault: ControlledFault; eventIndex: number }[] = [];
    const faultCounts = new Map<string, number>();
    let evidenceCount = 0;
    for (const [eventIndex, event] of events.entries()) {
      if (event.Action !== "output" || !event.Test?.startsWith("TestCanvasContract") || typeof event.Output !== "string") continue;
      const prefix = "CANVAS_CONTRACT_HTTP ";
      const start = event.Output.indexOf(prefix);
      if (start < 0) continue;
      const fact = JSON.parse(event.Output.slice(start + prefix.length).trim()) as RequestEvidence;
      evidenceCount++;
      assert(typeof fact.method === "string" && typeof fact.path === "string" && !/[?#]/.test(fact.path), `invalid HTTP evidence: ${profileName}`);
      assert(Number.isInteger(fact.status) && (fact.status >= 100 && fact.status <= 599 || fact.status === 0 && fact.transport_failure === true) && typeof fact.body_complete === "boolean" && typeof fact.transport_failure === "boolean", `invalid HTTP status/completion evidence: ${profileName}`);
      assert(fact.request_variant === undefined || fact.request_variant === "access-description", `unknown request variant: ${profileName}`);
      const matcher = routeMatchers.find(candidate => candidate.method === fact.method && candidate.expression.test(fact.path));
      if (inScope(fact.path)) assert(matcher, `uncataloged HTTP request: ${fact.method} ${fact.path}`);
      let classifiedLoss = false;
      if (fact.request_fault !== undefined) {
        const fault = controlledFaults.find(fault => fault.id === fact.request_fault);
        const scenario = event.Test.split("/")[1];
        assert(fault && fault.profile === profileName && fault.scenario === scenario && matcher?.route === routeKey(fault.method, fault.path), `controlled fault outside declared scenario/operation: ${profileName}`);
        if (fault?.kind === "response-loss-after-commit") {
          const witness = fact.response_loss;
          const binding = matcher && wireBindings.get(matcher.route);
          classifiedLoss = fact.status === 0 && fact.body_complete === false && fact.transport_failure === true && fact.wire === undefined && fact.request_variant === undefined && fact.declared_body_bytes === undefined && fact.sent_body_bytes === undefined && witness?.target_requests === 1 && witness.target_status === fault.target_status && witness.target_body_complete === true && Number.isSafeInteger(witness.target_body_bytes) && witness.target_body_bytes > 0 && witness.target_body_bytes <= (12 << 20) && Number.isSafeInteger(witness.request_body_bytes) && witness.request_body_bytes >= 0 && witness.request_body_bytes <= (6 << 20) && /^[a-f0-9]{64}$/.test(witness.request_sha256) && witness.client_bytes_sent === 0 && witness.connection_closed === true && witness.client_eof === true && witness.target_wire?.valid === true && witness.target_wire.version === wireRegistry.version && witness.target_wire.mode === "json" && witness.target_wire.schema === binding?.schema;
          assert(classifiedLoss, `response loss lacks one complete target 200, exact wire and client EOF: ${profileName}`);
          if (classifiedLoss) {
            controlledResponseLossRequests++;
            lossEvents.push({ fault, eventIndex });
          }
        } else {
          assert(fault && fact.response_loss === undefined && fact.declared_body_bytes === fault.declared_body_bytes && fact.sent_body_bytes === fault.sent_body_bytes && fact.status === fault.expected_status && fact.body_complete && !fact.transport_failure, `controlled fault lacks exact framing/complete failure response: ${profileName}`);
        }
        if (fault) {
          faultCounts.set(fault.id, (faultCounts.get(fault.id) ?? 0) + 1);
          controlledFaultRequests++;
        }
      } else {
        assert(fact.declared_body_bytes === undefined && fact.sent_body_bytes === undefined && fact.response_loss === undefined, `fault framing metadata without a declared fault: ${profileName}`);
      }
      if (!fact.body_complete || fact.transport_failure) {
        incompleteRequests++;
        if (!classifiedLoss) {
          unclassifiedIncompleteRequests++;
          assert(false, `unclassified incomplete HTTP request: ${profileName}: ${fact.method} ${fact.path}`);
        }
        continue;
      }
      if (inScope(fact.path)) {
        const binding = matcher && wireBindings.get(matcher.route);
        if (binding) {
          const wire = fact.wire;
          assert(wire?.valid === true && wire?.version === wireRegistry.version, `missing/invalid wire validation evidence: ${profileName}: ${matcher.route}`);
          if (fact.request_variant === "access-description") {
            assert(binding.binary && binding.descriptor_schema, `unbound access description: ${matcher.route}`);
            const expectedSchema = fact.status >= 400 ? "common.v1.schema.json#/$defs/error-envelope" : binding.descriptor_schema;
            assert(fact.status >= 400 && wire?.mode === "json" || fact.status === 200 && wire?.mode === "access-description", `incorrect access description status/mode: ${matcher.route}`);
            assert(wire?.schema === expectedSchema, `incorrect access description schema: ${matcher.route}`);
          } else if (fact.status === 304) {
            assert(wire?.mode === "not-modified" && wire.schema === (binding.schema ?? "") && (binding.not_modified || binding.binary), `unexpected not-modified wire evidence: ${matcher.route}`);
          } else if (binding.redirect && fact.status === 307) {
            assert(wire?.mode === "redirect" && wire.schema === "", `incorrect redirect wire evidence: ${matcher.route}`);
          } else if (binding.stream && fact.status >= 200 && fact.status < 300) {
            assert(wire?.mode === "stream" && wire.schema === "", `stream route mislabeled as JSON wire coverage: ${matcher.route}`);
          } else if (binding.binary && fact.status >= 200 && fact.status < 300) {
            assert(wire?.mode === "binary" && wire.schema === "", `binary route mislabeled as JSON wire coverage: ${matcher.route}`);
          } else if (binding.binary && fact.status === 416 && wire?.mode === "range-error") {
            assert(wire.schema === "", `range-error route mislabeled as JSON wire coverage: ${matcher.route}`);
          } else {
            const expectedSchema = fact.status >= 400 ? "common.v1.schema.json#/$defs/error-envelope" : binding.schema;
            assert(wire?.mode === "json" && wire.schema === expectedSchema, `incorrect wire schema evidence: ${matcher.route}`);
          }
          if (wire?.valid === true && wire.version === wireRegistry.version) wireValidatedRequests++;
        }
        if (matcher && profileName) observed.push({ profile: profileName, scenario: event.Test.split("/")[1] ?? "setup", route: matcher.route, status: fact.status, eventIndex });
      }
    }
    assert(evidenceCount > 0, `result missing HTTP request evidence: ${profileName}`);
    for (const fault of controlledFaults.filter(fault => fault.profile === profileName)) assert(faultCounts.get(fault.id) === fault.requests_per_scenario, `controlled fault request count differs from declared scenario: ${fault.id}`);
    for (const loss of lossEvents) {
      const restartIndex = events.findIndex((event, index) => index > loss.eventIndex && event.Test === `TestCanvasContract/${loss.fault.scenario}` && event.Output?.trim().endsWith(`CANVAS_CONTRACT_RESTART mode=${loss.fault.recovery_restart?.mode} generation=${loss.fault.recovery_restart?.generation}`));
      assert(restartIndex > loss.eventIndex, `response loss lacks subsequent owned-target crash evidence: ${loss.fault.id}`);
      for (const identity of loss.fault.recovery_operations ?? []) {
        const operation = operations.get(identity);
        assert(operation && observed.some(fact => fact.scenario === loss.fault.scenario && fact.eventIndex > restartIndex && fact.eventIndex > loss.eventIndex && fact.route === routeKey(operation.method, operation.path) && fact.status === 200), `response loss lacks subsequent successful recovery evidence: ${loss.fault.id}: ${identity}`);
      }
    }
    for (const operation of manifest.operations.filter(item => item.profile === profileName)) {
      for (const scenario of scenariosOf(operation)) {
        assert(passed.has(scenario), `result missing scenario: ${profileName}: ${scenario}`);
        assert(observed.some(fact => fact.route === routeKey(operation.method, operation.path) && fact.scenario === scenario), `mapped operation lacks HTTP evidence in its scenario: ${operation.id}: ${scenario}`);
      }
    }
    if (profileName) {
      results.set(profileName, passed);
      requestFacts.push(...observed);
      resultRuns++;
    }
  } else if (!argument.startsWith("--manifest=")) {
    errors.push(`unknown argument: ${argument}`);
  }
}
assert(executionTargets.size <= 1, "mixed implementation/database results cannot form one baseline");

if (errors.length) {
  console.error(errors.join("\n"));
  process.exit(1);
}
const implemented = manifest.operations.filter(operation => manifest.profiles[operation.profile].status === "REQUIRED");
const executed = implemented.filter(operation => results.get(operation.profile) && scenariosOf(operation).every(scenario => results.get(operation.profile)!.has(scenario)));
const observedRoutes = [...new Set(requestFacts.map(fact => fact.route))].sort();
console.log(JSON.stringify({
  catalog_total: operations.size,
  unique_method_path_total: catalogRoutes.size,
  direct_canvas_routes: manifest.coverage_policy.direct_canvas_routes,
  operation_implemented: implemented.length,
  operation_executed: results.size ? executed.length : null,
  scenario_implemented: mappedScenarios.size,
  scenario_executed: results.size ? new Set([...results.values()].flatMap(passed => [...passed])).size : null,
  result_runs: resultRuns,
  execution_target: executionTargets.size ? [...executionTargets][0] : null,
  method_paths_mapped_executed: results.size ? new Set(executed.map(operation => routeKey(operation.method, operation.path))).size : null,
  method_paths_exercised: results.size ? observedRoutes.length : null,
  method_paths_successful: results.size ? new Set(requestFacts.filter(fact => fact.status < 400).map(fact => fact.route)).size : null,
  requests_observed: results.size ? requestFacts.length : null,
  incomplete_requests: results.size ? incompleteRequests : null,
  unclassified_incomplete_requests: results.size ? unclassifiedIncompleteRequests : null,
  controlled_fault_requests: results.size ? controlledFaultRequests : null,
  controlled_response_loss_requests: results.size ? controlledResponseLossRequests : null,
  wire_schema_version: wireRegistry.version,
  wire_schema_bound_routes: wireBindings.size,
  wire_validated_requests: results.size ? wireValidatedRequests : null,
  observed_routes: results.size ? observedRoutes.map(route => ({ route, statuses: [...new Set(requestFacts.filter(fact => fact.route === route).map(fact => fact.status))].sort((first, second) => first - second) })) : null,
  explicitly_partial_operations: implemented.filter(operation => operation.implementation?.startsWith("partial-")).map(operation => operation.id),
  deferred: manifest.operations.filter(operation => manifest.profiles[operation.profile].status === "DEFERRED").map(operation => operation.id),
  out_of_scope: manifest.operations.filter(operation => manifest.profiles[operation.profile].status === "OUT_OF_SCOPE").map(operation => operation.id),
  remaining_paths: Object.fromEntries(Object.entries(manifest.profiles).filter(([, profile]) => profile.status === "REQUIRED").map(([name, profile]) => [name, profile.remaining_paths])),
  execution_basis: results.size ? "passed scenarios plus redacted HTTP request facts; setup/negative requests count as route touches, not branch/state-flow completeness" : "static mapping only; no runtime coverage claimed",
  contract_approval: "pending",
  behavior_decision_total: decisionIDs.size,
  unresolved_behavior_decisions: manifest.behavior_decisions.filter(decision => decision.approval !== "approved").map(decision => decision.id),
}, null, 2));
