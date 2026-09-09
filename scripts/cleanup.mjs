import {readdirSync,existsSync,readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {spawnSync} from 'node:child_process';
import {root} from './env.mjs';
import {cleanStorageJournal} from './storage-cleanup.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';
const release=acquireFixtureLease();
const failures=[];
try{
  for(const file of readdirSync(resolve(root,'.artifacts')).filter(name=>/^storage-.*\.json$/.test(name))){
    try{console.log(file,await cleanStorageJournal(resolve(root,'.artifacts',file)));}catch(error){failures.push(error);}
  }
  const cors=resolve(root,'.artifacts/cors-lease.json');
  if(existsSync(cors)&&JSON.parse(readFileSync(cors,'utf8')).status!=='restored'){
    if(spawnSync('node',['scripts/storage-cors.mjs','restore'],{cwd:root,stdio:'inherit'}).status!==0)failures.push(new Error('Recorded temporary CORS restoration failed'));
  }
}finally{release();}
if(existsSync(resolve(root,'.artifacts/schemas.json'))&&spawnSync('node',['scripts/schema.mjs','drop'],{cwd:root,stdio:'inherit'}).status!==0)failures.push(new Error('Owned schema cleanup failed'));
if(failures.length)throw new AggregateError(failures,'Fixture cleanup incomplete');
