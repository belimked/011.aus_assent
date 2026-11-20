// Prepare two distinct teams and a leader using the public admin API, not a modified seed.
const base = 'http://127.0.0.1:8083/api/v1/';
async function call(path, token, method = 'GET', data) {
  const res = await fetch(base + path, {
    method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: data ? JSON.stringify(data) : undefined,
  });
  if (!res.ok) throw Error(`${method} ${path}: ${res.status} ${await res.text()}`);
  return res.json();
}
const { token } = await call('auth/login', '', 'POST', {
  email: 'admin@axleo.local', password: process.env.AXLEO_SEED_PASSWORD, device: 'Web E2E bootstrap',
});
const team = await call('admin/teams', token, 'POST', { name: 'Other E2E team' });
const users = await call('admin/users', token);
const john = users.find(u => u.email === 'john@axleo.local');
const mary = users.find(u => u.email === 'mary@axleo.local');
await call('admin/users', token, 'PATCH', { id: john.id, version: john.version, teamId: team.id });
await call('admin/users', token, 'PATCH', { id: mary.id, version: mary.version, role: 'leader' });
await call('auth/logout', token, 'POST');
