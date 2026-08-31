# Mill Org Chart — Delegation Hierarchy

## Who can delegate to whom

```mermaid
graph TD
    CTO[👤 CTO]

    CTO -->|talks to| PE[🤖 Product Engineer<br/>skills: 18<br/>model: pro]

    PE -->|delegates to| PM[🤗 Product Manager<br/>skills: 4<br/>model: pro]
    PE -->|delegates to| ARCH[🤗 Architect<br/>skills: 4<br/>model: pro]
    PE -->|delegates to| REV[🤗 Reviewer<br/>skills: 2<br/>model: pro]
    PE -->|delegates to| TL[🤗 Tech Lead<br/>skills: 5<br/>model: pro]
    PE -->|delegates to| FE[🤗 Sr. Dev FE<br/>skills: 4<br/>model: free]
    PE -->|delegates to| BE[🤗 Sr. Dev BE<br/>skills: 4<br/>model: free]
    PE -->|delegates to| DATA[🤗 Sr. Dev Data<br/>skills: 4<br/>model: free]
    PE -->|delegates to| QA[🤗 QA/Docs<br/>skills: 2<br/>model: free]
    PE -->|delegates to| UX[🤗 UX Designer<br/>skills: 3<br/>model: pro]
    PE -->|delegates to| UI[🤗 UI Designer<br/>skills: 2<br/>model: pro]
    PE -->|delegates to| PA[🤗 Policy Author<br/>skills: 3<br/>model: pro]

    classDef human fill:#FFD700,stroke:#333,color:#000
    classDef active fill:#4CAF50,stroke:#333,color:#fff
    classDef pro fill:#9C27B0,stroke:#333,color:#fff
    classDef free fill:#2196F3,stroke:#333,color:#fff

    class CTO human
    class PE,PM active
    class PM,ARCH,TL,REV,UX,UI,PA pro
    class FE,BE,DATA,QA free
```

## What each arrow means

| From | To | Why |
|------|----|-----|
| CTO | Product Engineer | Technical direction |
| CTO | PM | Product decisions |
| Product Engineer | PM | Write product specs |
| Product Engineer | Architect | System architecture, ADRs |
| Product Engineer | Reviewer | Independent code review |
| Product Engineer | Tech Lead | Per-feature specs, task decomposition |
| Product Engineer | Sr. Dev BE/FE/Data | Implementation |
| Product Engineer | QA/Docs | Tests, changelog |
| Product Engineer | UX Designer | User flows, IA |
| Product Engineer | UI Designer | Components, tokens |
| Product Engineer | Policy Author | Policy maintenance (.mill/) |

## Who reviews whom

```mermaid
graph LR
    CTO -->|merge approval| PE

    PE -->|strategic review| ARCH
    PE -->|product review| PM
    PE -->|design review| UX
    PE -->|design specs| UI
    PE -->|code review| FE
    PE -->|code review| BE
    PE -->|code review| DATA
    PE -->|hands off to| REV
    PE -->|spec compliance| QA
    PE -->|policy review| PA

    classDef human fill:#FFD700,stroke:#333,color:#000
    classDef active fill:#4CAF50,stroke:#333,color:#fff
    class CTO human
    class PE,PM active
```

## Critical rules

| # | Rule | Why |
|---|------|-----|
| 1 | Only the Product Engineer delegates to all worker roles | Single dispatch point, no worker handoff |
| 2 | Every line of code passes through Tech Lead | Code review is Tech Lead's job |
| 3 | The Product Engineer never reviews code | The PE verifies process, not implementation |
| 4 | Architect handles cross-cutting decisions | Decisions before tactical work |
| 5 | Reviewer is independent second pair of eyes | Different from Tech Lead |
| 6 | Pro models decide, free models execute | The Product Engineer delegates. Sr. Devs implement. |

## Full pipeline: "Add dark mode"

```mermaid
sequenceDiagram
    participant CTO
    participant PE as Product Engineer
    participant pman as Product Manager
    participant uxd as UX Designer
    participant uied as UI Designer
    participant archi as Architect
    participant tlead as Tech Lead
    participant sdev as Sr. Dev
    participant revr as Reviewer
    participant qad as QA/Docs

    CTO->>PE: add dark mode to settings

    PE->>pman: write product spec
    pman-->>PE: spec: 8 criteria

    PE->>uxd: design user flow
    uxd-->>PE: flows ready

    PE->>uied: design tokens + components
    uied-->>PE: component specs

    PE->>archi: define dark mode architecture
    archi-->>PE: architecture approved

    PE->>tlead: decompose into implementation tasks
    tlead-->>PE: 3 atomic tasks

    PE->>sdev: implement SettingsCard dark mode
    sdev-->>PE: code committed, tests pass

    PE->>revr: code review
    revr-->>PE: APPROVED

    PE->>qad: write tests + changelog
    qad-->>PE: done

    PE->>PE: mill-verify + acceptance criteria
    PE->>CTO: ready to land
```

## Model tiers

Each role declares its tier in its `ROLE.md` frontmatter (`model:` — either
`free→paid` or `pro`). Three tiers:

| Tier | Who uses it | Purpose |
|------|------------|---------|
| pro | Product Engineer, PM, Architect, Tech Lead, Reviewer, UX, UI, Policy Author | Decisions, review, design |
| free | Sr. Devs, QA/Docs | Execution, tests, documentation |

The tier resolves to an (agent, model) pair through `.mill/agents` — see
`.claude/skills/delegate/SKILL.md` section 2. Projects switch
providers by changing config, not roles or docs.
