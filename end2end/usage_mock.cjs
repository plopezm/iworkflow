// end2end/usage_mock.cjs
// Mock de la API de uso OAuth de Claude para la suite E2E de usage_limits.
// Reproduce exactamente lo que usa el facade de Claude
// (internal/modules/usage_limits/infrastructure/adapters/outbound/claude):
//   GET {base}/api/oauth/usage
//     requiere `Authorization: Bearer <token>` (cualquiera) y `anthropic-beta: oauth-2025-04-20`
//     -> 200 {"five_hour":{"utilization":13.0,"resets_at":<ISO>},"seven_day":{"utilization":25.0,"resets_at":<ISO>}}
//     -> 401 si falta alguna de las dos cabeceras
// Los reinicios son fijos respecto al arranque del mock: 5 h -> dentro de 2 h 10 min, semana -> dentro de 3 días.
// Control:
//   GET /_health     -> 200 ok
// Escucha en 127.0.0.1:<USAGE_MOCK_PORT> (default 9997).
const http = require('node:http');

const PORT = process.env.USAGE_MOCK_PORT || '9997';
const BETA = 'oauth-2025-04-20';
const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

function send(res, status, body) {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
}

function usageBody(now) {
  return {
    five_hour: { utilization: 13.0, resets_at: new Date(now + 2 * HOUR_MS + 10 * MINUTE_MS).toISOString() },
    seven_day: { utilization: 25.0, resets_at: new Date(now + 3 * DAY_MS).toISOString() },
  };
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://127.0.0.1:${PORT}`);
  if (req.method === 'GET' && url.pathname === '/_health') return send(res, 200, { ok: true });
  if (req.method === 'GET' && url.pathname === '/api/oauth/usage') {
    const auth = req.headers.authorization || '';
    const authorized = /^Bearer \S+/.test(auth) && req.headers['anthropic-beta'] === BETA;
    if (!authorized) return send(res, 401, { error: 'unauthorized' });
    return send(res, 200, usageBody(Date.now()));
  }
  return send(res, 404, { error: 'not found' });
});

server.listen(Number(PORT), '127.0.0.1', () => {
  console.log(`usage mock listening on 127.0.0.1:${PORT}`);
});

const stop = () => server.close(() => process.exit(0));
process.on('SIGTERM', stop);
process.on('SIGINT', stop);
