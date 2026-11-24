// The demo seed's open homes are dated 2026-11-07. Give this flow its own
// current-day listing on the fresh CI server, rather than relying on a stale date.
/* global http, json, API, E2E_PASSWORD */
var base = API || 'http://127.0.0.1:8082/api/v1';
var signIn = http.post(base + '/auth/login', {
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({email: 'mary@axleo.local', password: E2E_PASSWORD, device: 'Maestro OFI setup'})
});
if (signIn.status !== 200) throw new Error('OFI setup login: ' + signIn.status + ' ' + signIn.body);
var token = json(signIn.body).token;
var day = new Date().toISOString().slice(0, 10);
var listing = http.post(base + '/listings', {
  headers: {'Content-Type': 'application/json', Authorization: 'Bearer ' + token},
  body: JSON.stringify({id: 'e2e-ofi-today', propertyId: 'p2', agent: 'Mary',
    price: 'E2E open home', status: 'listed', ofi: [{d: day, time: '10:00–10:30', attendees: []}]})
});
if (listing.status !== 201 && listing.status !== 200)
  throw new Error('OFI setup listing: ' + listing.status + ' ' + listing.body);
