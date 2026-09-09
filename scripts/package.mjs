import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,copyFileSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';

const platform=process.platform,arch=process.arch;
if(!['darwin','linux'].includes(platform))throw new Error('Packaging supports macOS and Linux hosts');
const base=resolve(root,'.artifacts/release'),name=`admin-go-${platform}-${arch}`,directory=resolve(base,name);
mkdirSync(resolve(directory,'bin'),{recursive:true});
const go=spawnSync('go',['version'],{encoding:'utf8'}).stdout.trim();assert.match(go,/go1\.26\.8\b/);
const vips=spawnSync('pkg-config',['--modversion','vips'],{encoding:'utf8'});assert.equal(vips.status,0,'libvips development/runtime installation required');
const libraries={};
for(const [binary,source] of [['admin-api','./cmd/api'],['admin-jobs','./cmd/jobs']]){
  const path=resolve(directory,'bin',binary);
  assert.equal(spawnSync('go',['build','-trimpath','-o',path,source],{cwd:root,stdio:'inherit'}).status,0);
  const deps=platform==='darwin'?spawnSync('otool',['-L',path],{encoding:'utf8'}):spawnSync('ldd',[path],{encoding:'utf8'});
  assert.equal(deps.status,0);assert.ok(!deps.stdout.includes('not found'),'Native runtime dependency missing');libraries[binary]=deps.stdout;
  const smoke=spawnSync(path,binary==='admin-jobs'?['unknown-job']:[],{env:{PATH:process.env.PATH},encoding:'utf8'});
  assert.equal(smoke.status,1,'Native executable should reach its configuration/argument validation');
  assert.match(smoke.stderr,binary==='admin-jobs'?/unknown job/:/APP_KEY and database configuration are required/);
}
copyFileSync(resolve(root,'README.md'),resolve(directory,'README.md'));
writeFileSync(resolve(directory,'RUNTIME.txt'),`This package requires libvips ${vips.stdout.trim()} (the build host version), including JPEG/PNG/WebP codecs and its shared libraries.\nIt is dynamically linked and must run on the same OS/architecture with compatible libraries installed.\nmacOS: brew install vips pkgconf\nDebian 12: apt-get install libvips42 ca-certificates tzdata\nBuild and verify a package on the deployment OS; this macOS package is not a Linux binary.\n\n${Object.entries(libraries).map(([name,value])=>name+'\n'+value).join('\n')}\n`);
writeFileSync(resolve(directory,'manifest.json'),JSON.stringify({platform,arch,go,libvips:vips.stdout.trim(),native_libraries:libraries,native_process_smoke:'passed',created_at:new Date().toISOString()},null,2));
const archive=resolve(base,name+'.tar.gz');assert.equal(spawnSync('tar',['-czf',archive,'-C',base,name],{stdio:'inherit'}).status,0);
console.log(archive);
