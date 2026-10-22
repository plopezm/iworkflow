"""Agente falso que habla un subconjunto del protocolo RPC de pi (JSONL)."""
import json
import os
import sys
import threading

EMIT_LOCK = threading.Lock()
# Prompt que deja un job en background corriendo y lo libera pasados `JOB_SECONDS`.
JOB_PROMPT = "jobs-wait"
JOB_STATUS_KEY = "gentle-jobs"
JOB_SECONDS = float(os.environ.get("FAKE_AGENT_JOB_SECONDS", "6"))


def emit(record):
    with EMIT_LOCK:
        sys.stdout.write(json.dumps(record) + "\n")
        sys.stdout.flush()


def set_jobs_status(text):
    emit({"type": "extension_ui_request", "id": "jobs", "method": "setStatus",
          "statusKey": JOB_STATUS_KEY, "statusText": text})


emit({"type": "extension_ui_request", "id": "boot", "method": "notify",
      "message": "args=" + " ".join(sys.argv[1:]) + " cwd=" + os.getcwd()
      + " host=" + os.environ.get("GENTLE_SHELL_INTERACTIVE_HOST", "")})

for raw in sys.stdin:
    command = json.loads(raw)
    kind = command.get("type")
    if kind == "prompt":
        emit({"id": command.get("id"), "type": "response", "command": "prompt", "success": True})
        emit({"type": "agent_start"})
        emit({"type": "message_update", "assistantMessageEvent": {"type": "text_delta", "delta": "eco"}})
        emit({"type": "message_end", "message": {"role": "assistant", "stopReason": "stop",
              "content": [{"type": "text", "text": "eco: " + command["message"]}]}})
        if command["message"] == JOB_PROMPT:
            set_jobs_status("\u29d7 1 job")
            emit({"type": "agent_settled"})
            threading.Timer(JOB_SECONDS, set_jobs_status, args=[None]).start()
            continue
        emit({"type": "extension_ui_request", "id": "dlg", "method": "confirm", "title": "¿Seguir?"})
        emit({"type": "agent_settled"})
    elif kind == "extension_ui_response":
        emit({"type": "extension_ui_request", "id": "ack", "method": "notify",
              "message": "respuesta " + json.dumps(command, sort_keys=True)})
    elif kind == "steer":
        emit({"id": command.get("id"), "type": "response", "command": "steer",
              "success": False, "error": "not streaming"})
