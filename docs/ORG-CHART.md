# Mill Org Chart — Delegation Hierarchy

## Who can delegate to whom

```mermaid
graph TD
    CTO[👤 CTO]
    
    CTO -->|talks to| STAFF[🤖 Staff<br/>skills: 17]
    CTO -->|talks to| PM[🤖 Product Manager<br/>skills: 4]
    
    STAFF -->|delegates to| PM
    STAFF -->|delegates to| ARCH[🤖 Architect<br/>skills: 4]
    STAFF -->|delegates to| TL[🤖 Tech Lead<br/>skills: 5]
    STAFF -->|delegates to| REV[🤖 Reviewer<br/>skills: 2]
    
    PM -->|delegates to| UX[🤖 UX Designer<br/>skills: 3]
    PM -->|delegates to| QA[🤖 QA/Docs<br/>skills: 2]
    
    UX -->|delegates to| UI[🤖 UI Designer<br/>skills: 2]
    UX -->|delegates to| QA
    
    UI -->|delegates to| QA
    
    ARCH -->|delegates to| TL
    ARCH -->|delegates to| QA
    
    TL -->|delegates to| FE[🤖 Sr. Dev FE<br/>skills: 4]
    TL -->|delegates to| BE[🤖 Sr. Dev BE<br/>skills: 4]
    TL -->|delegates to| DATA[🤖 Sr. Dev Data<br/>skills: 4]
    TL -->|delegates to| QA
    
    FE -->|delegates to| QA
    BE -->|delegates to| QA
    DATA -->|delegates to| QA
    
    REV -->|delegates to| QA
    
    QA -.->|shared service<br/>anyone can delegate| QA
    
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

| From | To | Example |
|------|----|---------|
| CTO | Staff | "necesito dark mode" |
| CTO | PM | "cuál es la prioridad de X?" |
| Staff | PM | "escribime la spec de esto" |
| Staff | Tech Lead | "descomponé esta spec en tasks" |
| Staff | Reviewer | "revisá este PR" |
| PM | UX Designer | "diseñá el flujo de onboarding" |
| UX | UI Designer | "creá los componentes visuales" |
| Tech Lead | Sr. Dev FE | "implementá el componente X" |
| Tech Lead | Sr. Dev BE | "implementá el endpoint Y" |
| Cualquiera | QA/Docs | "escribime tests para esto" |

## What each role reviews

```mermaid
graph LR
    CTO -->|approves merge| STAFF
    
    PM -->|reviews| UX
    UX -->|reviews| UI
    
    STAFF -->|reviews| ARCH
    ARCH -->|reviews| TL
    TL -->|reviews| FE
    TL -->|reviews| BE
    TL -->|reviews| DATA
    
    STAFF -->|reviews| REV
    REV -->|reviews| QA
    
    FE -->|reviews| QA
    BE -->|reviews| QA
    DATA -->|reviews| QA
    
    classDef human fill:#FFD700,stroke:#333,color:#000
    classDef active fill:#4CAF50,stroke:#333,color:#fff
    class CTO human
    class STAFF,PM active
```

## Full pipeline example: "Add dark mode"

```mermaid
sequenceDiagram
    participant CTO
    participant Staff
    participant PM
    participant UX
    participant UI
    participant TechLead
    participant SrDev
    participant Reviewer
    participant QA
    
    CTO->>Staff: add dark mode to settings
    
    Note over Staff: Staff delegates to PM
    Staff->>PM: spec: dark mode toggle
    PM-->>Staff: spec: 8 criteria
    
    Note over Staff: Staff delegates to UX
    Staff->>UX: design flow for dark mode
    UX->>UI: design dark tokens + components
    UI-->>UX: tokens + component specs
    UX-->>Staff: UX handoff
    
    Note over Staff: Staff delegates to Tech Lead
    Staff->>TechLead: decompose into tasks
    TechLead-->>Staff: 3 tasks for sr-dev-fe
    
    Note over Staff: Staff delegates to Sr. Dev
    Staff->>SrDev: implement task 1
    SrDev-->>Staff: code committed
    
    Note over Staff: Staff delegates to Reviewer
    Staff->>Reviewer: review spec compliance
    Reviewer-->>Staff: APPROVED
    
    Note over Staff: Staff delegates to QA
    Staff->>QA: write tests + changelog
    QA-->>Staff: tests pass, changelog updated
    
    Note over Staff: Staff verifies 7-step checklist
    Staff->>CTO: ready to land
```

## Rules that CANNOT be broken (enforced mechanically)

| Rule | How enforced |
|------|-------------|
| Staff never writes Go code | pre-commit hook |
| PM never writes Go code | pre-commit hook |
| Staff never delegates to Sr. Dev directly | delegation validation in `mill delegate --role` |
| PM never delegates to Sr. Dev directly | delegation validation |
| No role can change `.mill/role` except to staff/pm | role-enforce hook |
| QA/Docs accepts delegation from anyone | no delegation validation for qa-docs |
