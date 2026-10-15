"""Agente falso que habla un subconjunto del protocolo stream-json de `claude -p`."""
import json
import os
import sys


def emit(record):
    sys.stdout.write(json.dumps(record) + "\n")
    sys.stdout.flush()


def settle(**extra):
    emit({"type": "result", "subtype": "success", "is_error": False, **extra})


emit({"type": "system", "subtype": "init", "session_id": "s1"})
tool_id = "t1"

for raw in sys.stdin:
    message = json.loads(raw)
    kind = message.get("type")
    if kind == "user":
        text = message["message"]["content"]
        if text == "args":
            emit({"type": "assistant", "message": {"content": [{
                "type": "text",
                "text": "args=" + " ".join(sys.argv[1:]) + " cwd=" + os.getcwd()
                + " host=" + os.environ.get("GENTLE_SHELL_INTERACTIVE_HOST", "")}]}})
            settle()
        elif text == "tool":
            emit({"type": "assistant", "message": {"content": [{
                "type": "tool_use", "id": "t1", "name": "Bash", "input": {"command": "ls"}}]}})
            emit({"type": "control_request", "request_id": "r1", "request": {
                "subtype": "can_use_tool", "tool_name": "Bash", "display_name": "Bash",
                "description": "Lista", "input": {"command": "ls"}}})
            tool_id = "t1"
        elif text == "ask":
            emit({"type": "assistant", "message": {"content": [{
                "type": "tool_use", "id": "t2", "name": "AskUserQuestion", "input": {}}]}})
            emit({"type": "control_request", "request_id": "q1", "request": {
                "subtype": "can_use_tool", "tool_name": "AskUserQuestion",
                "display_name": "AskUserQuestion", "input": {"questions": [
                    {"question": "¿Color?", "header": "Color", "multiSelect": False, "options": [
                        {"label": "Rojo", "description": "cálido"}, {"label": "Azul", "description": ""}]},
                    {"question": "¿Cuántos?", "header": "", "multiSelect": False, "options": [
                        {"label": "Uno", "description": ""}, {"label": "Dos", "description": ""}]}]}}})
            tool_id = "t2"
        elif text == "slow":
            emit({"type": "stream_event", "event": {"type": "content_block_delta",
                  "delta": {"type": "text_delta", "text": "pensando"}}})
        else:
            emit({"type": "stream_event", "event": {"type": "content_block_delta",
                  "delta": {"type": "text_delta", "text": "eco"}}})
            emit({"type": "assistant", "message": {"content": [{"type": "text", "text": "eco: " + text}]}})
            settle()
    elif kind == "control_response":
        decision = message["response"]["response"]
        emit({"type": "user", "message": {"content": [{
            "type": "tool_result", "tool_use_id": tool_id, "is_error": False,
            "content": decision["behavior"] + " " + json.dumps(decision.get("updatedInput"), sort_keys=True, ensure_ascii=False)}]}})
        settle()
    elif kind == "control_request" and message["request"]["subtype"] == "interrupt":
        emit({"type": "control_response", "response": {
            "subtype": "success", "request_id": message["request_id"], "response": {"still_queued": []}}})
        emit({"type": "result", "subtype": "error_during_execution", "is_error": True,
              "terminal_reason": "aborted_streaming"})
