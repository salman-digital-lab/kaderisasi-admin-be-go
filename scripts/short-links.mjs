import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {createRequire} from 'node:module';
import {mkdirSync,writeFileSync,readFileSync,openSync,closeSync} from 'node:fs';
import {resolve} from 'node:path';
import {spawn} from 'node:child_process';
import {createServer} from 'node:net';
import {root,workspace,migrations,testEnvironment} from './env.mjs';
import {shortLinksBrowser} from './short-links-browser.mjs';
import {sourceEvidence} from './source-evidence.mjs';
const require=createRequire(resolve(migrations,'package.json'));
const {Client}=require('pg');
const schema=`go_rewrite_short_${randomBytes(8).toString('hex')}`;
const marker=`short-links ${schema}`;
const artifacts=resolve(root,'.artifacts/short-links');mkdirSync(artifacts,{recursive:true});
const env=testEnvironment({NODE_ENV:'test',DB_SCHEMA:schema,PGOPTIONS:`-c search_path=${schema}`,SHORT_URL_BASE_URL:'http://localhost:4000',GO_REWRITE_ARTIFACTS:artifacts});
const db=new Client({host:env.DB_HOST,port:Number(env.DB_PORT),user:env.DB_USER,password:env.DB_PASSWORD,database:env.DB_DATABASE,connectionTimeoutMillis:10000});
const report={schema,marker,status:'running',cleanup:'pending'};
function run(command,args,cwd,extra={}){return new Promise((resolve,reject)=>{const child=spawn(command,args,{cwd,env:{...env,...extra},stdio:'inherit'});child.on('error',reject);child.on('exit',code=>code===0?resolve():reject(new Error(`${command} exited ${code}`)));});}
try {
 for(const port of [3334,4000]) await new Promise((done,reject)=>{const server=createServer();server.once('error',reject);server.listen(port,'127.0.0.1',()=>server.close(done));});
 await db.connect();await db.query(`CREATE SCHEMA "${schema}"`);await db.query(`COMMENT ON SCHEMA "${schema}" IS '${marker}'`);
 await run('node',['ace','migration:run','--force','--disable-locks'],migrations);
 await run('cargo',['build','--locked'],resolve(workspace,'url-shortener'));
 await run('go',['test','-p=1','-race','-tags=integration','-count=1','-run','TestShortLinks|TestShortLinkCollisions','./internal/httpapi','./internal/shortlink'],root);
 if(process.argv.includes('--browser')){
  await run('go',['build','-o',resolve(artifacts,'admin-api'),'./cmd/api'],root);
  const fd=openSync(resolve(artifacts,'api.log'),'w',0o600);
  const api=spawn(resolve(artifacts,'admin-api'),[],{cwd:root,env,stdio:['ignore',fd,fd]});
  try{
   for(let i=0;i<100;i++){try{if((await fetch('http://localhost:3334/health')).ok)break;}catch{}if(i===99)throw new Error('API readiness failed');await new Promise(r=>setTimeout(r,100));}
   await shortLinksBrowser(db,schema,artifacts);
   report.browser='passed';
  }finally{if(api.exitCode===null){api.kill('SIGTERM');await new Promise(r=>api.once('exit',r));}closeSync(fd);}
 }
 report.checks=JSON.parse(readFileSync(resolve(artifacts,'checks.json'),'utf8'));
 report.source=sourceEvidence();
 report.status='passed';
} catch(error) {
 report.status='failed';
 throw error;
} finally {
 const owned=await db.query('SELECT obj_description(oid) AS marker FROM pg_namespace WHERE nspname=$1',[schema]);
 assert.equal(owned.rows[0]?.marker,marker);
 await db.query(`DROP SCHEMA "${schema}" CASCADE`);report.cleanup='complete';await db.end();
 writeFileSync(resolve(artifacts,'verification.json'),JSON.stringify(report,null,2)+'\n');
 console.log(JSON.stringify(report));
}
