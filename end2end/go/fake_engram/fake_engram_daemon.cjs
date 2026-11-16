// Fake Engram daemon started by fake_engram/engram: answers GET /health and GET /projects, stops on GET /_stop with the run's token (x-e2e-token)
// and on its own after two minutes, so it never outlives a test run.
const http = require('http');

const port = Number(process.argv[2]);
const server = http.createServer((req, res) => {
  // Writes to the output it inherited from the launcher, as a real daemon logging to stdout would: a closed pipe
  // would make this throw EPIPE and kill the daemon, which the settings e2e would then see as a start failure.
  process.stdout.write(`request ${req.url}\n`);
  if (req.url === '/_stop') {
    if (!process.env.IWF_E2E_FAKE_ENGRAM_TOKEN || req.headers['x-e2e-token'] !== process.env.IWF_E2E_FAKE_ENGRAM_TOKEN) {
      res.writeHead(403);
      res.end('forbidden');
      return;
    }
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
