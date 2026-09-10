import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {readFileSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {spawnSync} from 'node:child_process';
import {root,migrations,testEnvironment} from './env.mjs';
import {fixtureDatabase,resetFixture,legacyRequire,fixtureKey,fixturePassword} from './fixture-db.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';
import {borrowWorkspacePort,startServer,startWebBackend} from './server-process.mjs';
import {sourceEvidence} from './source-evidence.mjs';

const release=acquireFixtureLease();
const fixture=await fixtureDatabase('cross');
const env=testEnvironment();
const {S3Client,DeleteObjectCommand,HeadObjectCommand}=legacyRequire('@aws-sdk/client-s3');
const storage=new S3Client({endpoint:env.DRIVE_ENDPOINT,region:env.DRIVE_REGION,forcePathStyle:true,credentials:{accessKeyId:env.DRIVE_ACCESS_KEY_ID,secretAccessKey:env.DRIVE_SECRET_ACCESS_KEY}});
const run=randomBytes(8).toString('hex');
const bucket=env.DRIVE_BUCKET;
const ledger=resolve(root,`.artifacts/course-objects-${run}.json`);
const record={run,source:sourceEvidence(),schema:fixture.schema,bucket,status:'running',checks:[]};
const save=()=>writeFileSync(resolve(root,'.artifacts/courses.json'),JSON.stringify(record,null,2));
writeFileSync(ledger,'[]',{flag:'wx',mode:0o600});save();
const restorations=[];
let api,web;
let adminToken;
const tokenFor=(id,email,audience='kaderisasi-admin')=>legacyRequire('jsonwebtoken').sign({userId:id,email},fixtureKey,{expiresIn:audience==='kaderisasi-admin'?'15m':'1d',audience});
async function call(owner,label,method,path,body,options={}) {
  const headers={Accept:'application/json'};
  const token=options.token??(owner==='admin'?adminToken:undefined);
  if(token)headers.Authorization=`Bearer ${token}`;
  if(options.origin)headers.Origin=options.origin;
  if(options.cookie)headers.Cookie=options.cookie;
  if(!(body instanceof FormData))headers['Content-Type']='application/json';
  const response=await fetch(`http://127.0.0.1:${owner==='admin'?3334:3333}/v2${path}`,{method,headers,body:body===undefined?undefined:body instanceof FormData?body:JSON.stringify(body),signal:AbortSignal.timeout(120000)});
  const text=await response.text();
  assert.equal(response.status,options.status??200,`${label}: ${text.slice(0,500)}`);
  record.checks.push({label,owner,method,path,status:response.status});
  if(options.binary)return {text,headers:response.headers};
  const data=text && response.headers.get('content-type')?.includes('json')?JSON.parse(text):null;
  return data?.data;
}
function upload(contents,name='lesson.pdf') {
  const form=new FormData();form.append('file',new Blob([contents],{type:'application/pdf'}),name);return form;
}
try {
  const migrated=spawnSync('node',['ace','migration:run','--force'],{cwd:migrations,env:testEnvironment({NODE_ENV:'test',DB_SCHEMA:fixture.schema,PGOPTIONS:`-c search_path=${fixture.schema}`}),encoding:'utf8'});
  writeFileSync(resolve(root,'.artifacts/course-migrations.log'),migrated.stdout+migrated.stderr);
  assert.equal(migrated.status,0,'Course migration failed; see course-migrations.log');
  const hash=await new (legacyRequire('@adonisjs/hash/drivers/scrypt').Scrypt)({}).make(fixturePassword);
  await resetFixture(fixture.db,fixture.schema,hash);
  adminToken=tokenFor(1,'super@example.test');
  assert.equal(spawnSync('go',['build','-o',resolve(root,'.artifacts/admin-api'),'./cmd/api'],{cwd:root,stdio:'inherit'}).status,0);
  for(const port of [3334,3333])restorations.push(await borrowWorkspacePort(port,process.argv.includes('--borrow-workspace')));
  api=await startServer('go',fixture.schema,'courses-go',{courseJournal:ledger,origins:'http://localhost:3005,http://localhost:3000'});
  web=await startWebBackend(fixture.schema,'courses-web');

  const courseInput={title:'Kelas pengembangan diri',summary:'Materi uji LMS',description:'<p>Belajar bersama.</p>',minimum_level:3,status:'draft'};
  const course=await call('admin','create draft','POST','/courses',courseInput,{status:201});
  assert.equal(course.status,'draft');
  await call('admin','read draft details','GET',`/courses/${course.id}`);
  await call('admin','reject empty publication','PUT',`/courses/${course.id}`,{...courseInput,status:'published'},{status:422});
  await call('admin','reject invalid level','POST','/courses',{...courseInput,minimum_level:4},{status:422});
  await call('admin','reject unsafe YouTube hostname','POST',`/courses/${course.id}/lessons`,{title:'Unsafe',youtube_url:'https://youtube.com.evil.test/watch?v=dQw4w9WgXcQ'},{status:422});
  const lesson=await call('admin','create lesson','POST',`/courses/${course.id}/lessons`,{title:'Materi pertama',description:'<p>Catatan</p><script>alert(1)</script><a href="javascript:alert(1)">unsafe</a>',youtube_url:'https://youtu.be/dQw4w9WgXcQ'});
  assert.equal(lesson.youtube_video_id,'dQw4w9WgXcQ');
  const second=await call('admin','create draft lesson without video','POST',`/courses/${course.id}/lessons`,{title:'Materi kedua',youtube_url:''});
  await call('admin','reject publication with missing video','PUT',`/courses/${course.id}`,{...courseInput,status:'published'},{status:422});
  await call('admin','update lesson video','PUT',`/courses/${course.id}/lessons/${second.id}`,{title:'Materi kedua',youtube_url:'https://www.youtube.com/watch?v=dQw4w9WgXcQ'});
  await call('admin','publish course','PUT',`/courses/${course.id}`,{...courseInput,status:'published'});
  await call('admin','reject foreign origin','PUT',`/courses/${course.id}`,courseInput,{origin:'https://untrusted.example',status:403});
  const noRole=tokenFor(2,'requester@example.test');
  await call('admin','deny administrator without course permission','GET','/courses',undefined,{token:noRole,status:403});
  const adminRoutes=JSON.parse(readFileSync(resolve(root,'internal/httpapi/routes.json'),'utf8')).filter(route=>route.controller==='courses_controller');
  for(const route of adminRoutes) {
    const path=route.path.replace(/^\/v2/,'').replace(':id',String(course.id)).replace(':lessonId',String(lesson.id)).replace(':documentId','1');
    for(const [token,status,label] of [['',401,'authentication'],[noRole,403,'permission']]) {
      await call('admin',`native ${label}: ${route.action}`,route.method,path,route.method==='GET'?undefined:{},{token,status});
    }
    if(route.path.includes('/:id')) {
      await call('admin',`missing-resource: ${route.action}`,route.method,path.replace(`/courses/${course.id}`, '/courses/2147483647'),route.method==='GET'?undefined:route.action==='uploadDocument'?upload('%PDF-1.4\n%%EOF'):route.action.includes('Lesson')?{title:'Missing lesson',youtube_url:'dQw4w9WgXcQ'}:route.action==='reorder'?{lesson_ids:[]}:{...courseInput},{status:404});
      await call('admin',`invalid-identifier: ${route.action}`,route.method,path.replace(`/courses/${course.id}`, '/courses/invalid'),route.method==='GET'?undefined:{},{status:404});
    }
  }
  await call('admin','query:invalid-pagination','GET','/courses?page=-1&per_page=invalid');
  await fixture.db.query("UPDATE admin_users SET role_code='course_manager' WHERE id=2");
  await call('admin','Course Manager can read catalog','GET','/courses',undefined,{token:noRole});
  await call('admin','Course Manager can manage course','PUT',`/courses/${course.id}`,{...courseInput,status:'published'},{token:noRole});
  const members=[];
  for(const level of [0,3,6,10]) {
    const email=`learner-${level}@example.test`;
    const member=await call('admin',`create learner ${level}`,'POST','/members',{name:`Learner ${level}`,email,gender:'F',password:fixturePassword},{status:201});
    await fixture.db.query('UPDATE profiles SET level=$1 WHERE user_id=$2',[level,member.user.id]);
    const login=await call('web',`login learner ${level}`,'POST','/auth/login',{email,password:fixturePassword});
    members.push({id:member.user.id,level,email,token:login.token.token});
  }
  const [low,eligible,high,highest]=members;
  for(const member of members) {
    await call('admin',`learner ${member.level} cannot use admin API`,'GET','/courses',undefined,{token:member.token,status:401});
  }
  await call('web','admin token cannot read learner catalog','GET','/courses',undefined,{token:adminToken,status:401});
  await call('web','admin token cannot complete another learner lesson','PUT',`/courses/${course.id}/lessons/${lesson.id}/completion`,{completed:true},{token:adminToken,status:401});
  await call('web','cookie and bearer identities must match','GET','/courses',undefined,{token:eligible.token,cookie:`token=${low.token}`,status:401});
  const legacyLearner=legacyRequire('jsonwebtoken').sign({userId:low.id,email:low.email},fixtureKey,{expiresIn:'1d'});
  for(const [method,path] of [['GET','/auth/me'],['POST','/auth/session/migrate'],['GET','/courses']]) {
    await call('admin','legacy learner token cannot become admin session',method,path,method==='GET'?undefined:{},{token:legacyLearner,status:401});
  }
  await call('web','legacy learner course session requires fresh login','GET','/courses',undefined,{token:legacyLearner,status:401});
  await call('web','guest catalog denied','GET','/courses',undefined,{status:401});
  for(const minimum_level of [0,3,6,10]) {
    await call('admin',`set minimum level ${minimum_level}`,'PUT',`/courses/${course.id}`,{...courseInput,minimum_level,status:'published'});
    for(const member of members) {
      await call('web',`minimum ${minimum_level}: learner ${member.level}`,'GET',`/courses/${course.id}`,undefined,{token:member.token,status:member.level>=minimum_level?200:404});
    }
    if(minimum_level>0) {
      await fixture.db.query('UPDATE profiles SET level=$1 WHERE user_id=$2',[minimum_level-1,eligible.id]);
      await call('web',`immediately below level ${minimum_level}`,'GET',`/courses/${course.id}`,undefined,{token:eligible.token,status:404});
      await fixture.db.query('UPDATE profiles SET level=3 WHERE user_id=$1',[eligible.id]);
    }
  }
  await call('admin','restore minimum level','PUT',`/courses/${course.id}`,{...courseInput,status:'published'});
  for(const member of members) {
    const catalog=await call('web',`level ${member.level} catalog`,'GET','/courses',undefined,{token:member.token});
    assert.equal(catalog.data.length,member.level>=3?1:0);
    await call('web',`level ${member.level} direct lesson`,'GET',`/courses/${course.id}/lessons/${lesson.id}`,undefined,{token:member.token,status:member.level>=3?200:404});
  }
  const {rows:[orphan]}=await fixture.db.query("INSERT INTO public_users(email,password,account_status,created_at,updated_at) VALUES('no-profile@example.test',$1,'active',now(),now()) RETURNING id",[hash]);
  const orphanToken=tokenFor(orphan.id,'no-profile@example.test','kaderisasi-public');
  assert.equal((await call('web','missing profile has empty catalog','GET','/courses',undefined,{token:orphanToken})).data.length,0);
  await call('web','missing profile detail denied','GET',`/courses/${course.id}`,undefined,{token:orphanToken,status:404});
  const content=await call('web','sanitize lesson description','GET',`/courses/${course.id}/lessons/${lesson.id}`,undefined,{token:eligible.token});
  assert.ok(!content.lesson.description.includes('<script')&&!content.lesson.description.includes('javascript:'));
  assert.equal((await fixture.db.query('SELECT count(*)::int AS n FROM course_lesson_progress')).rows[0].n,0,'reads must not record visits');
  await call('web','visit second lesson without prerequisite','POST',`/courses/${course.id}/lessons/${second.id}/visit`,{}, {token:eligible.token});
  let detail=await call('web','resume latest lesson','GET',`/courses/${course.id}`,undefined,{token:eligible.token});
  assert.equal(detail.resume_lesson_id,second.id);
  await call('web','reject string completion','PUT',`/courses/${course.id}/lessons/${second.id}/completion`,{completed:'false'},{token:eligible.token,status:422});
  const completePath=`/courses/${course.id}/lessons/${second.id}/completion`;
  await Promise.all([1,2,3].map((n)=>call('web',`idempotent completion ${n}`,'PUT',completePath,{completed:true,user_id:high.id},{token:eligible.token})));
  const progress=(await fixture.db.query('SELECT * FROM course_lesson_progress WHERE user_id=$1 AND lesson_id=$2',[eligible.id,second.id])).rows;
  assert.equal(progress.length,1);assert.ok(progress[0].completed_at);
  assert.equal((await fixture.db.query('SELECT count(*)::int AS n FROM course_lesson_progress WHERE user_id=$1',[high.id])).rows[0].n,0);
  await call('web','visit keeps completion','POST',`/courses/${course.id}/lessons/${second.id}/visit`,{},{token:eligible.token});
  assert.equal((await fixture.db.query('SELECT completed_at FROM course_lesson_progress WHERE user_id=$1',[eligible.id])).rows[0].completed_at.toISOString(),progress[0].completed_at.toISOString());
  await call('web','undo completion','PUT',completePath,{completed:false},{token:eligible.token});
  assert.equal((await call('web','undo reflected in course','GET',`/courses/${course.id}`,undefined,{token:eligible.token})).completed_lessons,0);
  for(const item of [lesson,second]) await call('web','complete current curriculum','PUT',`/courses/${course.id}/lessons/${item.id}/completion`,{completed:true},{token:eligible.token});
  const third=await call('admin','add published lesson','POST',`/courses/${course.id}/lessons`,{title:'Materi tambahan',youtube_url:'dQw4w9WgXcQ'});
  detail=await call('web','new lesson changes denominator','GET',`/courses/${course.id}`,undefined,{token:eligible.token});
  assert.equal(detail.completed_lessons,2);assert.equal(detail.total_lessons,3);
  await call('admin','reject duplicate order','PUT',`/courses/${course.id}/lesson-order`,{lesson_ids:[lesson.id,lesson.id,third.id]},{status:422});
  await call('admin','reorder lessons','PUT',`/courses/${course.id}/lesson-order`,{lesson_ids:[third.id,second.id,lesson.id]});
  detail=await call('web','reorder preserves completion','GET',`/courses/${course.id}`,undefined,{token:eligible.token});
  assert.deepEqual(detail.lessons.map((l)=>l.id),[third.id,second.id,lesson.id]);assert.equal(detail.completed_lessons,2);
  let learners=await call('admin','learner reporting matches curriculum','GET',`/courses/${course.id}/learners`);
  assert.equal(learners.meta.total,1);assert.equal(learners.data[0].completed_lessons,2);assert.equal(learners.data[0].total_lessons,3);
  await call('admin','remove completed lesson','DELETE',`/courses/${course.id}/lessons/${second.id}`);
  assert.equal((await call('web','removed lesson excluded from progress','GET',`/courses/${course.id}`,undefined,{token:eligible.token})).completed_lessons,1);
  await call('web','removed lesson inaccessible','GET',`/courses/${course.id}/lessons/${second.id}`,undefined,{token:eligible.token,status:404});
  assert.equal((await fixture.db.query('SELECT count(*)::int AS n FROM course_lesson_progress WHERE lesson_id=$1',[second.id])).rows[0].n,1);
  await fixture.db.query('UPDATE profiles SET level=0 WHERE user_id=$1',[eligible.id]);
  await call('web','downgrade denies content immediately','GET',`/courses/${course.id}`,undefined,{token:eligible.token,status:404});
  await call('web','downgrade denies progress mutation','PUT',`/courses/${course.id}/lessons/${lesson.id}/completion`,{completed:true},{token:eligible.token,status:404});
  await fixture.db.query('UPDATE profiles SET level=3 WHERE user_id=$1',[eligible.id]);

  const pdf='%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n';
  const document=await call('admin','upload private PDF','POST',`/courses/${course.id}/lessons/${lesson.id}/documents`,upload(pdf),{status:201});
  const key=(await fixture.db.query('SELECT storage_key FROM course_documents WHERE id=$1',[document.id])).rows[0].storage_key;
  assert.ok(JSON.parse(readFileSync(ledger,'utf8')).includes(key),'object recorded before upload');
  const anonymous=await fetch(`${env.DRIVE_ENDPOINT.replace(/\/$/,'')}/${bucket}/${key}`,{signal:AbortSignal.timeout(15000)});
  assert.ok([401,403,404].includes(anonymous.status),'private PDF accessible anonymously');
  record.checks.push({label:'S3 denies anonymous PDF access',status:anonymous.status});
  const downloadPath=`/courses/${course.id}/lessons/${lesson.id}/documents/${document.id}/download`;
  const downloaded=await call('web','eligible PDF download','GET',downloadPath,undefined,{token:eligible.token,binary:true});
  assert.equal(downloaded.text,pdf);assert.match(downloaded.headers.get('cache-control'),/no-store/);
  const adminDownloaded=await call('admin','manager PDF download','GET',downloadPath,undefined,{binary:true});
  assert.equal(adminDownloaded.text,pdf);assert.match(adminDownloaded.headers.get('cache-control'),/no-store/);
  await call('web','low-level PDF download denied','GET',downloadPath,undefined,{token:low.token,status:404});
  await call('web','guest PDF download denied','GET',downloadPath,undefined,{status:401});
  await call('web','PDF parent mismatch denied','GET',`/courses/${course.id}/lessons/${third.id}/documents/${document.id}/download`,undefined,{token:highest.token,status:404});
  await call('admin','reject disguised non-PDF','POST',`/courses/${course.id}/lessons/${lesson.id}/documents`,upload('not a pdf'),{status:422});
  const maxPDF=Buffer.alloc(20*1024*1024,32);maxPDF.write('%PDF-1.4\n');maxPDF.write('%%EOF',maxPDF.length-5);
  await call('admin','accept exactly 20 MiB PDF','POST',`/courses/${course.id}/lessons/${lesson.id}/documents`,upload(maxPDF,'maximum.pdf'),{status:201});
  await call('admin','reject oversized PDF','POST',`/courses/${course.id}/lessons/${lesson.id}/documents`,upload(Buffer.concat([maxPDF,Buffer.from(' ')]),'large.pdf'),{status:422});
  await call('admin','remove PDF','DELETE',`/courses/${course.id}/lessons/${lesson.id}/documents/${document.id}`);
  await call('web','removed PDF inaccessible','GET',downloadPath,undefined,{token:eligible.token,status:404});
  await call('admin','archive course','PUT',`/courses/${course.id}`,{...courseInput,status:'archived'});
  assert.equal((await call('web','archived course hidden','GET','/courses',undefined,{token:highest.token})).data.length,0);
  await call('web','archive denies visit','POST',`/courses/${course.id}/lessons/${lesson.id}/visit`,{},{token:eligible.token,status:404});
  await call('admin','republish restores progress','PUT',`/courses/${course.id}`,{...courseInput,status:'published'});
  assert.equal((await call('web','progress retained after archive','GET',`/courses/${course.id}`,undefined,{token:eligible.token})).completed_lessons,1);
  const paginated=await call('admin','course pagination and search','GET','/courses?search=pengembangan&per_page=1');
  assert.equal(paginated.meta.total,1);assert.equal(paginated.data.length,1);
  await call('admin','remove extra published lesson','DELETE',`/courses/${course.id}/lessons/${third.id}`);
  await call('admin','retain last published lesson','DELETE',`/courses/${course.id}/lessons/${lesson.id}`,undefined,{status:422});
  await call('admin','reject blank published video','PUT',`/courses/${course.id}/lessons/${lesson.id}`,{title:'Materi pertama',youtube_url:''},{status:422});
  await call('admin','unpublish course','PUT',`/courses/${course.id}`,courseInput);
  for(const [method,path,body] of [
    ['GET',`/courses/${course.id}`],
    ['GET',`/courses/${course.id}/lessons/${lesson.id}`],
    ['POST',`/courses/${course.id}/lessons/${lesson.id}/visit`,{}],
    ['PUT',`/courses/${course.id}/lessons/${lesson.id}/completion`,{completed:true}],
  ]) await call('web','draft denies learner endpoint',method,path,body,{token:eligible.token,status:404});
  await call('admin','publish after staging','PUT',`/courses/${course.id}`,{...courseInput,status:'published'});
  const documentCount=(await fixture.db.query('SELECT count(*)::int AS n FROM course_documents')).rows[0].n;
  await api.stop();
  api=await startServer('go',fixture.schema,'courses-storage-failure',{driveBucket:`${bucket}-missing`,courseJournal:ledger,origins:'http://localhost:3005,http://localhost:3000'});
  await call('admin','storage upload failure is reported','POST',`/courses/${course.id}/lessons/${lesson.id}/documents`,upload(pdf),{status:500});
  assert.equal((await fixture.db.query('SELECT count(*)::int AS n FROM course_documents')).rows[0].n,documentCount,'failed storage upload must not create document metadata');
  await api.stop();
  api=await startServer('go',fixture.schema,'courses-go',{courseJournal:ledger,origins:'http://localhost:3005,http://localhost:3000'});
  if(process.argv.includes('--browser')) {
    const {runCourseBrowser}=await import('./course-browser.mjs');
    await runCourseBrowser({course,lesson,members,pdf,call,record,restorations});
  }
  record.status='passed';save();
  console.log(`Courses: ${record.checks.length} checks passed against real Go, AdonisJS, PostgreSQL, and private S3 storage.`);
} catch(error) {
  record.status='failed';record.failure=error.message;throw error;
} finally {
  const failures=[];
  for(const cleanup of [()=>web?.stop(),()=>api?.stop(),async()=>{
    const keys=[...new Set(JSON.parse(readFileSync(ledger,'utf8')))];
    for(const key of keys) {
      assert.match(key,/^courses\/[a-f0-9-]{36}\.pdf$/i);
      await storage.send(new DeleteObjectCommand({Bucket:bucket,Key:key}));
      await assert.rejects(storage.send(new HeadObjectCommand({Bucket:bucket,Key:key})),(error)=>error.$metadata?.httpStatusCode===404);
    }
    record.storage_cleanup={status:'cleaned',keys:keys.length};
  },()=>fixture.db.end(),...restorations.reverse()]) { try { await cleanup(); } catch(error) { failures.push(error); } }
  storage.destroy();record.cleanup=failures.length?'failed':'complete';save();release();
  if(failures.length)throw new AggregateError(failures,'Course fixture cleanup failed');
}
