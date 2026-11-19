// Restore the app's network path through the CI proxy.
/* global http, PROXY */
var proxy = typeof PROXY !== 'undefined' && PROXY ? PROXY : 'http://127.0.0.1:8092';
var res = http.post(proxy + '/__e2e/online', {body: ''});
if (res.status !== 200) throw new Error('could not switch CI proxy online: ' + res.status);
