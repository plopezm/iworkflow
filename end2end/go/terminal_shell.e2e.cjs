// end2end/go/terminal_shell.e2e.cjs
// Go-mode e2e of the "Terminal" tab of the settings: the shell executable the terminals start.
// The card (#terminal-shell-card) is lazy-loaded by the tab. Save, "Usar el predeterminado" and
// "Examinar…" are checked in the UI and after a reload, and a terminal opened after the save runs
// through the stored shell. The setting is global and the DB is shared, so every test that stores a
// shell restores the default in a finally block. The suite runs in Spanish.
//
// run_go.sh starts the server with `env -i`, so DISPLAY and WAYLAND_DISPLAY never reach it. The native
// picker then reports "unavailable" without running zenity or kdialog, which makes the "Examinar…" case
// deterministic. If the server were started with a display, that case would wait for a real dialog and
// fail on timeout; it never passes without the unavailable message.

const fs = require('fs');
const path = require('path');
const { openWorkspaceTerminal, run, screenHas } = require('../terminal.e2e.cjs').helpers;

const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const SHELL = '/bin/sh';
const CARD = '#terminal-shell-card';
const INPUT = '#terminal-shell';
const RESET = 'Usar el predeterminado';
const PICKER_UNAVAILABLE = 'no se pudo abrir el selector de archivos del sistema';

const card = (ctx) => ctx.page.locator(CARD);
const input = (ctx) => ctx.page.locator(INPUT);
const button = (ctx, name) => card(ctx).getByRole('button', { name, exact: true });

// Opens /settings and its Terminal tab, and waits for the lazy-loaded card (the input only exists there).
async function openTerminalTab(ctx) {
  await ctx.goto('/settings');
  await ctx.page.locator('#settings-page').getByRole('tab', { name: 'Terminal', exact: true }).click();
  await input(ctx).waitFor({ timeout: 10000 });
}

// Reloads the page and reopens the tab, as a user would to check that the value persisted.
async function reloadAndReopenTab(ctx) {
  await ctx.page.reload({ waitUntil: 'domcontentloaded' });
  await ctx.page.locator('#settings-page').getByRole('tab', { name: 'Terminal', exact: true }).click();
  await input(ctx).waitFor({ timeout: 10000 });
}

// Waits until the server-rendered input holds `value` (the card is swapped whole after every action).
async function waitForInputValue(ctx, value) {
  await ctx.page.waitForFunction(
    ([sel, v]) => document.querySelector(sel)?.value === v,
    [INPUT, value],
    { timeout: 10000 },
  );
}

// Stores `value` through the Guardar button and waits for the swapped card to show the stored state.
async function saveShell(ctx, value) {
  await input(ctx).fill(value);
  await button(ctx, 'Guardar').click();
  await button(ctx, RESET).waitFor({ timeout: 10000 });
}

// Puts the default back: "Usar el predeterminado" when a shell is stored, then checks the card is clean.
async function restoreDefault(ctx) {
  await openTerminalTab(ctx);
  if ((await button(ctx, RESET).count()) > 0) {
    await button(ctx, RESET).click();
    await button(ctx, RESET).waitFor({ state: 'detached', timeout: 10000 });
  }
  ctx.assert((await input(ctx).inputValue()) === '', 'restaurar el predeterminado dejó un shell en el campo');
}

