# Real Startup Engineering Org Structures & Practices

> Research compiled 2026-08-09 for mill's agent org design.
> **Legend:** ✅ Verified (source cited) | 🔮 Inferred (synthesized from multiple sources, no single canonical source)

---

## 1. Reporting Chains

### Seed Stage (1–5 Engineers)
- **Flat hierarchy.** All engineers report directly to the CTO (often a technical co-founder) or CEO. ✅ [[1]](#ref-1)
- CTO is the "Chief Coder": highly hands-on, splitting time between writing code, architecture, and hiring. ✅ [[9]](#ref-1)
- One senior engineer may act as an informal "Tech Lead" to oversee quality and architecture, but remains an IC. ✅ [[12]](#ref-1)
- Engineers are T-shaped generalists contributing across the full stack. ✅ [[11]](#ref-1)
- Coordination is informal: daily standups, Slack, close proximity. Formal processes avoided. ✅ [[14]](#ref-1)

### Series A (5–20+ Engineers)
- **Hierarchy introduced.** Once team passes ~8–10 engineers, one CTO can't manage everyone. ✅ [[22]](#ref-1)
- A **VP of Engineering** or **Engineering Manager (EM)** is hired to handle people management; CTO shifts to long-term strategy and architecture. ✅ [[24]](#ref-1)
- Senior engineers formally take "Tech Lead" roles for specific projects/pods, mentoring others and owning technical decisions. ✅ [[27]](#ref-1)
- **Cross-functional EPD pods** (Engineering, Product, Design) of 5–9 people own specific product areas end-to-end. ✅ [[28]](#ref-1)
- Lightweight processes (sprints, retros, roadmaps) introduced — enough structure to scale without bureaucracy. ✅ [[34]](#ref-1)

### Common Pitfalls
- **Hiring managers too early:** "Managers of managers" before team size supports them → bureaucracy. ✅ [[49]](#ref-1)
- **Stagnating as a "Coder CTO":** Failing to transition from hands-on → strategic leadership bottlenecks the company. ✅ [[51]](#ref-1)
- **Copying large-company structures:** Complex multi-layered hierarchies from day one kill speed. ✅ [[53]](#ref-1)

---

## 2. Code Review Chains

### Who Reviews Whose Code

| Role | Responsibility |
|:---|:---|
| **Junior Developer** | Primary author; also active reviewer of others' code to learn patterns and build context. ✅ [[6]](#ref-2) |
| **Senior Developer** | Key reviewer; focuses on architectural alignment, best practices, security, and mentoring juniors. ✅ [[9]](#ref-2) |
| **Tech Lead** | Final authority on code quality and design; ensures alignment with system architecture; manages the review process. ✅ [[11]](#ref-2) |
| **CTO** | Steps back from daily PR reviews as team grows; focuses on high-level strategy, technical debt, and team process. ✅ [[15]](#ref-2) |

### CTO Involvement Thresholds
- **Early stage (1–10 engineers):** CTO reviews everything — sets the standard. ✅ [[21]](#ref-2)
- **Growth stage (15–45 employees):** CTO shifts to establishing the *process* (CI/CD, linting, review guidelines). Day-to-day approvals → Tech Leads and Senior Engineers. ✅ [[7]](#ref-3)
- **Signs CTO should stop reviewing every PR:**
  - PRs sit waiting >24 hours — CTO is a bottleneck. ✅ [[9]](#ref-3)
  - Strong tech leads hired — CTO approval is redundant. ✅ [[11]](#ref-3)
  - Standards are codified (automated linters, clear patterns). ✅ [[13]](#ref-3)

### CTO's Evolving Review Role at Series A
- Review only **critical path changes**: core business logic, security/auth, data integrity, major architectural shifts. ✅ [[18]](#ref-3)
- Use reviews as a **teaching tool** for architectural integrity, not debugging implementation details. ✅ [[20]](#ref-3)
- Invest in documentation, training, and a **two-step review process** (peer review → lead/senior review). ✅ [[22]](#ref-3)
- Primary value at Series A is **strategic judgment**, not raw code output. ✅ [[28]](#ref-3)

### Best Practices
- **Shift from "gatekeeping" to "mentorship":** Encourage juniors to review senior code — builds understanding and collective ownership. ✅ [[17]](#ref-2)
- **Tiered review depth:** Small/low-risk changes → any peer. High-risk/architectural changes → Tech Lead. ✅ [[28]](#ref-2)
- **Automate the boring stuff:** Linters, static analysis, AI review assistants handle style/formatting/common bugs. Human reviewers focus on design, intent, business logic. ✅ [[31]](#ref-2)
- **Collective ownership:** Avoid one "architect" approving everything — single point of failure. Cross-team reviews spread domain knowledge. ✅ [[38]](#ref-2)
- **Don't wait for "perfect":** If a PR improves code health, approve it. Keep PRs small. ✅ [[42]](#ref-2)

---

## 3. Commit Practices

### Who Commits
- All engineers commit to feature branches. No restriction on who can push commits. 🔮 (universal industry practice)

### Who Merges (Squash-Merge)
- **Author-led merging** is the industry standard for fast-moving startups. ✅ [[1]](#ref-4)
- Developer who authored the PR is responsible for merging once reviewed, CI passes, and criteria met. ✅ [[2]](#ref-4)
- Promotes ownership and speed. If PR is approved but unmerged, the author — not a gatekeeper — clicks merge. ✅ [[3]](#ref-4)
- **Exception:** High-compliance/regulated environments may have a designated maintainer do final merge, but this creates bottlenecks TBD is designed to avoid. ✅ [[4]](#ref-4)

### Git Workflow

| Strategy | Best For | Key Characteristic |
|:---|:---|:---|
| **GitHub Flow** | Very small, fast-moving teams; web apps | Simple, PR-based, single main branch. ✅ [[4]](#ref-5) |
| **Trunk-Based (TBD)** | Teams aspiring to elite CI/CD performance | High frequency integration; "main is always ready." ✅ [[6]](#ref-5) |

- **Start with GitHub Flow** if brand-new — best balance of structure and speed. Code review gate without advanced CI/CD maturity. ✅ [[37]](#ref-5)
- **Evolve to TBD** as automated testing matures. Many teams find short-lived feature branches are "TBD with extra steps." ✅ [[40]](#ref-5)
- **AVOID GitFlow** — widely considered unnecessary and counterproductive for rapid-iteration startups. Only for highly regulated industries or multiple legacy versions. ✅ [[44]](#ref-5)

### Squash Merge Mechanics
1. Keep branches short-lived (hours to a day or two). ✅ [[6]](#ref-4)
2. Rebase onto main (don't merge main into feature) to avoid polluting history before squash. ✅ [[8]](#ref-4)
3. Squash and Merge collapses WIP/fixup commits into one clean commit. ✅ [[10]](#ref-4)
4. Auto-delete branches after merge. ✅ [[14]](#ref-4)
5. Use feature flags to merge incomplete code safely. ✅ [[15]](#ref-4)

### Why Squash
- Clean, linear `main` history. ✅ [[17]](#ref-4)
- Atomic commits → easy `git bisect` and reverts. ✅ [[19]](#ref-4)
- Reduced friction: developers can make messy local commits without exposing the process in permanent history. ✅ [[21]](#ref-4)

---

## 4. Post-Review Fix Workflow

### The Division of Responsibility
- **The author is responsible for fixing the code.** The reviewer identifies issues and suggests improvements; the author implements the changes. ✅ [[1]](#ref-6)
- **The reviewer is NOT responsible for writing the fix.** Their role: examine code for bugs, architectural flaws, maintainability, and standards adherence. ✅ [[10]](#ref-6)
- Author owns the PR: evaluates feedback, makes corrections, or defends their position if they disagree. ✅ [[8]](#ref-6)

### Startup-Specific Dynamics
- **Avoid "gatekeeping":** Toxic culture where reviewers hunt trivial errors to block merges → slows business and frustrates developers. Goal is "better code," not "perfect code." ✅ (from search result on startup code review culture)
- **Collective ownership:** Author owns the PR, team shares responsibility for codebase health. Reviewers are mentors and partners, not just inspectors. ✅ (same source)
- **Communication > Process:** If comment threads go back-and-forth >2–3 times, **jump on a call.** Excessive commenting on minor issues = poor communication; verbal clears it faster and preserves relationships. ✅ (same source)
- **Automation as baseline:** Automate formatting, linting, basic security checks so humans focus on business logic and system design. ✅ (same source)

### Best Practices
- **"Nit" = Optional:** Author can apply, acknowledge, or ignore with reasonable justification. ✅ [[17]](#ref-6)
- **Don't guess:** If a reviewer leaves vague "Why?" comment, author should ask for clarification — don't waste time guessing. ✅ (inferred from same source)
- **Focus on "Why":** Both parties explain reasoning behind suggestions and implementation choices to foster learning. ✅ (inferred)

### Speed vs. Learning Trade-off
- **Author fixes (default):** Slower iteration but builds author's understanding and ownership. The right default for startups where learning compounds. 🔮
- **Reviewer fixes (exception):** Faster for trivial/mechanical changes but robs author of learning opportunity. Appropriate when: reviewer pair-programs with author, author is out, or change is purely mechanical (typo, formatting). 🔮
- **Pairing as middle ground:** Reviewer and author fix together — speed AND learning. Recommended when issues are substantial enough to warrant discussion. 🔮

---

## 5. Design Review Chains

### Core Design Review Team
- Keep review group tight: **Primary Designer, Product Manager (PM), and at least one Engineer.** Too many stakeholders → "design by committee" → slowdown. ✅ (from search result on startup design review)
- **Structure by intent, not stage:** Use reviews to align on *why* you're building and how it solves user problems — not formal gate-keeping. ✅ (same source)
- **Informal and continuous:** Collaborative tools (Figma) where designs are always accessible. Reviews happen throughout the process, not just at the end. ✅ (same source)

### Roles and Responsibilities

| Role | Responsibility |
|:---|:---|
| **Product Manager** | Defines "what" and "why" (business goals, user problems), prioritizes features, ensures alignment with roadmap. ✅ |
| **Design Lead / Designer** | Owns "how" (experience and look), maintains design system, champions user perspective. ✅ |
| **Engineer** | Owns technical feasibility and implementation. Balances pixel-perfection with technical constraints and performance. ✅ |

### Visual QA (Bridging Design & Engineering)
- **Integrate early:** Review build in dev environment as soon as front-end is live. Designers catch spacing, interaction, alignment issues while code is fresh. Don't wait until feature is "finished." ✅
- **Lightweight tracking:** Simple checklist, dedicated Slack channel, or shared document — avoid complex ticketing for visual bugs. ✅
- **Design QA pass:** Small group (Designer, PM, Engineer) performs Visual QA before release to general testing. ✅
- **Automate visual regression:** Tools like Percy catch unintended UI changes across devices/browsers. Humans focus on nuanced design judgment. ✅

### Design Review Flow
```
Designer creates → PM reviews (intent alignment) → Engineer reviews (feasibility) 
→ Design Lead reviews (system coherence) → Design QA pass (Designer + PM + Engineer) 
→ Ship
```
🔮 (inferred from multiple sources)

---

## 6. Shared Services (QA, Docs, DevOps)

### Evolution Model

#### Phase 1: Generalist Era (0–10 Engineers)
- **Structure:** Flat. Every engineer owns their quality, deployment, and documentation. ✅
- **Philosophy:** "You build it, you run it." ✅
- **Shared services:** Non-existent. Developers handle everything using PaaS tools to minimize operational overhead. ✅

#### Phase 2: Need-Based Specialist Era (10–30 Engineers)
- **First dedicated specialists** introduced as technical debt or deployment complexity grows. ✅
- **Philosophy:** Support, don't gatekeep. ✅
- **Implementation:** Early specialists build **"paved roads"** — self-service tools for feature teams, not central clearinghouses that review every PR/deployment. ✅

#### Phase 3: Hybrid/Scale-Up Era (30–50+ Engineers)
- **Structure:** Pods/squads (focused on business outcomes) supported by a **Platform/Shared Services team.** ✅
- **Philosophy:** High-leverage enablement. Platform team provides standard tooling (CI/CD, monitoring, docs templates) so product squads remain autonomous. ✅

### When to Hire Specialists

| Role | When to Hire | Why |
|:---|:---|:---|
| **DevOps/SRE** | When infra/CI/CD/monitoring become significant distractions for product devs (downtime, scaling issues). ✅ | Provide standardized, automated foundations. |
| **QA Engineer** | When manual regression testing consumes large % of dev time, or production bugs become revenue/churn risk. ✅ | Shift from "manual testing" to "quality engineering" — automate test suites, build quality culture. |
| **Tech Writer** | When documentation complexity causes developers to spend excessive time answering questions or debugging. ✅ | Create self-service knowledge bases for rapid onboarding and product adoption. |

### The "Shared Services" Trap
- **The trap:** QA or DevOps acting as "gatekeepers" (reviewing, approving, executing work for other teams) → bottleneck that destroys agility. ✅
- **The solution:** Build **self-service platforms.** Developers provision environments using tools the DevOps team built — not by asking DevOps to do it. QA writes automation frameworks developers use — not manual testing of every feature. ✅

### Model: Shared Service or Fixed Chain?
- **NOT a fixed chain.** Shared services are invoked by anyone who needs them — not a mandatory step in every workflow. 🔮
- They function as **Platform/Enablement teams**: their success is measured by how much faster and more reliable they make *the rest of the team*, not by how much work they handle themselves. ✅
- **Hire for "Enablers":** First DevOps or QA hire should be an "Enablement Engineer" — measured by team velocity improvement, not personal throughput. ✅

---

## Summary Table: Seed → Series A Evolution

| Dimension | Seed (1–5 eng) | Series A (5–20+ eng) |
|:---|:---|:---|
| **Reporting** | Flat, all → CTO | EMs/VPE added, Tech Leads for pods |
| **CTO role** | Chief Coder, reviews all PRs | Strategic leader, reviews only critical path |
| **Code review** | CTO reviews everything | Senior → Junior, Tech Lead → Senior, peer review |
| **Git workflow** | GitHub Flow | GitHub Flow → Trunk-Based |
| **Who merges** | Author (or CTO if solo) | Author-led, CI-gated |
| **Post-review fixes** | Author fixes | Author fixes; pair for complex issues |
| **Design review** | Designer + PM + CTO | Designer + PM + Engineer; Design QA pass |
| **QA/DevOps/Docs** | Everyone does everything | Self-service platform teams, not gatekeepers |

---

## Sources

- <a id="ref-1"></a> [1] Engineering team structure evolution: Seed to Series A (aggregated from smithspektrum.com, marcgg.com, andrewchen.com, bvp.com, fcto.uk)
- <a id="ref-2"></a> [2] Startup code review roles and practices (aggregated from github.io, group107.com, meduzzen.com, medium.com, mergify.com, dev.to, startups.com)
- <a id="ref-3"></a> [3] CTO code review transition at Series A (aggregated from cto.la, dev.to, romainsimon.com, codacy.com, nyblom.io)
- <a id="ref-4"></a> [4] Squash merge and trunk-based development practices (aggregated from dadrian.io, flagsmith.com, arijitk.in, worktrunk.dev, stackoverflow.com, reddit.com, dev.to)
- <a id="ref-5"></a> [5] GitHub Flow vs Trunk-Based Development for startups (aggregated from medium.com, assembla.com, deployhq.com, mergify.com, dev.to)
- <a id="ref-6"></a> [6] Code review feedback: who fixes (aggregated from stackexchange.com, github.com, medium.com, graphite.com, legitsecurity.com)
