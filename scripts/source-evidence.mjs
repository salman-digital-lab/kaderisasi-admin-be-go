import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {readdirSync,readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {resolve,relative} from 'node:path';
import {root} from './env.mjs';

function files(directory){return readdirSync(directory,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?files(resolve(directory,entry.name)):/\.(go|json)$/.test(entry.name)?[resolve(directory,entry.name)]:[]);}
export function sourceEvidence(){
  const inputs=[...files(resolve(root,'cmd')),...files(resolve(root,'internal')),resolve(root,'go.mod'),resolve(root,'go.sum')].sort();
  const manifest=inputs.map(path=>({path:relative(root,path),sha256:createHash('sha256').update(readFileSync(path)).digest('hex')}));
  const digest=createHash('sha256').update(JSON.stringify(manifest)).digest('hex');
  let revision=null,dirty=true;
  try{revision=execFileSync('git',['rev-parse','HEAD'],{cwd:root,encoding:'utf8',stdio:['ignore','pipe','ignore']}).trim();dirty=execFileSync('git',['status','--porcelain'],{cwd:root,encoding:'utf8'}).trim()!=='';}catch{}
  const evidence={sha256:digest,revision,dirty,files:manifest};
  const directory=resolve(root,'.artifacts/source');mkdirSync(directory,{recursive:true});
  writeFileSync(resolve(directory,digest+'.json'),JSON.stringify(evidence,null,2)+'\n');
  return {sha256:digest,revision,dirty,manifest:relative(root,resolve(directory,digest+'.json'))};
}
