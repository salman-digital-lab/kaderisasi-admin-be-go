import assert from 'node:assert/strict';
import {existsSync,readFileSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root,testEnvironment} from './env.mjs';
import {legacyRequire} from './fixture-db.mjs';

const path=resolve(root,'.artifacts/cors-lease.json');
const expected=JSON.parse(readFileSync(resolve(root,'tests/storage-cors.json'),'utf8'));
const env=testEnvironment();
const {S3Client,GetBucketCorsCommand,PutBucketCorsCommand,DeleteBucketCorsCommand}=legacyRequire('@aws-sdk/client-s3');
const client=new S3Client({endpoint:env.DRIVE_ENDPOINT,region:env.DRIVE_REGION,forcePathStyle:env.DRIVE_DISK==='minio',credentials:{accessKeyId:env.DRIVE_ACCESS_KEY_ID,secretAccessKey:env.DRIVE_SECRET_ACCESS_KEY}});
async function current(){try{return (await client.send(new GetBucketCorsCommand({Bucket:env.DRIVE_BUCKET}))).CORSRules;}catch(error){if(error.name==='NoSuchCORSConfiguration')return null;throw error;}}
function canonical(rules){return JSON.stringify(rules?.map(rule=>Object.fromEntries(Object.entries(rule).sort(([a],[b])=>a.localeCompare(b)).map(([key,value])=>[key,Array.isArray(value)?[...value].sort():value]))));}
try{
  const action=process.argv[2];
  if(action==='apply'){
    if(!process.argv.includes('--approved-test-bucket-change'))throw new Error('Explicit approval for the shared test bucket setting is required');
    if(existsSync(path)&&JSON.parse(readFileSync(path,'utf8')).status!=='restored')throw new Error('Restore the previous recorded CORS change first');
    const previous=await current();assert.equal(previous,null,'The reviewed change assumes no existing CORS configuration');
    const record={status:'applying',endpoint:env.DRIVE_ENDPOINT,bucket:env.DRIVE_BUCKET,previous,applied:expected.CORSRules,started_at:new Date().toISOString()};
    writeFileSync(path,JSON.stringify(record,null,2),{mode:0o600});
    await client.send(new PutBucketCorsCommand({Bucket:env.DRIVE_BUCKET,CORSConfiguration:expected}));
    assert.equal(canonical(await current()),canonical(record.applied));
    record.status='applied';writeFileSync(path,JSON.stringify(record,null,2),{mode:0o600});
    console.log('Reviewed temporary test CORS applied and verified');
  }else if(action==='restore'){
    const record=JSON.parse(readFileSync(path,'utf8'));
    assert.equal(record.endpoint,env.DRIVE_ENDPOINT);assert.equal(record.bucket,env.DRIVE_BUCKET);
    const rules=await current();
    if(canonical(rules)!==canonical(record.previous)){
      assert.equal(canonical(rules),canonical(record.applied),'CORS changed externally; refusing to overwrite it');
      if(record.previous)await client.send(new PutBucketCorsCommand({Bucket:env.DRIVE_BUCKET,CORSConfiguration:{CORSRules:record.previous}}));
      else await client.send(new DeleteBucketCorsCommand({Bucket:env.DRIVE_BUCKET}));
    }
    assert.equal(canonical(await current()),canonical(record.previous));
    record.status='restored';record.restored_at=new Date().toISOString();writeFileSync(path,JSON.stringify(record,null,2),{mode:0o600});
    console.log('Original shared test bucket CORS restored and verified');
  }else throw new Error('Usage: storage-cors.mjs apply --approved-test-bucket-change | restore');
}finally{client.destroy();}
