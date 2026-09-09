import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { root, legacy, testEnvironment } from './env.mjs';

export const legacyRequire = createRequire(resolve(legacy, 'package.json'));
const { Client } = legacyRequire('pg');
export const fixtureKey = 'synthetic-contract-key-only-never-production';
export const fixturePassword = 'Synthetic-password-2026!';

export async function fixtureDatabase(suffix) {
  const manifest = JSON.parse(readFileSync(resolve(root, '.artifacts/schemas.json'), 'utf8'));
  if (manifest.status !== 'ready') throw new Error('Create owned schemas before running contracts');
  const schema = manifest.schemas.find(name => name.endsWith(`_${suffix}`));
  if (!schema || !/^go_rewrite_[a-f0-9]{16}_(baseline|candidate|cross)$/.test(schema)) throw new Error('Unowned fixture schema');
  const env = testEnvironment();
  const db = new Client({host:env.DB_HOST,port:Number(env.DB_PORT),user:env.DB_USER,password:env.DB_PASSWORD,database:env.DB_DATABASE,options:`-c search_path=${schema}`,connectionTimeoutMillis:10000,statement_timeout:30000});
  let connectionError;
  // pg emits idle socket failures outside an awaited query. Handle that event so
  // the next query rejects normally and the harness can run its finally blocks.
  db.on('error',error=>{
    connectionError??=error;
    console.error('Fixture database connection error:',error.code??error.name);
  });
  try { await db.connect(); } catch(error) { await db.end(); throw new Error('Fixture database connection failed', {cause:error}); }
  const ownership = await db.query('SELECT obj_description(oid) AS comment FROM pg_namespace WHERE nspname=$1', [schema]);
  if (ownership.rows[0]?.comment !== `admin Go rewrite fixture ${manifest.run}`) { await db.end(); throw new Error('Schema ownership mismatch'); }
  const current = await db.query('SELECT current_schema() AS schema, current_schemas(false)=ARRAY[$1]::name[] AS isolated', [schema]);
  if (current.rows[0].schema !== schema || !current.rows[0].isolated) { await db.end(); throw new Error('Unsafe search path'); }
  return {db,schema,manifest,assertHealthy(){if(connectionError)throw new Error('Fixture database connection was lost',{cause:connectionError});}};
}

export async function emptyFixture(db, schema) {
  const manifest=JSON.parse(readFileSync(resolve(root,'.artifacts/schemas.json'),'utf8'));
  if(!manifest.schemas.includes(schema)||!/^go_rewrite_[a-f0-9]{16}_(baseline|candidate|cross)$/.test(schema))throw new Error('Refusing to clear an unrecorded schema');
  const owner=await db.query('SELECT obj_description(oid) AS comment FROM pg_namespace WHERE nspname=$1',[schema]);
  if(owner.rows[0]?.comment!==`admin Go rewrite fixture ${manifest.run}`)throw new Error('Refusing to clear an unowned schema');
  // This is limited to an already ownership-checked, dedicated test schema.
  const tables = (await db.query("SELECT tablename FROM pg_tables WHERE schemaname=$1 AND tablename NOT LIKE 'adonis_%' ORDER BY tablename", [schema])).rows.map(row=>row.tablename);
  if (tables.some(table=>!/^[a-z_]+$/.test(table))) throw new Error('Unexpected table name');
  await db.query(`TRUNCATE ${tables.map(table=>`"${schema}"."${table}"`).join(',')} RESTART IDENTITY CASCADE`);
  return tables;
}

export async function resetFixture(db, schema, passwordHash) {
  const tables=await emptyFixture(db,schema);
  for (const [email, role, active] of [['super@example.test','super_admin',true],['requester@example.test',null,true],['inactive@example.test',null,false]]) {
    await db.query("INSERT INTO admin_users(email,normalized_email,password,display_name,is_active,role_code,created_at,updated_at) VALUES($1,$1,$2,'Synthetic administrator',$3,$4,'2024-01-02T03:04:05Z','2024-01-02T03:04:05Z')",[email,passwordHash,active,role]);
  }
  await db.query("INSERT INTO countries(code,name) VALUES('ID','Indonesia'),('MY','Malaysia')");
  return tables;
}

export async function snapshotFixture(db,tables) {
  const query = tables.map(table=>`SELECT '${table}' AS name, coalesce(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]'::jsonb) AS rows FROM "${table}" t`).join(' UNION ALL ');
  return Object.fromEntries((await db.query(query)).rows.map(row=>[row.name,row.rows]));
}
