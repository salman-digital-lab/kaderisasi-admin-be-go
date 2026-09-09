import {openSync,closeSync,writeFileSync,readFileSync,unlinkSync,mkdirSync} from 'node:fs';
import {randomUUID} from 'node:crypto';
import {resolve} from 'node:path';
import {root} from './env.mjs';

// A single fixture owner prevents one command from clearing a schema while a
// different test or comparison is using it.
export function acquireFixtureLease() {
  mkdirSync(resolve(root,'.artifacts'),{recursive:true});
  const path=resolve(root,'.artifacts/fixture-lease.json'),nonce=randomUUID();
  let fd;
  try {fd=openSync(path,'wx',0o600);}catch(error){
    if(error.code!=='EEXIST')throw error;
    const owner=JSON.parse(readFileSync(path,'utf8'));
    if(owner.root!==root||!Number.isInteger(owner.pid)||owner.pid<1)throw new Error('Fixture lease has an unknown owner');
    try{process.kill(owner.pid,0);throw new Error(`Fixture resources are already in use by process ${owner.pid}`);}catch(probe){if(probe.code!=='ESRCH')throw probe;}
    unlinkSync(path);fd=openSync(path,'wx',0o600);
  }
  writeFileSync(fd,JSON.stringify({root,pid:process.pid,nonce,started:new Date().toISOString()}));closeSync(fd);
  let released=false;
  const release=()=>{
    if(released)return;
    const owner=JSON.parse(readFileSync(path,'utf8'));
    if(owner.nonce!==nonce)throw new Error('Fixture lease ownership changed');
    unlinkSync(path);released=true;
  };
  process.once('exit',release);
  return ()=>{release();process.removeListener('exit',release);};
}
