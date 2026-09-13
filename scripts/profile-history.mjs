import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {fixtureDatabase,resetFixture,emptyFixture,legacyRequire,fixtureKey,fixturePassword} from './fixture-db.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';
import {borrowWorkspacePort,startServer,startWebBackend} from './server-process.mjs';

acquireFixtureLease();
assert.equal(spawnSync('go',['build','-o','.artifacts/admin-api','./cmd/api'],{cwd:root,stdio:'inherit'}).status,0);
const fixture=await fixtureDatabase('cross');
const token=legacyRequire('jsonwebtoken').sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'15m'});
const checks=[],restorations=[];
let go,web,publicToken;
async function call(backend,label,method,path,body,status=200) {
  const response=await fetch('http://localhost:'+(backend==='go'?3334:3333)+'/v2'+path,{
    method,headers:{'Content-Type':'application/json',Authorization:'Bearer '+(backend==='go'?token:publicToken??'')},
    body:body===undefined?undefined:JSON.stringify(body),
  });
  const data=await response.json();
  assert.equal(response.status,status,label+': '+JSON.stringify(data));
  checks.push(label);
  return data.data;
}
try {
  await resetFixture(fixture.db,fixture.schema,await new (legacyRequire('@adonisjs/hash/drivers/scrypt').Scrypt)({}).make(fixturePassword));
  for(const port of [3334,3333]) restorations.push(await borrowWorkspacePort(port,process.argv.includes('--borrow-workspace')));
  go=await startServer('go',fixture.schema,'history-go');
  web=await startWebBackend(fixture.schema,'history-web');
  const member=await call('go','Create history fixture','POST','/members',{name:'History fixture',email:'history@example.test',password:fixturePassword},201);
  const login=await call('web','Login shared member','POST','/auth/login',{email:'history@example.test',password:fixturePassword});
  publicToken=login.token.token;
  const path='/profiles/'+member.profile.id;
  for(const degree of ['high_school','diploma']) {
    for(const backend of ['web','go']) {
      await call(backend,backend+' saves '+degree,'PUT',backend==='go'?path:'/profiles',{education_history:[{degree,institution:'School',faculty:''}]});
      const adminRead=(await call('go','Admin reloads '+backend+' '+degree,'GET',path)).profile[0];
      const webRead=(await call('web','Public reloads '+backend+' '+degree,'GET','/profiles')).profile;
      assert.equal(adminRead.education_history[0].degree,degree);
      assert.equal(webRead.education_history[0].degree,degree);
      assert.equal((await fixture.db.query('SELECT education_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0].education_history[0].degree,degree);
    }
  }
  const education=[{degree:'bachelor',institution:'ITB',major:'Physics',intake_year:'2017'}];
  const work=[{job_title:'Engineer',company:'Company',start_year:'2021',end_year:null}];
  await call('web','Public accepts partial education and active work','PUT','/profiles',{education_history:education,work_history:work});
  let profile=(await call('go','Admin reads public history','GET',path)).profile[0];
  assert.equal(profile.education_history[0].faculty,'');
  assert.equal(profile.education_history[0].intake_year,2017);
  assert.equal(profile.work_history[0].start_year,2021);
  assert.equal(profile.work_history[0].end_year,undefined);
  await call('go','Admin saves imported education without faculty or year','PUT',path,{education_history:[{degree:'bachelor',institution:'ITB',major:'Physics',intake_year:null}],work_history:work});
  const persisted=(await fixture.db.query('SELECT education_history,work_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0];
  await call('go','Unrelated admin edit preserves histories','PUT',path,{name:'History fixture updated'});
  assert.deepEqual((await fixture.db.query('SELECT education_history,work_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0],persisted);
  await call('web','Unrelated public edit preserves histories','PUT','/profiles',{whatsapp:'628123456789'});
  assert.deepEqual((await fixture.db.query('SELECT education_history,work_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0],persisted);
  for(const backend of ['web','go']) {
    const endpoint=backend==='go'?path:'/profiles';
    for(const [label,body] of [
      ['reversed years',{work_history:[{job_title:'Engineer',company:'Company',start_year:2025,end_year:2021}]}],
      ['fractional year',{education_history:[{intake_year:2017.5}]}],
      ['missing work company',{work_history:[{job_title:'Engineer',company:' '}]}],
    ]) await call(backend,backend+' rejects '+label,'PUT',endpoint,body,500);
    await call(backend,backend+' clears both histories','PUT',endpoint,{education_history:[],work_history:[]});
    assert.deepEqual((await fixture.db.query('SELECT education_history,work_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0],{education_history:[],work_history:[]});
  }
  const imported=JSON.stringify([null,...education]);
  await fixture.db.query('UPDATE profiles SET education_history=$1::jsonb,work_history=$2::jsonb WHERE id=$3',[JSON.stringify(imported),JSON.stringify([null,...work]),member.profile.id]);
  const publicRead=(await call('web','Public reads serialized history and null siblings','GET','/profiles')).profile;
  profile=(await call('go','Admin reads serialized history and null siblings','GET',path)).profile[0];
  assert.deepEqual(profile.education_history,publicRead.education_history);
  assert.deepEqual(profile.work_history,publicRead.work_history);
  const activity=await call('go','Create activity using current education','POST','/activities',{
    name:'History activity',additional_config:{custom_selection_status:[],mandatory_profile_data:[{name:'current_education',required:false}],additional_questionnaire:[]},
  });
  await call('go','Register member with imported history','POST','/activities/'+activity.id+'/registrations',{user_id:member.profile.id,questionnaire_answer:{}});
  const list=await call('go','List registrations with virtual education field','GET','/activities/'+activity.id+'/registrations');
  assert.equal(list.data[0].current_education.institution,'ITB');
  const response=await fetch('http://localhost:3334/v2/activities/'+activity.id+'/registrations-export',{headers:{Authorization:'Bearer '+token}});
  assert.equal(response.status,200);
  const workbook=new (legacyRequire('exceljs').Workbook)();
  await workbook.xlsx.load(Buffer.from(await response.arrayBuffer()));
  const sheet=workbook.getWorksheet('Registrations');
  assert.equal(sheet.getCell(2,21).value,'ITB');
  assert.match(sheet.getCell(2,25).value,/ITB/);
  assert.match(sheet.getCell(2,26).value,/Engineer/);
  checks.push('Excel export survives imported history and null siblings');
  writeFileSync(resolve(root,'.artifacts/profile-history.json'),JSON.stringify({status:'passed',checks,at:new Date().toISOString()},null,2));
  console.log('Profile history: '+checks.length+' real API and database checks passed');
} finally {
  const failures=[];
  for(const cleanup of [()=>web?.stop(),()=>go?.stop(),()=>emptyFixture(fixture.db,fixture.schema),()=>fixture.db.end(),...restorations.reverse()]) {
    try { await cleanup(); } catch(error) { failures.push(error); }
  }
  if(failures.length) throw new AggregateError(failures,'History fixture cleanup or service restoration failed');
  console.log('Profile history fixture rows cleaned; workspace services restored');
}
