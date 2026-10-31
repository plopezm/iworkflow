// end2end/go/transcription.e2e.cjs
// Voice dictation of the prompt field: the mic records, the user reviews the recording, and the transcript is
// appended to the prompt as a new paragraph. The browser gets a fake microphone (Chromium flags), so the whole path
// runs: MediaRecorder, resampling to 16 kHz WAV, POST /transcriptions and the text back into the textarea.
//   - dictation_append: the server answer is stubbed, so the test pins the client behaviour without the model.
//   - dictation_real_model: the fake microphone plays a Spanish test recording of the Qwen3-ASR model and the real
//     recognizer answers. It runs only when run_go.sh found the model (IWF_ASR_MODEL_DIR); `make model` downloads it.
// The suite runs in Spanish (the language cookie of the harness), so the texts are the Spanish ones.
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/home/pablolm/.cache/iwf-playwright/node_modules/playwright');
const tag = Date.now().toString();
const TMP = process.env.SMOKE_TMP_DIR || process.env.WORK_TMP_DIR || '/tmp';
const MODEL_DIR = process.env.IWF_ASR_MODEL_DIR || '';

function uniqueGitRepo(prefix) {
  const dir = path.join(TMP, `dictation-repo-${prefix}-${tag}`);
  fs.mkdirSync(dir, { recursive: true });
  execSync(`git -C '${dir}' init -q -b main`, { stdio: 'ignore' });
  fs.writeFileSync(path.join(dir, 'README.md'), `dictation ${prefix}\n`);
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l add .`, { stdio: 'ignore' });
  execSync(`git -C '${dir}' -c user.name=t -c user.email=t@l commit -qm init`, { stdio: 'ignore' });
  return dir;
}

// Registers a workspace so the board can open the create dialog; the dialog lives on the main (non-ctx) browser.
async function registerWorkspace(page, base, name) {
  const dir = uniqueGitRepo(name);
  await page.goto(`${base}/workspaces`, { waitUntil: 'domcontentloaded' });
  await page.getByLabel('Nombre').first().fill(name);
  await page.getByLabel(/^Ruta absoluta/).first().fill(dir);
  await page.getByLabel(/Rama base/).first().fill('main');
  await page.getByRole('button', { name: /Registrar|Register/ }).first().click();
  await page.getByText(dir).first().waitFor({ timeout: 12000 });
}

// Runs fn with a page of a Chromium whose microphone is fake: it plays `audioFile`, or a beep without one.
async function withFakeMicrophone(ctx, audioFile, fn) {
  const args = ['--use-fake-device-for-media-stream', '--use-fake-ui-for-media-stream'];
  if (audioFile) args.push(`--use-file-for-fake-audio-capture=${audioFile}%noloop`);
  const browser = await chromium.launch(process.env.SMOKE_CHROMIUM_PATH ? { executablePath: process.env.SMOKE_CHROMIUM_PATH, args } : { args });
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 860 }, permissions: ['microphone'] });
    await context.addCookies([{ name: 'iw-lang', value: 'es', url: ctx.base }]);
    const page = await context.newPage();
    await fn(page);
  } finally {
    await browser.close();
  }
}

// Opens the create dialog of a workspace board and returns its prompt field.
async function openCreateDialog(page, base, workspaceName) {
  await registerWorkspace(page, base, workspaceName);
  await page.locator('aside').getByRole('link', { name: new RegExp(workspaceName) }).first().click();
  await page.getByRole('button', { name: 'Nueva tarea' }).first().click();
  const prompt = page.getByLabel('Prompt');
  await prompt.waitFor({ timeout: 10000 });
  return prompt;
}

async function recordFor(page, ms) {
  await page.getByRole('button', { name: 'Grabar audio' }).click();
  await page.getByRole('button', { name: 'Detener la grabación' }).waitFor({ timeout: 10000 });
  await page.waitForTimeout(ms);
  await page.getByRole('button', { name: 'Detener la grabación' }).click();
  await page.getByText('¿Está bien la grabación?').waitFor({ timeout: 10000 });
}

module.exports = {
  name: 'transcription',
  tests: [
    {
      name: 'dictation_append: la grabación se revisa antes de enviarse y el texto se añade como párrafo nuevo',
      fn: async (ctx) => {
        await withFakeMicrophone(ctx, '', async (page) => {
          const bodies = [];
          await page.route('**/transcriptions', async (route) => {
            bodies.push(route.request().postDataBuffer());
            await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ text: 'Texto dictado' }) });
          });
          const prompt = await openCreateDialog(page, ctx.base, `dict-${tag}`);
          await prompt.fill('Primera línea');

          // Discarding sends nothing and leaves the prompt as it was.
          await recordFor(page, 1200);
          await page.getByRole('button', { name: 'Descartar' }).click();
          ctx.assert(bodies.length === 0, 'descartar no debe enviar la grabación');
          ctx.assert((await prompt.inputValue()) === 'Primera línea', 'descartar no debe tocar el prompt');

          // Confirming sends a mono 16 kHz PCM16 WAV and appends the text after a blank line.
          await recordFor(page, 1200);
          await page.getByRole('button', { name: 'Transcribir' }).click();
          await page.waitForFunction(() => document.querySelector('textarea[name="prompt"]').value.includes('Texto dictado'), null, { timeout: 15000 });
          ctx.assert((await prompt.inputValue()) === 'Primera línea\n\nTexto dictado', `prompt inesperado: ${JSON.stringify(await prompt.inputValue())}`);
          ctx.assert(await page.locator('[data-dictation-panel]').isHidden(), 'el panel debe cerrarse tras transcribir');
          ctx.assert(bodies.length === 1, `se esperaba 1 envío, hubo ${bodies.length}`);
          const wav = bodies[0];
          ctx.assert(wav.subarray(0, 4).toString() === 'RIFF' && wav.subarray(8, 12).toString() === 'WAVE', 'el cuerpo no es un WAV');
          ctx.assert(wav.readUInt16LE(22) === 1 && wav.readUInt32LE(24) === 16000 && wav.readUInt16LE(34) === 16, 'el WAV debe ser mono, 16 kHz y 16 bits');
        });
      },
    },
    {
      name: 'dictation_real_model: el servidor transcribe una grabación real y la añade al prompt vacío',
      fn: async (ctx) => {
        const audio = MODEL_DIR && path.join(MODEL_DIR, 'test_wavs', 'es1.wav');
        if (!audio || !fs.existsSync(audio)) {
          console.log('  (modelo no descargado: se omite; ejecuta `make model`)');
          return;
        }
        await withFakeMicrophone(ctx, audio, async (page) => {
          const prompt = await openCreateDialog(page, ctx.base, `dict-real-${tag}`);
          await recordFor(page, 7000);
          await page.getByRole('button', { name: 'Transcribir' }).click();
          await page.waitForFunction(() => /talla/i.test(document.querySelector('textarea[name="prompt"]').value), null, { timeout: 60000 });
          const text = (await prompt.inputValue()).toLowerCase();
          // The browser compresses the fake microphone, so a word or two may differ from the reference text.
          ctx.assert(text.includes('recomiendo'), `la transcripción no contiene "recomiendo": ${text}`);
          ctx.assert(await page.locator('[data-dictation-panel]').isHidden(), 'el panel debe cerrarse tras transcribir');
          ctx.assert(!text.startsWith('\n'), 'un prompt vacío no debe empezar con un párrafo en blanco');
        });
      },
    },
  ],
};
