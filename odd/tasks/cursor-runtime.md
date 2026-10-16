# Feature: cursor-runtime

Integrar el agente de Cursor (`agent`) como un runtime más, hablando ACP (`agent acp`, JSON-RPC 2.0 por stdio).
Branch: `pi/integrar-con-cursor-3d929354-8150-4da3-8a38-8e010612a9af`.

## Specs

- S1. "integrarlo como un adaptador de agente más a la aplicación" — nuevo `AgentRuntime::Cursor` (id `cursor`) registrado en el router y en el selector de runtime.
- S2. "Hay que ver cómo podemos integrarlo y si tiene servidor RCP" — adaptador de proceso por tarea que lanza `agent acp` y habla ACP (no la TUI).
- S3. "El códec mantiene una máquina de estados y encola el Prompt hasta que session/new responde." — el `LineCodec` de ACP envía `initialize → authenticate → session/new` solo y retiene los prompts hasta tener `sessionId`.
- S4. "acceptEdits como politica" — `session/request_permission` de herramientas de edición se aprueba (`allow-once`); el resto se muestra como diálogo de confirmación.
- S5. Decoder ACP → `AgentEvent` (verificado en vivo): `agent_message_chunk` → `TextDelta`, `agent_thought_chunk` → `ThinkingDelta`, `tool_call`/`tool_call_update` → `ToolStarted`/`ToolFinished`, respuesta de `session/prompt` (`stopReason`) → `RunSettled`; `cursor/ask_question` y `cursor/create_plan` → diálogos.
- S6. Capacidades: dialogs, resume, model_selection (`session/set_model`); sin steer, follow_up, thinking_levels ni subagents.
- S7. Binario configurable (`IWF_CURSOR_BIN`, por defecto `agent`) y mismo lanzamiento que pi/claude (shell de login, entorno limpio, límites, parada ordenada).
- S8. "Te he instalado agent de cursor con un login hecho. Si tienes problemas usa la documentación" — la autenticación usa el login existente (`cursor_login`).

## Tasks

- [ ] T1 (S3) `LineCodec`: hooks `start` (líneas iniciales) y salida generada al decodificar (respuestas/handshake), con test en `agent_process`. — route: inline — commit
- [ ] T2 (S3, S4, S5) `cursor/acp_protocol.rs`: máquina de estados, decode, encode, permisos y diálogos; tests con la captura real `tests/fixtures/cursor_acp_session.jsonl`. — route: inline — commit
- [ ] T3 (S2, S6, S7, S8) `CursorAcpRuntimeAdapter` + almacén de sesión para resume + test de integración con script falso. — route: inline — commit
- [ ] T4 (S1, S6, S7) `AgentRuntime::Cursor`, config, DI, catálogo de modelos, selector. — route: inline — commit
- [ ] T5 Verificación: fmt, clippy ssr/hydrate, tests; e2e real mínima con `agent acp`. — route: inline — commit

## Log

- L1. Cursor tiene un agente, llamado “agent”. Quiero integrarlo como un adaptador de agente más a la aplicación. Hay que ver cómo podemos integrarlo y si tiene servidor RCP. En caso de que no tenga rpc, se podría implementar de alguna forma como una terminal que llama a ese ejecutable y se use como una TUI embebida? Podría ser una alternativa
- L2. 1: El códec mantiene una máquina de estados y encola el Prompt hasta que session/new responde. 2: acceptEdits como politica. Te he instalado agent de cursor con un login hecho. Si tienes problemas usa la documentación
- L3. (evidencia) Captura real: `initialize` devuelve `loadSession:true`; `session/new` tarda 3-8 s y devuelve `models.availableModels[{modelId,name}]`; `session/set_model {sessionId,modelId}` responde `{}`; `session/cancel` es notificación (como petición da -32601); en modo agent no hubo `request_permission` en `ls`/edición; `session/list` no responde (no usable) → resume con sessionId persistido; tras la captura la cuenta devolvió «Upgrade your plan to continue».
