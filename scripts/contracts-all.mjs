import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {root} from './env.mjs';
import {contractGroups as available} from './contract-groups.mjs';
const requested=process.argv.find(arg=>arg.startsWith('--groups='));
const groups=requested?requested.slice('--groups='.length).split(','):available;
if(groups.some(group=>!available.includes(group)))throw new Error('Unknown contract group');
for(const group of groups){
  const child=spawn('node',['scripts/contract.mjs',`--group=${group}`,...process.argv.slice(2).filter(arg=>!arg.startsWith('--groups='))],{cwd:root,stdio:'inherit'});
  const [code]=await once(child,'exit');if(code!==0){process.exitCode=code??1;break;}
}
