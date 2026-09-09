import assert from 'node:assert/strict';
import {spawnSync,execFileSync} from 'node:child_process';
import {writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root,workspace,legacy,testEnvironment} from './env.mjs';
import {fixtureDatabase,emptyFixture,fixtureKey} from './fixture-db.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';

acquireFixtureLease();
const results=[];
for(const [name,cwd,suffix] of [['admin-be',legacy,'baseline'],['web-be',resolve(workspace,'kaderisasi-web-be'),'cross']]) {
  const fixture=await fixtureDatabase(suffix);
  try {
    await emptyFixture(fixture.db,fixture.schema);
    const args=['ace','test','unit','--files=tests/unit/club_registration_workflow.spec.ts'];
    const run=spawnSync('node',args,{cwd,env:testEnvironment({NODE_ENV:'test',APP_KEY:fixtureKey,TZ:'UTC',DB_SCHEMA:fixture.schema,PGOPTIONS:`-c search_path=${fixture.schema}`,CLUB_REGISTRATION_INTEGRATION:'1'}),encoding:'utf8',timeout:60000});
    const output=(run.stdout??'')+(run.stderr??'');
    const log=resolve(root,`.artifacts/source-club-workflow-${name}.log`);
    writeFileSync(log,output);
    assert.equal(run.status,0,`${name} source club workflow failed; see ${log}`);
    assert.match(output,/1 passed/,'the selected opt-in test must execute');
    assert.doesNotMatch(output,/\d+ skipped/,'the required source workflow must not skip');
    const residual=[];
    for(const table of ['clubs','club_registrations','public_users','profiles','custom_forms']) {
      const count=Number((await fixture.db.query(`SELECT count(*) FROM ${table}`)).rows[0].count);
      residual.push({table,count});assert.equal(count,0,`${name} transaction left fixture rows in ${table}`);
    }
    results.push({application:name,revision:execFileSync('git',['rev-parse','HEAD'],{cwd,encoding:'utf8'}).trim(),command:['node',...args].join(' '),status:'passed',timezone:'UTC',log,residual});
  } finally {await fixture.db.end();}
}
writeFileSync(resolve(root,'.artifacts/source-club-workflows.json'),JSON.stringify({status:'passed',results},null,2)+'\n');
console.log('Both original opt-in club workflows passed against owned schemas; transaction cleanup verified');
