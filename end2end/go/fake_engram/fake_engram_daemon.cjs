// Fake Engram daemon started by fake_engram/engram: answers GET /health and GET /projects, stops on GET /_stop
// and on its own after two minutes, so it never outlives a test run.
const http = require('http');

const port = Number(process.argv[2]);
const server = http.createServer((req, res) => {
  if (req.url === '/_stop') {
    res.end('bye');
    setTimeout(() => process.exit(0), 20);
    return;
  }
  res.writeHead(200, { 'content-type': 'application/json' });
  if (req.url.startsWith('/projects')) res.end(JSON.stringify({ count: 0, projects: [] }));
  else res.end(JSON.stringify({ status: 'ok', service: 'engram', version: 'fake' }));
});
server.listen(port, '127.0.0.1');
setTimeout(() => process.exit(0), 120000).unref();
