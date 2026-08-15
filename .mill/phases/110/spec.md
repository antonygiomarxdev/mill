# Spec: Clasificación de fallo — taxonomía de fallos de rol y de entorno

Cierra la duda abierta del CTO (#4 en `local/recursion-spec.md`): define qué significa
"fallar un rol" y la reacción proporcional por categoría. Requisito P0 para la
delegación recursiva (#109). Se escribe en `local/` al lado de
`recursion-spec.md` y `model-resilience-spec.md`.

## Architecture

### Principio general

Un **rol falla** cuando no cumple su contrato de salida. La falla se detecta por
**señales observables** y se clasifica en 4 categorías (lane de fallo de rol) más
el lane **E** (fallo de entorno, que NO es fallo de rol). Cada categoría mapea a
una reacción única y proporcional. La clasificación reemplaza el matching
empotrado de `classifyResult` (`internal/cli/delegate.go`) por un registro
declarativo; la reacción se ejecuta en `runDispatchLoop54`
(`internal/cli/review_loop.go`).

```
session result ──► SignalRegistry (signals.go) ──► domain.FailureClass
        │                 stderr | exit | timeout | heartbeat        OK|EXEC|CONTRACT|GATE|RESULT|ENV
        │                  │ (priority: stderr → exit → heartbeat → env-guard)
        ▼
classifyFailure(result) ──► FailureReactor ──► state transition + ledger + lessons
                              EXEC     → retry(MaxRetries) → escalate(parent)
                              CONTRACT → reject artifact → re-delegate fresh
                              GATE     → rework(same role+worktree, gate feedback)
                              RESULT   → parent correction loop (review→produce)
                              ENV      → abort, NO retry, preserve worktree+logs, notify
```

### 1. Capa de señales — SignalRegistry (nuevo: `internal/domain/signals.go`)

Tabla declarativa, versionada y auditable de señales observables → `FailureClass`.
Reemplaza el `strings.Contains` de `classifyResult` con una tabla de datos
inmutable. Resolución por prioridad: **stderr → exit code → heartbeat → env guard**.

| Observable (predicado) | Señal | FailureClass |
|---|---|---|
| exit 4/9/130/137/143 | `FATAL` / crash | EXECUTION_FAILURE |
| stderr `connection refused` / `network timeout` | transient provider/network | EXECUTION_FAILURE |
| exit −1/−2 + stderr `blocked: time budget` | timeout con heartbeat vivo | EXECUTION_FAILURE |
| exit 0 + artefacto vacío/placeholder/`TODO`/`TBD` | contrato de salida no cumplido | CONTRACT_FAILURE |
| exit 1 desde hook `gate-frd`/`gate-spec`/`gate-tasks` (stderr nombra la gate) | gate rechazó artefacto | GATE_FAILURE |
| stderr `CHANGES_REQUESTED:` con `[criterion: ...]` | reviewer: incorrecto factualmente | RESULT_FAILURE |
| heartbeat ausente > umbral + proceso aún activo | rol colgado (no crasheado) | EXECUTION_FAILURE (hung) |
| `git` ausente / binary provider ausente / slots agotados (ErrShutdown) | entorno roto | ENVIRONMENT_FAILURE |

Cada `Adapter` expone `FailureSignals() []Signal` para señales propias del
provider; el core registra la unión con prioridad, evitando el acoplamiento a
strings de un provider específico en el core.

### 2. Capa de clasificación — `classifyFailure` (evolución de `classifyResult`)

`classifyResult` pasa de `classifyResult(exitCode, stderr) Classification` a
`classifyFailure(result SessionResult) FailureClass`, donde
`SessionResult` (ya extendido por `waitWithBudget` con `ExitCode`, `Commits`,
`Output`, `Stderr`) **gana** `Duration`, `HeartbeatStaleness`, `ArtifactPath`.

`domain.FailureClass` (nuevo enum en `internal/domain/classification.go`) es
**compatible** con el enum existente: `FailureClassOf(Classification)` mapea
`FATAL/TRANSIENT/RATE_LIMITED → EXECUTION_FAILURE`, `BLOCKED → ENVIRONMENT_FAILURE`
(cuando la causa es slot/binary) o `EXECUTION_FAILURE` (cuando es budget-timeout),
y `CHANGES_REQUESTED → RESULT_FAILURE`. La inspección de artefacto (CONTRACT) se
aplica **después** de `classifyResult` sólo cuando la salida del proceso es OK.

**Heartbeat**: cada sesión activa escribe `<worktree>/.mill/heartbeat`
(timestamp + role, frontmatter `agent_id`). Un *monitor de heartbeat*
(corutina en `runDispatchLoop54` alrededor de `session.Wait()`) refresca a los
N ticks. Si el proceso sigue activo pero el heartbeat está estancado, la
clasificación distingue *hung* (EXECUTION_FAILURE, señal `heartbeat_stale`) de
*crasheado* — evitando el timeout genérico actual que no distingue un rol colgado
de uno terminado.

### 3. Capa de reacción — FailureReactor (integrado en `runDispatchLoop54`)

El `switch finalClassification` actual se reemplaza por un `switch
FailureClass` con las reacciones por categoría. Cada transición se graba en el
ledger con los nuevos campos `failure_class`, `phase`, `role`.

| FailureClass | Reacción | State transition |
|---|---|---|
| EXECUTION_FAILURE | Retry en cadena de modelos (model-chain fallback de `retryDispatch`). Agota `Config.MaxRetries` (default 4) → escala al rol padre (`delegates_to` de `ROLE.md`). | `running → retrying → (running \| escalated)` |
| CONTRACT_FAILURE | Rechaza el artefacto (no se acepta `output`/commits). Re-delega produce a un rol + sesión limpios. | `produce → rejected → produce` (nueva sesión) |
| GATE_FAILURE | Re-trabajo en el MISMO rol y worktree. Feed del stderr de la gate → produce. | `produce → gate_failed → rework` |
| RESULT_FAILURE | Loop de corrección del padre: review feedback `[criterion:...]` → produce. Tras `escalationThreshold` (3) reworks, escala a Staff. | `review → changes_requested → correction` |
| ENVIRONMENT_FAILURE | **Abort sin retry** (no advancea cadena de modelos). Preserva worktree + logs. Notifica motivo al final del run. | `running → aborted` (terminal, preserva) |

**Escalada a padre** (`routing_56.go`): `escalateToParent(issue, role)` valida
`delegates_to` vía `role.ParseFrontmatter` (igual que `validateDelegation`) y
re-delega el issue al siguiente rol en la cadena. La profundidad está acotada por
`Config.MaxDepth` (default = hoja del ORG-CHART) con hard-stop en Staff.

**ENVIRONMENT_FAILURE vs cleanup**: la política actual de `isIrrecoverable`
(FATAL/AUTH/NO_CREDIT → `cleanupWorktree`) se extiende: ENVIRONMENT_FAILURE
**NO** llama a `cleanupWorktree` (preserva); EXECUTION_FAILURE con retry NO
limpia (mantiene el worktree para el retry); sólo OK/`done` permite cleanup o
promoción a landing. `validateDelegateBinaries` (`delegate.go`) dispara
ENVIRONMENT_FAILURE (marca estado `aborted`, no retorno de error directo) cuando
el binario provider/binario ausente.

### 4. Consistencia de estado

`domain.Task` (`internal/domain/task.go`) gana `Phase` (`TaskPhase`:
`dispatch|produce|review|rework|rejected|gate_failed|aborted`) y `FailureClass`.
`UpdateStatus` → `Transition(phase, status, verdict, commits, failureClass)`.

- `state.Save()` (`.mill/state.json`) ya es atómico: write-temp + fsync +
  backup-rotation (`.1`/`.2` en `internal/state/state.go`). Cubre la
  atomicidad de fase + FailureClass como unidad.
- Ledger (`.mill/ledger/<issue>.jsonl`): `Entry` gana `FailureClass`, `Phase`,
  `Role`; `Append` es append-only JSONL (source of truth audit-able).
- `lessons.md` por rol (`.mill/lessons/<role>.md`): capta `FailureClass` +
  causa-raíz observable al finalizar; operación **append**, no rewrite, para
  no perder histórico.
- Orden de escritura: **ledger → state → lessons** (fail-fast en ledger implica
  estado en memoria no persistido; state reconstruible del ledger).
- **ENVIRONMENT_FAILURE preserva** el worktree (marca `.mill/aborted` →
  `mill clean --aborted` reclama worktrees huérfanos); resto de clases
  mantienen política actual (cleanup sólo en éxito terminal o abort
  irrecoverable).

## Components affected

### Dominio (`internal/domain`)
- `classification.go` — añadir enum `FailureClass`
  (`CLASS_OK | EXECUTION_FAILURE | CONTRACT_FAILURE | GATE_FAILURE |
  RESULT_FAILURE | ENVIRONMENT_FAILURE`) y `FailureClassOf(Classification) FailureClass`.
- `signals.go` (nuevo) — `SignalRegistry`: tabla declarativa
  `Signal{Predicates, FailureClass, Description}`; `Resolve(result) FailureClass`
  con prioridad stderr → exit → heartbeat → env.
- `status.go` — añadir `TaskPhase` (`TaskPhaseDispatch|TaskPhaseProduce|
  TaskPhaseReview|TaskPhaseRework|TaskPhaseRejected|TaskPhaseGateFailed|
  TaskPhaseAborted`), `TaskAborted` a `TaskStatus`, `Task.AbortReason`.
- `task.go` — añadir `Phase TaskPhase` y `FailureClass FailureClass` al struct;
  `UpdateStatus` → `Transition(phase, status, verdict, commits, fc)`.
- `session.go` — `SessionResult` gana `Duration`, `HeartbeatStaleness`,
  `ArtifactPath`; `Session.End` registra heartbeat final.

### Adapter (`internal/adapter`)
- `adapter.go` — `Adapter` gana `FailureSignals() []Signal`; `Session` gana
  `Heartbeat() <-chan struct{}` (o `HeartbeatPath() string`).
- `commandcode.go` — `liveSession` escribe `.mill/heartbeat` cada N ticks
  mientras `cmd` corre; `waitWithBudget` distingue timeout con heartbeat vivo
  (EXEC) de heartbeat ausente (hung).

### CLI / orquestación (`internal/cli`)
- `delegate.go` — `classifyResult` → `classifyFailure` usando `SignalRegistry`;
  `retryDispatch` cambia condición de advance de modelo de
  `RATE_LIMITED/TRANSIENT` → `EXECUTION_FAILURE`; `validateDelegateBinaries`
  marca estado `aborted` + ENVIRONMENT_FAILURE en lugar de retorno de error
  directo.
- `review_loop.go` — `runDispatchLoop54` integra el `FailureReactor`: el
  `switch finalClassification` pasa a `switch FailureClass` con las reacciones
  tabuladas; el monitor de heartbeat corre como goroutine alrededor de
  `session.Wait()`.
- `routing_56.go` — añadir `escalateToParent(issue, role)` que valida
  `delegates_to` y re-delega; `escalateTier`/`resolveModel` sin cambios.
- `slots.go`/`slot_delegate.go` — exhaución de slots → ENVIRONMENT_FAILURE
  (notifica "slots agotados") en vez de bloquear indefinidamente.

### Persistencia
- `internal/state/state.go` — guardado atómico ya implementado (`.tmp` + fsync
  + backup-rotation); sólo se añaden campos al task.
- `internal/ledger/ledger.go` — `Entry` gana `FailureClass`, `Phase`, `Role`;
  `Append` ya es append-only JSONL.

### Gates (worktree)
- `internal/cli/static/scaffold/.mill/checks/gate-{frd,spec,tasks}` — stderr
  que nombra la gate enruta a GATE_FAILURE (señal `gate-(frd|spec|tasks)`).

### Tests
- `internal/domain/classification_test.go` — cases para `FailureClassOf` +
  determinismo del `SignalRegistry`.
- `internal/cli/delegate_test.go` — `classifyFailure` por categoría; heartbeat
  stale → EXECUTION_FAILURE (hung).
- `internal/cli/review_loop_test.go` — reactor: retry→escalate,
  contract→re-delegate, gate→rework, env→abort+preserve+notify.

## Risks

1. **False positives de CONTRACT_FAILURE.** Inspicionar el artefacto (vacío/
   placeholder) puede clasificar mal un rol que produce output mínimo
   intencional. *Mitigación:* el contrato de artefacto se define por
   role-frontmatter (`artifact_contract:`) y el chequeo de placeholder usa una
   lista allowlist de strings conocidos (`TODO`, `placeholder`, `TBD`), no un
   "vacío estricto".

2. **Heartbeat introduce races de concurrencia.** Refrescar el heartbeat mientras
   `session.Wait()` puede haber terminado crea condiciones de carrera.
   *Mitigación:* el monitor usa `context.Done` + `sync.Mutex` sobre
   `HeartbeatStaleness`; se detiene el monitor antes de que `cmd.Wait()`
   retorne. Test con `-race`.

3. **ENVIRONMENT_FAILURE preserve ≠ leak.** Preservar el worktree en fallo de
   entorno evita corromper el trabajo, pero acumula worktrees huérfanos.
   *Mitigación:* se marca `.mill/aborted` y `mill clean --aborted` reclama; el
   usuario ve el motivo en la notificación final del run.

4. **Escalada a parent puede no converger.** Si cada rol intermedio escala al
   padre sin converger, la recursión no termina. *Mitigación:*
   `Config.MaxDepth` (default = hoja del ORG-CHART) con hard-stop en Staff +
   notificación al usuario.

5. **SignalRegistry rígido vs providers heterogéneos.** Los patrones de stderr
   son propios del adapter. *Mitigación:* `Adapter.FailureSignals()` permite al
   provider registrar su tabla; el core registra la unión con prioridad
   declarada.

6. **Consistencia cross-store no transaccional.** state.json es atómico, pero
   ledger (append) y lessons.md son operaciones distintas. *Mitigación:* orden
   ledger → state → lessons; lessons.md es *append* (no rewrite); invariante:
   ledger es source of truth audit-able, state es best-effort reconstruible
   del ledger.

7. **GATE_FAILURE vs CONTRACT_FAILURE overlap.** *Mitigación:* distinción de
   fase — GATE_FAILURE = "hook exit 1, artefacto existe pero falla el
   criterio de gate"; CONTRACT_FAILURE = "hook pasa pero artefacto es
   placeholder/empty". Prioridad: hook exit code → heartbeat/timeout →
   inspección de artefacto.
