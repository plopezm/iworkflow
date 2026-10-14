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
      background: cssVar("--c-bg", "#09090b"),
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
    var ws = null;
    var disposed = false;
    var everOpened = false;
    var openedAt = 0;
    var quickFailures = 0;
    var retryTimer = null;

    function sendResize() {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    }

    // El navegador móvil suspende y mata el socket al minimizar, pero la sesión del
    // servidor sigue viva: se reconecta y el servidor reenvía el scrollback.
    function connect() {
      var socket = new WebSocket(socketUrl(sessionId, term.cols, term.rows));
      socket.binaryType = "arraybuffer";
      ws = socket;
      socket.onmessage = function (event) {
        if (event.data instanceof ArrayBuffer) {
          term.write(new Uint8Array(event.data));
        }
      };
      var opened = false;
      socket.onopen = function () {
        opened = true;
        // En una reconexión el scrollback se reenvía completo: se limpia para no duplicarlo.
        if (everOpened) term.reset();
        everOpened = true;
        openedAt = Date.now();
        sendResize();
      };
      socket.onclose = function () {
        if (disposed || ws !== socket) return;
        // Sin abrir (sesión inexistente) o abierto un instante (shell terminada): no se
        // reintenta sin fin. Una caída tras uso normal sí se reintenta.
        var failed = !opened || Date.now() - openedAt < 2000;
        quickFailures = failed ? quickFailures + 1 : 0;
        if (document.hidden && !opened) {
          quickFailures = 0;
          return;
        }
        if (quickFailures >= 3 || (!everOpened && failed)) {
          term.write("\r\n[session closed]\r\n");
          return;
        }
        scheduleReconnect(500 * (quickFailures + 1));
      };
    }

    function scheduleReconnect(delay) {
      if (disposed || retryTimer !== null) return;
      retryTimer = setTimeout(function () {
        retryTimer = null;
        connect();
      }, delay);
    }

    // Al volver a primer plano o recuperar red, reconecta de inmediato si el socket murió.
    function ensureConnected() {
      if (disposed || document.hidden) return;
      if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return;
      if (retryTimer !== null) {
        clearTimeout(retryTimer);
        retryTimer = null;
      }
      quickFailures = 0;
      connect();
    }
    document.addEventListener("visibilitychange", ensureConnected);
    window.addEventListener("online", ensureConnected);
    window.addEventListener("pageshow", ensureConnected);
    connect();

    var dataSub = term.onData(function (data) {
      if (ws && ws.readyState === WebSocket.OPEN) {
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
      cleanup: function () {
        disposed = true;
        if (retryTimer !== null) clearTimeout(retryTimer);
        document.removeEventListener("visibilitychange", ensureConnected);
        window.removeEventListener("online", ensureConnected);
        window.removeEventListener("pageshow", ensureConnected);
        observer.disconnect();
        themeObserver.disconnect();
        dataSub.dispose();
        resizeSub.dispose();
        if (ws) ws.close();
        term.dispose();
      },
    };
  }

  var KEYS = {
    tab: "\t",
    ctrl_c: "\x03",
    up: "A",
    down: "B",
    right: "C",
    left: "D",
  };

  // Envía una tecla con nombre como si se hubiera pulsado en el teclado.
  function sendKey(handle, key) {
    if (!handle || !handle.term || !Object.prototype.hasOwnProperty.call(KEYS, key)) return;
    var seq = KEYS[key];
    if (seq.length === 1 && seq >= "A" && seq <= "D") {
      var app = handle.term.modes && handle.term.modes.applicationCursorKeysMode;
      seq = (app ? "\x1bO" : "\x1b[") + seq;
    }
    handle.term.input(seq, true);
    handle.term.focus();
  }

  function close(handle) {
    if (handle && handle.cleanup) {
      handle.cleanup();
      handle.cleanup = null;
    }
  }

  window.iwfTerminal = { open: open, close: close, sendKey: sendKey };
})();
