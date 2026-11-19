// Admin → phone: create a to-do for Mary through the admin API, as the web console does.
/* global http, json, output, API, E2E_PASSWORD */
var base = typeof API !== 'undefined' && API ? API : 'http://127.0.0.1:8082/api/v1';
var login = http.post(base + '/auth/login', {
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({email: 'admin@axleo.local', password: E2E_PASSWORD, device: 'Maestro'})
});
if (login.status !== 200) throw new Error('admin login: ' + login.status + ' ' + login.body);
var auth = {Authorization: 'Bearer ' + json(login.body).token};
var users = http.get(base + '/admin/users', {headers: auth});
if (users.status !== 200) throw new Error('admin users: ' + users.status);
var mary = json(users.body).filter(function (u) { return u.email === 'mary@axleo.local'; })[0];
if (!mary) throw new Error('mary@axleo.local missing from seeded users');
var created = http.post(base + '/admin/todos', {
  headers: {'Content-Type': 'application/json', Authorization: auth.Authorization},
  body: JSON.stringify({assigneeId: mary.id, d: '2026-11-08', t: 'E2E admin assigned task', kind: 'call'})
});
if (created.status !== 201) throw new Error('admin todo: ' + created.status + ' ' + created.body);
output.todoId = json(created.body).id;
