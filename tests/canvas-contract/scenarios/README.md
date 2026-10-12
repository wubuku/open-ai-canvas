# Canvas contract scenarios

The executable scenario definitions currently live beside the Go tests so the
runner can keep setup, request sequences, and assertions in one reviewable
place. Every subtest name is a stable scenario ID; the manifest remains the
coverage index and is the place to record deferred profiles or approved
contract changes.

The executable scenario definitions currently cover these profiles:

Core document:

- `canvas.authz.unauthenticated`
- `canvas.create.read`
- `canvas.list`
- `canvas.list.pagination`
- `canvas.write.invalid-input`
- `canvas.conditional-read`
- `canvas.update.cas`
- `canvas.concurrent-cas`
- `canvas.update.repair-missing-resources`
- `canvas.authz.foreign-user`
- `canvas.authz.foreign-delete`
- `canvas.history.lifecycle`
- `canvas.history.retention-and-forced-preimage`
- `canvas.compression`
- `canvas.delete.lifecycle`
- `canvas.authz.logout`

Share/public:

- `canvas.share.lifecycle`
- `canvas.share.authz`
- `canvas.share.invalid-token`
- `canvas.share.invalid-input-and-replay`
- `canvas.share.resource-allowlist-and-live-document`
- `canvas.share.concurrent-rotate-and-revoke`
- `canvas.share.concurrent-first-create`
- `canvas.share.resource-description-and-revocation`
- `canvas.share.expiry-and-recovery`
- `canvas.share.resource-object-storage-failure`

Project integration:

- `project.canvas.foreign`
- `project.canvas.validation`
- `project.canvas.feature-gate`
- `project.canvas.lifecycle`
- `project.canvas.cross-project-and-reassociation`
- `project.canvas.multi-unit-roles-and-replay`
- `project.canvas.role-overflow-partial-state`
- `project.canvas.concurrent-link-settlement`
- `project.canvas.concurrent-cross-project-reassociation`

Project/workbench core:

- `project.workbench.lifecycle`
- `project.workbench.feature-gate`
- `project.workbench.authz`
- `project.workbench.invalid-mutations`
- `project.workbench.delete`
- `project.workbench.duplicate-creation-and-replay`
- `project.workbench.cross-project-units`
- `project.workbench.concurrent-reorder`
- `project.workbench.deletion-cascade`
- `project.workbench.bulk-import-boundaries`
- `project.workbench.import-body-budget-and-framing`
- `project.workbench.concurrent-opposite-order`
- `project.workbench.reorder-versus-create`
- `project.workbench.reorder-versus-delete`
- `project.workbench.reorder-versus-import`

Resource integrity:

- `canvas.resource.unauthenticated`
- `canvas.resource.metadata-and-access`
- `canvas.resource.foreign-access`
- `canvas.resource.import.invalid-input`
- `canvas.resource.import.success`
- `canvas.resource.import.replay`
- `canvas.resource.import.failure-and-ssrf`
- `canvas.resource.reference-validation`
- `canvas.resource.chunked-upload`
- `canvas.resource.chunk-validation`
- `canvas.resource.multichunk-replay`
- `canvas.resource.limits.single-file`
- `canvas.resource.limits.daily-replay-and-concurrency`
- `canvas.resource.limits.rate-and-scope`
- `canvas.resource.limits.stored-reservations`
- `canvas.resource.limits.session-slots`
- `canvas.resource.delivery.real-expiry-and-conditional-cache`
- `canvas.resource.limits.concurrent-slot-admission`
- `canvas.resource.upload.concurrent-chunks-and-completion`
- `canvas.resource.import.slow-timeout`
- `canvas.resource.file.access-description`
- `canvas.resource.upload.chunk-versus-completion`
- `canvas.resource.upload.invalid-overwrite-recovery`
- `canvas.resource.upload.interrupted-chunk-recovery`
- `canvas.resource.object-storage-http-origin`

Import:

- `canvas.import.invalid-input`
- `canvas.import.scope-and-feature-gate`
- `canvas.import.success-and-replay`
- `canvas.import.media-topology`
- `canvas.import.conversion-limits`
- `canvas.import.resource-save-and-isolation`
- `canvas.import.upstream-failures`
- `canvas.import.upstream-timeout`

Legacy assets:

- `canvas.legacy-asset.unauthenticated`
- `canvas.legacy-asset.foreign-write-matrix`
- `canvas.legacy-asset.mixed-owner-rollback`
- `canvas.legacy-asset.folder-lifecycle`
- `canvas.legacy-asset.read-update-batch`
- `canvas.legacy-user-data.snapshot`
- `canvas.legacy-asset.delete`

Restart recovery (private owned-process fixture required):

- `canvas.recovery.document-and-history`
- `canvas.recovery.share-and-resource`
- `canvas.recovery.project-links`
- `canvas.recovery.upload-session-loss`
- `canvas.recovery.deleted-state`
- `canvas.recovery.graceful-and-write`
- `canvas.recovery.response-loss-and-restart`

Public resource:

- `canvas.public-resource.delivery`
- `canvas.public-resource.named-file`
- `canvas.public-resource.range`
- `canvas.public-resource.invalid-signature`
- `canvas.public-resource.missing-and-expired-signature`
- `canvas.public-resource.resource-binding`
- `canvas.public-resource.access-description`
- `canvas.public-resource.remote-origin-http-failure`

Generation/Agent read boundary:

- `generation.boundary.unauthenticated`
- `generation.boundary.owner-read-only`
- `generation.boundary.cross-user-creation-read`

The current Go baseline records `canvas.authz.foreign-delete` as HTTP 200 with
no deletion effect: the owner can still read the canvas at the same revision.
This is a `CURRENT-FACT-REVIEW-REQUIRED` compatibility fact, not an approved
Java design rule. Change the manifest and scenario together if the target
contract is deliberately tightened.

Foreign project canvas listing and foreign share POST/DELETE currently return
HTTP 500/internal; foreign `GET /api/projects/:id/units` does too, while foreign
`DELETE /api/projects/:id` returns 404. They are pending contract decisions, not
approved Java behavior. The repair scenario exercises a valid preimage, strict resource
validation, owner/CAS rejection, and exactly one before_resource_repair history
entry. A damaged preimage is exercised by the companion resource-integrity
profile through the explicit Go purge route. Resource validation checks missing
and foreign references, rejected asset replacement, purge-induced dangling
references, repair history, and rejected restore of now-missing media. It does
not claim purge refuses references, inspect asynchronous physical deletion, or
inject object-store failures. KEEP/OVERTURN of purge remains a migration decision.

The `import` profile has an isolated fixed-origin TLS fixture for success,
scope/feature rejection, HTTP/business failures, invalid/truncated/oversized JSON,
redirect denial, real upstream timeout, partial conversion and fresh-identity replay.
Full media/topology conversion and downstream resource-import/save flows remain
unexecuted; its conversion response does not persist a canvas or resource.
The standalone public-resource
profile now exercises local signed delivery, named files, Range, signature
failure/expiry, resource binding, replay, cache headers, and a local
S3-compatible origin's success/failure/recovery path. The share-resource
profile also exercises its S3 origin success/failure/recovery path; remote CDN
delivery, remote TTL-boundary and conditional-cache checks remain explicit
remaining paths. Fifteen project/unit operations have an executable
workbench core profile, not the entire workbench. Remaining workflow/shot/asset
and character/voice families are deferred. The generation boundary only proves
authenticated read isolation and safe missing-ID handling; task mutation,
external generation, writeback, Agent events/approvals, and memory/profile writes
remain separate gates.
