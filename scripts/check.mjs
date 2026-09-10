import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {readdirSync,readFileSync,writeFileSync,mkdirSync,existsSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';

function files(directory,extension){return readdirSync(directory,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?files(resolve(directory,entry.name),extension):entry.name.endsWith(extension)?[resolve(directory,entry.name)]:[]);}
const goFiles=['cmd','internal'].flatMap(dir=>files(resolve(root,dir),'.go'));
const formatted=spawnSync('gofmt',['-l',...goFiles],{cwd:root,encoding:'utf8'});
assert.equal(formatted.status,0);assert.equal(formatted.stdout.trim(),'','gofmt changes required:\n'+formatted.stdout);
const commands=[['go',['vet','./...']],['go',['build','./...']],['go',['vet','-tags=integration','./...']],['go',['mod','verify']]];
for(const [command,args] of commands){assert.equal(spawnSync(command,args,{cwd:root,stdio:'inherit'}).status,0,`${command} ${args.join(' ')}`);}
const generated=files(resolve(root,'internal/dbgen'),'.go');
const before=new Map(generated.map(path=>[path,readFileSync(path,'utf8')]));
const goBin=spawnSync('go',['env','GOPATH'],{encoding:'utf8'}).stdout.trim();
const sqlc=existsSync(resolve(goBin,'bin/sqlc'))?resolve(goBin,'bin/sqlc'):'sqlc';
assert.equal(spawnSync(sqlc,['version'],{encoding:'utf8'}).stdout?.trim(),'v1.31.1','sqlc 1.31.1 is required');
assert.equal(spawnSync(sqlc,['generate'],{cwd:root,stdio:'inherit'}).status,0);
assert.deepEqual(files(resolve(root,'internal/dbgen'),'.go').sort(),[...before.keys()].sort(),'sqlc generated file set drift');
for(const [path,content] of before)assert.equal(readFileSync(path,'utf8'),content,'sqlc generated output drift: '+path);
for(const path of [...files(resolve(root,'scripts'),'.mjs'),...files(resolve(root,'tests'),'.mjs')])assert.equal(spawnSync('node',['--check',path],{cwd:root,stdio:'inherit'}).status,0);
assert.equal(spawnSync('node',['--test','scripts/contract-normalize.test.mjs'],{cwd:root,stdio:'inherit'}).status,0,'Contract normalization integrity');
mkdirSync(resolve(root,'.artifacts'),{recursive:true});writeFileSync(resolve(root,'.artifacts/check.json'),JSON.stringify({status:'passed',checks:['gofmt','go vet','production build','integration vet','go mod verify','sqlc generated output','JavaScript syntax','contract normalization integrity'],at:new Date().toISOString()},null,2));
console.log('Formatting, vet, builds, dependencies, generated queries, harness syntax, and normalization integrity pass');
