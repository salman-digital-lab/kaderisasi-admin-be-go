import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, testEnvironment } from './env.mjs';

const env = testEnvironment();
const manifest = JSON.parse(readFileSync(resolve(root, '.artifacts/schemas.json'), 'utf8'));
if (manifest.status !== 'ready') throw new Error('Schemas must be ready');
const schema = manifest.schemas.find(x => x.endsWith('_candidate'));
const child = spawnSync('pg_dump', ['--schema-only', '--no-owner', '--no-privileges', `--schema=${schema}`], {
  encoding: 'utf8', env: { ...env, PGHOST: env.DB_HOST, PGPORT: env.DB_PORT, PGUSER: env.DB_USER, PGPASSWORD: env.DB_PASSWORD, PGDATABASE: env.DB_DATABASE },
});
if (child.status !== 0) throw new Error('Schema-only export failed');
mkdirSync(resolve(root, 'database/queries'), { recursive: true });
const sql = child.stdout.replaceAll(schema, 'public').split('\n').filter(line => !/^\\|^SET |^SELECT pg_catalog.set_config|^CREATE SCHEMA |^COMMENT ON SCHEMA/.test(line)).join('\n');
writeFileSync(resolve(root, 'database/schema.sql'), '-- GENERATED FROM ACE MIGRATIONS. sqlc input only; never execute as a migration.\n' + sql.trimEnd()+'\n');
const columns={};
for(const match of sql.matchAll(/CREATE TABLE public\.(\w+) \(([\s\S]*?)\n\);/g)){
  columns[match[1]]=[...match[2].matchAll(/^    ([a-z_]+)\s+/gm)].map(x=>x[1]);
}
writeFileSync(resolve(root,'internal/database/columns.json'),JSON.stringify(columns,null,2)+'\n');
console.log('Exported migration-generated schema for sqlc.');
