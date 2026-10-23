// end2end/jira_mock.cjs
// Mock del Jira Server/Data Center REST v2 para la suite E2E de iworkflow.
// Reproduce EXACTAMENTE el subconjunto que usa ReqwestJiraClientAdapter
// (src/modules/jira/infrastructure/adapters/outbound/api/reqwest_jira_client_adapter.rs):
//   GET  {base}/rest/api/2/myself
//   GET  {base}/rest/api/2/search?jql=&fields=&startAt=&maxResults=
//   GET  {base}/rest/api/2/issue/<KEY>?fields=
//   GET  {base}/rest/api/2/issue/<KEY>/comment?maxResults=
//   POST {base}/rest/api/2/issue/<KEY>/comment           body {"body": str}
//   GET  {base}/rest/api/2/issue/<KEY>/transitions
//   POST {base}/rest/api/2/issue/<KEY>/transitions       body {"transition":{"id":str}}
// Control (no Jira):
//   POST /_reset   -> re-siembra issues, limpia comentarios añadidos y estados
//   GET  /_health  -> 200 ok
//
// Escucha en 127.0.0.1:<JIRA_MOCK_PORT> (default 9999).
// El cliente envía `Authorization: Bearer <token>` y `Accept: application/json`.
const http = require('node:http');
const url = require('node:url');

const PORT = process.env.JIRA_MOCK_PORT || '9999';

const SEED = [
  {
    key: 'ABC-1',
    fields: {
      summary: 'Falla el login',
      description: 'El login falla en el navegador.',
      status: { name: 'To Do' },
      issuetype: { name: 'Bug' },
      priority: { name: 'High' },
      assignee: { displayName: 'Ana', name: 'ana' },
      reporter: { displayName: 'Luis', name: 'luis' },
      labels: ['backend', 'urgente'],
      updated: '2024-05-01T10:00:00.000+0200',
    },
  },
  {
    key: 'ABC-2',
    fields: {
      summary: 'Mejorar el onboarding',
      description: 'Simplificar el primer uso.',
      status: { name: 'In Progress' },
      issuetype: { name: 'Task' },
      priority: { name: 'Medium' },
      assignee: { displayName: 'Ana', name: 'ana' },
      reporter: { name: 'luis' },
      labels: ['onboarding'],
      updated: '2024-05-02T09:00:00.000+0200',
    },
  },
  {
    key: 'ABC-3',
    fields: {
      summary: 'Documentar la API',
      description: 'Escribir la documentación de la API.',
      status: { name: 'Done' },
      issuetype: { name: 'Task' },
      priority: null,
      assignee: null,
      reporter: { displayName: 'Luis', name: 'luis' },
      labels: [],
      updated: '2024-05-03T12:00:00.000+0200',
    },
  },
];

const TRANSITIONS = [
  { id: '11', name: 'Iniciar', to: { name: 'In Progress' } },
  { id: '31', name: 'Cerrar', to: { name: 'Done' } },
  { id: '10', name: 'Reabrir', to: { name: 'In Progress' } },
];

let state = {};

function seed() {
  state = { issues: {}, comments: {} };
  for (const i of SEED) {
    state.issues[i.key] = { key: i.key, fields: JSON.parse(JSON.stringify(i.fields)) };
    state.comments[i.key] = [];
  }
}
seed();

function json(res, value, status = 200) {
  res.writeHead(status, { 'Content-Type': 'application/json', 'Access-Control-Allow-Origin': '*' });
  res.end(JSON.stringify(value, null, 2));
}

function error(res, status, message) {
  json(res, { errorMessages: [message || 'error'], errors: {} }, status);
}

function issueDoc(key) {
  return state.issues[key] ? { ...state.issues[key] } : null;
}

async function readBody(req) {
  return new Promise((resolve) => {
    const chunks = [];
    req.on('data', (d) => chunks.push(d));
    req.on('end', () => resolve(Buffer.concat(chunks).toString()));
  });
}

function parseJson(raw) {
  const t = String(raw || '').trim();
  if (!t) return null;
  try {
    return JSON.parse(t);
  } catch {
    return null;
  }
}

const handlers = {};

handlers['GET myself'] = () => ({ status: 200, body: { id: '10001', displayName: 'E2E User', name: 'e2e' } });

handlers['GET search'] = () => {
  const all = Object.values(state.issues).map((i) => ({ ...i }));
  return { status: 200, body: { startAt: 0, total: all.length, issues: all } };
};

