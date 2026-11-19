// CI-only pass-through proxy in front of the Go server so flows can simulate losing signal.
// The app is built with EXPO_PUBLIC_API_URL pointing here; nothing in the app or server changes.
//   POST /__e2e/offline → every other request gets 503 (as a dead network would fail fetch)
//   POST /__e2e/online  → forward again
// Env: E2E_PROXY_PORT (8092), E2E_UPSTREAM (http://127.0.0.1:8082)
const http = require('node:http');
const port = Number(process.env.E2E_PROXY_PORT || 8092);
const upstream = new URL(process.env.E2E_UPSTREAM || 'http://127.0.0.1:8082');
let offline = false;
http.createServer((req, res) => {
  if (req.url === '/__e2e/offline' || req.url === '/__e2e/online') {
    offline = req.url === '/__e2e/offline';
    console.log(new Date().toISOString(), offline ? 'OFFLINE' : 'ONLINE');
    res.writeHead(200, {'Content-Type': 'text/plain'}).end(offline ? 'offline' : 'online');
    return;
  }
  if (offline) {
    console.log(new Date().toISOString(), 'dropped', req.method, req.url);
    req.resume();
    res.writeHead(503, {'Content-Type': 'text/plain'}).end('CI network disconnected');
    return;
  }
  const out = http.request({host: upstream.hostname, port: upstream.port, path: req.url, method: req.method, headers: req.headers}, reply => {
    res.writeHead(reply.statusCode || 502, reply.headers);
    reply.pipe(res);
  });
  out.on('error', err => { if (!res.headersSent) res.writeHead(502); res.end(err.message); });
  req.pipe(out);
}).listen(port, '127.0.0.1', () => console.log(`e2e proxy :${port} → ${upstream.origin}`));
