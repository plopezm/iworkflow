// end2end/settings.e2e.cjs
// Pruebas e2e de /settings (agent_settings + preferences). Cubre los casos de uso
// alcanzables desde la barra de ajustes del agente (sidebar, visible en /settings)
// y la sección de preferencias (Idioma / Recursos / Red) de la pantalla de configuración.
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
// porque LEPTOS_SITE_ADDR está fijado (end2end/run_tests.sh). Entonces la tarjeta Red
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

// Espera a que el atributo title de una píldora contenga `needle`; devuelve el valor final.
async function waitForTitle(pill, needle, { timeout = 10000 } = {}) {
  const start = Date.now();
  let last = '';
  while (Date.now() - start < timeout) {
    try {
      last = (await pill.first().getAttribute('title')) || '';
    } catch {
      last = '';
    }
    if (last.includes(needle)) return last;
    await new Promise((r) => setTimeout(r, 200));
  }
  return last;
}

// El <label class="select-pill"> que lleva el atributo title (el valor actual).
// El select interno no tiene atributo title, así que waitForTitle debe leer aquí.
function pillLabel(ctx, label) {
  return ctx.page.locator('.select-pill').filter({ hasText: label }).first();
}

// El <select> interno de la píldora cuyo nombre accesible es `label`.
function pillSelect(ctx, label) {
  return pillLabel(ctx, label).locator('select').first();
}

