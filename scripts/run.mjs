import {spawn,spawnSync} from 'node:child_process';
import {readFileSync,mkdirSync} from 'node:fs';
import {once} from 'node:events';
import {resolve} from 'node:path';
import {parseEnv} from 'node:util';
import {root,workspace} from './env.mjs';

const environment=process.argv.find(arg=>arg.startsWith('--environment='))?.split('=')[1]??'test';
if(!['test','prod'].includes(environment))throw new Error('Environment must be test or prod');
const action=process.argv.slice(2).find(arg=>!arg.startsWith('--'))??'api';
if(!['api','close:registration','clubs:close-registration','clubs:update-visibility'].includes(action))throw new Error('Unknown API/job entrypoint');
const configured=parseEnv(readFileSync(resolve(workspace,`docs/.env.${environment}.be`),'utf8'));
const binary=resolve(root,'bin',action==='api'?'admin-api':'admin-jobs');
mkdirSync(resolve(root,'bin'),{recursive:true});
if(spawnSync('go',['build','-o',binary,action==='api'?'./cmd/api':'./cmd/jobs'],{cwd:root,stdio:'inherit'}).status!==0)process.exit(1);
const child=spawn(binary,action==='api'?[]:[action],{cwd:root,env:{...process.env,...configured,PORT:'3334'},stdio:'inherit'});
for(const signal of ['SIGTERM','SIGINT'])process.on(signal,()=>child.kill(signal));
const [code]=await once(child,'exit');process.exitCode=code??1;