function issueResult(key) {
  const doc = issueDoc(key);
  if (!doc) return { status: 404, body: { errorMessages: ['Issue not found'] } };
  return { status: 200, body: doc };
}

handlers['GET issue'] = (key) => issueResult(key);
handlers['GET comment'] = (key) => {
  if (!issueDoc(key)) return { status: 404, body: { errorMessages: ['Issue not found'] } };
  return { status: 200, body: { comments: state.comments[key].map((c) => ({ ...c })) } };
};
handlers['GET transitions'] = (key) => {
  if (!issueDoc(key)) return { status: 404, body: { errorMessages: ['Issue not found'] } };
  return { status: 200, body: { transitions: TRANSITIONS.map((t) => ({ ...t })) } };
};

async function addComment(req, key) {
  if (!issueDoc(key)) return error(resPlaceholder(), 404, 'Issue not found');
  const raw = await readBody(req);
  const parsed = parseJson(raw) || { body: raw };
  const body = String(parsed.body);
  const id = String(Date.now() + Math.random().toString(36).slice(2));
  const comment = {
    id,
    author: { displayName: 'E2E', name: 'e2e' },
    body,
    created: '2024-05-10T10:00:00.000+0200',
  };
  state.comments[key].push(comment);
  return comment;
}

function resPlaceholder() {
  return { writeHead: () => {}, end: () => {} };
}

async function run(req, res, route, key, action) {
  if (route === 'myself' && req.method === 'GET') {
    const r = handlers['GET myself']();
    json(res, r.body, r.status);
    return;
  }
  if (route === 'search' && req.method === 'GET') {
    const r = handlers['GET search']();
    json(res, r.body, r.status);
    return;
  }
  if (key && route === 'issue') {
    if (action === null) {
      if (req.method === 'GET') {
        const r = handlers['GET issue'](key);
        json(res, r.body, r.status);
        return;
      }
    }
    if (action === 'comment') {
      if (req.method === 'GET') {
        const r = handlers['GET comment'](key);
        json(res, r.body, r.status);
        return;
      }
      if (req.method === 'POST') {
        if (!issueDoc(key)) return error(res, 404, 'Issue not found');
        const raw = await readBody(req);
        const parsed = parseJson(raw) || { body: raw };
        const body = String(parsed.body);
        const id = String(Date.now() + Math.random().toString(36).slice(2));
        state.comments[key].push({
          id,
          author: { displayName: 'E2E', name: 'e2e' },
          body,
          created: '2024-05-10T10:00:00.000+0200',
        });
        return json(res, { id, author: { displayName: 'E2E', name: 'e2e' }, body, created: '2024-05-10T10:00:00.000+0200' });
      }
    }
    if (action === 'transitions') {
      if (req.method === 'GET') {
        const r = handlers['GET transitions'](key);
        json(res, r.body, r.status);
        return;
      }
      if (req.method === 'POST') {
        const raw = await readBody(req);
        const parsed = parseJson(raw);
        const tid = String(parsed?.transition?.id || '');
        const transition = TRANSITIONS.find((t) => t.id === tid);
        const doc = issueDoc(key);
        if (!doc) return error(res, 404, 'Issue not found');
        if (!transition) return error(res, 400, 'Transition id no válido');
        doc.fields.status = { name: transition.to.name };
        return json(res, {});
      }
    }
  }
  error(res, 404, 'ruta no soportada');
}

const server = http.createServer(async (req, res) => {
  const u = new url.URL(req.url, `http://${req.headers.host || 'localhost'}`);
  const path = u.pathname;
  void u.searchParams;

  if (path === '/_health') return json(res, { ok: true, service: 'jira-mock', issues: Object.keys(state.issues) });
  if (path === '/_reset') {
    seed();
    return json(res, { reset: true, issues: Object.keys(state.issues) });
  }

  if (!path.startsWith('/rest/api/2/')) return error(res, 404, 'ruta desconocida');
  const clean = path.replace(/^\/rest\/api\/2\//, '').replace(/^\//, '');
  const parts = clean.split('/').filter(Boolean);
  const route = parts.length ? parts[0] : null;
  let key = null;
  let action = null;
  if (route === 'issue' && parts.length >= 2) {
    key = parts[1];
    if (parts.length >= 3) action = parts[2];
  }

  await run(req, res, route, key, action);
});

server.listen(PORT, '127.0.0.1', () => {
  console.log(`jira-mock escuchando en http://127.0.0.1:${PORT}`);
});
