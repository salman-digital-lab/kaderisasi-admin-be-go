import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {fixtureDatabase,resetFixture,legacyRequire,fixturePassword} from './fixture-db.mjs';
import {borrowWorkspacePort,startServer} from './server-process.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';

acquireFixtureLease();

const {Scrypt}=legacyRequire('@adonisjs/hash/drivers/scrypt');
const {db,schema}=await fixtureDatabase('cross');
const hash=await new Scrypt({}).make(fixturePassword);
await resetFixture(db,schema,hash);
if(spawnSync('go',['build','-o',resolve(root,'.artifacts/admin-api'),'./cmd/api'],{cwd:root,stdio:'inherit'}).status!==0){await db.end();throw new Error('Go build failed');}
const restore=await borrowWorkspacePort(3334,process.argv.includes('--borrow-workspace'));
const checks=[];
let server,current;
async function select(kind){if(server)await server.stop();server=undefined;server=await startServer(kind,schema,`transfer-${kind}`);current=kind;}
async function call(label,method,path,body,{access,cookie,status=200}={}){
  const headers={'Content-Type':'application/json',Accept:'application/json'};
  if(access)headers.Authorization=`Bearer ${access}`;
  if(cookie)headers.Cookie=cookie;
  const response=await fetch('http://127.0.0.1:3334'+path,{method,headers,body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(10000)});
  const text=await response.text();assert.equal(response.status,status,`${current} ${label}: ${text}`);
  const data=JSON.parse(text);checks.push({backend:current,label,status:response.status});
  return {data:data.data,access:data.data?.access_token,cookie:response.headers.getSetCookie()[0]?.split(';')[0]};
}
try{
  await select('adonis');
  const original=await call('Adonis password login','POST','/v2/auth/login',{email:'super@example.test',password:fixturePassword});
  await call('Adonis creates password for transfer','PUT','/v2/admin-users/2/password',{password:'Adonis-generated-password!'},{access:original.access});
  await select('go');
  const consumed=await call('Go consumes Adonis access token','GET','/v2/auth/me',undefined,{access:original.access});
  assert.equal(consumed.data.user.id,1);
  const rotated=await call('Go rotates Adonis signed refresh cookie','POST','/v2/auth/refresh',{},original);
  await call('Go verifies Adonis-created password','POST','/v2/auth/login',{email:'requester@example.test',password:'Adonis-generated-password!'});
  await call('Go creates password for transfer','PUT','/v2/admin-users/2/password',{password:'Go-generated-password!'},{access:rotated.access});
  await select('adonis');
  await call('Adonis consumes Go access token','GET','/v2/auth/me',undefined,{access:rotated.access});
  const returned=await call('Adonis rotates Go signed refresh cookie','POST','/v2/auth/refresh',{},rotated);
  await call('Adonis verifies Go-created password','POST','/v2/auth/login',{email:'requester@example.test',password:'Go-generated-password!'});
  await select('go');
  await call('Go rotates session returned by Adonis','POST','/v2/auth/refresh',{},returned);
  await call('Go rejects reuse from before transfer','POST','/v2/auth/refresh',{}, {...original,status:401});
  const active=(await db.query("SELECT count(*)::int AS total FROM admin_refresh_tokens WHERE admin_user_id=1 AND revoked_at IS NULL")).rows[0].total;
  assert.equal(active,0,'replay revokes the complete family across both implementations');
  checks.push({label:'Cross-language refresh family fully revoked',status:'passed'});
  writeFileSync(resolve(root,'.artifacts/session-transfer.json'),JSON.stringify({schema,status:'passed',checks},null,2));
  console.log(`Session/password transfer: ${checks.length} checks passed in both directions`);
}finally{if(server)await server.stop();await db.end();await restore();}
