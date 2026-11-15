"""Demo agent for the documentation GIFs (scripts/docs/record_gifs.sh).

It speaks the subset of the pi RPC protocol the server decodes (JSONL on stdin/stdout) and, unlike the
smoke fakes, it really edits its working directory (the task worktree): for a prompt it creates
src/<slug>.js with a small helper and appends a line to README.md, so the diff of the task is real.
It uses no model and no network. Streams are paced with short sleeps so the live view reads like a run.
"""
import json
import pathlib
import re
import subprocess
import sys
import time

# Pacing of the recording: a word every STREAM_DELAY_S, a pause between tool steps of STEP_PAUSE_S.
STREAM_DELAY_S = 0.025
STEP_PAUSE_S = 0.3
STOPWORDS = frozenset({"a", "an", "the", "for", "to", "of", "and", "in", "on", "with", "add", "create", "make", "new", "some"})
README = pathlib.Path("README.md")


def emit(record):
    sys.stdout.write(json.dumps(record) + "\n")
    sys.stdout.flush()


def slug_of(prompt):
    words = [w for w in re.findall(r"[a-z0-9]+", prompt.lower()) if w not in STOPWORDS][:3]
    return "_".join(words) or "task"


def camel_of(slug):
    head, *tail = slug.split("_")
    return head + "".join(part.capitalize() for part in tail)


def stream(delta_type, text):
    for chunk in re.findall(r"\S+\s*", text):
        emit({"type": "message_update", "assistantMessageEvent": {"type": delta_type, "delta": chunk}})
        time.sleep(STREAM_DELAY_S)


def tool(name, args, run):
    call_id = f"call-{name}-{int(time.time() * 1000)}"
    emit({"type": "tool_execution_start", "toolCallId": call_id, "toolName": name, "args": args})
    time.sleep(STEP_PAUSE_S)
    result, is_error = run()
    emit({"type": "tool_execution_end", "toolCallId": call_id, "toolName": name, "isError": is_error,
          "result": {"content": [{"type": "text", "text": result}]}})
    time.sleep(STEP_PAUSE_S / 2)


def read_readme():
    return (README.read_text(encoding="utf-8") if README.exists() else "(no README.md)"), False


def write_module(path, title, camel):
    body = (
        f"// {title}\n"
        f"export function {camel}(input) {{\n"
        "  return String(input)\n"
        "    .trim()\n"
        "    .toLowerCase()\n"
        "    .replace(/[^a-z0-9]+/g, '-')\n"
        "    .replace(/^-+|-+$/g, '');\n"
        "}\n"
    )
    pathlib.Path(path).parent.mkdir(parents=True, exist_ok=True)
    pathlib.Path(path).write_text(body, encoding="utf-8")
    return f"wrote {path} ({len(body)} bytes)", False


def check_module(path):
    try:
        done = subprocess.run(["node", "--check", path], capture_output=True, text=True, timeout=20)
    except (OSError, subprocess.SubprocessError) as err:
        return f"node --check skipped: {err}", False
    if done.returncode != 0:
        return done.stderr.strip() or "syntax error", True
    return "syntax ok", False


def append_readme(path, title):
    line = f"- `{path}`: {title}\n"
    current = README.read_text(encoding="utf-8") if README.exists() else "# Project\n\n"
    if not current.endswith("\n"):
        current += "\n"
    README.write_text(current + line, encoding="utf-8")
    return f"appended 1 line to README.md", False


def handle_prompt(prompt):
    prompt = prompt.strip() or "the requested change"
    slug = slug_of(prompt)
    camel = camel_of(slug)
    module = f"src/{slug}.js"
    title = prompt.splitlines()[0][:90]

    emit({"type": "agent_start"})
    stream("thinking_delta", f"The task asks for: {title}. I will read the project before I change anything. ")
    tool("read", {"path": "README.md"}, read_readme)
    stream("thinking_delta", "The README is short. A small helper module keeps the change isolated and easy to review. ")
    stream("text_delta", f"I'll add `{camel}()` in `{module}` and list it in the README. ")
    tool("edit", {"path": module, "summary": f"create {camel}()"}, lambda: write_module(module, title, camel))
    tool("bash", {"command": f"node --check {module}"}, lambda: check_module(module))
    tool("edit", {"path": "README.md", "summary": "list the new module"}, lambda: append_readme(module, title))
    final = f"Done. `{camel}()` lives in `{module}`, the README lists it, and the syntax check passes."
    stream("text_delta", final)
    emit({"type": "message_end", "message": {"role": "assistant", "stopReason": "stop", "content": [
        {"type": "thinking", "thinking": "Small isolated change: a helper module plus one README line."},
        {"type": "text", "text": final},
    ]}})
    emit({"type": "agent_settled"})


def main():
    for raw in sys.stdin:
        raw = raw.strip()
        if not raw:
            continue
        try:
            command = json.loads(raw)
        except json.JSONDecodeError:
            continue
        kind = command.get("type")
        if kind == "prompt":
            emit({"id": command.get("id"), "type": "response", "command": "prompt", "success": True})
            handle_prompt(str(command.get("message", "")))
        elif kind == "abort":
            emit({"id": command.get("id"), "type": "response", "command": "abort", "success": True})
            emit({"type": "agent_settled"})
        elif kind in ("steer", "follow_up"):
            emit({"id": command.get("id"), "type": "response", "command": kind, "success": False,
                  "error": "not streaming"})


if __name__ == "__main__":
    main()
