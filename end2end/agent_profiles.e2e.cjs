// end2end/agent_profiles.e2e.cjs
// Suite del módulo agent_profiles: cubre los 14 casos de uso alcanzables desde
// la UI (list_profiles, create/duplicate/rename/delete_profile,
// export/import_profile, list_agent_roles, create_agent_role,
// get/set_global_routing, set_profile_routing, list_model_options,
// list_profile_thinking_levels).
//
// Notas de entorno:
// - Los perfiles y roles viven en archivos (profiles.json, <rol>.md), NO en la BD:
//   cada test usa nombres únicos (`ap*-${tag}`) y se limpia lo que crea.
// - El routing global es models.json: el test de set_global_routing restaura el
//   valor original tras el guardado.
// - Los selects "Modelo" se llenan por list_model_options; en el entorno smoke
//   el catálogo puede ser vacío y solo aparece "Heredar / por defecto" (más el
//   valor actual de la fila, que el editor conserva como fallback).
const fs = require('fs');
const path = require('path');
const tag = Date.now().toString();

// ---- helpers ----

function escapeRe(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Nombre único y válido (slug) para no chocar con los perfiles/roles previos.
function uniqueName(prefix) {
  return `${prefix}-${tag}`;
}

// Espere a que el valor de un input/textarea/select contenga `needle`; devuelve el valor final.
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

// Tarjeta "Perfiles" (h2.card-title "Perfiles") en /profiles.
function profilesCard(ctx) {
  return ctx.page
    .locator('section.card')
    .filter({ has: ctx.page.getByRole('heading', { name: /Perfiles/ }) });
}

// Tarjeta "Roles de agente" en /profiles.
function rolesCard(ctx) {
  return ctx.page
    .locator('section.card')
    .filter({ has: ctx.page.getByRole('heading', { name: /Roles de agente/ }) });
}

// Tarjeta "Routing global" en /models.
function modelsCard(ctx) {
  return ctx.page
    .locator('section.card')
    .filter({ has: ctx.page.getByRole('heading', { name: /Routing global/ }) });
}

// Tarjeta derecha de la pantalla de perfiles correspondiente al perfil `name`.
function rightCard(ctx, name) {
  return ctx.page
    .locator('section.card')
    .filter({
      has: ctx.page.getByRole('heading', { name: new RegExp(escapeRe(name), 'i') }),
    });
}

// Tarjeta derecha del perfil activo (el texto «Es el perfil activo» solo lo
// muestra la tarjeta del perfil activo seleccionado).
function activeProfileCard(ctx) {
  return ctx.page
    .locator('section.card')
    .filter({ has: ctx.page.getByText(/Es el perfil activo/).first() });
}

// Chip del perfil `name` dentro de la tarjeta de perfiles.
function profileChip(ctx, name) {
  return profilesCard(ctx).getByRole('button', { name: new RegExp(escapeRe(name), 'i') });
}

// Diálogo de nombre compartido (Nuevo / Duplicar / Renombrar): espera el panel,
// escribe `name` en "Nombre del perfil" y confirma con "Aceptar".
async function submitNameDialog(ctx, name) {
  const panel = ctx.page.locator('#modal-panel');
  await panel.waitFor({ timeout: 5000 });
  const input = panel.getByLabel('Nombre del perfil');
  await input.fill(name);
  await panel.getByRole('button', { name: 'Aceptar' }).first().click();
}

// Crea un perfil `name` desde el botón "Nuevo" y espera el toast + el chip.
async function createProfileViaUI(ctx, name) {
  await ctx.goto('/profiles');
  await profilesCard(ctx).getByRole('button', { name: 'Nuevo' }).first().click();
  await submitNameDialog(ctx, name);
  await assertToast(ctx, /Perfil creado/);
  const count = await profileChip(ctx, name).count();
  ctx.assert(
    count === 1,
    `el perfil "${name}" no aparece en la lista (${count} chips)`
  );
}

// Elimina el perfil `name` vía UI: selecciona su chip, abre el ConfirmPanel y
// confirma con el botón "Eliminar" del panel (no es confirm() nativo).
async function deleteProfileViaUI(ctx, name) {
  await profileChip(ctx, name).first().click();
  await rightCard(ctx, name)
    .getByRole('button', { name: 'Eliminar' })
    .first()
    .click();
  // El ConfirmPanel muestra el mensaje y el botón de confirmación.
  await ctx.page
    .getByText(new RegExp(`Eliminar el perfil «${escapeRe(name)}»`))
    .first()
    .waitFor({ timeout: 5000 });
  // El ConfirmPanel añade un segundo botón "Eliminar" (el último en el DOM).
  await ctx.page.getByRole('button', { name: 'Eliminar' }).last().click();
  await assertToast(ctx, /Perfil eliminado/);
  const count = await profileChip(ctx, name).count();
  ctx.assert(count === 0, `el perfil "${name}" sigue en la lista (${count} chips)`);
}

// Espera un toast cuyo texto contenga el patrón.
async function assertToast(ctx, textOrRe) {
  await ctx.page
    .getByText(textOrRe)
    .first()
    .waitFor({ timeout: 8000 });
}

// Opciones (value/text) de un select.
async function selectOptions(select) {
  return select.evaluate((sel) =>
    [...sel.options].map((o) => ({ value: o.value, text: o.text.trim() }))
  );
}

// Cambia el select a la primera opción distinta del valor actual; devuelve el valor actual.
async function pickDifferent(select) {
  const opts = await selectOptions(select);
  const current = await select.inputValue();
  const alt = opts.find((o) => o.value !== current);
  if (!alt) throw new Error(`no hay opción distinta al valor actual "${current}"`);
  await select.selectOption({ value: alt.value });
  return current;
}

// Asegura que el editor del routing (dentro de `card`) tenga al menos una fila.
// Si no, añade en borrador una fila "orchestrator" (no se guarda: solo el DOM).
async function ensureFirstRow(ctx, card) {
  let rows = card.locator('li.card');
  if ((await rows.count()) === 0) {
    const addAgent = card.getByLabel(/Agente a añadir/);
    await addAgent.selectOption({ label: 'orchestrator' });
    await card.getByRole('button', { name: /Añadir fila/ }).first().click();
    rows = card.locator('li.card');
  }
  ctx.assert((await rows.count()) > 0, 'el RoutingEditor no tiene filas');
  return rows.first();
}

// Guarda el routing del editor (`card`).
async function saveRouting(ctx, card) {
  await card.getByRole('button', { name: 'Guardar enrutado' }).first().click();
}

// El diálogo "Nuevo rol de agente" (se usa en /profiles y /models).
async function createRoleViaUI(ctx, name, description, prompt) {
  const btn = ctx.page.getByRole('button', { name: /Nuevo rol/ });
  await btn.first().click();
  const panel = ctx.page.locator('#modal-panel');
  await panel.waitFor({ timeout: 5000 });
  await panel.getByLabel(/Nombre \(slug\)/).first().fill(name);
  await panel.getByLabel(/Descripción/).first().fill(description);
  await panel.getByLabel(/Prompt/).first().fill(prompt);
  await panel.getByRole('button', { name: /Crear rol/ }).first().click();
  await assertToast(ctx, /Rol creado/);
}

// Limpieza best-effort del fichero del rol creado (no existe la acción de borrar
// roles en la UI); solo toca el fichero que este test creó.
async function cleanupRoleFile(roleName) {
  try {
    const home = process.env.HOME || process.env.USERPROFILE;
    const file = path.join(home, '.pi', 'agent', 'agents', `${roleName}.md`);
    if (fs.existsSync(file)) fs.unlinkSync(file);
  } catch (e) {
    // Mejor esfuerzo: si el entorno usa otro PI_AGENTS_DIR no se limpia.
  }
}

// ---- tests ----
module.exports = {
  name: 'agent_profiles',
  tests: [
    {
      name: 'list_profiles: la tarjeta de perfiles muestra los perfiles o el placeholder vacío',
      noProblems: true,
      fn: async (ctx) => {
        await ctx.goto('/profiles');
        const card = profilesCard(ctx);
        await card.first().waitFor({ timeout: 5000 });
        const chips = card.locator('button.chip');
        const placeholder = card.getByText('Aún no hay perfiles.');
        const chipCount = await chips.count();
        if (chipCount > 0) {
          await chips.first().waitFor({ timeout: 5000 });
        } else {
          await placeholder.first().waitFor({ timeout: 5000 });
        }
        const placeholderCount = await placeholder.count();
        ctx.assert(
          chipCount > 0 || placeholderCount > 0,
          'no se ve ni un perfil ni el placeholder vacío'
        );
      },
    },
    {
      name: 'create_profile: el botón "Nuevo" crea un perfil (diálogo "Nombre del perfil" + "Aceptar")',
      fn: async (ctx) => {
        const name = uniqueName('ap-create');
        await createProfileViaUI(ctx, name);
        // Limpia lo creado para no dejar perfiles en el almacén real.
        await deleteProfileViaUI(ctx, name);
      },
    },
    {
      name: 'duplicate_profile: "Duplicar" abre el diálogo precargado con "{name}-copia" y crea el duplicado',
      fn: async (ctx) => {
        const name = uniqueName('ap-dup');
        await ctx.goto('/profiles');
        // Duplica el perfil activo: se selecciona y se usa su tarjeta derecha.
        const activeChip = profilesCard(ctx).locator('button.chip').filter({ hasText: /Activo/ });
        ctx.assert(
          (await activeChip.count()) >= 1,
          'falta un perfil activo con badge "Activo" para duplicar'
        );
        await activeChip.first().click();
        await activeProfileCard(ctx).getByRole('button', { name: /Duplicar/ }).first().click();
        const panel = ctx.page.locator('#modal-panel');
        await panel.waitFor({ timeout: 5000 });
        const input = panel.getByLabel('Nombre del perfil');
        const prefill = await waitForValue(input, '-copia');
        ctx.assert(
          prefill.includes('-copia'),
          `el diálogo no está precargado con "-copia" (got "${prefill}")`
        );
        await input.fill(name);
        await panel.getByRole('button', { name: 'Aceptar' }).first().click();
        await assertToast(ctx, /Perfil duplicado/);
        ctx.assert((await profileChip(ctx, name).count()) === 1, 'el duplicado no está en la lista');
        await deleteProfileViaUI(ctx, name);
      },
    },
    {
      name: 'rename_profile: "Renombrar" cambia el nombre del perfil en la lista',
      fn: async (ctx) => {
        const from = uniqueName('ap-rename');
        const to = uniqueName('ap-rename2');
        await createProfileViaUI(ctx, from);
        await rightCard(ctx, from).getByRole('button', { name: /Renombrar/ }).first().click();
        const panel = ctx.page.locator('#modal-panel');
        await panel.waitFor({ timeout: 5000 });
        const input = panel.getByLabel('Nombre del perfil');
        const prefill = await waitForValue(input, from);
        ctx.assert(prefill === from, `el diálogo no está precargado con "${from}" (got "${prefill}")`);
        await input.fill(to);
        await panel.getByRole('button', { name: 'Aceptar' }).first().click();
        await assertToast(ctx, /Perfil renombrado/);
        ctx.assert((await profileChip(ctx, to).count()) === 1, 'el perfil renombrado no está en la lista');
        ctx.assert((await profileChip(ctx, from).count()) === 0, 'el nombre anterior sigue en la lista');
        await deleteProfileViaUI(ctx, to);
      },
    },
    {
      name: 'delete_profile: "Eliminar" con ConfirmPanel borra un perfil no activo (y el aviso de que el activo no se puede eliminar)',
      fn: async (ctx) => {
        const name = uniqueName('ap-del');
        await createProfileViaUI(ctx, name);
        const activeChip = profilesCard(ctx).locator('button.chip').filter({ hasText: /Activo/ });
        ctx.assert((await activeChip.count()) >= 1, 'falta un perfil activo con badge "Activo"');
        await deleteProfileViaUI(ctx, name);
        // El activo no se puede eliminar: lo seleccionamos y comprobamos el aviso.
        await activeChip.first().click();
        await ctx
          .page.getByText(/Es el perfil activo: no se puede eliminar/)
          .first()
          .waitFor({ timeout: 5000 });
      },
    },
    {
      name: 'export_profile: "Exportar" dispara la descarga "<name>.profile.json" y el toast "Exportado ..."',
      fn: async (ctx) => {
        const name = uniqueName('ap-export');
        await createProfileViaUI(ctx, name);
        const downloadPromise = ctx.page.waitForEvent('download', { timeout: 10000 });
        await rightCard(ctx, name)
          .getByRole('button', { name: /Exportar/ })
          .first()
          .click();
        const download = await downloadPromise;
        const expected = `${name}.profile.json`;
        ctx.assert(
          download.suggestedFilename() === expected,
          `la descarga esperada es "${expected}", got "${download.suggestedFilename()}"`
        );
        await assertToast(ctx, new RegExp(`Exportado ${escapeRe(expected)}`));
        await deleteProfileViaUI(ctx, name);
      },
    },
    {
      name: 'import_profile: "Importar" con el JSON pegado crea un perfil nuevo',
      fn: async (ctx) => {
        const name = uniqueName('ap-import');
        await ctx.goto('/profiles');
        await profilesCard(ctx).getByRole('button', { name: /Importar/ }).first().click();
        const panel = ctx.page.locator('#modal-panel');
        await panel.waitFor({ timeout: 5000 });
        // Envelope `gentle-pi.agent_model_profile` v1 con un routing válido.
        const json = JSON.stringify({
          kind: 'gentle-pi.agent_model_profile',
          version: 1,
          name: name,
          config: {
            orchestrator: { model: 'claude-bridge/claude-sonnet-5-5', thinking: 'medium' },
          },
        });
        await panel.getByLabel(/O pega el JSON/).first().fill(json);
        // Nombre opcional vacío: conserva el nombre del JSON.
        await panel.getByRole('button', { name: /Importar/ }).first().click();
        await assertToast(ctx, /Perfil importado/);
        ctx.assert((await profileChip(ctx, name).count()) === 1, 'el perfil importado no está en la lista');
        await deleteProfileViaUI(ctx, name);
      },
    },
    {
      name: 'list_agent_roles: la tarjeta "Roles de agente" muestra los roles (o su estado vacío)',
      fn: async (ctx) => {
        await ctx.goto('/profiles');
        const card = rolesCard(ctx);
        await card.first().waitFor({ timeout: 5000 });
        const items = card.locator('li');
        const placeholder = card.getByText('Aún no hay roles de agente.');
        const itemCount = await items.count();
        if (itemCount > 0) {
          await items.first().waitFor({ timeout: 5000 });
        } else {
          await placeholder.first().waitFor({ timeout: 5000 });
        }
        ctx.assert((await placeholder.count()) > 0 || itemCount > 0, 'falta el listado de roles o su placeholder');
      },
    },
    {
      name: 'create_agent_role: el diálogo "Nuevo rol" (nombre/descripción/prompt) crea un rol',
      fn: async (ctx) => {
        const roleName = uniqueName('ap-role');
        await ctx.goto('/profiles');
        await createRoleViaUI(
          ctx,
          roleName,
          'Role creado por el test e2e de agent_profiles',
          'Cuerpo del rol de prueba e2e.'
        );
        const card = rolesCard(ctx);
        ctx.assert(
          (await card.getByText(roleName).count()) > 0,
          `el rol "${roleName}" no aparece en la lista`
        );
        await cleanupRoleFile(roleName);
      },
    },
    {
      name: 'get_global_routing: /models muestra la tarjeta "Routing global" con al menos una fila',
      fn: async (ctx) => {
        await ctx.goto('/models');
        const card = modelsCard(ctx);
        await card.first().waitFor({ timeout: 5000 });
        await ensureFirstRow(ctx, card);
        const row = card.locator('li.card');
        await row.first().waitFor({ timeout: 5000 });
      },
    },
    {
      name: 'set_global_routing: cambiar una fila del Routing global y guardar muestra el toast de guardado',
      fn: async (ctx) => {
        await ctx.goto('/models');
        const card = modelsCard(ctx);
        await card.first().waitFor({ timeout: 5000 });
        const firstRow = await ensureFirstRow(ctx, card);
        const thinking = firstRow.getByLabel('Razonamiento').first();
        await thinking.waitFor({ timeout: 5000 });
        const current = await pickDifferent(thinking);
        await saveRouting(ctx, card);
        await assertToast(ctx, /Routing global guardado \(\d+ agentes\)/);
        // Restaurar el valor original para no dejar models.json modificado.
        await thinking.selectOption({ value: current });
        await saveRouting(ctx, card);
        await thinking.first().waitFor({ timeout: 5000 });
        const restored = await thinking.inputValue();
        ctx.assert(restored === current, `no se restauró el valor original (got "${restored}")`);
      },
    },
    {
      name: 'set_profile_routing: cambiar una fila del editor del perfil y guardar muestra "Enrutado guardado"',
      fn: async (ctx) => {
        await ctx.goto('/profiles');
        // Selecciona explícitamente el perfil activo (badge "Activo").
        const activeChip = profilesCard(ctx).locator('button.chip').filter({ hasText: /Activo/ });
        ctx.assert((await activeChip.count()) >= 1, 'falta un perfil activo con badge "Activo"');
        await activeChip.first().click();
        const card = ctx.page
          .locator('section.card')
          .filter({ has: ctx.page.getByText(/Es el perfil activo/).first() });
        const firstRow = await ensureFirstRow(ctx, card);
        const thinking = firstRow.getByLabel('Razonamiento').first();
        await thinking.waitFor({ timeout: 5000 });
        const current = await pickDifferent(thinking);
        await saveRouting(ctx, card);
        await assertToast(ctx, /Enrutado guardado/);
        // Restaurar el valor original para no dejar el perfil modificado.
        await thinking.selectOption({ value: current });
        await saveRouting(ctx, card);
        await thinking.first().waitFor({ timeout: 5000 });
        ctx.assert(
          (await thinking.inputValue()) === current,
          `no se restauró el valor original (got "${await thinking.inputValue()}")`
        );
      },
    },
    {
      name: 'list_model_options: el select "Modelo" de una fila tiene opciones',
      fn: async (ctx) => {
        await ctx.goto('/models');
        const card = modelsCard(ctx);
        await card.first().waitFor({ timeout: 5000 });
        const firstRow = await ensureFirstRow(ctx, card);
        const modelSelect = firstRow.getByLabel('Modelo').first();
        await modelSelect.waitFor({ timeout: 5000 });
        const count = await modelSelect.evaluate((sel) => sel.options.length);
        ctx.assert(count >= 1, `el select "Modelo" no tiene opciones (${count})`);
      },
    },
    {
      name: 'list_profile_thinking_levels: el select "Razonamiento" de una fila tiene niveles',
      fn: async (ctx) => {
        await ctx.goto('/models');
        const card = modelsCard(ctx);
        await card.first().waitFor({ timeout: 5000 });
        const firstRow = await ensureFirstRow(ctx, card);
        const thinkingSelect = firstRow.getByLabel('Razonamiento').first();
        await thinkingSelect.waitFor({ timeout: 5000 });
        const opts = await selectOptions(thinkingSelect);
        ctx.assert(opts.length >= 2, `el select "Razonamiento" no tiene niveles (${opts.length})`);
      },
    },
  ],
};
