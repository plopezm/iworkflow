// end2end/go/preferences.e2e.cjs
// Parity copy of end2end/settings.e2e.cjs for the Go build (module: preferences).
//
// Differences from the Rust script (end2end/settings.e2e.cjs):
//   - The agent settings bar tests (get_agent_settings, update_agent_settings, list_*) were removed:
//     they belong to the agent_settings module, which has its own script.
//   - Helpers that only served those tests were removed (pillLabel, pillSelect, pillTitle,
//     waitForTitle, goToSettingsBar).
//   - Everything else keeps the Rust selectors and texts, so the same DOM must render them. The Go
//     templates keep the ids and labels (#network-access, #network-port, label "Puerto del servidor",
//     the buttons "Guardar puerto" / "Reiniciar ahora" / "Generar un token nuevo", the checkboxes).
//   - The "Red" tab is the segmented control of the /settings page, composed by the parent. This
//     script assumes that page exists in the Go server; it does not test the page itself.
//
// Prerequisites to run against the Go server: the parent wires internal/app with this module and the
// /settings page (including the Red tab and the Idioma section); the server starts with the same
// environment as the previous smoke run (IWORKFLOW_ADDR pinned, so the network controls are locked).
//
// Patrones de selector (verificados contra la UI real):
//   - La barra del agente es un <label class="select-pill"> por control, con un
//     <span class="sr-only"> que da el nombre accesible y un <select class="select-pill-input">.
//   - Los nombres accesibles de los selects: "Agente", "Modelo", "Razonamiento", "Perfil".
//   - La sección "Red" se alcanza con el tab "Red" del control segmentado.
//   - Los botones de idioma son <button> cuyo texto es el nombre propio de la lengua
//     (English, Español, 中文, Deutsch, Français), que no se traduce con la UI.
//
// ADVERTENCIA (verificación): en el entorno smoke el acceso desde la red está BLOQUEADO
// porque IWORKFLOW_ADDR está fijado (end2end/run_tests.sh). Entonces la tarjeta Red
// es estática: el badge "Solo este equipo", el aviso de bloqueo y un checkbox "Permitir..."
// DESHABILITADO; el campo de puerto está DESHABILITADO; y la sección de token / el botón
// "Reiniciar ahora" NO se renderizan (sólo aparecen cuando hay acceso desde la red /
// hay reinicio pendiente). Estos tests se adaptan: en un entorno desbloqueado realizan la
// guardación real; en el entorno bloqueado (el de la verificación) afirman el estado
// bloqueado real en vez de fingir una guardación.
const tag = Date.now().toString();

// ---- helpers ----

// Espera a que el valor de un input contenga `needle`; devuelve el valor final.
async function waitForValue(locator, needle, { timeout = 10000 } = {}) {
  const start = Date.now();
  let last = '';
  while (Date.now() - start < timeout) {
    try {
      last = await locator.first().inputValue();
    } catch {
      last = '';
    }
    if (last.includes(needle)) return last;
    await new Promise((r) => setTimeout(r, 250));
  }
  return last;
}

// Navega a /settings y a la sección Red, esperando a que la tarjeta de acceso se cargue.
async function gotoRed(ctx) {
  await ctx.goto('/settings');
  // La página debe estar en español (el harness arranca con iw-lang=es y el test de
  // idioma deja el estado en español).
  await ctx.page
    .getByRole('tab', { name: 'Red' })
    .first()
    .click();
  await ctx.page
    .getByText('Acceso desde la red')
    .first()
    .waitFor({ timeout: 10000 });
}

// En el entorno smoke, el acceso desde la red está bloqueado por IWORKFLOW_ADDR.
async function isNetworkLocked(ctx) {
  const n = await ctx
    .page.getByText(/IWORKFLOW_ADDR|fijada por la variable de entorno/)
    .first()
    .count()
    .catch(() => 0);
  return n > 0;
}

// ¿El servidor es alcanzable desde la red (badge "Accesible desde la red" = active=true)?
async function isReachable(ctx) {
  const t = (await ctx
    .page
    .getByText(/Solo este equipo|Accesible desde la red/)
    .first()
    .textContent())
    .trim();
  return t.includes('Accesible desde la red');
}

