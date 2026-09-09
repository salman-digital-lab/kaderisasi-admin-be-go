import {existsSync,readFileSync} from 'node:fs';
import {spawnSync} from 'node:child_process';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {fixtureDatabase} from './fixture-db.mjs';
const manifest=resolve(root,'.artifacts/schemas.json');
if(!existsSync(manifest)||JSON.parse(readFileSync(manifest,'utf8')).status==='cleaned'){
  process.exitCode=spawnSync('node',['scripts/schema.mjs','create'],{cwd:root,stdio:'inherit'}).status??1;
}else{
  for(const name of ['baseline','candidate','cross']){const f=await fixtureDatabase(name);await f.db.end();}
  console.log('Reusing the three recorded, ownership-verified fixture schemas');
}
