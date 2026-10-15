# Feature: claude-runtime

Integrar Claude Code (`claude`) como un runtime de agente más, igual que pi pero con su propio protocolo stream-json.
Branch: `pi/integrar-claude-code-dc7e3f52-dabb-4e91-ad81-5b4bec02ab8c`.

## Specs

- S1. "integrarlo como un adaptador de agente más a la aplicación" — nuevo `AgentRuntime::Claude` (id `claude`) registrado en el router junto a gentle-shell y pi.
- S2. "Si podemos integrarlo igual que pi, pero usando su propio protocolo de Claude" — adaptador de proceso por tarea (stdio) que habla `claude -p --input-format stream-json --output-format stream-json --verbose --include-partial-messages --permission-prompt-tool stdio`, no el RPC de pi.
- S3. Decoder claude → `AgentEvent` (verificado en sondeo): `system/init`, `stream_event` (`text_delta`, `thinking_delta`), `assistant` (`text`/`tool_use`), `user` con `tool_result`, `control_request/can_use_tool` → `DialogRequested(Confirm)`, `result` → `RunSettled`.
- S4. Encoder: prompt = `{"type":"user","message":{"role":"user","content":...}}` (con imágenes como bloques); abort = `control_request` subtype `interrupt`; respuesta de diálogo = `control_response` `{"subtype":"success","request_id":..,"response":{"behavior":"allow","updatedInput":<input>}}` (deny: `{"behavior":"deny","message":..}`).
- S5. Capacidades: resume (`--resume <session_id>`/`--continue`), model_selection (`--model`), thinking_levels (`--effort`), dialogs; steer/follow_up/subagents según validación (si no, no declarar).
- S6. Mismo lanzamiento que pi: shell de login, entorno limpio, `systemd-run` con límites, parada ordenada, `kill_on_drop`.
- S7. Binario configurable (`claude_bin`, por defecto `claude`) como `pi_bin`.
- S8. "Para usar Claude hay que seleccionarlo en el selector del menú izquierdo principal, que por defecto es pi, al elegirlo pasamos a usar el adapter de Claude en lugar del de pi" — Claude aparece en ese selector de runtime y, al elegirlo, las tareas lanzadas usan `ClaudeStreamRuntimeAdapter` (no el de pi).
- S9. "Permisos: acceptEdits si." — Claude se lanza con `--permission-mode acceptEdits` (las ediciones de ficheros se aceptan; el resto de herramientas pide confirmación por diálogo).
- S10. "Coge los AGENTS.md del proyecto?" — `claude` carga el `AGENTS.md` del worktree. Verificado en vivo: con solo `AGENTS.md` en el directorio de trabajo obedeció su regla. No requiere código; el repo no tiene `CLAUDE.md`.
- S11. "Añade los diálogos." — `AskUserQuestion` (preguntas con opciones, `multiSelect`) se muestra como diálogos `Select` y su respuesta vuelve en `answers`; `ExitPlanMode` se muestra como confirmación con el plan; los permisos muestran el comando/fichero real, no solo la descripción del modelo.
- S12. "Podemos arreglar los niveles de thinking?" — la UI ofrece solo los niveles que el runtime elegido admite (Claude: low, medium, high, xhigh, max; pi/gentle-shell: todos) y un ajuste guardado incompatible con el runtime no se aplica.

## Tasks

X — route: inline — commit
- [x] T2 (S3) `claude/stream_protocol.rs`: decode con tests de cada tipo de línea. — route: inline — commit
- [x] T3 (S4) Encoder de comandos y respuestas de control con tests. — route: inline — commit
- [x] T4a (S6) Extraer núcleo común `agent_process` (`ProcessRuntime` + `LineCodec`) y migrar pi; test de integración de pi como red de seguridad.
- [x] T4b (S2, S5, S6, S8) `ClaudeStreamRuntimeAdapter` (proceso, capacidades, resume, stop) + test de integración con script falso. — route: inline — commit
- [x] T5 (S5, S8) Catálogo de modelos de claude + docs/ayuda; verificación e2e real mínima. — route: inline — commit
- [ ] T6 (S9) `--permission-mode acceptEdits` en los argumentos base. — route: inline — commit
- [ ] T7 (S10) Documentar y dejar constancia de que `claude` lee `AGENTS.md` (verificado en vivo). — route: inline — commit
- [ ] T8 (S11) Diálogos de Claude: `AskUserQuestion` → `Select` (varias preguntas, `multiSelect`), `ExitPlanMode` → confirmación del plan, permisos con el comando/fichero real; `LineCodec::encode` devuelve `Option` para respuestas parciales. — route: inline — commit
- [ ] T9 (S12) Niveles de thinking por runtime: dominio `ThinkingLevel` por runtime, server fn con runtime, `UpdateAgentSettings`/`PrepareLaunch` descartan niveles incompatibles, el selector se actualiza al cambiar de runtime. — route: gentle-ai-worker (superficie `src/modules/agent_settings/**`) — commit

