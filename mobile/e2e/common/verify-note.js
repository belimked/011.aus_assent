// Phone → admin: the property note typed on the phone must reach the server (owner c1 = Mary Chen).
/* global http, json, API, E2E_PASSWORD */
var base = typeof API !== 'undefined' && API ? API : 'http://127.0.0.1:8082/api/v1';
var want = typeof NOTE !== 'undefined' && NOTE ? NOTE : 'E2E synced property note';
var login = http.post(base + '/auth/login', {
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({email: 'admin@axleo.local', password: E2E_PASSWORD, device: 'Maestro'})
});
if (login.status !== 200) throw new Error('admin login: ' + login.status);
var auth = {Authorization: 'Bearer ' + json(login.body).token};
// Push is async (2 s debounce); poll up to ~30 s.
var found = false;
for (var i = 0; i < 30 && !found; i++) {
  var res = http.get(base + '/admin/contacts?q=Mary', {headers: auth});
  if (res.status !== 200) throw new Error('admin contacts: ' + res.status);
  found = res.body.indexOf(want) >= 0;
  if (!found) { var until = Date.now() + 1000; while (Date.now() < until) { /* wait */ } }
}
if (!found) throw new Error('note "' + want + '" not on server: sync push did not arrive');
