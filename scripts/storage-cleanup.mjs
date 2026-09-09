import {readFileSync,unlinkSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root,testEnvironment} from './env.mjs';
import {legacyRequire} from './fixture-db.mjs';

export async function cleanStorageJournal(path){
  if(!path.startsWith(resolve(root,'.artifacts')+'/storage-')||!path.endsWith('.json'))throw new Error('Refusing unknown storage journal');
  const keys=JSON.parse(readFileSync(path,'utf8'));
  if(!Array.isArray(keys)||keys.some(key=>typeof key!=='string'||!key.match(/[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}/i)||key.includes('..')))throw new Error('Refusing unowned storage keys');
  const env=testEnvironment(),{S3Client,DeleteObjectCommand,HeadObjectCommand}=legacyRequire('@aws-sdk/client-s3');
  const client=new S3Client({endpoint:env.DRIVE_ENDPOINT,region:env.DRIVE_REGION,forcePathStyle:env.DRIVE_DISK==='minio',credentials:{accessKeyId:env.DRIVE_ACCESS_KEY_ID,secretAccessKey:env.DRIVE_SECRET_ACCESS_KEY}});
  const unique=[...new Set(keys)],removed=[];
  try{
    for(const key of unique){
      await client.send(new DeleteObjectCommand({Bucket:env.DRIVE_BUCKET,Key:key}));
      try{await client.send(new HeadObjectCommand({Bucket:env.DRIVE_BUCKET,Key:key}));throw new Error('Deleted fixture object remains accessible');}catch(error){if(error.$metadata?.httpStatusCode!==404)throw error;}
      removed.push(key);
    }
    writeFileSync(path.replace('/storage-','/cleaned-storage-'),JSON.stringify({status:'cleaned',attempted:keys.length,removed},null,2),{mode:0o600});
    unlinkSync(path);
    return {attempted:keys.length,removed:removed.length};
  }finally{client.destroy();}
}
