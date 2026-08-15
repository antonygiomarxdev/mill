# FRD: Delegación recursiva — delegación automática en cadena de mando

## User need

El usuario delega un objetivo y lo recibe resuelto de extremo a extremo, sin orquestar niveles manualmente. Hoy Mill delega en un solo nivel: el loop produce-review es de un único nivel y cada salto de rol (Staff → Architect → Tech Lead → Sr Dev) exige intervención manual. El usuario quiere confiar la meta a Mill y que la cadena de mando complete recursivamente el flujo PM → FRD → Architect → spec(s) → Tech Lead → tasks → Sr Dev → implementación, con revisión en cada nivel, hasta la hoja ejecutora.

## Functional requirements

1. **Recursión automática siguiendo la cadena de mando.** Quien posee el issue delega automáticamente al siguiente rol según ORG-CHART, sin saltos de nivel (Staff nunca delega directo a un Sr Dev). La recursión continúa sin intervención humana hasta que no queden roles a los que delegar.

2. **Tope en la hoja ejecutora.** La recursión termina en el "peón" (rol hoja que no delega: Sr Dev, QA). Se prohíbe la recursión infinita; el rol hoja es la condición de término.

3. **Flujo de criterios por fase.** Los artefactos fluyen como contratos: PM produce FRD (gate-frd), Architect produce spec(s) basados en el FRD (gate-spec), Tech Lead produce tasks (gate-tasks), Sr Dev implementa. Cada fase valida la salida de la anterior.

4. **Vista de usuario configurable.** El usuario elige ver solo el resultado final o el árbol completo de delegaciones (niveles, roles, artefactos intermedios). La configuración vive en mill.yml.

5. **Revisión por nivel.** Cada rol revisa la salida del rol al que delegó antes de aceptarla; "hecho" en un nodo exige cumplir los criterios de su fase (frd → spec → tasks).

6. **Aprendizaje por rol.** Cada nivel escribe logs por nivel y un lessons.md por rol, para detectar el gap por rol y alimentar la siguiente iteración. Los logs y lessons se preservan en el worktree.

7. **Binario disponible en el worktree.** Antes de delegar a un worktree hijo, Mill copia el binario mill al worktree; sin binario, la recursión no puede arrancar.

8. **Recursión consciente de slots.** El sistema de slots (máx. 4) es consciente de la recursión: un worktree hijo ocupa su propio slot y no agota los slots del padre sin control; la cola respeta la concurrencia configurada.

9. **Modelo de costos por nivel.** Los niveles "que piensan" usan tier pro y los "que ejecutan" tier cheap, según ORG-CHART y mill.yml. La recursión asigna el modelo según el rol, no según la profundidad.

## Out of scope

- Profundidad arbitraria más allá del org chart: la cadena está limitada a los roles definidos en ORG-CHART.
- Editar la cadena en pleno vuelo: el usuario no reordena ni salta niveles una vez iniciada la delegación.
- Ramas paralelas y fan-out ilimitado: la concurrencia queda limitada por el slot manager existente.
- Observabilidad del progreso de cadenas largas (#105): pendiente, no se construye aquí.
- Auto-reparación de fallos de rol: la política de fallo se define en el FRD de Clasificación de fallo (#110).

## Priority

**P0** — desbloquea el propósito central de Mill. Sin recursión, Mill es un runner de un solo nivel; la cadena de mando (Staff → Architect → Tech Lead → Sr Dev) existe en ORG-CHART pero no puede ejecutarse sin orquestación manual. Todo el pipeline posterior (specs, tasks, implementación) depende de esta capacidad.
