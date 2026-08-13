# RFC 9457 Problem Details — Adoption Research for Mill

**Date:** 2026-08-13  
**Author:** Architect (via research deliverable for Issue #149)  
**Audience:** CTO, Staff, Tech Lead, Sr Dev  
**Read time:** ~10 minutes

---

## 0. Scope and method

This is research only. No code is changed. The question under investigation:
does adopting [RFC 9457 — Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457)
as the shape of Mill's error and diagnostic reporting pay for itself, given
that Mill is not an HTTP service?

Primary sources consulted:

- **RFC 9457** itself, fetched in full from the RFC Editor
  (`https://www.rfc-editor.org/rfc/rfc9457.txt`). This is the current
  revision; it obsoletes RFC 7807 and is the normative reference.
- **RFC 7807** (fetched from `https://www.rfc-editor.org/rfc/rfc7807.txt`) for
  historical context and the `about:blank` default, which 9457 preserves.
- **The Mill source tree** at the `agent/149` worktree, audited at
  `internal/domain/classification.go`, `internal/domain/signals.go`,
  `internal/ledger/ledger.go`, `internal/cli/delegate.go`,
  `internal/cli/review_loop.go`, `internal/cli/routing_56.go`,
  `internal/adapter/commandcode.go`, and `docs/PRODUCT.md`.

The related issues (#105, #110, #139, #143) are GitHub issues and are
reconstructed here from their traces in the source and in
`.mill/roles/staff/lessons.md`, since their bodies are not in the repo.

---

## 1. Mill's current failure vocabulary

Mill does not emit a single structured shape today. It has two parallel,
inconsistent classification paths and a ledger that discards the evidence
that justified each verdict.

### 1.1 The two classification paths

The enum `Classification` (`internal/domain/classification.go:7-21`) names
fine-grained outcomes: `OK`, `CHANGES_REQUESTED`, `FATAL`, `MAX_TURNS`,
`AUTH`, `NO_CREDIT`, `RATE_LIMITED`, `TRANSIENT`, `BLOCKED`.

The enum `FailureClass` (`classification.go:27-38`) groups those into coarse
buckets: `CLASS_OK`, `EXECUTION_FAILURE`, `CONTRACT_FAILURE`,
`GATE_FAILURE`, `RESULT_FAILURE`, `ENVIRONMENT_FAILURE`, `FATAL`.

These are wired by two *different* code paths that do not agree:

1. **`FailureClassOf(c Classification)`** (`classification.go:41-53`) maps a
   `Classification` to a `FailureClass` with a hand-written switch. Notice
   that `ClassificationAuth`, `ClassificationNoCredit`,
   `ClassificationRateLimited` and `ClassificationMaxTurns` have **no case**
   and fall through to the `default` arm (`EXECUTION_FAILURE`). The
   function exists but is effectively dead in the hot path.

2. **`classifyFailure(result)** (`internal/cli/delegate.go:836-838`) wraps
   `domain.NewSignalRegistry().Resolve(result)` (`internal/domain/signals.go:46-53`),
   which maps a `SessionResult` (exit code + stderr + output + heartbeat
   staleness + env error) to a `FailureClass` via a priority-ordered table of
   predicate `Signal`s (`signals.go:63-147`). This is the live path.

> The issue brief refers to `classifyResult`. The symbol in the tree is
> `classifyFailure` (`delegate.go:836`). There is no `classifyResult`.
> This matters: the two paths already disagree on what a "classification
> result" is, which is itself the #110 problem.

### 1.2 What the signal table actually classifies

`defaultSignals()` (`signals.go:63-147`) returns eight ordered predicates.
Each maps to one of six non-`CLASS_OK` buckets:

| # | Predicate (stderr / exit-code / output) | FailureClass |
|---|---|---|
| 1 | stderr `connection refused` / `network timeout` | EXECUTION_FAILURE |
| 2 | exit 1 + stderr `gate-frd`/`gate-spec`/`gate-tasks` | GATE_FAILURE |
| 3 | stderr `changes_requested:` + `criterion` | RESULT_FAILURE |
| 4 | exit -1/-2 + stderr `blocked:` | EXECUTION_FAILURE |
| 5 | exit 4/9/130/137/143 (killed) | EXECUTION_FAILURE |
| 6 | exit 0 + whitespace/TODO/TBD/placeholder output | CONTRACT_FAILURE |
| 7 | heartbeat stale while process active | EXECUTION_FAILURE |
| 8 | `EnvError != nil` | ENVIRONMENT_FAILURE |

Observation: signal #4 collapses every `blocked:` into `EXECUTION_FAILURE`.
In Mill, "blocked" carries two very different meanings — a model hitting a
wall-clock budget, and an agent hitting an underspecified task (#139). Both
currently land in the same bucket, then both trigger the same
`retryDispatch` model-chain fallback (`delegate.go:421-473`) that is
useless for an underspecified task. #139 and #139's "defect repeatability"
both want those distinguished.

### 1.3 What the ledger keeps and discards

`ledger.Entry` (`internal/ledger/ledger.go:16-40`) is JSONL, append-only,
one file per issue at `.mill/ledger/<issue>.jsonl`. Its shape:

```
Timestamp, Issue, Event, Status, Verdict, Classification(string),
Round, File, Version, AgentID, FailureClass, Phase, Role,
ParentIssue, Depth
```

**The discarded field is the room.** The produce-phase entry
(`internal/cli/review_loop.go:118-131`) records `Classification:
string(finalClass)` and `FailureClass: finalClass`, but it does **not**
write the `SessionResult` that produced `finalClass` — not the exit code,
not the stderr, not the model that ran, not the round's diff. The
review-phase entry (`review_loop.go:202-212`) is the same. The escalation
entries (`:301-314`, `:324-338`) and the final `complete` entry
(`:377-388`) likewise record a `FailureClass` string and nothing more.
`recordError` (`delegate.go:382-393`) is the starkest case: it appends an
entry with `Event`, `Status`, and `Timestamp` only — no `FailureClass`,
no `Classification`, no cause at all.

This is the #105/#143 gap in concrete form. `docs/PRODUCT.md:127-141`
states the requirement directly: a delegation must make visible "what the
verdict rests on." Today the ledger makes visible only the verdict's
*label*.

### 1.4 How failures surface to humans today

There are two egress points, both lossy:

- **CLI stderr.** `App.Err` (`internal/cli/app.go:23-24`, defaults to
  `os.Stderr`) receives `fmt.Fprintf(a.Err, ...)` lines with prefixes like
  `ESCALATION:` (`review_loop.go:297,347`), `escalation:`
  (`routing_56.go:352,403,411`), `review: build gate FAILED`
  (`review_loop.go:166`), and `delegate: adapter capabilities`
  (`delegate.go:139`). These are free-text, prefix-scanned lines — not a
  contract. The `blocked:` escalation (#139) currently emits
  `"ESCALATION: Review cycle exhausted for issue %d\n"` plus the raw
  `reviewFeedbacks` (`review_loop.go:346-354`), then hands the worktree to
  `escalateToParent` (`routing_56.go:345-415`) which itself only prints to
  stderr and — at the Staff hard-stop — logs `"notifying CTO"` to stderr
  (`:352`) without ever posting a GitHub issue comment. There is **no
  `gh issue comment` code path** anywhere in the tree; the only `gh`
  operations are `issue view` (`internal/issue/reader.go:14-45`) and
  `issue edit --add-label` (`reader.go:47-66`). The README's stated
  "comment on the GitHub issue describing the blocker" workflow
  (`README.md:106-112`, `.mill/roles/staff/ROLE.md:186-188`) is *specified*
  but *not implemented*.

- **Issue comments.** Not implemented. See above.

---

## 2. RFC 9457 at a glance (and what 9457 changed from 7807)

Per RFC 9457 §3.1, a problem details object has at most these members:

| Member | Type | Role per the RFC |
|---|---|---|
| `type` | string (URI reference) | **primary** identifier of the problem *kind*; consumers MUST use it as such. |
| `title` | string | short human-readable summary of the kind; stable across occurrences. |
| `status` | number | the HTTP status code for this occurrence; **advisory only**. |
| `detail` | string | human-readable explanation **specific to this occurrence**. |
| `instance` | string (URI reference) | identifies **this specific occurrence**. |

Plus arbitrary **extension members** (§3.2), which consumers MUST ignore if
unrecognized.

Three things 9457 changed from 7807 that matter here:

- **§4.2** introduced the *HTTP Problem Types* registry and
  **Specification Required** registration policy. 7807 had no registry.
- **§3.1.1** now *explicitly permits non-resolvable type URIs* and warns that
  choosing a non-resolvable URI (e.g. a `tag:` URI) is a one-way door:
  switching to a resolvable URI later "would require ... switching to a
  resolvable URI, creating a new identity for the problem type and thus
  introducing a breaking change."
- **§3** clarifies how to handle *multiple* problems (represent the most
  relevant/urgent one; do not batch disparate types).

The model's defining invariants, unchanged between 7807 and 9457:
- `type` (kind) is stable and separate from `detail`/`instance` (occurrence).
- `status` is explicitly **advisory** ("it conveys the HTTP status code used
  for the convenience of the consumer" — 9457 §3.1.2).
- Extensions are forward-compatible by ignore-on-unknown (9457 §3.2).

---

## 3. Advantages — what Mill gains

Each item names the issue it closes and the code path it would touch, not
vague "better observability."

### 3.1 It separates kind from occurrence. This is the #105/#143 fix.

Today `classification.go` stores a `FailureClass` string in the ledger and
throws away the `SessionResult` that produced it. RFC 9457's `type` (kind)
vs `detail`+extensions (this occurrence) is exactly that separation, made
explicit and named rather than ad hoc. Concretely: a `GATE_FAILURE` entry
would carry `type: "urn:problem:mill:gate-failure"`, `title: "Gate failed"`,
`detail: "gate-build: go build ./... failed (exit 2):\n<build output>"`
(replacing the stderr line at `review_loop.go:164-165`), and extensions
`exit_code`, `phase: "produce"`, `role`, `round`, `model`, and a pointer
to `instance`. The ledger entry would stop being a label with no basis.

This is the direct remedy for `PRODUCT.md:140`: "A green verdict whose basis
cannot be inspected is worse than a red one." Currently that basis is
deleted at `review_loop.go:118-131` / `:202-212`.

### 3.2 It gives `type` a stable, machine-primary identity. This is the #110 fix.

#110 is that `FailureClass` is "an enum that each of five call sites
interprets differently." Evidence: `FailureClassOf` (`classification.go:41`)
and `classifyFailure` (`delegate.go:836`) are *two* mappings from inputs to
the same `FailureClass` enum, neither a superset of the other, neither
tested against the other. The signal predicates in `signals.go` each hardcode
a `FailureClass` literal inline, so adding a new failure kind means editing N
sites with no shared registry.

RFC 9457 solves this by making `type` the primary identifier and saying
consumers MUST use it (9457 §3.1). If each `Signal` carried a `ProblemType`
URI, the signal table would be the single source of truth for "what kind of
failure this is," replacing the free-floating `FailureClass` literals.
That is strictly stronger than an enum the call sites re-interpret.

### 3.3 It gives occurrences an identity. This is the #105 observability enabler.

RFC 9457's `instance` identifies "this specific occurrence of the problem"
(`instance`, §3.1.5) — "the concept 'the time Joe didn't have enough
credit last Thursday.'" Mill has no such cross-cutting identity today.
A ledger entry is keyed by `(timestamp, issue, round)`; a worktree is keyed
by `agent/<issue>`; an escalation is a stderr line. Three representations of
the same failure, no shared handle.

An `instance` URI (e.g. `urn:uuid:<x>` or
`urn:mill:ledger:<issue>:<entry-offset>`) would let the ledger entry, the
worktree's `.mill/heartbeat`, an issue comment, and a CLI stderr message
all point at the same occurrence. That is what makes "what it rests on"
inspectable from any of those surfaces — the #105 requirement.

### 3.4 It gives "raising a hand" a structured shape. This is the #139 fix.

#139 has two faces: (a) an agent that stops because work is underspecified
must state *precisely what is missing*, in a form an observer can act on;
(b) a stable `type` makes "this role produced this class of defect before"
countable.

RFC 9457 gives (a) the `detail` field (this occurrence's specific complaint)
plus extensions for the structured bits (e.g. `missing: ["criterion: ..."]`,
`phase`, `role`). It gives (b) the `type` URI as a countable key. Today the
`blocked:` signal (`signals.go:87-105`) fires both "budget exceeded" and
"analysis paralysis" and "underspecified" into one `EXECUTION_FAILURE`
bucket, then `retryDispatch` retries with the next model — which is correct
for a rate limit and wrong for an underspecified task. A dedicated
`type: "urn:problem:mill:underspecified"` would let the reactor stop
retrying and escalate with the *criterion* that was missing.

### 3.5 It is forward-compatible by construction.

9457 §3.2: "Clients consuming problem details MUST ignore any such
extensions that they don't recognize." Mill's ledger is append-only JSONL,
parsed back by `ledger.ReadEntries` (`ledger.go:67-99`) with strict
`json.Unmarshal`. Adding evidence fields to `Entry` today is a schema change
that every reader must already absorb. The problem-details extension
contract de-risks exactly that: new fields are tolerated, not breaking.

### 3.6 It reuses a vocabulary the surrounding ecosystem already knows.

`application/problem+json` and the six-member shape are widely recognized
(OpenAPI models errors in this shape; Go's `net/http` has no native
support but the pattern is idiomatic). An operations engineer reading a
`status`/`type`/`detail`/`instance` document knows it without a Mill
manual. The alternative — a Mill-specific error JSON with ad-hoc keys —
requires every consumer to learn a private schema.

---

## 4. Disadvantages — the honest case against

### 4.1 `status` has no home here. (The deepest mismatch.)

`status` is defined by the RFC as "the HTTP status code ([HTTP], §15)
generated by the origin server for this occurrence of the problem"
(9457 §3.1.2). Mill has no HTTP origin server; its exit points are a CLI
exit code and a JSONL ledger. Two ways to fill `status`: (a) omit it — but
then Mill discards the coarse `FailureClass` bucket, which is the one piece
of structure it currently has; or (b) *map* `FailureClass` onto fake HTTP
codes (e.g. `RESULT_FAILURE` → 422, `EXECUTION_FAILURE` → 500). The RFC
itself warns against this: `status` "duplicates the information available in
the HTTP status code itself, bringing the possibility of disagreement" and
generic software "is unlikely to know of or respect the status code
conveyed in this member" (9457 §5, §3.1.2). Inventing HTTP codes for a
non-HTTP program reintroduces exactly the "five call sites interpret it
differently" problem #110 complains about — now for `status` instead of
`FailureClass`.

### 4.2 `type` as a URI invites the #99 rot, and the RFC leans on it.

The RFC *encourages* dereferenceable type URIs: "If the type URI is a
locator ... dereferencing it SHOULD provide human-readable documentation
for the problem type" (9457 §3.1). The precedent in this codebase for
"an authoritative list held in Go source that the provider can change out
from under us" is `Capabilities().Models`:

```
// internal/adapter/commandcode.go:23-33
func (a *CommandCodeAdapter) Capabilities() Capabilities {
    return Capabilities{
        Models: []string{
            "claude-sonnet-5", "claude-sonnet-4-6", "claude-fable-5",
            "claude-opus-5", "claude-haiku-4-5",
            "deepseek-v4-pro", "deepseek-v4-flash", "laguna-s-2.1-free",
        },
        ...
```

`validateModelChain` (`delegate.go:506-522`) rejects any model not in this
hardcoded slice, and staff lessons #17
(`.mill/roles/staff/lessons.md:313-327`) record the pattern explicitly:
"a structure that parsed correctly and changed nothing at dispatch because
it populated a different map than the resolver reads (#116)." A `type` URI
registry is the same shape — a list of identifiers held in source that
describes a surface owned by an external party. When the provider renames a
model or a failure mode, the URI list rots; and because the RFC frames
`type` as authoritative, dead links are more misleading than a plain enum.

The RFC does permit non-resolvable URIs (9457 §3.1.1: "The type URI is
allowed to be a non-resolvable URI"), and `tag:` / `urn:` schemes satisfy
"stable identifier without a doc link." But once you stop dereferencing
`type`, you have paid the URI syntax cost for exactly what a well-maintained
enum-plus-doc-table gives you cheaper. The advantage in §3.2 (stable primary
identifier) survives; the advantage in §3.1 (`type` dereferences to
docs) does not — and the RFC's own security guidance
(§5: "Generators providing links to occurrence information are encouraged to
avoid making implementation details ... available") cautions that
dereferenceable `instance` links can leak internals.

### 4.3 The model is shaped around the HTTP client/server error narrative.

RFC 9457 is designed for "the client asked for something and the server
rejected or errored on it." Mill's failures are not all request errors:
they include *process outcomes* (model budget exhausted, analysis
paralysis, heartbeat stale, cyclic delegation graph, gate build failed,
reviewer rejected). Several map awkwardly onto an
request-was-bad / server-errored axis. The `type`+`title`+`status`+`detail`+`instance`
vocabulary is general enough to carry them, but the *narrative gravity* of
the format pulls readers toward HTTP-status reasoning — and §4.2's own
registry (4.2) is named "HTTP Problem Types," reinforcing the coupling.

### 4.4 Media-type baggage with no HTTP transport.

`application/problem+json` is registered with IANA **for HTTP**
(9457 §6). Mill's failure surfaces are CLI stderr and JSONL ledger lines,
neither of which negotiates a content type. Emitting
`application/problem+json` objects to stderr or into a JSONL ledger is a
partial, non-standard interpretation: the RFC's Appendix C ("Using Problem
Details with Other Formats") anticipates *embedding the model* in other
formats, but explicitly stops short of endorsing the media type outside
HTTP. In practice this means Mill would adopt the *object shape* and
quietly drop the media type — which is fine, but it also means
interoperability tooling that selects on `Content-Type: application/problem+json`
won't recognize Mill's output. The reuse value in §3.6 is
conditional on that media type, which Mill cannot honestly set.

### 4.5 Retrofitting the signal table is discipline work that can itself drift.

Each `Signal` (`signals.go:21-25`) is currently `{Predicate, FailureClass,
Description}`. To emit problem details, every signal needs a `ProblemType`
URI, a `title`, and a set of extension fields derived from the
`SessionResult`. That is 8 signals × n fields, and nothing in the type system
enforces that a signal's `type` URI matches its `title` or that two signals
don't claim the same kind. The RFC gives no guard against internal
inconsistency — only against *consumer* forward-compatibility
(§3.2). The result could be a more elaborate version of the very
#110 problem it was meant to solve: a registry that rots in a different
dimension (kind identity instead of enum interpretation).

### 4.6 The ledger is append-only JSONL and the format is already "good enough" for machines.

The ledger is read back by one consumer (`ledger.ReadEntries`,
`ledger.go:67-99`) and is gitignored-state (`ARCHITECTURE.md:68`:
"`.mill/` is gitignored"). Its audience is Mill itself and the Staff agent
doing post-hoc inspection, not external HTTP clients. The marginal tooling
benefit in §3.6 is thin for a format with no HTTP consumers and no
content-type negotiation. The case for adopting the *shape* rests entirely on
internal consistency and evidence-retention, not on ecosystem reuse.

---

## 5. What adoption would touch

Per the issue brief, converted to concrete file:function:symbol references
in the tree.

### 5.1 Core domain (the shape definition)

- `internal/domain/classification.go` — `FailureClass` would gain a
  companion `ProblemType`/URI; `FailureClassOf` would either be deleted (it
  is the dead path) or made the canonical kind→type mapping.
- `internal/domain/signals.go` — `Signal` struct (`signals.go:21-25`) gains
  `ProblemType` URI + `Title`; `defaultSignals()` (`signals.go:63-147`) annotates
  each of the 8 predicates; `Resolve` (`signals.go:46-53`) returns a structured
  `Problem` rather than a bare `FailureClass`.
- `internal/domain/task.go` — `Task.FailureClass` (`:13`) becomes or wraps
  a `Problem`; `Task.Transition` (`:38-44`) accepts it.
- `internal/domain/session.go` — `Session.End` (`:44-56`) and the
  `SessionResult` would carry the resolved problem.

### 5.2 Persistence (retain the evidence — the #105/#143 win)

- `internal/ledger/ledger.go` — `Entry` (`ledger.go:16-40`) gains
  `Type`, `Title`, `Detail`, `Instance` plus extensions (`ExitCode`,
  `Stderr`, `Output`, `Model`, `Role`, `Round`). `Append`/`ReadEntries`
  must round-trip them. This is the highest-leverage touchpoint: it is
  literally where the evidence is currently deleted.
- `ledger.ReadEntries` malformed-line handling (`ledger.go:90-93`) must
  tolerate the new optional fields.

### 5.3 Classification and dispatch

- `internal/cli/delegate.go` — `classifyFailure` (`:836-838`) returns a
  `Problem`; `retryDispatch` (`:403-473`) consumes it to decide retry vs.
  escalate (the `blocked:` conflation in §1.2); `recordError`
  (`:382-393`) would write a real problem instead of an entry with no
  failure class at all.
- `internal/cli/routing_56.go` — `escalateToParent` (`:345-415`) would
  emit a problem document for the Staff hard-stop instead of the bare
  `"notifying CTO"` stderr line (`:352`).

### 5.4 The produce→review loop

- `internal/cli/review_loop.go` — every `ledger.Entry` literal (produce
  `:118-131`, review `:202-212`, rework `:243-254`, gate `:260-271`, reject
  `:278-290`, escalate `:301-314`, env `:324-338`, complete `:377-388`)
  gains the new fields. The `blocked:`/`changes_requested:` evidence that
  is today only echoed to stderr (`review_loop.go:346-354`) and fed back as
  `reworkFeedback` (`review_loop.go:234-235`) becomes structured.

### 5.5 CLI surface

- `internal/cli/app.go` / `delegate.go` — `fmt.Fprintf(a.Err,
  "ESCALATION: ...")` lines (`review_loop.go:297,347`,
  `routing_56.go:352,403,411`) could be replaced or augmented with
  structured stderr. Whether the CLI *prints* the document (vs keeping it
  machine-local) is a UX decision the adoption scope must pin down.

### 5.6 What is NOT worth converting (explicit out-of-scope list)

- **The `Verdict` enum** (`internal/domain/verdict.go:8-17`:
  `approved`/`changes`/`changes_requested`/`rejected`). This is a review
  *outcome*, not an error. Problem details describe failures; a review's
  "changes requested" is a deliverable, not a defect.
- **`TaskStatus` / `SessionStatus`** (`internal/domain/status.go:7-22`).
  Lifecycle state (pending/running/done/error/aborted) is not a diagnostic.
- **The `application/problem+json` media type itself.** Mill has no HTTP
  layer (`ARCHITECTURE.md:48-52`: the only adapter spawns a CLI process).
  Adopting the *object shape* is viable; adopting the *media type* is not.
- **Issue comments.** There is no `gh issue comment` implementation today
  (`internal/issue/reader.go` exposes only `ReadBody`/`AddLabel`). Posting
  problem documents as GitHub issue comments is a *separate* feature,
  not a consequence of the shape change. It should not be bundled into
  this adoption.

---

## 6. Alternatives considered

### 6.1 Keep the enums; add structured evidence fields to the ledger entry only

Add `ExitCode`, `Stderr`, `Output` directly to `ledger.Entry`
(`ledger.go:16-40`) with no `type`/`title`/`status`/`detail`/`instance`
vocabulary. This is the lowest-friction fix for #105/#143 (evidence is
retained) and touches only `ledger.go` + the eight `ledger.Entry` literals
in `review_loop.go`.

- **Gains:** evidence is no longer discarded; #110 is *not* addressed
  (still two divergent classification paths, still call-site-interpreted
  enums).
- **Cost:** trivial relative to full adoption.
- **Verdict:** a strict subset of the proposed adoption. If RFC 9457 is
  rejected as too heavy, this is the fallback that still closes the
  evidence gap.

### 6.2 JSON-RPC 2.0 error object

The shape `{code: integer, message: string, data: any}`
(JSON-RPC 2.0 Specification, `https://www.jsonrpc.org/specification`).

- **Gains:** compact, no URI requirement, `data` is an arbitrary extension
  bag (forward-compatible by convention).
- **Loss vs. 9457:** `code` is an opaque integer (less self-documenting than
  a `type` URI); no `instance`/`title`/`detail` split; no standardized
  notion of "this occurrence" vs "this kind." Machine-oriented, not
  observability-oriented — the wrong direction for #105.
- **Fit:** good for a process-to-process result channel; poor for a ledger
  humans read back.

### 6.3 LSP `Diagnostic`

`{code, severity, source, range, message}` (Language Server Protocol,
`https://microsoft.github.io/language-server-protocol/`).

- **Gains:** `code` + `message` is a kind/occurrence split; `source`
  attributes the producer.
- **Loss:** `range` (editor locations) and `source` do not map to Mill. The
  shape is editor-bound; forcing it onto process verdicts is impedance.

### 6.4 gRPC / `google.rpc.Status`

`{code: integer, message: string, details: []Any}`
( gRPC Status proto, `https://grpc.io/docs/guides/status-codes/`).

- **Gains:** `details[]` is a typed extension array — genuinely forward-
  compatible, and `code` is the well-known gRPC status enum.
- **Loss:** `code` is an enum (same #110 problem, moved from `FailureClass`
  to gRPC codes); the request/response axis is baked in. Only compelling if
  Mill adopts gRPC transports, which (per §5.6) it does not.

### 6.5 SARIF

Static Analysis Results Interchange Format (`https://sarifweb.azurewebsites.net/`).

- **Gains:** purpose-built for structured findings with stable rule IDs
  (≈ `type` URI) and occurrence identity (≈ `instance`).
- **Loss:** designed for static-analysis reports; it is a 700+ field schema
  with its own tooling ecosystem. The weight is disproportionate to a
  verdict ledger, and it would import an XML/JSON schema nobody at Mill
  needs.
- **Fit:** not applicable.

### 6.6 Unix exit codes + stderr

The existing de facto contract: exit codes 4/9/130/137/143 map to kills
(`signals.go:108-115`), exit 1 + `gate-*` maps to GATE_FAILURE
(`:74-86`), exit -1/-2 + `blocked:` maps to EXECUTION_FAILURE
(`:87-105`). Humans read the stderr lines.

- **Gains:** zero dependencies, works everywhere.
- **Loss:** this is the *status quo*; it is precisely what fails #105/#143
  because stderr is transient and exit codes are a 1-bit signal.
- **Verdict:** the baseline being improved upon, not a contending design.

---

## 7. Recommendation

**Adopt RFC 9457's *object model only* — `type` + `title` + `detail` +
`instance` + extensions — and explicitly *decouple* it from the three
HTTP-specific obligations the model was designed around.** Do not adopt
`application/problem+json` as a media type, do not invent HTTP status codes
for `status`, and do not require `type` to dereference unless Mill later
adds a docs endpoint (out of scope).

### 7.1 How the five members map to Mill

| Member | Mill mapping | Rationale |
|---|---|---|
| `type` | a stable URI per failure kind. **Prefer `urn:problem:mill:<class>:<signal>` over `https://...`.** | Gives #110 a single primary identifier. Non-resolvable URNs satisfy 9457 §3.1.1 *and* avoid the #99-style documentation-rot burden. Do **not** mint `https://` type URIs unless someone commits to serving them. |
| `title` | short, stable summary of the kind (e.g. "Gate failure"). | Replaces the free-text `Description` field on each `Signal` (`signals.go:25`) with a normative, stable headline. |
| `status` | **adopt Mill's `FailureClass` as the coarse bucket, not an HTTP code.** | The RFC allows `status` to be advisory (9457 §3.1.2). Using `FailureClass` as `status` keeps the existing coarse signal without inventing HTTP semantics (contrary to §4.1's fake-codes trap). |
| `detail` | the occurrence-specific explanation: the actual stderr / build output / rejection rationale. | This is the #105/#143 "what it rests on" field — the evidence the ledger currently discards. |
| `instance` | a per-occurrence identifier: `urn:uuid:<x>` or `urn:mill:ledger:<issue>:<offset>`. | Gives the ledger entry, the worktree, and (future) issue comments a shared handle (#105 observability; #139 cross-reference). |
| extensions | `exit_code`, `stderr`, `model`, `phase`, `round`, `role`, `heartbeat_staleness`, `commits`, `reviewer_verdict`. | Forward-compatible by the RFC's own rule (§3.2). This is where the discarded `SessionResult` lives again. |

### 7.2 Why this wins on the substantive trade-offs

- It closes **#105** (#105's own words: "what it rests on is not"): `detail`
  + extensions retain the evidence the ledger deletes today.
- It closes **#143** (reviewer approved the agent's narration, not the diff;
  `provider_config` parsed correctly and changed nothing): a stable `type`
  URI plus structured `extensions` makes "what was this verdict based on"
  mechanically inspectable rather than prose-scannable.
- It closes **#110** with a single kind registry: each `Signal` declares
  one `type`; there is no second divergent `FailureClassOf` mapping to drift
  from it.
- It closes **#139** (raising a hand): an underspecified-task failure gets a
  distinct `type` (`urn:problem:mill:underspecified`) with a `detail`
  stating the missing criterion and `missing` extension listing what is
  absent — instead of collapsing into `EXECUTION_FAILURE` and triggering a
  pointless model-chain retry (`delegate.go:421-473`).

### 7.3 What it does NOT do, and the cost it does not pay

- It does **not** require serving `type` URIs, so it sidesteps the
  `Capabilities().Models`-style rot the issue #99 warns about. The #99
  precedent is itself evidence that a source-held identifier list drifts
  when the owning surface is external — and the recommendation deliberately
  avoids that trap by choosing non-resolvable URNs.
- It does **not** add HTTP semantics to a non-HTTP program, so it does not
  inherit §4.1's fake-status-code problem.
- It does **not** retrofit `Verdict` (`verdict.go`) or lifecycle
  `TaskStatus`/`SessionStatus` (`status.go`); those are not diagnostics.
- It does **not** add issue-comment posting. That remains an unimplemented
  `gh` feature (see §5.6) and should be a separate brief.

### 7.4 The one decision the CTO must make

The recommendation turns on a single binary choice, and it is the *only*
point where the RFC's HTTP heritage forces a real decision:

> **Dereferenceable `type` URIs (`https://...`) vs. stable non-resolvable
> URNs (`urn:problem:mill:...`)?**

- **`https://...` type URIs** give the full RFC promise — dereference to
  documentation, human-readable by design — but commit Mill to *serving*
  those URIs (a docs endpoint) and to keeping that doc set in sync, which
  is exactly the #99 rot surface the issue names. They are also, per 9457
  §3.1.1, a one-way door: switching to a non-resolvable URI later breaks
  `type` identity.
- **`urn:problem:mill:...` URIs** give the stable, machine-primary `type`
  identity that #110 wants, avoid the rot, and are explicitly permitted
  by 9457 §3.1.1 — but they give up the "dereference-to-docs" property,
  which means Mill keeps a *separate* kind→documentation table (a doc
  page, not a URN) rather than deriving docs from the URI.

The recommendation is to choose **URNs** unless and until the CTO decides
Mill should serve a problem-details documentation endpoint — because the
#99 precedent establishes that the organization is already on the hook for
keeping externally-owned identifier lists current, and adding a second such
list (this one carrying the weight of "the primary identifier consumers
MUST use") multiplies that risk. If URNs are chosen, the kind→documentation
mapping lives as a table in `docs/` (e.g. `docs/problem-types.md`), not as
served URIs.

### 7.5 Ordering (if approved)

1. `internal/domain` — define `Problem` (type/title/status/detail/instance
   + extensions) and annotate `Signal` (`signals.go`).
2. `internal/ledger` — add fields to `Entry`; stop discarding evidence.
3. `internal/cli` — wire `classifyFailure` to return a `Problem`; update the
   eight `ledger.Entry` literals and the escalation stderr.
4. `docs/problem-types.md` — the kind→documentation table (since `type`
   URIs are non-resolvable by the recommendation).

---

## 8. Summary table

| Issue | Current gap (code) | How 9457 object model closes it | Stated risk / trade-off |
|---|---|---|---|
| #105 (observability) | ledger stores `FailureClass` but not the `SessionResult` that produced it (`review_loop.go:118-131`); no occurrence identity | `detail` + extensions retain evidence; `instance` gives a cross-surface identity | §4.2: `instance`/`type` URIs can leak internals if dereferenced (9457 §5) |
| #110 (classification) | `FailureClassOf` (`classification.go:41`) and `classifyFailure` (`delegate.go:836`) are two divergent mappings; signals hardcode `FailureClass` literals | one `type` URI per `Signal` is the single primary identifier | §4.5: a new registry can drift in the identity dimension unless enforced |
| #139 (raising a hand) | `blocked:` collapses budget- vs. underspecified-task into `EXECUTION_FAILURE` (`signals.go:87-105`); escalation only prints stderr (`:297,347`, `routing_56.go:352`) | distinct `type` per hand-raise cause; structured `detail` + `missing` extension | §4.4: no media type; issue-comment posting is a separate, unimplemented feature |
| #143 (defect evidence) | reviewer verdict + evidence not co-located (`reviewer/ROLE.md:62` wants it; code discards stderr) | verdict lives in a `Verdict` (unchanged); the *failure* is a `Problem` with `detail`+extensions | §4.1: `status` would need redefining for non-HTTP; recommendation maps it to `FailureClass` |
| #99 (rot) | `Capabilities().Models` (`commandcode.go:23-33`) + `validateModelChain` (`delegate.go:506`) drift from provider | avoided by choosing non-resolvable URN `type` URIs per 9457 §3.1.1 | §4.2: the cost is a separate doc table instead of dereferenceable URIs |
