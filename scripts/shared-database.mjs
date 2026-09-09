import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {fixtureDatabase,resetFixture,legacyRequire,fixtureKey,fixturePassword} from './fixture-db.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';
import {borrowWorkspacePort,startServer,startWebBackend} from './server-process.mjs';

acquireFixtureLease();
const fixture=await fixtureDatabase('cross');
const checks=[],restorations=[];
let go,web;
const adminToken=legacyRequire('jsonwebtoken').sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'15m'});
async function call(backend,label,method,path,body,{token,status=200}={}){
  const headers={'Content-Type':'application/json',Accept:'application/json'};
  if(backend==='go')headers.Authorization=`Bearer ${adminToken}`;
  if(token)headers.Authorization=`Bearer ${token}`;
  const response=await fetch(`http://127.0.0.1:${backend==='go'?3334:3333}/v2${path}`,{method,headers,body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(15000)});
  const data=await response.json();assert.equal(response.status,status,`${label}: ${JSON.stringify(data)}`);
  checks.push({backend,label,method,path,status});return data.data;
}
try{
  const {Scrypt}=legacyRequire('@adonisjs/hash/drivers/scrypt');
  await resetFixture(fixture.db,fixture.schema,await new Scrypt({}).make(fixturePassword));
  assert.equal(spawnSync('go',['build','-o',resolve(root,'.artifacts/admin-api'),'./cmd/api'],{cwd:root,stdio:'inherit'}).status,0);
  for(const port of [3334,3333])restorations.push(await borrowWorkspacePort(port,process.argv.includes('--borrow-workspace')));
  go=await startServer('go',fixture.schema,'shared-go');web=await startWebBackend(fixture.schema);
  const member=await call('go','Create public credentials in Go','POST','/members',{name:'Shared member',email:'shared@example.test',gender:'F',password:fixturePassword},{status:201});
  let loggedIn=await call('web','Authenticate Go password in web-be','POST','/auth/login',{email:'shared@example.test',password:fixturePassword});
  const token=loggedIn.token.token;
  assert.ok(token,'web-be returns a public access token');
  let profile=await call('web','Read Go-created profile','GET','/profiles',undefined,{token});
  assert.equal(profile.profile.name,'Shared member');assert.equal(profile.userData.id,member.user.id);
  await call('go','Update shared profile in Go','PUT',`/profiles/${member.profile.id}`,{name:'Go updated member',extra_data:{preferred_name:'Shared',current_activity_focus:['academic']}});
  profile=await call('web','Read Go profile changes','GET','/profiles',undefined,{token});
  assert.equal(profile.profile.name,'Go updated member');assert.deepEqual(profile.profile.extra_data.current_activity_focus,['academic']);
  await call('web','Update profile through public backend','PUT','/profiles',{whatsapp:'628123456789',extra_data:{preferred_name:'Web updated'}},{token});
  profile=await call('go','Read public backend changes in Go','GET',`/profiles/${member.profile.id}`);
  assert.equal(profile.profile[0].whatsapp,'628123456789');assert.equal(profile.profile[0].extra_data.preferred_name,'Web updated');
  const activity=await call('go','Create shared activity','POST','/activities',{name:'Shared activity',is_published:1,activity_start:'2026-02-28',additional_config:{custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]}});
  await call('go','Register member in Go','POST',`/activities/${activity.id}/registrations`,{user_id:member.profile.id,questionnaire_answer:{}});
  let registrations=await call('web','Read Go registration through web-be','GET','/profiles/activities',undefined,{token});
  assert.equal(registrations[0].activity.name,'Shared activity');assert.equal(registrations[0].certificate_state,'not_eligible');
  await call('go','Graduate shared registration','PUT','/activity-registrations',{registrations_id:[registrations[0].id],status:'LULUS KEGIATAN'});
  const template=await call('go','Create shared certificate template','POST','/certificate-templates',{name:'Shared template',templateData:{backgroundUrl:null,canvasWidth:800,canvasHeight:566,elements:[{id:'name',type:'variable-text',variable:'{{name}}',x:0,y:100,width:800,height:80}]}},{status:201});
  await call('go','Publish shared template','POST',`/certificate-templates/${template.id}/publish`,{expectedVersion:1});
  await call('go','Assign shared template','PUT',`/activities/${activity.id}`,{certificate_template_id:template.id});
  const issued=await call('go','Persist certificate snapshots in Go','POST','/certificates/issue-single',{registration_id:registrations[0].id},{status:201});
  const code=issued.certificate.certificate_code;
  const downloaded=await call('web','Consume Go certificate snapshot','GET',`/certificates/code/${code}/download`,undefined,{token});
  assert.deepEqual(downloaded.template,issued.template);assert.deepEqual(downloaded.participant,issued.participant);assert.deepEqual(downloaded.activity,issued.activity);
  assert.equal(downloaded.certificate.certificate_code,code);
  const publicCertificate=await call('web','Public certificate redaction','GET',`/certificates/code/${code}`);
  assert.equal(publicCertificate.participant.name,'Go updated member');assert.equal(publicCertificate.participant.email,undefined);assert.equal(publicCertificate.participant.user_id,undefined);
  const verified=await call('web','Verify Go issuance publicly','GET',`/certificates/verify/${code}`);
  assert.equal(verified.valid,true);assert.equal(verified.activity_date,'28 Februari 2026');
  registrations=await call('web','Read issued state in public profile','GET','/profiles/activities',undefined,{token});
  assert.equal(registrations[0].certificate_code,code);assert.equal(registrations[0].certificate_state,'issued_active');
  await call('web','Create account in public backend','POST','/auth/register',{fullname:'Public-created member',email:'public-created@example.test',password:fixturePassword});
  const other=await call('web','Authenticate public-created account','POST','/auth/login',{email:'public-created@example.test',password:fixturePassword});
  await call('web','Enforce certificate ownership','GET',`/certificates/code/${code}/download`,undefined,{token:other.token.token,status:403});
  await call('go','Reset public-created password in Go','PUT',`/profiles/auth/${other.user.id}`,{password:'Go-updated-shared-password!'});
  await call('web','Authenticate reset shared password','POST','/auth/login',{email:'public-created@example.test',password:'Go-updated-shared-password!'});
  await call('go','Revoke shared issuance','POST',`/certificates/${issued.certificate.id}/revoke`,{reason:'Shared backend verification'});
  const revoked=await call('web','Public verification reflects Go revocation','GET',`/certificates/verify/${code}`);
  assert.equal(revoked.valid,false);assert.equal(revoked.state,'issued_revoked');
  await call('web','Reject download after Go revocation','GET',`/certificates/code/${code}/download`,undefined,{token,status:410});
  writeFileSync(resolve(root,'.artifacts/shared-database.json'),JSON.stringify({status:'passed',schema:fixture.schema,checks},null,2));
  console.log(`Shared database: ${checks.length} checks passed against real Go and web-be APIs`);
}finally{
  const failures=[];
  for(const cleanup of [()=>web?.stop(),()=>go?.stop(),()=>fixture.db.end(),...restorations.reverse()]){try{await cleanup();}catch(error){failures.push(error);}}
  if(failures.length)throw new AggregateError(failures,'Shared database cleanup or restoration failed');
}
