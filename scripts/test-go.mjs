import { spawn } from 'node:child_process';
import { readFileSync,readdirSync } from 'node:fs';
import { once } from 'node:events';
import { resolve } from 'node:path';
import { root, testEnvironment } from './env.mjs';
import { fixtureDatabase, emptyFixture } from './fixture-db.mjs';
import { acquireFixtureLease } from './fixture-lease.mjs';
import {sourceEvidence} from './source-evidence.mjs';
import {cleanStorageJournal} from './storage-cleanup.mjs';

acquireFixtureLease();

const manifest=JSON.parse(readFileSync(resolve(root,'.artifacts/schemas.json'),'utf8'));
if(manifest.status!=='ready')throw new Error('Create fixture schemas first');
const schema=manifest.schemas.find(x=>x.endsWith('_candidate'));
const fixture=await fixtureDatabase('candidate');
try { await emptyFixture(fixture.db,schema); } finally { await fixture.db.end(); }
const source=sourceEvidence();
console.log(JSON.stringify({event:'suite_source',source,dns_mode:process.env.GO_REWRITE_DIRECT_DNS==='1'?'direct-storage-fallback':'system'}));
const child=spawn('go',['test','-tags=integration','-race','-count=1','-p','1','-timeout=30m',...process.argv.slice(2)],{cwd:root,env:testEnvironment({NODE_ENV:'test',DB_SCHEMA:schema,PGOPTIONS:`-c search_path=${schema}`,GO_REWRITE_ARTIFACTS:resolve(root,'.artifacts')}),stdio:'inherit'});
try{
  const [code,signal]=await once(child,'exit');process.exitCode=code??1;if(signal)console.error('Tests interrupted:',signal);
}finally{
  const failures=[];
  for(const name of readdirSync(resolve(root,'.artifacts')).filter(name=>/^storage-.*\.json$/.test(name))){
    try{console.log('Go test storage cleanup:',await cleanStorageJournal(resolve(root,'.artifacts',name)));}catch(error){failures.push(error);}
  }
  if(failures.length)throw new AggregateError(failures,'Go test storage cleanup failed');
}
if(sourceEvidence().sha256!==source.sha256)throw new Error('Application or harness sources changed during Go tests; rerun the suite');
