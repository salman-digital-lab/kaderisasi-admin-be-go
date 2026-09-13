import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {resolve} from 'node:path';
import {spawnSync} from 'node:child_process';
import {root,migrations,testEnvironment} from './env.mjs';
import {fixtureDatabase,resetFixture,legacyRequire,fixtureKey,fixturePassword} from './fixture-db.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';
import {borrowWorkspacePort,startServer,startWebBackend} from './server-process.mjs';
import {sourceEvidence} from './source-evidence.mjs';

const release=acquireFixtureLease();
const fixture=await fixtureDatabase('cross');
const record={status:'running',schema:fixture.schema,source:sourceEvidence(),checks:[]};
const save=()=>writeFileSync(resolve(root,'.artifacts/scoring.json'),JSON.stringify(record,null,2));
const restorations=[];
let api,web;
const token=legacyRequire('jsonwebtoken').sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'1h',audience:'kaderisasi-admin'});
const noRole=legacyRequire('jsonwebtoken').sign({userId:2,email:'requester@example.test'},fixtureKey,{expiresIn:'1h',audience:'kaderisasi-admin'});
async function call(owner,label,method,path,body,options={}) {
  const headers={Accept:'application/json'};
  const access=options.token??(owner==='admin'?token:undefined);
  if(access)headers.Authorization=`Bearer ${access}`;
  if(!(body instanceof FormData))headers['Content-Type']='application/json';
  const response=await fetch(`http://127.0.0.1:${owner==='admin'?3334:3333}/v2${path}`,{method,headers,body:body instanceof FormData?body:body===undefined?undefined:JSON.stringify(body)});
  const text=await response.text();
  assert.equal(response.status,options.status??200,`${label}: ${text}`);
  record.checks.push({owner,label,method,path,status:response.status});
  if(response.headers.get('content-type')?.includes('json'))return JSON.parse(text).data;
}
const rubric={revision:0,groups:[{id:'character',name:'Karakter',criteria:[{id:'honesty',name:'Shiddiq',maximum:100,weight:1},{id:'trust',name:'Amanah',maximum:50,weight:3}]}],grades:[{label:'B',minimum:0},{label:'A',minimum:80}],note:'Catatan kegiatan'};
const upload=(buffer,hash)=>{const body=new FormData();body.append('file',new Blob([buffer]),'scores.xlsx');if(hash)body.append('preview_hash',hash);return body;};
async function workbook(activityId,mode='template') {
  const response=await fetch(`http://127.0.0.1:3334/v2/activities/${activityId}/scoring/excel?mode=${mode}`,{headers:{Authorization:`Bearer ${token}`}});
  assert.equal(response.status,200,await response.clone().text());
  record.checks.push({owner:'admin',label:`Excel ${mode} download`,method:'GET',path:`/activities/${activityId}/scoring/excel?mode=${mode}`,status:response.status});
  const book=new (legacyRequire('exceljs').Workbook)();await book.xlsx.load(Buffer.from(await response.arrayBuffer()));return book;
}
try {
  mkdirSync(resolve(root,'.artifacts/scoring-browser'),{recursive:true});save();
  const migrated=spawnSync('node',['ace','migration:run','--force'],{cwd:migrations,env:testEnvironment({NODE_ENV:'test',DB_SCHEMA:fixture.schema,PGOPTIONS:`-c search_path=${fixture.schema}`}),encoding:'utf8'});
  writeFileSync(resolve(root,'.artifacts/scoring-migrations.log'),migrated.stdout+migrated.stderr);
  assert.equal(migrated.status,0,'Scoring migration failed; see scoring-migrations.log');
  const passwordHash=await new (legacyRequire('@adonisjs/hash/drivers/scrypt').Scrypt)({}).make(fixturePassword);
  await resetFixture(fixture.db,fixture.schema,passwordHash);
  assert.equal(spawnSync('go',['build','-o',resolve(root,'.artifacts/admin-api'),'./cmd/api'],{cwd:root,stdio:'inherit'}).status,0);
  for(const port of [3334,3333])restorations.push(await borrowWorkspacePort(port,process.argv.includes('--borrow-workspace')));
  api=await startServer('go',fixture.schema,'scoring-go',{origins:'http://localhost:3005,http://localhost:3000'});
  web=await startWebBackend(fixture.schema,'scoring-web');
  const activity=await call('admin','create activity','POST','/activities',{name:'Penilaian kegiatan uji',activity_type:1});
  const other=await call('admin','create another activity','POST','/activities',{name:'Kegiatan lain',activity_type:1});
  const member=await call('admin','create participant','POST','/members',{name:'Peserta Penilaian',email:'score-participant@example.test',password:fixturePassword},{status:201});
  const outsider=await call('admin','create other participant','POST','/members',{name:'Peserta Lain',email:'score-outsider@example.test',password:fixturePassword},{status:201});
  const login=await call('web','participant login','POST','/auth/login',{email:'score-participant@example.test',password:fixturePassword});
  const otherLogin=await call('web','other participant login','POST','/auth/login',{email:'score-outsider@example.test',password:fixturePassword});
  const participantToken=login.token.token;
  const registration=await call('admin','register member','POST',`/activities/${activity.id}/registrations`,{user_id:member.profile.id,questionnaire_answer:{reason:'keep'}});
  const guest=(await fixture.db.query("INSERT INTO activity_registrations(activity_id,status,guest_data,questionnaire_answer,created_at,updated_at) VALUES($1,'TERDAFTAR','{\"name\":\"Peserta Tamu\"}','{}',now(),now()) RETURNING id",[activity.id])).rows[0];
  const foreign=(await fixture.db.query("INSERT INTO activity_registrations(activity_id,status,guest_data,created_at,updated_at) VALUES($1,'TERDAFTAR','{\"name\":\"Foreign\"}',now(),now()) RETURNING id",[other.id])).rows[0];
  const base=`/activities/${activity.id}/scoring`;
  const publicPath=`/activities/${activity.slug}/registration`;
  await call('admin','initial rubric absent','GET',`${base}/rubric`);
  await call('admin','save rubric','PUT',`${base}/rubric`,rubric);
  await call('admin','stale rubric rejected','PUT',`${base}/rubric`,rubric,{status:409});
  for(const route of JSON.parse(readFileSync(resolve(root,'internal/httpapi/routes.json'),'utf8')).filter(route=>route.controller==='scoring_controller')) {
    const path=route.path.replace('/v2','').replace(':id',String(activity.id)).replace(':registrationId',String(registration.id));
    for(const [access,status] of [['',401],[noRole,403],[participantToken,401]])await call('admin',`${route.action}: authorization ${status}`,route.method,path,route.method==='GET'?undefined:{},{token:access,status});
    await call('admin',`${route.action}: invalid identifier`,route.method,path.replace(`/activities/${activity.id}`, '/activities/invalid'),route.method==='GET'?undefined:{},{status:404});
    const input=route.action==='saveRubric'?rubric:route.action==='save'?{revision:0,rubric_revision:1,draft:{scores:{honesty:1},note:''}}:route.action==='publish'||route.action==='withdraw'?{rubric_revision:1,selections:[{registration_id:registration.id,revision:1}]}:route.action==='preview'||route.action==='commit'?upload(Buffer.from('fixture')):undefined;
    const missing=path.replace(`/activities/${activity.id}`, '/activities/2147483647')+(route.action==='excel'?'?mode=template':'');
    await call('admin',`${route.action}: missing activity`,route.method,missing,input,{status:404});
  }
  const draft=(revision,scores,note='Draf pribadi')=>({revision,rubric_revision:1,draft:{scores,note}});
  const batch=(...entries)=>({rubric_revision:1,selections:entries.map(([registration_id,revision])=>({registration_id,revision}))});
  await call('admin','cross-activity write denied','PUT',`${base}/registrations/${foreign.id}`,draft(0,{honesty:80}),{status:404});
  await call('admin','out-of-range rejected','PUT',`${base}/registrations/${registration.id}`,draft(0,{trust:51}),{status:422});
  await call('admin','unknown criterion rejected','PUT',`${base}/registrations/${registration.id}`,draft(0,{unknown:1}),{status:422});
  await call('admin','save partial draft','PUT',`${base}/registrations/${registration.id}`,draft(0,{honesty:100}));
  await call('admin','locked rubric rejected','PUT',`${base}/rubric`,{...rubric,revision:1},{status:409});
  await call('admin','incomplete publication rejected','POST',`${base}/publish`,batch([registration.id,1]),{status:422});
  const privateResult=await call('web','draft invisible to owner','GET',publicPath,undefined,{token:participantToken});assert.equal(privateResult.scoring_result,null);assert.ok(!('scoring_data' in privateResult));
  const general=await call('admin','general registration hides scoring','GET',`/activity-registrations/${registration.id}`);assert.ok(!('scoring_data' in general));
  await call('admin','general update cannot overwrite scores','PUT','/activity-registrations',{registrations_id:[registration.id],status:'TERDAFTAR',scoring_data:{published:{result:{total:999}}}});
  await call('admin','complete draft','PUT',`${base}/registrations/${registration.id}`,draft(1,{honesty:100,trust:35},'Catatan terbit'));
  await call('admin','save guest incomplete','PUT',`${base}/registrations/${guest.id}`,draft(0,{honesty:0}));
  await call('admin','bulk incomplete is atomic','POST',`${base}/publish`,batch([registration.id,2],[guest.id,1]),{status:422});
  assert.equal((await call('web','failed batch remains private','GET',publicPath,undefined,{token:participantToken})).scoring_result,null);
  await call('admin','publish owner','POST',`${base}/publish`,batch([registration.id,2]));
  const published=(await call('web','owner receives normalized published snapshot','GET',publicPath,undefined,{token:participantToken})).scoring_result;
  assert.equal(published.result.total,77.5);assert.equal(published.result.grade,'B');assert.equal(published.note,'Catatan terbit');assert.ok(!('published_by' in published));
  await call('web','other user cannot read owner result','GET',publicPath,undefined,{token:otherLogin.token.token,status:404});
  const version=(await fixture.db.query('SELECT snapshot FROM activity_scoring_publications WHERE registration_id=$1 ORDER BY id',[registration.id])).rows[0].snapshot;
  await call('admin','draft correction','PUT',`${base}/registrations/${registration.id}`,draft(3,{honesty:100,trust:50},'Nouvelle note privée'));
  assert.equal((await call('web','correction stays private','GET',publicPath,undefined,{token:participantToken})).scoring_result.result.total,77.5);
  const beforeConcurrent=4;
  const concurrent=await Promise.all([1,2].map(async n=>{
    const response=await fetch(`http://127.0.0.1:3334/v2${base}/registrations/${registration.id}`,{method:'PUT',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(draft(beforeConcurrent,{honesty:100,trust:50},`Correction ${n}`))});return response.status;
  }));assert.deepEqual(concurrent.sort(),[200,409]);record.checks.push({label:'concurrent edits: exactly one save succeeds'});
  await call('admin','republish correction','POST',`${base}/publish`,batch([registration.id,5]));
  assert.equal((await call('web','republished total','GET',publicPath,undefined,{token:participantToken})).scoring_result.result.total,100);
  assert.deepEqual((await fixture.db.query('SELECT snapshot FROM activity_scoring_publications WHERE registration_id=$1 ORDER BY id',[registration.id])).rows[0].snapshot,version);
  await call('admin','withdraw result','POST',`${base}/withdraw`,batch([registration.id,6]));
  assert.equal((await call('web','withdrawn result hidden','GET',publicPath,undefined,{token:participantToken})).scoring_result,null);
  await call('admin','preserve activity history','DELETE',`/activities/${activity.id}`,{confirmation:activity.name},{status:409});
  await call('admin','preserve registration history','DELETE',`/activity-registrations/${registration.id}`,undefined,{status:409});
  const book=await workbook(activity.id);const sheet=book.getWorksheet('Scores');sheet.getCell('E3').value=86.4;sheet.getCell('F3').value=40;
  const buffer=await book.xlsx.writeBuffer();
  const preview=await call('admin','Excel preview','POST',`${base}/import/preview`,upload(buffer));assert.equal(preview.errors.length,0);assert.equal(preview.changes.length,1);
  await call('admin','Excel commit','POST',`${base}/import/commit`,upload(buffer,preview.hash));
  await call('admin','Excel replay rejected','POST',`${base}/import/commit`,upload(buffer,preview.hash),{status:409});
  assert.equal((await call('web','import never publishes','GET',publicPath,undefined,{token:participantToken})).scoring_result,null);
  const invalidBook=await workbook(activity.id);invalidBook.getWorksheet('Scores').getCell('E3').value={formula:'1+1',result:2};
  const invalidBuffer=await invalidBook.xlsx.writeBuffer();
  const invalidPreview=await call('admin','formula preview errors','POST',`${base}/import/preview`,upload(invalidBuffer));assert.ok(invalidPreview.errors.length>0);
  await call('admin','formula commit rejected','POST',`${base}/import/commit`,upload(invalidBuffer,invalidPreview.hash),{status:422});
  const wrong=await workbook(activity.id);wrong.getWorksheet('Metadata').getCell('B2').value=other.id;
  await call('admin','wrong-activity template denied','POST',`${base}/import/preview`,upload(await wrong.xlsx.writeBuffer()),{status:409});
  assert.equal((await call('admin','filtered list','GET',`${base}?search=Peserta%20Tamu&state=incomplete`)).total,1);
  assert.equal((await call('admin','pagination preserves count','GET',`${base}?page=999`)).total,2);
  await workbook(activity.id,'draft');await workbook(activity.id,'published');record.checks.push({label:'draft and published Excel exports open successfully'});
  await call('admin','publish imported result','POST',`${base}/publish`,batch([registration.id,8]));
  const raw=(await fixture.db.query('SELECT questionnaire_answer,status FROM activity_registrations WHERE id=$1',[registration.id])).rows[0];assert.deepEqual(raw.questionnaire_answer,{reason:'keep'});assert.equal(raw.status,'TERDAFTAR');
  if(process.argv.includes('--browser')) {
    const {runScoringBrowser}=await import('./scoring-browser.mjs');
    await runScoringBrowser({activity,member,participantToken,call,record,restorations,fixture});
  }
  assert.equal(sourceEvidence().sha256,record.source.sha256,'Sources changed during scoring verification; rerun against stable sources');
  record.status='passed';console.log(`Scoring: ${record.checks.length} checks passed.`);
} catch(error) {record.status='failed';record.failure=error.message;throw error;}
finally {
  const failures=[];
  for(const cleanup of [()=>web?.stop(),()=>api?.stop(),async()=>{
    // The fixture lease and ownership check above restrict cleanup to this test schema.
    await fixture.db.query('DELETE FROM activity_scoring_publications');
    await fixture.db.query('DELETE FROM activities');
    await fixture.db.query('DELETE FROM public_users');
    assert.equal((await fixture.db.query('SELECT count(*)::int AS n FROM activity_scoring_publications')).rows[0].n,0);
    record.scoring_cleanup='complete';
  },()=>fixture.db.end(),...restorations.reverse()]) {try {await cleanup();}catch(error){failures.push(error);}}
  record.cleanup=failures.length?'failed':'complete';save();release();
  if(failures.length)throw new AggregateError(failures,'Scoring cleanup failed');
}
