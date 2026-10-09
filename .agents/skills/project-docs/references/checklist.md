# Documentation Quality Checklist

## Open AI Canvas Adaptation

- [ ] Uncertain or actively changing research is under `docs/drafts/`.
- [ ] Confirmed findings use focused, low-conflict edits in the appropriate formal document.
- [ ] Existing `AGENTS.md`, `README.md`, `docs/index.md`, and published MDX were changed only for a clear navigation, correctness, or maintenance reason.
- [ ] The document distinguishes implemented behavior, documented intent, and pending validation.
- [ ] Claims cite current repository paths, commands, tests, or configuration.
- [ ] The docs build surface was checked before documenting a build command.
- [ ] Upstream merge-conflict risk is called out for changes outside `docs/drafts/`, with additive edits preferred over rewrites.

## Pre-Creation Checklist

Before starting documentation:

- [ ] Assess current documentation state
- [ ] Identify documentation gaps
- [ ] Create DOCUMENTATION_PLAN.md with priorities
- [ ] Order documents by P0 → P3
- [ ] Allocate time estimate per document

## README.md Checklist

- [ ] One-line description with keywords
- [ ] Features list (5-8 items max)
- [ ] Quick start in ≤5 steps
- [ ] Architecture diagram or link
- [ ] Comparison table vs alternatives
- [ ] Documentation navigation links
- [ ] License badge
- [ ] ≤200 lines total
- [ ] Tested commands work
- [ ] Bilingual header added (if applicable)

## AGENTS.md Checklist

- [ ] Project overview (type, tech stack, status)
- [ ] Documentation index with relative paths
- [ ] Quick commands listed
- [ ] Module overview table
- [ ] Agent constraints (DO NOT modify, ALWAYS verify, BEFORE changing)
- [ ] Maintenance rules
- [ ] ≤120 lines total
- [ ] All P0/P1 docs indexed
- [ ] Links use relative paths

## MEMORY.md Checklist

- [ ] Core workflow (DDDML-first if applicable)
- [ ] Key commands reference
- [ ] Important directories listed
- [ ] Service and port info
- [ ] Authentication and tokens
- [ ] High-frequency cautions
- [ ] Quick reference for daily work

## Cursor Rules Checklist (.mdc)

- [ ] YAML front matter complete (name, description, applied_to)
- [ ] Rule content clear and actionable
- [ ] Examples provided
- [ ] Numbered naming convention followed (NNN_name.mdc)

## Navigation Hub Checklist

- [ ] `docs/index.md` explains the repository documentation layers
- [ ] `docs/drafts/README.md` explains research staging and promotion
- [ ] Quick navigation links resolve
- [ ] The index distinguishes published docs, design records, plans, and drafts
- [ ] No generic `docs/README.md` is created when the repository already uses `docs/index.md`

## Architecture.md Checklist

- [ ] Design principles (3-5 items)
- [ ] System overview diagram
- [ ] Module dependency structure
- [ ] Core patterns explained
- [ ] Data flow diagram
- [ ] Database schema description
- [ ] Key design decisions table
- [ ] Configuration reference
- [ ] 200-400 lines
- [ ] Code examples compile

## Development Guide Checklist

- [ ] Clear prerequisites listed
- [ ] Step-by-step setup instructions
- [ ] All commands tested
- [ ] Common issues addressed
- [ ] Related documents linked (not duplicated)
- [ ] 200-400 lines

## API Testing Checklist

- [ ] Base URL documented
- [ ] Authentication method explained
- [ ] Error response format
- [ ] Each endpoint documented:
  - [ ] HTTP method and path
  - [ ] Description
  - [ ] Request body schema
  - [ ] Response schema
  - [ ] Example request/response
  - [ ] Error codes
- [ ] Pagination format
- [ ] 300-600 lines
- [ ] Examples verified

## Troubleshooting Guide Checklist

- [ ] Organized by symptom (not component)
- [ ] Startup issues section
- [ ] Retrieval issues section (if applicable)
- [ ] API issues section
- [ ] Performance issues section
- [ ] Each problem has:
  - [ ] Symptoms description
  - [ ] Diagnosis steps
  - [ ] Solution steps
- [ ] Command examples
- [ ] Log output interpretation
- [ ] 200-400 lines
- [ ] Solutions verified

## Archive Checklist

- [ ] File renamed with date prefix (YYYY-MM-DD_)
- [ ] Copied to docs/archive/
- [ ] Archive index updated
- [ ] Original references updated
- [ ] Original file removed

## Draft Document Checklist

- [ ] Named with descriptive topic (not TASK_PROGRESS, NOTES, TEMP)
- [ ] Placed in docs/drafts/
- [ ] Content is actionable (not just notes)
- [ ] Purpose is clear

## Bilingual Checklist (if applicable)

For each main document:

- [ ] English version created (`.md`)
- [ ] Chinese version created (`-zh-CN.md`)
- [ ] Both have navigation header:
  ```markdown
  > 📖 English | 📖 中文
  ```
- [ ] Chinese version accurately translated
- [ ] Same structure in both versions
- [ ] Cross-links between versions work

## File Naming Checklist

- [ ] Uses date prefix for historical files (YYYY-MM-DD_)
- [ ] No spaces in filenames (use - or _)
- [ ] Descriptive names (not generic like "notes.md")
- [ ] Consistent with project conventions

## Link Validation Checklist

- [ ] All relative paths work
- [ ] No dead links to moved documents
- [ ] External links are valid
- [ ] Cross-references between documents work

## Final Validation

Before declaring documentation complete:

- [ ] All code examples compile/run
- [ ] All commands tested on clean environment
- [ ] Links between documents work
- [ ] No broken images or references
- [ ] Consistent formatting throughout
- [ ] Peer review obtained
- [ ] Commits follow convention
