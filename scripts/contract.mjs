import { spawnSync,execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { isDeepStrictEqual } from 'node:util';
import assert from 'node:assert/strict';
import { root, legacy, testEnvironment } from './env.mjs';
import { fixtureDatabase, resetFixture, snapshotFixture, legacyRequire, fixturePassword, fixtureKey } from './fixture-db.mjs';
import { borrowWorkspacePort, startServer } from './server-process.mjs';
import { referenceCases, authorizationCases, authCases } from './contract-cases.mjs';
import { ContractNormalizer } from './contract-normalize.mjs';
import { adminCases } from './contract-admin.mjs';
import { memberCases } from './contract-members.mjs';
import { activityCases } from './contract-activities.mjs';
import { registrationCases } from './contract-registrations.mjs';
import { clubCases } from './contract-clubs.mjs';
import { clubMemberCases } from './contract-club-members.mjs';
import { achievementCases } from './contract-achievements.mjs';
import { templateCases } from './contract-templates.mjs';
import { certificateCases } from './contract-certificates.mjs';
import { googleCases,googleFixture } from './contract-google.mjs';
import { imageCases } from './contract-images.mjs';
import { routeEdgeCases } from './contract-route-edges.mjs';
import {protocolCases} from './contract-protocol.mjs';
import { queryEdgeCases } from './contract-query-edges.mjs';
import { cleanStorageJournal } from './storage-cleanup.mjs';
import {contractGroups} from './contract-groups.mjs';
import {sourceEvidence} from './source-evidence.mjs';
import { acquireFixtureLease } from './fixture-lease.mjs';

acquireFixtureLease();

const group=process.argv.find(arg=>arg.startsWith('--group='))?.split('=')[1]??'reference';
if(!contractGroups.includes(group))throw new Error('Unknown contract group');
const routes=JSON.parse(readFileSync(resolve(root,'internal/httpapi/routes.json'),'utf8'));
const jwt=legacyRequire('jsonwebtoken');
const {Scrypt}=legacyRequire('@adonisjs/hash/drivers/scrypt');
const passwordHash=await new Scrypt({}).make(fixturePassword);
const artifacts=resolve(root,'.artifacts/contracts');mkdirSync(artifacts,{recursive:true});
const adonisRevision=execFileSync('git',['rev-parse','HEAD'],{cwd:legacy,encoding:'utf8'}).trim();
assert.equal(adonisRevision,JSON.parse(readFileSync(resolve(root,'docs/BASELINE.json'),'utf8')).revision,'Adonis baseline changed; refresh the inventory and compatibility scenarios');
const source=sourceEvidence();
const build=spawnSync('go',['build',...(['google','images','query-edges','protocol'].includes(group)?['-tags=integration']:[]),'-o',resolve(root,'.artifacts/admin-api'),'./cmd/api'],{cwd:root,stdio:'inherit'});
if(build.status!==0)throw new Error('Go build failed');
assert.equal(sourceEvidence().sha256,source.sha256,'Go sources changed during compilation; rerun the suite');
const restore=await borrowWorkspacePort(3334,process.argv.includes('--borrow-workspace'));
const results={};
const keys=group==='google'?await googleFixture():null;
const started=Date.now();
try {
  for(const [kind,suffix] of [['adonis','baseline'],['go','candidate']]) {
    const {db,schema,assertHealthy}=await fixtureDatabase(suffix);
    let server;
    let journal;
    try {
      const tables=await resetFixture(db,schema,passwordHash);
      if(['activities','clubs','templates','images','route-edges','query-edges'].includes(group)) {journal=resolve(root,`.artifacts/storage-contract-${group}-${kind}.json`);writeFileSync(journal,'[]',{flag:'wx',mode:0o600});}
      server=await startServer(kind,schema,`${kind}-${group}`,{journal,googleKeys:keys?.url,productionContract:['images','query-edges','protocol'].includes(group)});
      const responses=[];
      const normalizer=new ContractNormalizer(started);
      const verifiedHashes=new Set([passwordHash]);
      const tokens={super:jwt.sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'15m'}),requester:jwt.sign({userId:2,email:'requester@example.test'},fixtureKey,{expiresIn:'15m'})};
      const h={async seed(sql,values=[]){return (await db.query(sql,values)).rows;},async upload(name,path,fields={},contents){
        const sharp=legacyRequire('sharp');
        const png=contents??await sharp({create:{width:640,height:480,channels:3,background:'#6496c8'}}).png().toBuffer();
        const form=new FormData();for(const [key,value] of Object.entries(fields))form.append(key,String(value));form.append('file',new Blob([png],{type:'image/png'}),'fixture.png');
        return h.call(name,'POST',path,undefined,'super',{form});
      },async inspectObject(key,expected){
        assert.ok(JSON.parse(readFileSync(journal,'utf8')).includes(key),'storage writes must be journaled by the actual SDK adapter');
        const env=testEnvironment(),{S3Client,GetObjectCommand,GetObjectAclCommand}=legacyRequire('@aws-sdk/client-s3');
        const client=new S3Client({endpoint:env.DRIVE_ENDPOINT,region:env.DRIVE_REGION,forcePathStyle:env.DRIVE_DISK==='minio',credentials:{accessKeyId:env.DRIVE_ACCESS_KEY_ID,secretAccessKey:env.DRIVE_SECRET_ACCESS_KEY}});
        try {
          const result=await client.send(new GetObjectCommand({Bucket:env.DRIVE_BUCKET,Key:key}));
          const bytes=Buffer.from(await result.Body.transformToByteArray()),metadata=await legacyRequire('sharp')(bytes).metadata();
          assert.equal(metadata.width,expected.width);assert.equal(metadata.height,expected.height);assert.equal(metadata.format,'webp');assert.equal(metadata.orientation,undefined);
          let publicRead=null;if(env.DRIVE_DISK==='minio'){
            const acl=await client.send(new GetObjectAclCommand({Bucket:env.DRIVE_BUCKET,Key:key}));
            publicRead=acl.Grants.some(grant=>grant.Grantee?.URI==='http://acs.amazonaws.com/groups/global/AllUsers'&&grant.Permission==='READ');
          }
          responses.push(normalizer.normalize({name:'stored-image:'+key,storage:{key,contentType:result.ContentType,cacheControl:result.CacheControl,width:metadata.width,height:metadata.height,format:metadata.format,publicRead}}));
        }finally{client.destroy();}
      },async call(name,method,path,body,actor='super',options={}){
        const headers={Accept:'application/json','Content-Type':'application/json','X-Request-ID':'synthetic-contract-request'};
        if(actor!=='none')headers.Authorization=`Bearer ${tokens[actor]}`;
        if(options.cookie)headers.Cookie=options.cookie;
        Object.assign(headers,options.headers);
        if(options.form)delete headers['Content-Type'];
        const response=await fetch(`http://127.0.0.1:3334${path}`,{method,headers,body:options.rawBody??options.form??(body===undefined?undefined:JSON.stringify(body)),redirect:'manual',signal:AbortSignal.timeout(30000)});
        const buffer=Buffer.from(await response.arrayBuffer());let data;
        const workbook=response.headers.get('content-type')?.includes('spreadsheetml');
        if(workbook){
          const book=new (legacyRequire('exceljs').Workbook)();await book.xlsx.load(buffer);
          data={sheets:book.worksheets.map(sheet=>({name:sheet.name,rows:Array.from({length:sheet.rowCount},(_,row)=>Array.from({length:sheet.columnCount},(_,column)=>sheet.getCell(row+1,column+1).value??null))}))};
        }else{const text=buffer.toString('utf8');try{data=JSON.parse(text)}catch{data=text}}
        const result={name,method,path,status:response.status,body:data};
        if(workbook){result.contentType=response.headers.get('content-type');result.attachment=response.headers.get('content-disposition');}
        if(['auth','google','protocol'].includes(group)) {
          result.contentType=response.headers.get('content-type');
          result.cookies=response.headers.getSetCookie().map(cookie=>normalizer.cookie(cookie));
          h.lastCookie=response.headers.getSetCookie()[0]?.split(';')[0];
          if(data?.data?.access_token)assert.equal(data.data.access_token,data.data.token.token,'legacy token alias');
        }
        if(!['GET','HEAD','OPTIONS'].includes(method)){
          result.database=await snapshotFixture(db,tables);
          for(const table of ['admin_users','public_users'])for(const user of result.database[table])if(user.password&&!verifiedHashes.has(user.password)){
            assert.ok(await new Scrypt({}).verify(user.password,fixturePassword),'created password must verify in Adonis');verifiedHashes.add(user.password);
          }
          for(const record of result.database.admin_refresh_tokens){
            const lifetime=Date.parse(record.expires_at)-Date.parse(record.created_at);
            assert.ok(Math.abs(lifetime-30*86400000)<2000,'refresh lifetime');
            if(record.parent_token_id){const parent=result.database.admin_refresh_tokens.find(row=>row.id===record.parent_token_id);assert.equal(parent.family_id,record.family_id);assert.equal(parent.replaced_by_token_id,record.id);}
          }
        }
        responses.push(normalizer.normalize(result));
        return data;
      }};
      if(group==='protocol')await protocolCases(h,fixturePassword);else if(group==='query-edges')await queryEdgeCases(h);else if(group==='route-edges')await routeEdgeCases(h,routes);else if(group==='images')await imageCases(h);else if(group==='reference')await referenceCases(h);else if(group==='authorization')await authorizationCases(h,routes);else if(group==='admin')await adminCases(h,fixturePassword);else if(group==='members')await memberCases(h,fixturePassword);else if(group==='activities')await activityCases(h);else if(group==='registrations')await registrationCases(h);else if(group==='clubs')await clubCases(h);else if(group==='club-members')await clubMemberCases(h);else if(group==='achievements')await achievementCases(h);else if(group==='templates')await templateCases(h);else if(group==='certificates')await certificateCases(h);else if(group==='google')await googleCases(h,keys);else await authCases(h,fixturePassword);
      await db.query('SELECT 1');assertHealthy();
      results[kind]=responses;
      writeFileSync(resolve(artifacts,`${group}-${kind}.json`),JSON.stringify(responses,null,2),{mode:0o600});
      console.log(`${kind}: ${responses.length} ${group} scenarios executed`);
     } finally {
      const cleanupErrors=[];
      for(const clean of [async()=>{if(server)await server.stop()},async()=>{await db.end()},async()=>{if(journal)console.log(`${kind} storage cleanup:`,await cleanStorageJournal(journal))}]){
        try{await clean()}catch(error){cleanupErrors.push(error)}
      }
      if(cleanupErrors.length)throw new AggregateError(cleanupErrors,'Contract teardown failed');
    }
  }
} finally { await restore(); if(keys){await keys.close();assert.ok(keys.counts.pem>0&&keys.counts.jwks>0,'both validators must consume real controlled key responses');} }

const differences=[];
for(let i=0;i<results.adonis.length;i++) {
  const baseline=results.adonis[i],candidate=results.go[i];
  if(!isDeepStrictEqual(baseline,candidate))differences.push({name:baseline.name,baseline,candidate});
}
const report={group,source,adonis_revision:adonisRevision,environment:['images','query-edges','protocol'].includes(group)?'production':'test',scenarios:results.adonis.length,passed:results.adonis.length-differences.length,failed:differences.length,differences};
writeFileSync(resolve(artifacts,`${group}-report.json`),JSON.stringify(report,null,2),{mode:0o600});
console.log(`${group}: ${report.passed}/${report.scenarios} equivalent; ${report.failed} differences`);
if(differences.length){console.log(differences.map(row=>row.name).join('\n'));process.exitCode=1;}
