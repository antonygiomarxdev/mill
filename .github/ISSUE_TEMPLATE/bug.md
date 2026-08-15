---
name: Bug Report
about: Something in Mill does not behave as documented
title: "[BUG] "
labels: ["bug"]
assignees: []
---

## What happened

<!-- One paragraph. What you did, what you expected, what occurred instead. -->

## Reproduction

<!-- The exact commands, in order. Paste the real output, not a description of it.
     If a command produced no output, say so — silence is often the defect. -->

```
$ 
```

## Where it broke

<!-- If this happened while following a document, say which and which step.
     "README, install step 3" is more useful than "during installation". -->

## Environment

- **Orca version**: <!-- grep -ao '"version": *"[0-9.]*"' /tmp/.mount_orca*/resources/app.asar | head -1 -->
- **Agent**: <!-- command-code / claude / codex — and the model, if you know it -->
- **OS**:
- **Mill commit**: <!-- git -C <your mill checkout> rev-parse --short HEAD -->

## Already checked

<!-- Mill depends on Orca; several behaviours that look like Mill bugs are Orca's,
     and some are already fixed upstream. Before filing, please check:

     - Is the Orca version current?  gh release list --repo stablyai/orca --limit 3
     - Is it already reported?       gh search issues --repo stablyai/orca "<symptom>" --state all

     If either applies, say so here. Both checks were skipped during development
     and cost hours. -->

- [ ] Orca is on the latest release
- [ ] Searched the Orca tracker for this symptom
