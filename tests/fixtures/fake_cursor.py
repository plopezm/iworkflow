"""Agente falso que habla un subconjunto de ACP (`agent acp`)."""
import json
import os
import sys

session = {"id": None, "model": None, "loaded": None}


def emit(record):
    sys.stdout.write(json.dumps(record) + "\n")
    sys.stdout.flush()


def respond(request_id, result):
    emit({"jsonrpc": "2.0", "id": request_id, "result": result})


def update(payload):
    emit({"jsonrpc": "2.0", "method": "session/update",
          "params": {"sessionId": session["id"], "update": payload}})


def chunk(text):
    update({"sessionUpdate": "agent_message_chunk", "content": {"type": "text", "text": text}})


def read():
    return json.loads(sys.stdin.readline())


for raw in sys.stdin:
    message = json.loads(raw)
    method = message.get("method")
    request_id = message.get("id")
    params = message.get("params", {})
    if method == "initialize":
        respond(request_id, {"protocolVersion": 1, "agentCapabilities": {"loadSession": True}})
    elif method == "authenticate":
        respond(request_id, {})
    elif method == "session/new":
        session["id"] = "fake-1"
        respond(request_id, {"sessionId": "fake-1"})
    elif method == "session/load":
        session["id"] = params["sessionId"]
        session["loaded"] = params["sessionId"]
        chunk("historia que debe ignorarse")
        respond(request_id, {})
    elif method == "session/set_model":
        session["model"] = params["modelId"]
        respond(request_id, {})
    elif method == "session/cancel":
        respond(session.get("prompt_id"), {"stopReason": "cancelled"})
    elif method == "session/prompt":
        text = params["prompt"][0]["text"]
        session["prompt_id"] = request_id
        if text == "slow":
            chunk("pensando")
        elif text == "permission":
            emit({"jsonrpc": "2.0", "id": 77, "method": "session/request_permission",
                  "params": {"sessionId": session["id"],
                             "toolCall": {"toolCallId": "c1", "title": "Run", "kind": "execute",
                                          "rawInput": {"command": "ls"}},
                             "options": [{"optionId": "allow-once", "kind": "allow_once", "name": "Allow"},
                                         {"optionId": "reject-once", "kind": "reject_once", "name": "Reject"}]}})
            answer = read()
            chunk("chose=" + answer["result"]["outcome"]["optionId"])
            respond(request_id, {"stopReason": "end_turn"})
        elif text == "abandoned":
            # Pide permiso y cierra el turno sin esperar la respuesta (el usuario tardó).
            emit({"jsonrpc": "2.0", "id": 78, "method": "session/request_permission",
                  "params": {"sessionId": session["id"],
                             "toolCall": {"toolCallId": "c2", "title": "Run", "kind": "execute",
                                          "rawInput": {"command": "flutter pub get"}},
                             "options": [{"optionId": "allow-once", "kind": "allow_once", "name": "Allow"},
                                         {"optionId": "reject-once", "kind": "reject_once", "name": "Reject"}]}})
            chunk("turno cerrado")
            respond(request_id, {"stopReason": "end_turn"})
        elif text == "info":
            chunk("args=" + " ".join(sys.argv[1:]) + " cwd=" + os.getcwd()
                  + " model=" + str(session["model"]) + " loaded=" + str(session["loaded"]))
            respond(request_id, {"stopReason": "end_turn"})
        else:
            chunk("eco: " + text)
            respond(request_id, {"stopReason": "end_turn"})
