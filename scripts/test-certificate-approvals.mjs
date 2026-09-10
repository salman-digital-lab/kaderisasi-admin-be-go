import { randomBytes } from 'node:crypto';
import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { root, migrations, testEnvironment } from './env.mjs';

const require = createRequire(resolve(migrations, 'package.json'));
const { Client } = require('pg');
const env = testEnvironment();
const run = randomBytes(8).toString('hex');
const schema = `go_rewrite_${run}_candidate`;
const marker = `certificate approval test ${run}`;
const artifacts = resolve(root, '.artifacts/certificate-approvals', run);
mkdirSync(artifacts, { recursive: true });
const evidence = { schema, marker, status: 'running', cleaned: false };
const record = () => writeFileSync(resolve(artifacts, 'result.json'), JSON.stringify(evidence, null, 2));
record();
const client = new Client({ host: env.DB_HOST, port: Number(env.DB_PORT), user: env.DB_USER, password: env.DB_PASSWORD, database: env.DB_DATABASE, connectionTimeoutMillis: 10000 });
await client.connect();
async function command(command, args, cwd) {
  const child = spawn(command, args, { cwd, env: testEnvironment({ NODE_ENV: 'test', DB_SCHEMA: schema, PGOPTIONS: `-c search_path=${schema}`, ADMIN_BOOTSTRAP_EMAILS: 'bootstrap@example.test', GO_REWRITE_ARTIFACTS: artifacts }), stdio: 'inherit' });
  const [code] = await once(child, 'exit');
  if (code !== 0) throw new Error(`${command} failed with exit ${code}`);
}
try {
  await client.query('BEGIN');
  await client.query(`CREATE SCHEMA "${schema}"`);
  await client.query(`COMMENT ON SCHEMA "${schema}" IS '${marker}'`);
  await client.query('COMMIT');
  // This freshly created UUID schema belongs exclusively to this process.
  // Avoid the database-wide advisory lock used by unrelated Ace test schemas.
  await command('node', ['ace', 'migration:run', '--force', '--disable-locks', '--compact-output'], migrations);
  await command('go', ['test', '-tags=integration', '-race', '-count=1', '-timeout=5m', './internal/httpapi', '-run', 'TestCertificateApprovalWorkflow|TestCertificateIssuanceSnapshotsAndRevocation'], root);
  evidence.status = 'passed';
} catch (error) {
  await client.query('ROLLBACK');
  evidence.status = 'failed';
  process.exitCode = 1;
  console.error(error.message);
} finally {
  const result = await client.query('SELECT obj_description(oid) AS comment FROM pg_namespace WHERE nspname=$1', [schema]);
  if (result.rows[0]?.comment === marker) {
    await client.query(`DROP SCHEMA "${schema}" CASCADE`);
    evidence.cleaned = true;
  }
  record();
  await client.end();
  console.log(JSON.stringify(evidence));
}
