# Mill Org Chart — Delegation Hierarchy

## Who can delegate to whom

```mermaid
graph TD
    CTO[👤 CTO]
    
    CTO -->|talks to| STAFF[🤖 Staff<br/>skills: 17<br/>model: pro]
    CTO -->|talks to| PM[🤖 Product Manager<br/>skills: 4<br/>model: pro]
    
    STAFF -->|delegates to| PM
    STAFF -->|delegates to| ARCH[🤖 Architect<br/>skills: 4<br/>model: pro]
    STAFF -->|delegates to| REV[🤖 Reviewer<br/>skills: 2<br/>model: pro]
    
    PM -->|delegates to| UX[🤖 UX Designer<br/>skills: 3<br/>model: pro]
    PM -->|delegates to| QA[🤖 QA/Docs<br/>skills: 2<br/>model: cheap]
    
    UX -->|delegates to| UI[🤖 UI Designer<br/>skills: 2<br/>model: pro]
    UX -->|delegates to| QA
    
    UI -->|delegates to| QA
    
    ARCH -->|delegates to| TL[🤖 Tech Lead<br/>skills: 5<br/>model: pro]
    ARCH -->|delegates to| QA
    
    TL -->|delegates to| FE[🤖 Sr. Dev FE<br/>skills: 4<br/>model: cheap]
    TL -->|delegates to| BE[🤖 Sr. Dev BE<br/>skills: 4<br/>model: cheap]
    TL -->|delegates to| DATA[🤖 Sr. Dev Data<br/>skills: 4<br/>model: cheap]
    TL -->|delegates to| QA
    
    FE -->|delegates to| QA
    BE -->|delegates to| QA
    DATA -->|delegates to| QA
    
    REV -->|delegates to| QA
    
    QA -.->|shared service<br/>any role| QA
    
    classDef human fill:#FFD700,stroke:#333,color:#000
    classDef active fill:#4CAF50,stroke:#333,color:#fff
    classDef pro fill:#9C27B0,stroke:#333,color:#fff
    classDef cheap fill:#2196F3,stroke:#333,color:#fff
    
    class CTO human
    class STAFF,PM active
    class PM,ARCH,TL,REV,UX,UI pro
    class FE,BE,DATA,QA cheap
```

## What each arrow means

| From | To | Why |
|------|----|-----|
| CTO | Staff | Technical direction |
| CTO | PM | Product decisions |
| **Staff** | **PM** | Write product specs |
| **Staff** | **Architect** | System architecture, ADRs |
| **Staff** | **Reviewer** | Independent code review |
| PM | UX Designer | User flows, IA |
| UX | UI Designer | Components, tokens |
| **Architect** | **Tech Lead** | Per-feature specs, task decomposition |
| **Tech Lead** | **Sr. Dev FE/BE/Data** | Implementation (THE ONLY ROLE that can) |
| Tech Lead | QA/Docs | Tests, changelog |
| Reviewer | QA/Docs | Tests, docs |
| Anyone | QA/Docs | Shared service |

## Who reviews whom (review chain)

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
| 1 | **Only Tech Lead delegates to Sr. Devs** | Architect is strategic, Tech Lead is tactical |
| 2 | **Every line of code passes through Tech Lead** | Code review is Tech Lead's job, not Staff's |
| 3 | **Staff never reviews code** | Staff is managerial — verifies process, not implementation |
| 4 | **Architect sits between Staff and Tech Lead** | Cross-cutting decisions before tactical decomposition |
| 5 | **Reviewer is independent** | Second pair of eyes, different from Tech Lead |
| 6 | **Expensive models (pro) never do cheap work** | Staff, PM, Architect delegate. Sr. Devs execute. |

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

## What each role costs and why delegation matters

| Role | Model | Cost | Should NEVER |
|------|-------|------|-------------|
| Staff | deepseek-v4-pro | $0.36/session | Write code, review code, write specs |
| PM | deepseek-v4-pro | $0.36/session | Write code, design UI, touch architecture |
| Architect | deepseek-v4-pro | $0.36/session | Review individual PRs, implement |
| Tech Lead | deepseek-v4-pro | $0.36/session | Write production code |
| Reviewer | deepseek-v4-pro | $0.36/session | Fix code, design architecture |
| UX/UI | deepseek-v4-pro | $0.36/session | Write code |
| Sr. Devs | laguna-free | $0.00/session | Decide architecture, skip review |
| QA/Docs | laguna-free | $0.00/session | Decide scope, skip tests |

**Principle:** expensive model = decisions. Cheap model = execution. Staff is the most expensive — every token spent on implementation is waste.
