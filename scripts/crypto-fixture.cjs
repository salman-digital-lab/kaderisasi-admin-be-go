const { createRequire } = require('node:module');
const { resolve } = require('node:path');
const { readFileSync } = require('node:fs');
const legacy = createRequire(resolve(__dirname, '../../kaderisasi-admin-be/package.json'));
const { Scrypt } = legacy('@adonisjs/hash/drivers/scrypt');
const { Encryption } = legacy('@adonisjs/encryption');
const jwt = legacy('jsonwebtoken');
const key = 'synthetic-test-key-never-use-outside-tests';
const password = 'Synthetic-password-2026!';
const refresh = 'synthetic-opaque-refresh-token';
const encryption = new Encryption({ secret: key });
(async () => {
  if (process.argv[2] === 'verify') {
    const data = JSON.parse(readFileSync(0, 'utf8'));
    const checked = jwt.verify(data.access, key, { clockTimestamp: 1700000100 });
    const cookie = encryption.verifier.unsign(data.cookie.slice(2), 'admin_refresh_token');
    if (!(await new Scrypt({}).verify(data.hash, password)) || checked.userId !== 42 || cookie !== refresh) throw new Error('Go to Adonis compatibility failed');
    console.log('verified');
  } else {
    console.log(JSON.stringify({ key, password, refresh, hash: await new Scrypt({}).make(password), cookie: 's:' + encryption.verifier.sign(refresh, undefined, 'admin_refresh_token'), access: jwt.sign({ userId:42, email:'fixture@example.test', iat:1700000000 }, key, { expiresIn:'15m' }) }));
  }
})().catch(error => { console.error(error.message); process.exitCode=1; });
