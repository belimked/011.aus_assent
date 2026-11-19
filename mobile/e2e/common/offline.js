// Cut the app off from the server: the CI proxy answers 503 to every API call.
/* global http, PROXY */
var proxy = typeof PROXY !== 'undefined' && PROXY ? PROXY : 'http://127.0.0.1:8092';
var res = http.post(proxy + '/__e2e/offline', {body: ''});
if (res.status !== 200) throw new Error('could not switch CI proxy offline: ' + res.status);
