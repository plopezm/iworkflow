// end2end/gitlab_mock.cjs
// Mock mínimo de la API REST v4 de GitLab para la suite E2E (módulo code_hosting).
// Cubre exactamente lo que usa GitLabHttpClientAdapter:
//   GET  /api/v4/user                                         (token válido: GITLAB_MOCK_TOKEN)
//   GET  /api/v4/projects/:p/repository/branches/:b           (siempre 200: la rama "está publicada")
//   GET  /api/v4/projects/:p/merge_requests?source_branch=    (lista, más reciente primero)
//   POST /api/v4/projects/:p/merge_requests                   (crea un MR abierto)
//   GET  /api/v4/projects/:p/merge_requests/:iid
//   GET  /api/v4/projects/:p/merge_requests/:iid/pipelines    (vacío)
//   PUT  /api/v4/projects/:p/merge_requests/:iid/merge        (pasa a "merged")
// Control: GET /_health, GET /_reset.
const http = require('node:http');

const PORT = process.env.GITLAB_MOCK_PORT || '9998';
const TOKEN = process.env.GITLAB_MOCK_TOKEN || 'gl-good-token';

let mrs = [];
let nextIid = 1;

function send(res, status, body) {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://127.0.0.1:${PORT}`);
  const p = url.pathname;
  if (p === '/_health') return send(res, 200, { ok: true });
  if (p === '/_reset') { mrs = []; nextIid = 1; return send(res, 200, { ok: true }); }
  if (req.headers['private-token'] !== TOKEN) return send(res, 401, { message: '401 Unauthorized' });

  let body = '';
  req.on('data', (c) => (body += c));
  req.on('end', () => {
    if (p === '/api/v4/user') return send(res, 200, { username: 'e2e-user' });
    if (/\/repository\/branches\//.test(p)) return send(res, 200, { name: 'branch' });
    let m = p.match(/\/projects\/[^/]+\/merge_requests$/);
    if (m && req.method === 'GET') {
      const src = url.searchParams.get('source_branch');
      return send(res, 200, mrs.filter((x) => !src || x.source_branch === src).reverse());
    }
    if (m && req.method === 'POST') {
      const input = JSON.parse(body || '{}');
      const mr = {
        iid: nextIid++,
        title: input.title,
        state: 'opened',
        source_branch: input.source_branch,
        target_branch: input.target_branch,
        web_url: `http://127.0.0.1:${PORT}/mr/${nextIid - 1}`,
        detailed_merge_status: 'mergeable',
        has_conflicts: false,
      };
      mrs.push(mr);
      return send(res, 201, mr);
    }
    m = p.match(/\/merge_requests\/(\d+)\/pipelines$/);
    if (m) return send(res, 200, []);
    m = p.match(/\/merge_requests\/(\d+)\/merge$/);
    if (m && req.method === 'PUT') {
      const mr = mrs.find((x) => x.iid === Number(m[1]));
      if (!mr) return send(res, 404, { message: '404 Not found' });
      mr.state = 'merged';
      return send(res, 200, mr);
    }
    m = p.match(/\/merge_requests\/(\d+)$/);
    if (m) {
      const mr = mrs.find((x) => x.iid === Number(m[1]));
      return mr ? send(res, 200, mr) : send(res, 404, { message: '404 Not found' });
    }
    return send(res, 404, { message: `no mock for ${req.method} ${p}` });
  });
});

server.listen(Number(PORT), '127.0.0.1', () => console.log(`gitlab-mock escuchando en http://127.0.0.1:${PORT}`));
