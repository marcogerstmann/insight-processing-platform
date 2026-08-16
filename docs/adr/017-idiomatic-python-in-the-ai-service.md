---
id: ADR-017
title: Idiomatic Python in the AI Service
status: Accepted
date: 2026-08-16
related: [ADR-005, ADR-006, ADR-009]
---

# ADR-017: Idiomatic Python in the AI Service

## Decision

`services/ai/` keeps the same hexagonal **boundaries** as `internal/` — domain, ports, application,
adapters — but expresses them at Python granularity, not Go's:

| Go idiom in `internal/`                                              | Python equivalent in `services/ai/`     |
| ---------------------------------------------------------------------| ---------------------------------------- |
| `interface` per file, one package (`internal/ports/*.go`)            | `typing.Protocol`, collected in one `ports.py` |
| `Service` interface + `service` struct + `var _ Service = ...`       | one class or function; no interface for a single implementation |
| `(Result, error)` returns                                            | exceptions (`PermanentError` and friends) |
| `struct` + constructor function                                      | `@dataclass(frozen=True)` |
| one type per file, several small packages                            | modules, not packages, wherever a package would hold one file |
| manual DI in `main.go` ([ADR-006](006-manual-di-and-paired-entrypoints.md)) | a composition root wires dependencies directly — same principle, idiomatic in both |

## Context

The Go service's layering is a load-bearing part of this repo's portfolio argument — see
[ADR-005](005-hexagonal-architecture.md). Adding a second, Python-based service creates a specific risk in
both directions:

- Copy Go's file-per-interface, package-per-concept structure literally, and Python ends up with five
  directories holding one nearly-empty file each: correct in spirit, but a shape no Python reviewer would
  choose and no Python codebase actually has. `internal/ports/*.go` is nine files because Go interfaces
  don't cost anything to declare per-file; `typing.Protocol` doesn't have that constraint, so paying for it
  anyway is cosplay, not architecture.
- Go the other way — flat modules, no ports, adapters imported straight into use cases — and the "hexagonal
  architecture" claim this repo makes only holds for half of it.

The instruction that resolved it: keep the **boundary**, drop the **ceremony**. Same direction of
dependency, native expression of it.

## Rationale

**`Protocol`, not `abc.ABC`, is a correctness point.** Go's `interface` is satisfied structurally —
`internal/adapters/outbound/readwise` never imports `internal/ports`. `typing.Protocol` gives Python the
same property: `SsmSecretProvider` implements `ports.SecretProvider`'s shape without importing it.
`abc.ABC` would require the adapter to inherit from a base class declared in `ports`, which means importing
it — inverting the exact dependency direction ports-and-adapters exists to protect. An `ABC`-based port
would compile (run) but would no longer be a hexagonal boundary, just a shared base class.

**Exceptions replace `(Result, error)`.** Go returns errors because Go has no other cheap way to signal
failure without exceptions-as-control-flow being idiomatic. Python's idiomatic failure signal *is* an
exception. `errors.py` carries `PermanentError`, mirroring `internal/apperr`'s taxonomy from
[ADR-009](009-error-taxonomy-and-dlq-routing.md): a permanent failure is caught explicitly by an inbound
handler and routed to a DLQ; everything else propagates and the runtime redelivers. `except PermanentError`
is the direct translation of `errors.As(err, &apperr.PermanentError{})` — same routing decision, no tuple
threading it through every call site.

**Ruff's `banned-api` is the compiler Python doesn't have.** `internal/` gets its import direction enforced
for free — the Go compiler refuses to build a cyclic or unauthorized import once packages are split that
way. Python has no build-time package-privacy concept equivalent to `internal/`, so the same guarantee has
to be opted into: `lint.flake8-tidy-imports.banned-api` bans importing `ipp_ai.adapters` anywhere, and
`per-file-ignores` re-permits it only in the composition root (`__main__.py`, and the event handler in
`adapters/inbound/event_subscription.py` since IPP-95) that is allowed to construct adapters and wire them
into the application layer. This moves the
dependency-direction check from code review — where it can be missed — to `ruff check`, which is what
`make ai-lint` runs and CI will gate on (IPP-96).

**One-package-per-type doesn't survive the port.** Go's `internal/domain` is eleven files because Go
convention favors small files. A Python package with the same one-concept-per-file granularity would be
mostly `__init__.py` boilerplate around a five-line class. `services/ai/src/ipp_ai/domain/` grows a module
per concept as those concepts actually appear (starting with `secret.py`), not preemptively.

## Consequences

- `services/ai/adapters/inbound/` stayed empty through IPP-92 and IPP-93 — there was nothing inbound to put
  there, and creating it anyway would have been exactly the empty-package-for-shape's-sake this ADR argues
  against elsewhere. IPP-95 is what finally gives it a file: `event_subscription.py`, the EventBridge
  subscription handler.
- `ports.py` and `errors.py` are single modules, not packages — deliberately, per the table above. Adding a
  tenth port does not spawn a tenth file; it grows `ports.py`. Revisit only if the file becomes hard to scan
  in one screen, the same bar [ADR-005](005-hexagonal-architecture.md) sets for the Go `internal/ports`
  package.
- The layering guarantee lives in `pyproject.toml`, not in file-system privacy. Anyone can technically
  `import ipp_ai.adapters` from `domain/` and Python will not stop them at import time — `ruff check` will,
  and CI enforces that ([IPP-96](https://marcogerstmann.atlassian.net/browse/IPP-96)) the same way
  `golangci-lint` gates the Go side.
- Two services now each answer "why does the code look the way it does" with their own ADR read together:
  this one for *how* the Python service expresses the shared architecture, and a future ADR-018
  ([IPP-116](https://marcogerstmann.atlassian.net/browse/IPP-116)) for *why* there are two services and two
  languages at all.
