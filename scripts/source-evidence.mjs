import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {readdirSync,readFileSync,mkdirSync,writeFileSync,existsSync} from 'node:fs';
import {resolve,relative} from 'node:path';
import {root,workspace,legacy} from './env.mjs';

function files(directory){return readdirSync(directory,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?(entry.name==='node_modules'?[]:files(resolve(directory,entry.name))):/\.(go|json|sql|mjs|cjs)$/.test(entry.name)?[resolve(directory,entry.name)]:[]);}
export function sourceEvidence(){
  // Changes to fixtures, comparison rules or query generation invalidate old
  // evidence just as application changes do. Never include environment files.
  const inputs=[...['cmd','internal','database','scripts','tests'].flatMap(dir=>files(resolve(root,dir))),...['go.mod','go.sum','package.json','package-lock.json','sqlc.yaml','playwright.config.mjs','Makefile','docs/BASELINE.json'].map(path=>resolve(root,path))].sort();
  const applications={};
  for(const name of ['admin-be','web-be','admin-fe','web-fe','adonis-reference']){
    const directory=name==='adonis-reference'?legacy:resolve(workspace,'kaderisasi-'+name);
    const paths=execFileSync('git',['ls-files','--cached','--others','--exclude-standard','-z'],{cwd:directory,encoding:'utf8'}).split('\0');
    // Include real source/configuration and tests, excluding environment files,
    // generated Next declarations, and build outputs changed by normal checks.
    for(const path of new Set(paths)){
      if(!/\.(ts|tsx|js|mjs|cjs|mts|json|sql|css)$/.test(path)||/(^|\/)(\.env[^/]*|next-env\.d\.ts)$/.test(path))continue;
      const absolute=resolve(directory,path);
      if(existsSync(absolute))inputs.push(absolute);
    }
    applications[name]=execFileSync('git',['rev-parse','HEAD'],{cwd:directory,encoding:'utf8'}).trim();
  }
  inputs.sort();
  const manifest=inputs.map(path=>({path:relative(root,path),sha256:createHash('sha256').update(readFileSync(path)).digest('hex')}));
  const digest=createHash('sha256').update(JSON.stringify(manifest)).digest('hex');
  let revision=null,dirty=true;
  try{revision=execFileSync('git',['rev-parse','HEAD'],{cwd:root,encoding:'utf8',stdio:['ignore','pipe','ignore']}).trim();dirty=execFileSync('git',['status','--porcelain'],{cwd:root,encoding:'utf8'}).trim()!=='';}catch{}
  const evidence={version:3,sha256:digest,revision,dirty,applications,files:manifest};
  const directory=resolve(root,'.artifacts/source');mkdirSync(directory,{recursive:true});
  writeFileSync(resolve(directory,digest+'.json'),JSON.stringify(evidence,null,2)+'\n');
  return {version:3,sha256:digest,revision,dirty,applications,manifest:relative(root,resolve(directory,digest+'.json'))};
}