// El título (valor actual visible) de la píldora `label`.
async function pillTitle(ctx, label) {
  return (await pillLabel(ctx, label).getAttribute('title')) || '';
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

// En el entorno smoke, el acceso desde la red está bloqueado por LEPTOS_SITE_ADDR.
async function isNetworkLocked(ctx) {
  const n = await ctx
    .page.getByText(/LEPTOS_SITE_ADDR|fijada por la variable de entorno/)
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

// Navega a /settings y espera a la barra del agente (las 4 pílduras).
async function goToSettingsBar(ctx) {
  await ctx.goto('/settings');
  // El "Se aplica a las tareas nuevas" sólo aparece cuando la barra está hidratada.
  await ctx.page.getByText('Se aplica a las tareas nuevas').first().waitFor({ timeout: 15000 });
}

// ---- tests ----
module.exports = {
  name: 'settings',
  tests: [
    {
      name: 'get_agent_settings: /settings muestra la barra de ajustes (runtime / modelo / razonamiento / perfil)',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const agente = pillSelect(ctx, 'Agente');
        const modelo = pillSelect(ctx, 'Modelo');
        const razonamiento = pillSelect(ctx, 'Razonamiento');
        const perfil = pillSelect(ctx, 'Perfil');
        await agente.first().waitFor({ timeout: 15000 });
        await modelo.first().waitFor({ timeout: 15000 });
        await razonamiento.first().waitFor({ timeout: 15000 });
        await perfil.first().waitFor({ timeout: 15000 });
        const aTitle = await pillTitle(ctx, 'Agente');
        const mTitle = await pillTitle(ctx, 'Modelo');
        const rTitle = await pillTitle(ctx, 'Razonamiento');
        const pTitle = await pillTitle(ctx, 'Perfil');
        ctx.assert(aTitle.length > 0, `el runtime no se muestra (title="${aTitle}")`);
        ctx.assert(mTitle.length > 0, `el modelo no se muestra (title="${mTitle}")`);
        ctx.assert(rTitle.length > 0, `el razonamiento no se muestra (title="${rTitle}")`);
        ctx.assert(pTitle.length > 0, `el perfil no se muestra (title="${pTitle}")`);
        // La barra indica que los cambios afectan a tareas nuevas.
        await ctx.page.getByText('Se aplica a las tareas nuevas').first().waitFor({ timeout: 8000 });
      },
    },
    {
      name: 'update_agent_settings: cambiar el nivel de razonamiento y guardar se refleja en la barra',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const razonamiento = pillSelect(ctx, 'Razonamiento');
        await razonamiento.first().waitFor({ timeout: 15000 });
        const antes = await pillTitle(ctx, 'Razonamiento');
        ctx.assert(antes.length > 0, `razonamiento inicial vacío (title="${antes}")`);
        // Lee el valor actual y las opciones; elige una DISTINTA del actual para
        // garantizar una guardación real (no depende de un estado previo.
        const opts = await razonamiento.evaluate((sel) =>
          Array.from(sel.options).map((o) => ({ v: o.value, t: o.text }))
        );
        const original = await razonamiento.inputValue();
        ctx.assert(opts.length >= 2, `el select de razonamiento necesita >=2 opciones (n=${opts.length})`);
        const target = opts.find((o) => o.v !== original);
        ctx.assert(target, `no hay un nivel distinto del actual (${original})`);
        await razonamiento.selectOption({ value: target.v });
        const despues = await waitForTitle(pillLabel(ctx, 'Razonamiento'), target.t, { timeout: 15000 });
        ctx.assert(
          despues.includes(target.t),
          `el nivel de razonamiento no cambió a "${target.t}" (title="${despues}")`
        );
        ctx.assert(despues !== antes, `la barra no reflejó el cambio (antes="${antes}")`);
        // Restaura el valor original para no dejar el estado modificado.
        await razonamiento.selectOption({ value: original });
        const restaurado = await waitForTitle(pillLabel(ctx, 'Razonamiento'), antes, { timeout: 15000 });
        ctx.assert(
          restaurado.includes(antes),
          `no se restauró el razonamiento a "${antes}" (title="${restaurado}")`
        );
      },
    },
    {
      name: 'list_model_options: el select Modelo tiene al menos una opción',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const modelo = pillSelect(ctx, 'Modelo');
        await modelo.first().waitFor({ timeout: 15000 });
        const n = await modelo.evaluate((sel) => sel.options.length);
        ctx.assert(n >= 1, `el select Modelo no tiene opciones (${n})`);
        // En el entorno smoke el catálogo está vacío y sólo aparece "Por defecto".
      },
    },
    {
      name: 'list_profile_options: el select Perfil tiene al menos una opción',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const perfil = pillSelect(ctx, 'Perfil');
        await perfil.first().waitFor({ timeout: 15000 });
        const n = await perfil.evaluate((sel) => sel.options.length);
        ctx.assert(n >= 1, `el select Perfil no tiene opciones (${n})`);
      },
    },
    {
      name: 'list_thinking_levels: el select Razonamiento tiene al menos una opción',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const razonamiento = pillSelect(ctx, 'Razonamiento');
        await razonamiento.first().waitFor({ timeout: 15000 });
        const n = await razonamiento.evaluate((sel) => sel.options.length);
        ctx.assert(n >= 1, `el select Razonamiento no tiene opciones (${n})`);
      },
    },
    {
      name: 'list_runtime_options: el select Agente tiene al menos una opción',
      fn: async (ctx) => {
        await goToSettingsBar(ctx);
        const agente = pillSelect(ctx, 'Agente');
        await agente.first().waitFor({ timeout: 15000 });
        const n = await agente.evaluate((sel) => sel.options.length);
        ctx.assert(n >= 1, `el select Agente no tiene opciones (${n})`);
      },
    },
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
          // Entorno bloqueado (LEPTOS_SITE_ADDR): el control existe pero no se puede guardar.
          ctx.assert(deshab, 'se esperaba el control deshabilitado (bloqueado)');
          ctx.assert(
            (await isNetworkLocked(ctx)),
            'el control está bloqueado pero falta el aviso de bloqueo (LEPTOS_SITE_ADDR)'
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
          ctx.assert(await isNetworkLocked(ctx), 'falta el aviso de bloqueo (LEPTOS_SITE_ADDR)');
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
