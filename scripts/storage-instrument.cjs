// Loaded only into the comparison's Adonis child, before Adonis imports Drive.
const {createRequire}=require('node:module');
const {readFileSync,writeFileSync,renameSync}=require('node:fs');
const {resolve}=require('node:path');
const legacy=createRequire(resolve(require('./legacy-path.cjs'),'package.json'));
const {S3Client}=legacy('@aws-sdk/client-s3');
const journal=process.env.GO_REWRITE_STORAGE_LEDGER;
const fixtureEnvironment=process.env.NODE_ENV==='test'||(process.env.NODE_ENV==='production'&&process.env.GO_REWRITE_PRODUCTION_CONTRACT==='1');
if(!fixtureEnvironment||!/^go_rewrite_[a-f0-9]{16}_baseline$/.test(process.env.DB_SCHEMA??'')||!journal)throw new Error('Adonis storage instrumentation requires its isolated baseline schema');
const original=S3Client.prototype.send;
S3Client.prototype.send=function(command,...args){
  const kind=command.constructor.name;
  if(['PutObjectCommand','CopyObjectCommand','CreateMultipartUploadCommand'].includes(kind)){
    if(command.input.Bucket!==process.env.DRIVE_BUCKET)throw new Error('Unexpected storage bucket');
    const keys=JSON.parse(readFileSync(journal,'utf8'));keys.push(command.input.Key);
    writeFileSync(journal+'.pending',JSON.stringify(keys),{mode:0o600});renameSync(journal+'.pending',journal);
  }
  return original.call(this,command,...args);
};
