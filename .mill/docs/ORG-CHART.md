# Mill Org Chart — Delegation Hierarchy

## Who can delegate to whom

```mermaid
graph TD
    CTO[👤 CTO]
    
    CTO -->|talks to| STAFF[🤖 Staff<br/>skills: 16<br/>model: pro]
    CTO -->|talks to| PM[🤖 Product Manager<br/>skills: 4<br/>model: pro]
    
    STAFF -->|delegates to| PM
    STAFF -->|delegates to| ARCH[🤖 Architect<br/>skills: 4<br/>model: pro]
    STAFF -->|delegates to| REV[🤖 Reviewer<br/>skills: 2<br/>model: pro]
    
    PM -->|delegates to| UX[🤖 UX Designer<br/>skills: 3<br/>model: pro]
    PM -->|delegates to| QA[🤖 QA/Docs<br/>skills: 2<br/>model: free]
    
    UX -->|delegates to| UI[🤖 UI Designer<br/>skills: 2<br/>model: pro]
    UX -->|delegates to| QA
    
    UI -->|delegates to| QA
    
    ARCH -->|delegates to| TL[🤖 Tech Lead<br/>skills: 5<br/>model: pro]
    ARCH -->|delegates to| QA
    
    TL -->|delegates to| FE[🤖 Sr. Dev FE<br/>skills: 3<br/>model: free]
    TL -->|delegates to| BE[🤖 Sr. Dev BE<br/>skills: 3<br/>model: free]
    TL -->|delegates to| DATA[🤖 Sr. Dev Data<br/>skills: 3<br/>model: free]
    TL -->|delegates to| QA
    
    FE -->|delegates to| QA
    BE -->|delegates to| QA
    DATA -->|delegates to| QA
    
    REV -->|delegates to| QA
    
    QA -.->|shared service<br/>any role| QA
    
    classDef human fill:#FFD700,stroke:#333,color:#000
    classDef active fill:#4CAF50,stroke:#333,color:#fff
    classDef pro fill:#9C27B0,stroke:#333,color:#fff
    classDef free fill:#2196F3,stroke:#333,color:#fff
    
    class CTO human
    class STAFF,PM active
    class PM,ARCH,TL,REV,UX,UI pro
    class FE,BE,DATA,QA free
```

## What each arrow means

| From | To | Why |
|------|----|-----|
| CTO | Staff | Technical direction |
| CTO | PM | Product decisions |
| Staff | PM | Write product specs |
| Staff | Architect | System architecture, ADRs |
| Staff | Reviewer | Independent code review |
| PM | UX Designer | User flows, IA |
| UX | UI Designer | Components, tokens |
| Architect | Tech Lead | Per-feature specs, task decomposition, code review |
| Tech Lead | Sr. Dev FE/BE/Data | Implementation — THE ONLY ROLE that can |
| Tech Lead | QA/Docs | Tests, changelog |
| Reviewer | QA/Docs | Tests, docs |
| Anyone | QA/Docs | Shared service |

## Who reviews whom

```mermaid
graph LR
    CTO -->|merge approval| STAFF
    
    PM -->|product review| UX
    UX -->|design review| UI
    
    STAFF -->|strategic review| ARCH
    ARCH -->|architecture review| TL
    
    TL -->|code review - ALWAYS| FE
    TL -->|code review - ALWAYS| BE
    TL -->|code review - ALWAYS| DATA
    
    TL -->|hands off to| REV
    REV -->|spec compliance| QA
    
    classDef human fill:#FFD700,stroke:#333,color:#000
    classDef active fill:#4CAF50,stroke:#333,color:#fff
    class CTO human
    class STAFF,PM active
```

## Critical rules

| # | Rule | Why |
|---|------|-----|
| 1 | Only Tech Lead delegates to Sr. Devs | Architect is strategic, Tech Lead is tactical |
| 2 | Every line of code passes through Tech Lead | Code review is Tech Lead's job |
| 3 | Staff never reviews code | Staff verifies process, not implementation |
| 4 | Architect sits between Staff and Tech Lead | Cross-cutting decisions before tactical work |
| 5 | Reviewer is independent second pair of eyes | Different from Tech Lead |
| 6 | Pro models decide, free models execute | Staff delegates. Sr. Devs implement. |

## Full pipeline: "Add dark mode"

```mermaid
sequenceDiagram
    participant CTO
    participant Staff
    participant PM
    participant UX
    participant UI
    participant Arch as Architect
    participant TL as Tech Lead
    participant SD as Sr. Dev
    participant Rev as Reviewer
    participant QA
    
    CTO->>Staff: add dark mode to settings
    
    Staff->>PM: write product spec
    PM-->>Staff: spec: 8 criteria
    
    Staff->>UX: design user flow
    UX->>UI: design tokens + components
    UI-->>UX: component specs
    UX-->>Staff: UX handoff
    
    Staff->>Arch: define dark mode architecture
    Arch->>TL: decompose into implementation tasks
    TL-->>Arch: 3 atomic tasks
    Arch-->>Staff: architecture + tasks approved
    
    Staff->>TL: implement task 1
    TL->>SD: implement SettingsCard dark mode
    SD-->>TL: code committed, tests pass
    
    TL->>TL: code review
    TL-->>Staff: code approved
    
    Staff->>Rev: independent review
    Rev-->>Staff: APPROVED
    
    Staff->>QA: write tests + changelog
    QA-->>Staff: done
    
    Staff->>Staff: 7-step verification
    Staff->>CTO: ready to land
```

## Model tiers

Models are configured in `mill.yml`, not hardcoded in docs or roles. Three tiers:

| Tier | Who uses it | Purpose |
|------|------------|---------|
| pro | Staff, PM, Architect, Tech Lead, Reviewer, UX, UI | Decisions, review, design |
| free | Sr. Devs, QA/Docs | Execution, tests, documentation |

The actual model mapping (e.g., `deepseek-v4-pro` for pro, `laguna-free` for free) lives in `mill.yml` per project. Projects switch providers by changing config, not roles or docs.