// ---- tests ----
module.exports = {
  name: 'preferences',
  tests: [
    {
      name: 'save_language: cambiar el idioma y volverlo a "es" refleja el cambio y deja la UI en español',
      fn: async (ctx) => {
        await ctx.goto('/settings');
        const h1 = ctx.page.locator('h1.page-title');
        await h1.first().waitFor({ timeout: 10000 });
        const titulo = async () => (await h1.first().textContent()).trim();
        const antes = await titulo();
        ctx.assert(
          antes === 'Configuración',
          `la UI no arranca en español (h1="${antes}"); el test de idioma deja el estado en "es"`
        );
        // Cambia a alemán: save_language + reload en el nuevo idioma.
        await ctx.page.getByRole('button', { name: 'Deutsch' }).first().click();
        await ctx.page.waitForLoadState('load').catch(() => {});
        let t = await titulo();
        const deadline = Date.now() + 10000;
        while (!t.includes('Einstellungen') && Date.now() < deadline) {
          await new Promise((r) => setTimeout(r, 300));
          t = await titulo();
        }
        ctx.assert(
          t.includes('Einstellungen'),
          `la UI no cambió al alemán tras elegir "Deutsch" (h1="${t}")`
        );
        // Vuelve a español (los botones de idioma usan su nombre propio, no se traducen).
        await ctx.page.getByRole('button', { name: 'Español' }).first().click();
        await ctx.page.waitForLoadState('load').catch(() => {});
        t = await titulo();
        const deadline2 = Date.now() + 10000;
        while (!t.includes('Configuración') && Date.now() < deadline2) {
          await new Promise((r) => setTimeout(r, 300));
          t = await titulo();
        }
        ctx.assert(
          t === 'Configuración',
          `el idioma no se restauró a español (h1="${t}"); el estado compartido debe quedar en "es"`
        );
      },
    },
    {
      name: 'get_network_access: la sección Red muestra el estado actual de acceso desde la red',
      fn: async (ctx) => {
        await gotoRed(ctx);
        // El badge indica el estado actual (el del servidor en marcha).
        const badge = await (
          await ctx
            .page
            .getByText(/Solo este equipo|Accesible desde la red/)
            .first()
            .textContent()
        );
        ctx.assert(
          badge.includes('Solo este equipo') || badge.includes('Accesible desde la red'),
          `el badge de estado no es válido ("${badge}")`
        );
        // El título de la tarjeta.
        await ctx.page.getByText('Acceso desde la red').first().waitFor({ timeout: 8000 });
        // El aviso de seguridad.
        await ctx
          .page.getByText(/no tiene autenticación y sus agentes y terminales/)
          .first()
          .waitFor({ timeout: 8000 });
      },
    },
    {
      name: 'save_network_access: el control "Permitir el acceso" se bloquea en el entorno smoke; en uno desbloqueado refleja el toggle',
      fn: async (ctx) => {
        await gotoRed(ctx);
        const checkbox = ctx.page
          .getByRole('checkbox', { name: /Permitir el acceso desde otros dispositivos de la red/ })
          .first();
        await checkbox.waitFor({ timeout: 10000 });
        const deshab = await checkbox.isDisabled();
        if (deshab) {
          // Entorno bloqueado (IWORKFLOW_ADDR): el control existe pero no se puede guardar.
          ctx.assert(deshab, 'se esperaba el control deshabilitado (bloqueado)');
          ctx.assert(
            (await isNetworkLocked(ctx)),
            'el control está bloqueado pero falta el aviso de bloqueo (IWORKFLOW_ADDR)'
          );
          // No se finge una guardación: la afirmación es el estado real (bloqueado).
        } else {
          // Entorno desbloqueado: guardación real + restauración.
          const antes = await checkbox.isChecked();
          await checkbox.click();
          const despues = await checkbox.isChecked();
          ctx.assert(despues !== antes, 'el checkbox no reflejó el toggle');
          // Al habilitar, aparece el prompt de reinicio (enabled != active).
          await ctx
            .page.getByText(/Reinicia el servidor para aplicar el cambio/)
            .first()
            .waitFor({ timeout: 5000 })
            .catch(() => {});
          // NO pulsa "Reiniciar ahora": no queremos matar el servidor del test.
          // Restaura el estado original.
          await checkbox.click();
          const back = await checkbox.isChecked();
          ctx.assert(back === antes, 'el checkbox no volvió a su estado original');
        }
      },
    },
    {
      name: 'save_listen_port: el campo "Puerto del servidor" refleja el puerto; bloqueado en el entorno smoke',
      fn: async (ctx) => {
        await gotoRed(ctx);
        const port = ctx.page.getByLabel('Puerto del servidor');
        await port.first().waitFor({ timeout: 10000 });
        const deshab = await port.first().isDisabled();
        const valor = await port.first().inputValue();
        ctx.assert(
          valor === '' || /^\d+$/.test(valor),
          `el valor del puerto no es numérico ("${valor}")`
        );
        if (deshab) {
          ctx.assert(deshab, 'se esperaba el campo de puerto bloqueado');
          ctx.assert(await isNetworkLocked(ctx), 'falta el aviso de bloqueo (IWORKFLOW_ADDR)');
        } else {
          // Entorno desbloqueado: guarda un puerto único y verifica la reflexión.
          const original = valor;
          const nuevo = String(18000 + Number((Date.now() % 7000)));
          await port.first().fill(nuevo);
          await ctx
            .page.getByRole('button', { name: 'Guardar puerto' })
            .first()
            .click();
          const reflejado = await waitForValue(port, nuevo);
          ctx.assert(
            reflejado.includes(nuevo),
            `el puerto no se reflejó en el input (got="${reflejado}")`
          );
          // Restaura el puerto original.
          await port.first().fill(original);
          await ctx
            .page.getByRole('button', { name: 'Guardar puerto' })
            .first()
            .click();
        }
      },
    },
    {
      name: 'save_require_access_token: el control "Exigir el token" solo es alcanzable con acceso desde la red',
      fn: async (ctx) => {
        await gotoRed(ctx);
        // El checkbox de token y su aviso sólo se renderizan cuando el servidor es
        // alcanzable desde la red (enabled || active). En el entorno smoke bloqueado/
        // privado no lo es, así que el control no existe: lo afirmamos (no lo fingimos).
        const n = (await ctx
          .page.getByRole('checkbox', { name: /Exigir el token de acceso/ })
          .first()
          .count()
          .catch(() => 0));
        const locked = await isNetworkLocked(ctx);
        if (locked) {
          ctx.assert(n === 0, `en el entorno bloqueado el token no es alcanzable, pero hay ${n} checkbox(es)`);
        } else {
          // En un entorno desbloqueado el checkbox sólo existe si hay acceso desde la red.
          if (await isReachable(ctx)) {
            ctx.assert(
              n >= 1,
              `el acceso es alcanzable pero falta el checkbox de token (n=${n})`
            );
          }
        }
      },
    },
    {
      name: 'regenerate_access_token: la sección de token solo se muestra con acceso; en el entorno smoke está ausente',
      fn: async (ctx) => {
        await gotoRed(ctx);
        const n = (await ctx
          .page.getByText('Generar un token nuevo')
          .first()
          .count()
          .catch(() => 0));
        const locked = await isNetworkLocked(ctx);
        if (locked) {
          // Entorno bloqueado/private: la sección de token no se renderiza.
          ctx.assert(n === 0, `en el entorno bloqueado el botón "Generar un token nuevo" no debe renderizarse (n=${n})`);
        } else {
          // En un entorno desbloqueado la sección existe solo si hay acceso desde la red.
          if (await isReachable(ctx)) {
            ctx.assert(
              n >= 1,
              `el acceso es alcanzable pero falta el botón "Generar un token nuevo" (n=${n})`
            );
          }
        }
      },
    },
    {
      name: 'restart_server: el control "Reiniciar ahora" existe solo si hay reinicio pendiente (ausente en el entorno bloqueado)',
      fn: async (ctx) => {
        await gotoRed(ctx);
        const n = (await ctx
          .page.getByRole('button', { name: 'Reiniciar ahora' })
          .first()
          .count()
          .catch(() => 0));
        const locked = await isNetworkLocked(ctx);
        if (locked) {
          // Con el acceso bloqueado no hay reinicio pendiente y el botón no se renderiza.
          ctx.assert(
            n === 0,
            `en el entorno bloqueado no debe haber botón "Reiniciar ahora" (n=${n}); además NO lo pulsamos para no matar el servidor del test`
          );
        } else {
          // En un entorno desbloqueado podría haber pendiente (por ejemplo si se cambió un ajuste).
          // Aquí no forzamos el cambio: solo comprobamos la coherencia con el prompt.
          if (n > 0) {
            await ctx.page.getByText(/Reinicia el servidor para aplicar el cambio/).first().waitFor({ timeout: 5000 });
          }
        }
      },
    },
  ],
};
