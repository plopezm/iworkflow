// Resaltado de sintaxis (highlight.js) para el visor y el editor de ficheros del proyecto.
// Requiere `hljs` (vendor/highlightjs/highlight.min.js) ya cargado.
(function () {
  "use strict";

  // Por encima de este tamaño se muestra texto plano para no bloquear la interfaz.
  var MAX_HIGHLIGHT_CHARS = 300000;

  var BY_FILENAME = {
    dockerfile: "dockerfile",
    makefile: "makefile",
    "cargo.lock": "ini",
    ".env": "ini",
    ".gitconfig": "ini",
    ".editorconfig": "ini",
  };

  var BY_EXTENSION = {
    jsx: "javascript",
    mjs: "javascript",
    cjs: "javascript",
    tsx: "typescript",
    mts: "typescript",
    cts: "typescript",
    vue: "xml",
    svelte: "xml",
    svg: "xml",
    html: "xml",
    htm: "xml",
    xhtml: "xml",
    sh: "bash",
    zsh: "bash",
    fish: "bash",
    toml: "ini",
    cfg: "ini",
    conf: "ini",
    env: "ini",
    lock: "ini",
    jsonc: "json",
    json5: "json",
    h: "c",
    hpp: "cpp",
    hh: "cpp",
    cc: "cpp",
    cxx: "cpp",
    cs: "csharp",
    kt: "kotlin",
    kts: "kotlin",
    gradle: "java",
    mod: "go",
    yml: "yaml",
    md: "markdown",
    mdx: "markdown",
    py: "python",
    rb: "ruby",
    rs: "rust",
    pl: "perl",
    ps1: "powershell",
    scss: "scss",
    sass: "scss",
    gql: "graphql",
    graphqls: "graphql",
    sql: "sql",
    swift: "swift",
    lua: "lua",
    php: "php",
    r: "r",
    patch: "diff",
    diff: "diff",
    wat: "wasm",
  };

  function languageFor(filename) {
    var name = String(filename || "").split("/").pop().toLowerCase();
    if (BY_FILENAME[name]) return BY_FILENAME[name];
    var dot = name.lastIndexOf(".");
    var ext = dot >= 0 ? name.slice(dot + 1) : "";
    var candidate = BY_EXTENSION[ext] || ext;
    return candidate && window.hljs.getLanguage(candidate) ? candidate : null;
  }

  function escapeHtml(text) {
    return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }

  function highlight(text, filename) {
    var language = text.length <= MAX_HIGHLIGHT_CHARS ? languageFor(filename) : null;
    if (!language) return escapeHtml(text);
    try {
      return window.hljs.highlight(text, { language: language, ignoreIllegals: true }).value;
    } catch (e) {
      return escapeHtml(text);
    }
  }

  // El `\n` final hace que la última línea vacía ocupe el mismo alto que en el textarea.
  function render(codeEl, text, filename) {
    codeEl.innerHTML = highlight(text, filename) + "\n";
  }

  // Sincroniza el scroll del textarea con la capa resaltada y añade el sangrado con Tab.
  function attachEditor(textarea, layer, gutter) {
    function sync() {
      layer.scrollTop = textarea.scrollTop;
      layer.scrollLeft = textarea.scrollLeft;
      if (gutter) gutter.scrollTop = textarea.scrollTop;
    }
    function onKeydown(ev) {
      if (ev.key !== "Tab" || ev.ctrlKey || ev.metaKey || ev.altKey || ev.shiftKey) return;
      ev.preventDefault();
      var indent = /^\t/m.test(textarea.value) ? "\t" : "    ";
      // execCommand conserva el historial de deshacer del textarea.
      if (!document.execCommand("insertText", false, indent)) {
        var start = textarea.selectionStart;
        textarea.setRangeText(indent, start, textarea.selectionEnd, "end");
        textarea.dispatchEvent(new Event("input", { bubbles: true }));
      }
    }
    textarea.addEventListener("scroll", sync);
    textarea.addEventListener("keydown", onKeydown);
    sync();
    return function detach() {
      textarea.removeEventListener("scroll", sync);
      textarea.removeEventListener("keydown", onKeydown);
    };
  }

  window.iwfCode = {
    render: render,
    attachEditor: attachEditor,
    languageFor: languageFor,
  };
})();
