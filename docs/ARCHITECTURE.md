# Mill Architecture — Session Model

## Two active roles per session

Only Staff and PM can be the active role in a CTO session. They delegate everything using `mill delegate --role <target>`.

```mermaid
graph TD
    CTO[👤 CTO] -->|product question| PM[🤖 PM - active]
    CTO -->|technical direction| STAFF[🤖 Staff - active]
    
    PM -->|mill delegate --role ux| UX[🤖 UX Designer - spawned]
    PM -->|mill delegate --role ui| UI[🤖 UI Designer - spawned]
    
    STAFF -->|mill delegate --role pm| PM2[🤖 PM - spawned]
    STAFF -->|mill delegate --role tech-lead| TL[🤖 Tech Lead - spawned]
    STAFF -->|mill delegate --role architect| ARCH[🤖 Architect - spawned]
    STAFF -->|mill delegate --role reviewer| REV[🤖 Reviewer - spawned]
    
    TL -->|mill delegate --role sr-dev-fe| FE[🤖 Sr. Dev FE]
    TL -->|mill delegate --role sr-dev-be| BE[🤖 Sr. Dev BE]
    TL -->|mill delegate --role sr-dev-data| DATA[🤖 Sr. Dev Data]
    
    REV -->|mill delegate --role qa-docs| QA[🤖 QA/Docs]
    
    UX -->|mill delegate --role ui| UI
    UX -->|mill delegate --role qa-docs| QA
    PM2 -->|mill delegate --role ux| UX
    PM2 -->|mill delegate --role ui| UI
    FE -->|mill delegate --role qa-docs| QA
    BE -->|mill delegate --role qa-docs| QA
    DATA -->|mill delegate --role qa-docs| QA
    
    classDef human fill:#FFD700,stroke:#333,color:#000
    classDef active fill:#4CAF50,stroke:#333,color:#fff
    classDef spawned fill:#2196F3,stroke:#333,color:#fff
    
    class CTO human
    class STAFF,PM active
    class UX,UI,PM2,TL,ARCH,REV,FE,BE,DATA,QA spawned
```

## What each active role can do directly

| Role | Can do directly | Must delegate |
|------|----------------|---------------|
| **Staff** | Talk to CTO, verify results, decide merge-readiness | Everything else: specs, design, implementation, review |
| **PM** | Talk to CTO, write product specs, prioritize | UX design, UI design, implementation |

## What NEVER happens

```mermaid
graph TD
    WRONG1[❌ Staff writes code] -.-> X1[BLOCKED by pre-commit]
    WRONG2[❌ Staff switches to sr-dev role] -.-> X2[BLOCKED by role-enforce]
    WRONG3[❌ PM writes code] -.-> X3[BLOCKED by pre-commit]
    WRONG4[❌ PM delegates to sr-dev directly] -.-> X4[BLOCKED by delegation chain]
    
    classDef wrong fill:#D32F2F,color:#fff
    classDef blocked fill:#FF9800,color:#000
    
    class WRONG1,WRONG2,WRONG3,WRONG4 wrong
    class X1,X2,X3,X4 blocked
```

## Session lifecycle

```mermaid
sequenceDiagram
    participant CTO
    participant Staff
    participant Runner
    participant TL as Tech Lead (spawned)
    participant SD as Sr. Dev (spawned)
    
    CTO->>Staff: "add dark mode to settings"
    Staff->>Runner: mill delegate --role tech-lead 390
    Runner->>TL: spawn in worktree
    TL->>TL: decompose into tasks
    TL->>TL: raise hand if ambiguous
    TL-->>Runner: done — tasks in issue
    
    Staff->>Runner: mill delegate --role sr-dev-fe 390
    Runner->>SD: spawn in worktree
    SD->>SD: implement from brief
    SD->>SD: pre-commit gauntlet
    SD->>SD: raise hand if blocked
    SD-->>Runner: done — code committed
    
    Staff->>Staff: verify (7-step checklist)
    Staff->>CTO: "ready to land"
```

## Key rules

1. **Staff and PM are the ONLY roles the CTO talks to directly.**
2. **Staff and PM NEVER execute work themselves** — they use `mill delegate --role <target>`.
3. **`mill delegate` validates the delegation chain.** Staff → Tech Lead ✅. Staff → Sr. Dev ❌.
4. **Spawned agents don't know who spawned them.** They know their role, their task, and their `reviewed_by` (from frontmatter).
5. **If a spawned agent has doubts → BLOCKED → escalation chain → resolved → resume.**
6. **The runner enforces everything mechanically.** Pre-commit hooks, delegation validation, classification.
