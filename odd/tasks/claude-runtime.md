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

## Tasks

X — route: inline — commit
- [x] T2 (S3) `claude/stream_protocol.rs`: decode con tests de cada tipo de línea. — route: inline — commit
- [x] T3 (S4) Encoder de comandos y respuestas de control con tests. — route: inline — commit
- [ ] T4 (S2, S5, S6) `ClaudeStreamRuntimeAdapter` (proceso, capacidades, resume, stop) + test de integración con script falso. — route: inline — commit
- [ ] T5 (S5, S8) Catálogo de modelos de claude + docs/ayuda; verificación e2e real mínima. — route: inline — commit

## Log

- L1. Cursor tiene un agente, llamado “claude”. Quiero integrarlo como un adaptador de agente más a la aplicación. Hay que ver cómo podemos integrarlo y si tiene servidor RCP. En caso de que no tenga rpc, se podría implementar de alguna forma como una terminal que llama a ese ejecutable y se use como una TUI embebida? Podría ser una alternativa. A ver que podemos hacer
- L2. Si podemos integrarlo igual que pi, pero usando su propio protocolo de Claude, adelante, más limpio
- L3. (evidencia) Sondeo real: `can_use_tool` llega como `control_request`; `control_response` allow ejecuta la herramienta; `interrupt` devuelve `control_response` success y el turno termina con `result` `error_during_execution`/`aborted_streaming`.

- L4. Para usar Claude hay que seleccionarlo en el selector del menú izquierdo principal, que por defecto es pi, al elegirlo pasamos a usar el adapter de Claude en lugar del de pi

## Evidence

- T1: RED observado (no compilaba `AgentRuntime::Claude`); 60 tests agent_settings ok; clippy ssr + hydrate ok; fmt ok.
- T2+T3: RED observado (16/18 fallaban con implementación vacía); 18 tests `claude::stream_protocol` ok; clippy ssr y fmt ok. Decoder y encoder comparten `ClaudeProtocol` (estado de permisos), por eso un solo commit. Steer/follow-up se codifican como mensaje de usuario pero no se declaran capacidades hasta validar en T5.
