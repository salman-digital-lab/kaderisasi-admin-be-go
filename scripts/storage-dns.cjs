const dns = require('node:dns');
const {syncBuiltinESMExports} = require('node:module');
const installed = new Set();

// Opt-in workaround for a stale macOS getaddrinfo result. It performs real DNS
// queries through the configured resolver and only affects the test storage
// hostname. No address, TLS behavior, response, or storage operation is mocked.
function installStorageDNS(hostname) {
  if (installed.has(hostname)) return;
  installed.add(hostname);
  const lookup = dns.lookup;
  dns.lookup = function (host, options, callback) {
    if (typeof options === 'function') { callback = options; options = {}; }
    return lookup.call(dns, host, options, (error, address, family) => {
      if (host !== hostname || error?.code !== 'ENOTFOUND') return callback(error, address, family);
      const config = typeof options === 'number' ? {family: options} : options ?? {};
      const requestedFamily = config.family === 6 ? 6 : 4;
      const resolve = requestedFamily === 6 ? dns.resolve6 : dns.resolve4;
      resolve.call(dns, host, (resolveError, addresses) => {
        if (resolveError || !addresses.length) return callback(resolveError ?? error);
        const values = addresses.map(address => ({address, family: requestedFamily}));
        if (config.all) callback(null, values);
        else callback(null, values[0].address, values[0].family);
      });
    });
  };
  syncBuiltinESMExports();
}
module.exports = {installStorageDNS};
if (process.env.GO_REWRITE_DIRECT_DNS === '1' && process.env.DRIVE_ENDPOINT) installStorageDNS(new URL(process.env.DRIVE_ENDPOINT).hostname);
