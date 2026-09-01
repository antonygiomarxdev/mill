# Contributing

Mill triages issues against the repository, not against their title. A verdict
names the file or command that decides it. `docs/BACKLOG-TRIAGE-2026-08-31.md`
is the worked example.

## Reporting a bug

A report with a command beats a report with a description. Mill's own issues
are the model: the good ones carry the exact command that shows the defect and
its raw output.

Include, in order:

1. What you ran — the exact command or sequence.
2. What it printed — paste the real output, not a summary. Silence counts;
   say so when a command produced nothing.
3. What you expected instead.
4. Where it broke — which document and which step, if you were following one.
5. Environment — Orca version, agent and model, OS, and the Mill commit.

Before filing, check the behaviour is not Orca's: several Mill symptoms are
upstream. Confirm you are on a current release and search the Orca tracker.

## Asking for something

A request is actionable when a maintainer can tell from it when it is done.
State the problem (who it is for), the change, and how success is measured.
State what is out of scope. "It would be nice if ..." with no way to verify
completion is a wish, not a request.

## Labels

Choose one type and, when it applies, one priority.

| Label | When |
| --- | --- |
| `bug` | Something does not behave as documented |
| `enhancement` | New capability or change to existing behaviour |
| `onboarding` | Friction while setting Mill up or using it the first time |
| `priority:P0` | Blocker — cannot ship without |
| `priority:P1` | Fragility — breaks under real conditions |
| `priority:P2` | Polish or backlog |

## What happens next

Issues are triaged against the repository as it exists, not against the
title's claim. Each verdict — the issue is live, already done, or describes
code the repository has since removed — cites the file or command that decides
it. `docs/BACKLOG-TRIAGE-2026-08-31.md` is the worked example.