module.exports = {
  name: 'terminal_shell',
  tests: [
    {
      name: 'tab: la pestaña Terminal muestra la tarjeta vacía, sin "Usar el predeterminado"',
      fn: async (ctx) => {
        await openTerminalTab(ctx);
        ctx.assert((await input(ctx).inputValue()) === '', 'el campo del shell no está vacío por defecto');
        ctx.assert((await button(ctx, RESET).count()) === 0, '"Usar el predeterminado" se ve sin shell guardado');
        ctx.assert(await card(ctx).getByLabel('Ejecutable del shell').isVisible(), 'falta la etiqueta del campo');
        ctx.assert(await button(ctx, 'Guardar').isVisible(), 'falta el botón Guardar');
        ctx.assert(await button(ctx, 'Examinar…').isVisible(), 'falta el botón Examinar…');
      },
    },
    {
      name: 'save: guardar "/bin/sh" mantiene el valor, muestra "Usar el predeterminado" y persiste tras recargar',
      fn: async (ctx) => {
        await openTerminalTab(ctx);
        try {
          await saveShell(ctx, SHELL);
          ctx.assert((await input(ctx).inputValue()) === SHELL, 'el campo perdió el valor guardado');
          await reloadAndReopenTab(ctx);
          await waitForInputValue(ctx, SHELL);
          ctx.assert(await button(ctx, RESET).isVisible(), 'tras recargar falta "Usar el predeterminado"');
        } finally {
          await restoreDefault(ctx);
        }
      },
    },
    {
      name: 'reset: "Usar el predeterminado" vacía el campo, oculta el botón y persiste tras recargar',
      fn: async (ctx) => {
        await openTerminalTab(ctx);
        try {
          await saveShell(ctx, SHELL);
          await button(ctx, RESET).click();
          await button(ctx, RESET).waitFor({ state: 'detached', timeout: 10000 });
          ctx.assert((await input(ctx).inputValue()) === '', 'el campo no se vació al usar el predeterminado');
          await reloadAndReopenTab(ctx);
          await waitForInputValue(ctx, '');
          ctx.assert((await button(ctx, RESET).count()) === 0, 'tras recargar sigue "Usar el predeterminado"');
        } finally {
          await restoreDefault(ctx);
        }
      },
    },
    {
      name: 'browse: "Examinar…" sin selector del sistema muestra el aviso y no cambia el shell guardado',
      fn: async (ctx) => {
        await openTerminalTab(ctx);
        try {
          await saveShell(ctx, SHELL);
          await button(ctx, 'Examinar…').click();
          // Times out (and fails) if the message never shows: a missing message is not a pass.
          await card(ctx).getByText(PICKER_UNAVAILABLE).waitFor({ timeout: 15000 });
          ctx.assert((await input(ctx).inputValue()) === SHELL, 'Examinar cambió el shell guardado');
          await reloadAndReopenTab(ctx);
          await waitForInputValue(ctx, SHELL);
        } finally {
          await restoreDefault(ctx);
        }
      },
    },
    {
      name: 'invalid: un shell con un carácter de control muestra "shell no válido" y no se guarda',
      fn: async (ctx) => {
        await openTerminalTab(ctx);
        // A tab inside the path is a control character; a newline cannot reach a text input at all.
        await input(ctx).fill('/bin\tsh');
        await button(ctx, 'Guardar').click();
        await card(ctx).getByText('shell no válido').waitFor({ timeout: 10000 });
        ctx.assert((await button(ctx, RESET).count()) === 0, 'el shell inválido quedó guardado');
        await reloadAndReopenTab(ctx);
        ctx.assert((await button(ctx, RESET).count()) === 0, 'tras recargar hay un shell guardado');
        ctx.assert((await input(ctx).inputValue()) === '', 'tras recargar el campo no está en el predeterminado');
      },
    },
    {
      name: 'terminal: una terminal abierta tras guardar el shell ejecuta el programa elegido y su salida se ve',
      fn: async (ctx) => {
        // Wrapper program: it prints a marker before handing over to /bin/sh. Plain /bin/sh could be the
        // platform default too, so the marker is what proves the stored setting was launched.
        const wrapper = path.join(TMP, `iwf-shell-${tag}.sh`);
        const marker = `shell-del-ajuste-${tag}`;
        fs.writeFileSync(wrapper, `#!/bin/sh\necho ${marker}\nexec ${SHELL}\n`, { mode: 0o755 });
        await openTerminalTab(ctx);
        try {
          await saveShell(ctx, wrapper);
          await openWorkspaceTerminal(ctx, `term-shell-${tag}`);
          await screenHas(ctx.page, marker);
          await run(ctx.page, "echo marca-sh''ell");
          await screenHas(ctx.page, 'marca-shell');
        } finally {
          await restoreDefault(ctx);
          fs.rmSync(wrapper, { force: true });
        }
      },
    },
  ],
};
