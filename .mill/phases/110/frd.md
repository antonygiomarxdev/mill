# FRD: Clasificación de fallo — taxonomía de fallos de rol y de entorno

## User need

Cuando una delegación falla, el usuario necesita saber POR QUÉ falló — si fue el rol o el entorno — y que Mill reaccione correctamente en cada caso. Hoy no existe una definición de "fallar un rol": cualquier fallo se trata igual, sin distinguir un crash de un artefacto vacío, un gate fallido de un proveedor caído. El usuario quiere diagnósticos precisos y reacciones proporcionales (reintentar, re-trabajar, escalar o abortar preservando el trabajo), en lugar de reintentos ciegos o abandono silencioso.

## Functional requirements

1. **Taxonomía de 4 categorías + lane de entorno.** Un rol falla cuando no cumple su contrato de salida. Se clasifica en: (1) fallo de ejecución, (2) fallo de contrato, (3) fallo de gate, (4) fallo de resultado; más el lane (E) de fallo de entorno, que NO es fallo de rol.

2. **Detección por señales.** Cada categoría se detecta por señales observables: patrones de stderr, códigos de salida, timeouts y ausencia de heartbeat. Los patrones se registran de forma central para que la clasificación sea determinista y auditable.

3. **Fallo de ejecución → reintentar/escalar.** Crash, timeout o proveedor caído. Mill reintenta un número acotado de veces y, agotados los reintentos, escala al rol padre.

4. **Fallo de contrato → rechazar/re-delegar.** Artefacto incorrecto (vacío, placeholder, o del rol equivocado). Mill rechaza el artefacto y re-delega la tarea, sin aceptar la salida.

5. **Fallo de gate → re-trabajo en el mismo rol.** El artefacto existe pero no pasa el gate de su fase (frd/spec/tasks). El mismo rol re-trabaja contra los criterios del gate.

6. **Fallo de resultado → loop de corrección del padre.** El artefacto pasa el gate pero es factualmente incorrecto. El rol padre lo detecta en su revisión e inicia el loop de corrección con el productor.

7. **Fallo de entorno → abortar sin reintento, preservar el worktree, notificar al usuario.** Upstream caído, binario ausente o slots agotados. No es culpa del rol: se aborta sin reintento, se preservan el worktree y los logs, y se notifica al usuario con el motivo.

8. **Heartbeat.** Cada rol en ejecución emite un heartbeat. La ausencia de heartbeat distingue un rol colgado de un rol fallado y dispara la clasificación correcta en lugar de un timeout genérico.

9. **Consistencia de estado.** La clasificación y la reacción transicionan el issue por una máquina de estados consistente; el worktree, los logs por nivel y el lessons.md del rol quedan en un estado coherente tras cualquier reacción, sin corromper artefactos intermedios.

## Out of scope

- Alertas push (email, Slack, webhook): la notificación es en la salida de Mill, no un sistema de alertas externo.
- Analítica de costos por fallo: medir el costo de reintentos y re-trabajos es futuro.
- Auto-reparación más allá del reintento acotado: Mill no repara por sí mismo un proveedor caído ni regenera slots.
- Diagnóstico de causa raíz profundo de un fallo de resultado: el padre corrige el resultado; no se construye un depurador de razonamiento.

## Priority

**P0** — define qué significa "fallar un rol", la única duda abierta del CTO para cerrar la política de fallo. Sin esta taxonomía, la recursión (#109) no puede decidir qué hacer cuando un nivel intermedio falla: reintenta a ciegas, abandona o corrompe el trabajo. Es prerrequisito de una delegación recursiva confiable.
