import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { once } from 'node:events';
import { resolve } from 'node:path';
import { parseEnv } from 'node:util';
import { root, workspace } from './env.mjs';
import { fixtureDatabase, resetFixture, legacyRequire, fixturePassword, fixtureKey } from './fixture-db.mjs';
import { acquireFixtureLease } from './fixture-lease.mjs';
import { borrowWorkspacePort, startServer, startAdminFrontend, startPublicFrontend, startWebBackend } from './server-process.mjs';
import { cleanStorageJournal } from './storage-cleanup.mjs';

acquireFixtureLease();
const fixture=await fixtureDatabase('candidate');
await fixture.db.end();
const build=spawnSync('go',['build','-o',resolve(root,'.artifacts/admin-api'),'./cmd/api'],{cwd:root,stdio:'inherit'});
if(build.status!==0)throw new Error('Go build failed');
const restorations=[];
const publicBrowser=process.argv.includes('--public');
let api,frontend,web,journal;
try{
  for(const port of publicBrowser?[3334,3333,3000]:[3334,3005])restorations.push(await borrowWorkspacePort(port,process.argv.includes('--borrow-workspace')));
  const journalPath=resolve(root,'.artifacts/storage-browser-go.json');
  writeFileSync(journalPath,'[]',{flag:'wx',mode:0o600});
  journal=journalPath;
  api=await startServer('go',fixture.schema,'browser-go',{journal,origins:publicBrowser?'http://localhost:3005,http://localhost:3000':undefined});
  const frontendEnv=parseEnv(readFileSync(resolve(workspace,publicBrowser?'docs/.env.test.web-fe':'docs/.env.test.admin-fe'),'utf8'));
  if(publicBrowser){
    const seed=await fixtureDatabase('candidate');
    try{await resetFixture(seed.db,seed.schema,await new (legacyRequire('@adonisjs/hash/drivers/scrypt').Scrypt)({}).make(fixturePassword));}finally{await seed.db.end();}
    const token=legacyRequire('jsonwebtoken').sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'15m'});
    for(const [path,body] of [['provinces',{name:'Browser Province'}],['cities',{name:'Browser City',province_id:1}]]){
      const response=await fetch(`http://localhost:3334/v2/${path}`,{method:'POST',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(body)});
      if(!response.ok)throw new Error('Could not prepare public reference fixtures');
    }
    web=await startWebBackend(fixture.schema,'browser-web-be');frontend=await startPublicFrontend({...process.env,...frontendEnv});
  }
  else frontend=await startAdminFrontend({...process.env,...frontendEnv});
  const results=resolve(root,'.artifacts/browser',new Date().toISOString().replace(/[:.]/g,'-'));
  mkdirSync(results,{recursive:true});
  writeFileSync(resolve(root,'.artifacts/browser/latest.json'),JSON.stringify({results}));
  const child=spawn('node',['node_modules/@playwright/test/cli.js','test',...process.argv.slice(2).filter(x=>!['--borrow-workspace','--public'].includes(x))],{cwd:root,env:{...process.env,GO_REWRITE_BROWSER:'1',GO_REWRITE_PUBLIC_BROWSER:publicBrowser?'1':'0',GO_REWRITE_BROWSER_RESULTS:results},stdio:'inherit'});
  const [code]=await once(child,'exit');process.exitCode=code??1;
}finally{
  const failures=[];
  for(const cleanup of [()=>frontend?.stop(),()=>web?.stop(),()=>api?.stop(),async()=>{if(journal)console.log('Browser storage cleanup:',await cleanStorageJournal(journal));},...restorations.reverse()]){
    try{await cleanup();}catch(error){failures.push(error);}
  }
  if(failures.length)throw new AggregateError(failures,'Browser cleanup or workspace restoration failed');
}
