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

  // xterm.js no desplaza con el dedo: su `.xterm-screen` tapa el viewport nativo. Se traduce
  // el arrastre vertical a líneas (con inercia). En la pantalla alternativa (TUIs) se emite
  // un evento de rueda sobre el terminal para que xterm lo convierta en eventos de ratón.
  var LONG_PRESS_MS = 450;

  function enableTouchScroll(term, el) {
    var lastY = 0;
    var startY = 0;
    var lastTime = 0;
    var velocity = 0;
    var accumulatedDelta = 0;
    var dragging = false;
    var scrolled = false;
    var raf = 0;
    var pressTimer = null;
    var selecting = false;
    var anchor = 0;

    // Celda (índice lineal en el buffer) bajo un punto de la pantalla.
    function cellAt(touch) {
      var screen = el.querySelector(".xterm-screen") || el;
      var rect = screen.getBoundingClientRect();
      var col = Math.floor(((touch.clientX - rect.left) / (rect.width || 1)) * term.cols);
      var row = Math.floor(((touch.clientY - rect.top) / (rect.height || 1)) * term.rows);
      col = Math.min(Math.max(col, 0), term.cols - 1);
      row = Math.min(Math.max(row, 0), term.rows - 1);
      return (term.buffer.active.viewportY + row) * term.cols + col;
    }

    function selectRange(a, b) {
      var from = Math.min(a, b);
      term.select(from % term.cols, Math.floor(from / term.cols), Math.abs(a - b) + 1);
    }

    function cancelPress() {
      if (pressTimer !== null) clearTimeout(pressTimer);
      pressTimer = null;
    }

    function lineHeight() {
      var screen = el.querySelector(".xterm-screen");
      var height = screen ? screen.clientHeight : 0;
      return height > 0 && term.rows > 0 ? height / term.rows : 16;
    }

    function scrollBy(pixels) {
      var lh = lineHeight();
      accumulatedDelta += pixels;
      var lines = Math.trunc(accumulatedDelta / lh);
      if (lines === 0) return;
      accumulatedDelta -= lines * lh;

      if (term.buffer.active.type === "alternate") {
        var target = el.querySelector(".xterm") || el.querySelector(".xterm-screen") || el;
        var rect = target.getBoundingClientRect();
        target.dispatchEvent(
          new WheelEvent("wheel", {
            deltaY: lines * lh,
            deltaMode: 0,
            clientX: rect.left + rect.width / 2,
            clientY: rect.top + rect.height / 2,
            bubbles: true,
            cancelable: true,
          })
        );
      } else {
        term.scrollLines(lines);
      }
    }

    function stopFling() {
      if (raf) cancelAnimationFrame(raf);
      raf = 0;
    }

    function fling() {
      velocity *= 0.94;
      if (Math.abs(velocity) < 0.05) {
        raf = 0;
        return;
      }
      scrollBy(velocity * 16);
      raf = requestAnimationFrame(fling);
    }

    function onStart(event) {
      if (event.touches.length !== 1) return;
      stopFling();
      dragging = true;
      scrolled = false;
      velocity = 0;
      accumulatedDelta = 0;
      startY = lastY = event.touches[0].clientY;
      lastTime = event.timeStamp;
      selecting = false;
      cancelPress();
      var touch = event.touches[0];
      // Pulsación larga sin mover el dedo: empieza una selección que el arrastre amplía.
      pressTimer = setTimeout(function () {
        pressTimer = null;
        if (!dragging || scrolled) return;
        selecting = true;
        anchor = cellAt(touch);
        selectRange(anchor, anchor);
        if (navigator.vibrate) navigator.vibrate(15);
      }, LONG_PRESS_MS);
    }

    function onMove(event) {
      if (!dragging || event.touches.length !== 1) return;
      var y = event.touches[0].clientY;
      if (selecting) {
        event.preventDefault();
        selectRange(anchor, cellAt(event.touches[0]));
        return;
      }
      var deltaY = lastY - y;
      if (!scrolled && Math.abs(y - startY) < 4) return;
      scrolled = true;
      cancelPress();
      event.preventDefault();
      var dt = Math.max(event.timeStamp - lastTime, 1);
      // Arrastrar hacia abajo muestra líneas anteriores (desplazamiento negativo).
      scrollBy(deltaY);
      velocity = 0.7 * velocity + 0.3 * (deltaY / dt);
      lastY = y;
      lastTime = event.timeStamp;
    }

    function onEnd(event) {
      if (!dragging) return;
      dragging = false;
      cancelPress();
      if (selecting) {
        selecting = false;
        return;
      }
      // Un toque corto limpia la selección anterior.
      if (!scrolled && event.type === "touchend" && term.hasSelection && term.hasSelection()) {
        term.clearSelection();
      }
      // Sin inercia si el dedo estuvo quieto antes de soltar o el gesto se canceló.
      var held = event.timeStamp - lastTime > 120;
      if (event.type === "touchend" && scrolled && !held && Math.abs(velocity) > 0.08) {
        raf = requestAnimationFrame(fling);
      }
    }

    el.style.touchAction = "none";
    el.addEventListener("touchstart", onStart, { passive: true });
    el.addEventListener("touchmove", onMove, { passive: false });
    el.addEventListener("touchend", onEnd);
    el.addEventListener("touchcancel", onEnd);
    return function () {
      cancelPress();
      stopFling();
      el.removeEventListener("touchstart", onStart);
      el.removeEventListener("touchmove", onMove);
      el.removeEventListener("touchend", onEnd);
      el.removeEventListener("touchcancel", onEnd);
    };
  }

  // Copia la selección al portapapeles. Devuelve true si el evento se consumió.
  // Ctrl/Cmd+Shift+C siempre copia; Ctrl/Cmd+C solo copia si hay selección (si no, llega
  // al shell como SIGINT). Cmd+C en macOS también.
  function handleCopyKey(term, event) {
    if (event.type !== "keydown") return false;
    if (!(event.ctrlKey || event.metaKey) || event.altKey) return false;
    if (String(event.key).toLowerCase() !== "c") return false;
    var selection = term.hasSelection() ? term.getSelection() : "";
    if (!selection && !(event.shiftKey || event.metaKey)) return false;
    if (selection && navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(selection).catch(function () {});
    }
    if (event.preventDefault) event.preventDefault();
    return true;
  }

  function isMobileViewport() {
    return window.innerWidth <= 640;
  }

  function termFontSize() {
    return isMobileViewport() ? 12 : 14;
  }

  function open(el, sessionId) {
    var currentMobile = isMobileViewport();
    var term = new window.Terminal({
      cursorBlink: true,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
      fontSize: termFontSize(),
      scrollback: 10000,
      theme: theme(),
      allowProposedApi: true,
      smoothScrollDuration: 100,
    });
    var fit = new window.FitAddon.FitAddon();
    term.loadAddon(fit);
    term.open(el);
    term.attachCustomKeyEventHandler(function (event) {
      // false = xterm no procesa la tecla (no llega al shell).
      return !handleCopyKey(term, event);
    });

    function safeFit() {
      try {
        fit.fit();
        if (term.element) {
          term.refresh(0, term.rows - 1);
        }
      } catch (_) {}
    }

    safeFit();

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
        safeFit();
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

    // Al volver a primer plano o recuperar red, reconecta de inmediato si el socket murió
    // y refresca el layout y repintado para evitar pantallas negras o franjas.
    function ensureConnected() {
      if (disposed || document.hidden) return;
      safeFit();
      if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
        sendResize();
        return;
      }
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
    window.addEventListener("orientationchange", safeFit);
    connect();

    var dataSub = term.onData(function (data) {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(encoder.encode(data));
      }
    });
    var resizeSub = term.onResize(sendResize);

    var resizeRaf = 0;
    var observer = new ResizeObserver(function () {
      if (resizeRaf) cancelAnimationFrame(resizeRaf);
      resizeRaf = requestAnimationFrame(function () {
        var mobileNow = isMobileViewport();
        if (mobileNow !== currentMobile) {
          currentMobile = mobileNow;
          term.options.fontSize = termFontSize();
        }
        safeFit();
      });
    });
    observer.observe(el);
    var stopTouchScroll = enableTouchScroll(term, el);

    var themeObserver = new MutationObserver(function () {
      term.options.theme = theme();
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    // Pases diferidos para asegurar dimensiones exactas tras el montaje del DOM.
    requestAnimationFrame(function () {
      safeFit();
      sendResize();
    });
    setTimeout(safeFit, 150);
    setTimeout(safeFit, 350);

    term.focus();
    return {
      term: term,
      cleanup: function () {
        disposed = true;
        if (retryTimer !== null) clearTimeout(retryTimer);
        if (resizeRaf) cancelAnimationFrame(resizeRaf);
        document.removeEventListener("visibilitychange", ensureConnected);
        window.removeEventListener("online", ensureConnected);
        window.removeEventListener("pageshow", ensureConnected);
        window.removeEventListener("orientationchange", safeFit);
        observer.disconnect();
        stopTouchScroll();
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
    esc: "\x1b",
    ctrl_c: "\x03",
    up: "A",
    down: "B",
    right: "C",
    left: "D",
  };

  // Pega el portapapeles del sistema (requiere contexto seguro y permiso del navegador).
  function pasteClipboard(handle) {
    var clipboard = navigator.clipboard;
    if (!clipboard || !clipboard.readText) return Promise.resolve();
    return clipboard
      .readText()
      .then(function (text) {
        if (text) handle.term.paste(text);
        handle.term.focus();
      })
      .catch(function () {});
  }

  // Copia la selección actual (botón de la barra, útil en táctil).
  function copyClipboard(handle) {
    var term = handle.term;
    var text = term.hasSelection && term.hasSelection() ? term.getSelection() : "";
    var clipboard = navigator.clipboard;
    if (!text || !clipboard || !clipboard.writeText) return Promise.resolve();
    return clipboard
      .writeText(text)
      .then(function () {
        term.focus();
      })
      .catch(function () {});
  }

  var ACTIONS = { paste: pasteClipboard, copy: copyClipboard };

  // Envía una tecla con nombre como si se hubiera pulsado en el teclado.
  function sendKey(handle, key) {
    if (!handle || !handle.term) return;
    if (Object.prototype.hasOwnProperty.call(ACTIONS, key)) {
      ACTIONS[key](handle);
      return;
    }
    if (!Object.prototype.hasOwnProperty.call(KEYS, key)) return;
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

  window.iwfTerminal = {
    open: open,
    close: close,
    sendKey: sendKey,
    enableTouchScroll: enableTouchScroll,
    handleCopyKey: handleCopyKey,
  };
})();