## Log

- L1. Cursor tiene un agente, llamado “claude”. Quiero integrarlo como un adaptador de agente más a la aplicación. Hay que ver cómo podemos integrarlo y si tiene servidor RCP. En caso de que no tenga rpc, se podría implementar de alguna forma como una terminal que llama a ese ejecutable y se use como una TUI embebida? Podría ser una alternativa. A ver que podemos hacer
- L2. Si podemos integrarlo igual que pi, pero usando su propio protocolo de Claude, adelante, más limpio
- L3. (evidencia) Sondeo real: `can_use_tool` llega como `control_request`; `control_response` allow ejecuta la herramienta; `interrupt` devuelve `control_response` success y el turno termina con `result` `error_during_execution`/`aborted_streaming`.

- L4. Para usar Claude hay que seleccionarlo en el selector del menú izquierdo principal, que por defecto es pi, al elegirlo pasamos a usar el adapter de Claude en lugar del de pi
- L5. Permisos: acceptEdits si. Coge los AGENTS.md del proyecto? Añade los diálogos. Podemos arreglar los niveles de thinking?
- L6. (evidencia) Sondeos en vivo: `AskUserQuestion` llega como `control_request` `can_use_tool` con `input.questions[{question,header,options[{label,description}],multiSelect}]` y se responde con `allow` + `updatedInput{...input, answers:{<pregunta>:<etiqueta>}}`; `ExitPlanMode` llega con `input.plan` y `planFilePath`; `AGENTS.md` se obedece sin `CLAUDE.md`.

## Evidence

- T1: RED observado (no compilaba `AgentRuntime::Claude`); 60 tests agent_settings ok; clippy ssr + hydrate ok; fmt ok.
- T2+T3: RED observado (16/18 fallaban con implementación vacía); 18 tests `claude::stream_protocol` ok; clippy ssr y fmt ok. Decoder y encoder comparten `ClaudeProtocol` (estado de permisos), por eso un solo commit. Steer/follow-up se codifican como mensaje de usuario pero no se declaran capacidades hasta validar en T5.
- T4a: línea base previa 12/12 en `pi_rpc_runtime_adapter_test`; tras el refactor 12/12, 546 tests lib `modules::tasks`, clippy ssr y `--all-targets` ok. Corregidos 2 `cmp_owned` preexistentes en tests de project_tree (commit aparte).
- T4b: RED observado (el test de integración no compilaba sin el adaptador); 8/8 `claude_stream_runtime_adapter_test` (agente falso `fake_claude.py`: eco, args, permiso allow/deny, abort, diálogo desconocido, capacidades), 549 tests lib tasks, clippy ssr `--all-targets` + hydrate, fmt ok. Un test inicial comprobaba flags que `with_command` sustituye; se movió al unitario `default_args_run_claude_in_stream_json_mode`.
- T5: RED observado (efecto `--effort off|minimal` no soportado por claude y catálogo vacío); mapeo off→sin flag, minimal→low; catálogo estático `fable/opus/sonnet/haiku`; README y ARCHITECTURE actualizados. E2E real con `claude` (haiku): pide permiso `can_use_tool`, se confirma, Bash escribe `a.txt`="hola", un único `RunStarted`.
- Verificación independiente (`gentle-ai-verify`, solo lectura): S2-S8 cubiertos, sin bloqueantes. Hallazgo importante corregido con TDD: el texto de `thinking` se perdía (el bloque final llega vacío; ahora se acumula de los deltas y se adjunta al siguiente mensaje o herramienta). Corregido también: `result` limpia permisos pendientes/estado. Aceptados sin cambio: `RunStarted` sale con el primer delta (no al enviar), `stop` con permiso pendiente espera el timeout, pi borra la instantánea de subagentes antes del spawn.
- Cierre final: fmt ok; clippy ssr `--all-targets` y hydrate ok; 958 tests lib `modules::` ok; `claude_stream_runtime_adapter_test` 8/8 (+1 e2e real ignorado, ejecutado aparte ok); `pi_rpc_runtime_adapter_test` 12/12. Sin verificar: UI del selector en navegador (solo código), `--continue` con `--replay-user-messages` (reemisión de historial), steer/follow-up/subagentes de claude (no declarados).
