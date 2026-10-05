// Glue entre xterm.js y el WebSocket `/terminal/ws/session/{sessionId}`.
// Requiere `Terminal` (xterm.js) y `FitAddon.FitAddon` (addon-fit.js) ya cargados.
(function () {
  "use strict";

  function cssVar(name, fallback) {
    var value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback;
  }

  function theme() {
    return {
      background: cssVar("--c-surface", "#0d0f14"),
      foreground: cssVar("--c-fg", "#f3f6f9"),
      cursor: cssVar("--c-accent", "#7fb4ca"),
      selectionBackground: cssVar("--c-surface-3", "#232a40"),
    };
  }

  function socketUrl(sessionId, cols, rows) {
    var scheme = location.protocol === "https:" ? "wss:" : "ws:";
    return (
      scheme + "//" + location.host + "/terminal/ws/session/" + encodeURIComponent(sessionId) +
      "?cols=" + cols + "&rows=" + rows
    );
  }

  function open(el, sessionId) {
    var term = new window.Terminal({
      cursorBlink: true,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 14,
      theme: theme(),
    });
    var fit = new window.FitAddon.FitAddon();
    term.loadAddon(fit);
    term.open(el);
    fit.fit();

    var encoder = new TextEncoder();
    var ws = new WebSocket(socketUrl(sessionId, term.cols, term.rows));
    ws.binaryType = "arraybuffer";

    function sendResize() {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    }

    ws.onmessage = function (event) {
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data));
      }
    };
    ws.onopen = sendResize;
    ws.onclose = function () {
      term.write("\r\n[session closed]\r\n");
    };

    var dataSub = term.onData(function (data) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(encoder.encode(data));
      }
    });
    var resizeSub = term.onResize(sendResize);

    var observer = new ResizeObserver(function () {
      try {
        fit.fit();
      } catch (_) {}
    });
    observer.observe(el);

    var themeObserver = new MutationObserver(function () {
      term.options.theme = theme();
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    term.focus();
    return {
      term: term,
      ws: ws,
      cleanup: function () {
        observer.disconnect();
        themeObserver.disconnect();
        dataSub.dispose();
        resizeSub.dispose();
        ws.onclose = null;
        ws.close();
        term.dispose();
      },
    };
  }

  function close(handle) {
    if (handle && handle.cleanup) {
      handle.cleanup();
      handle.cleanup = null;
    }
  }

  window.iwfTerminal = { open: open, close: close };
})();
