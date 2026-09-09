import { createRequire } from 'node:module';
import { randomBytes } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { parseEnv } from 'node:util';
import { root, workspace, legacy, testEnvironment } from './env.mjs';
import { acquireFixtureLease } from './fixture-lease.mjs';

acquireFixtureLease();

const require = createRequire(resolve(legacy, 'package.json'));
const { Client } = require('pg');
const env = testEnvironment();
const prod = parseEnv(readFileSync(resolve(workspace, 'docs/.env.prod.be'), 'utf8'));
if (['DB_HOST', 'DB_PORT', 'DB_DATABASE'].every(key => env[key] === prod[key])) throw new Error('Test and production database targets must differ');
const client = new Client({ host: env.DB_HOST, port: Number(env.DB_PORT), user: env.DB_USER, password: env.DB_PASSWORD, database: env.DB_DATABASE,connectionTimeoutMillis:10000,statement_timeout:30000 });
const artifacts = resolve(root, '.artifacts');
mkdirSync(artifacts, { recursive: true });
const manifestPath = resolve(artifacts, 'schemas.json');
const quote = value => {
  if (!/^go_rewrite_[a-f0-9]{16}_(baseline|candidate|cross)$/.test(value)) throw new Error('Refusing unowned schema');
  return `"${value}"`;
};
await client.connect();
try {
  if (process.argv[2] === 'create') {
    if(existsSync(manifestPath)&&JSON.parse(readFileSync(manifestPath,'utf8')).status!=='cleaned')throw new Error('Clean the previous fixture run before creating another');
    const run = randomBytes(8).toString('hex');
    const manifest = { run, created_at: new Date().toISOString(), schemas: [], storage_keys: [], status: 'creating' };
    for (const suffix of ['baseline', 'candidate', 'cross']) {
      const name = `go_rewrite_${run}_${suffix}`;
      manifest.schemas.push(name);
      writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n');
      // Record the intended resource first. Creation and its ownership marker
      // commit atomically, so interruption leaves either no schema or an owned one.
      await client.query('BEGIN');
      try{
        await client.query(`CREATE SCHEMA ${quote(name)}`);
        await client.query(`COMMENT ON SCHEMA ${quote(name)} IS 'admin Go rewrite fixture ${run}'`);
        await client.query('COMMIT');
      }catch(error){await client.query('ROLLBACK');throw error;}
      const childEnv = testEnvironment({ PGOPTIONS: `-c search_path=${name}`, ADMIN_BOOTSTRAP_EMAILS: '', NODE_ENV: 'test' });
      const migrated = spawnSync('node', ['ace', 'migration:run', '--force'], { cwd: legacy, env: childEnv, encoding: 'utf8' });
      writeFileSync(resolve(artifacts, `${suffix}-migrations.log`), migrated.stdout + migrated.stderr);
      if (migrated.status !== 0) throw new Error(`Ace migration failed for ${suffix}; see artifact log`);
      const tables = await client.query('SELECT count(*)::int AS total FROM information_schema.tables WHERE table_schema=$1', [name]);
      if (tables.rows[0].total < 20) throw new Error('Migration did not populate the isolated schema');
      // The forward-only RBAC migration may bootstrap an account from Adonis's
      // dotenv defaults. A fresh owned schema must instead use synthetic actors.
      await client.query(`DELETE FROM ${quote(name)}.admin_users`);
      console.log(`${suffix}: ${tables.rows[0].total} tables migrated`);
    }
    manifest.status = 'ready';
    writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n');
  } else if (process.argv[2] === 'drop') {
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    if (manifest.storage_keys.length||readdirSync(artifacts).some(name=>/^storage-.*\.json$/.test(name))) throw new Error('Clean owned storage objects before dropping schemas');
    for (const name of manifest.schemas) {
      const result = await client.query('SELECT obj_description(oid) AS comment FROM pg_namespace WHERE nspname=$1', [name]);
      if (!result.rows.length) continue;
      if (result.rows[0].comment !== `admin Go rewrite fixture ${manifest.run}`) throw new Error('Schema ownership marker mismatch');
      await client.query(`DROP SCHEMA ${quote(name)} CASCADE`);
    }
    manifest.status = 'cleaned';
    const remaining=await client.query('SELECT nspname FROM pg_namespace WHERE nspname=ANY($1::text[])',[manifest.schemas]);
    if(remaining.rows.length)throw new Error('Owned schemas remain after cleanup');
    manifest.cleaned_at=new Date().toISOString();
    writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n');
    writeFileSync(resolve(artifacts,`cleaned-schemas-${manifest.run}.json`),JSON.stringify(manifest,null,2)+'\n');
    console.log('Owned fixture schemas cleaned.');
  } else throw new Error('Usage: node scripts/schema.mjs create|drop');
} finally { await client.end(); }
