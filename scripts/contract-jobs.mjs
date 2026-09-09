import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root,legacy,testEnvironment} from './env.mjs';
import {fixtureDatabase,emptyFixture,fixtureKey,legacyRequire} from './fixture-db.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';

acquireFixtureLease();
const jobs=['close:registration','clubs:close-registration','clubs:update-visibility'];
const {DateTime}=legacyRequire('luxon');
const day=DateTime.now().setZone('Asia/Jakarta').startOf('day'),month=day.startOf('month');
assert.equal(spawnSync('go',['build','-o',resolve(root,'.artifacts/admin-jobs'),'./cmd/jobs'],{cwd:root,stdio:'inherit'}).status,0);
const results={};
for(const [kind,suffix] of [['adonis','baseline'],['go','candidate']]){
  const f=await fixtureDatabase(suffix);
  try{
    await emptyFixture(f.db,f.schema);
    for(const [i,date] of [day.minus({days:1}),day,day.plus({days:1}),null].entries()){
      await f.db.query("INSERT INTO activities(name,slug,is_published,registration_end,created_at,updated_at) VALUES($1,$1,true,$2,'2024-01-01','2024-01-01')",['Job fixture '+i,date?.toISODate()??null]);
      await f.db.query("INSERT INTO clubs(name,is_registration_open,is_show,registration_end_date,end_period,created_at,updated_at) VALUES($1,true,true,$2,$3,'2024-01-01','2024-01-01')",['Job fixture '+i,date?.toISODate()??null,[month.minus({days:1}),month,month.plus({days:1}),null][i]?.toISODate()??null]);
    }
    const steps=[];
    for(const job of jobs)for(const iteration of [1,2]){
      const env=testEnvironment({NODE_ENV:'test',APP_KEY:fixtureKey,DB_SCHEMA:f.schema,PGOPTIONS:`-c search_path=${f.schema}`,TZ:'Asia/Jakarta',LOG_LEVEL:'info'});
      const result=kind==='adonis'?spawnSync('node',['ace',job],{cwd:legacy,env,encoding:'utf8',timeout:30000}):spawnSync(resolve(root,'.artifacts/admin-jobs'),[job],{cwd:root,env,encoding:'utf8',timeout:30000});
      writeFileSync(resolve(root,`.artifacts/job-${kind}-${job.replaceAll(':','-')}-${iteration}.log`),(result.stdout??'')+(result.stderr??''));
      assert.equal(result.status,0,`${kind} ${job} ${iteration} exited unsuccessfully`);
      const activities=(await f.db.query('SELECT id,is_published,registration_end::text,updated_at::text FROM activities ORDER BY id')).rows;
      const clubs=(await f.db.query('SELECT id,is_registration_open,is_show,registration_end_date::text,end_period::text,updated_at::text FROM clubs ORDER BY id')).rows;
      steps.push({job,iteration,activities,clubs});
      if(iteration===2)assert.deepEqual(steps.at(-1).activities,steps.at(-2).activities,'activity job idempotence');
      if(iteration===2)assert.deepEqual(steps.at(-1).clubs,steps.at(-2).clubs,'club job idempotence');
    }
    assert.deepEqual(steps.at(-1).activities.map(a=>a.is_published),[false,true,true,true]);
    assert.deepEqual(steps.at(-1).clubs.map(c=>c.is_registration_open),[false,true,true,true]);
    assert.deepEqual(steps.at(-1).clubs.map(c=>c.is_show),[false,true,true,true]);
    results[kind]=steps;
  }finally{await f.db.end();}
}
assert.deepEqual(results.go,results.adonis,'Adonis and Go job effects');
writeFileSync(resolve(root,'.artifacts/job-contracts.json'),JSON.stringify({status:'passed',timezone:'Asia/Jakarta',day:day.toISODate(),scenarios:6,results},null,2));
console.log('All three job commands match Adonis, including repeated execution');
