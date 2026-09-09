import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {performance} from 'node:perf_hooks';
import {root,legacy,testEnvironment} from './env.mjs';
import {fixtureDatabase,resetFixture,fixtureKey,fixturePassword,legacyRequire} from './fixture-db.mjs';
import {borrowWorkspacePort,startServer} from './server-process.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';

acquireFixtureLease();
const results=[];
const f=await fixtureDatabase('cross');
let server,restore;
const token=legacyRequire('jsonwebtoken').sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'15m'});
const workloads=[['health','/health',100],['reference','/v2/countries',40],['members','/v2/profiles?per_page=20',40],['dashboard','/v2/dashboard/stats',40]];
function rss(pid){const result=spawnSync('ps',['-p',String(pid),'-o','rss='],{encoding:'utf8'});assert.equal(result.status,0);return Number(result.stdout.trim())/1024;}
async function request(path){const start=performance.now();const response=await fetch('http://127.0.0.1:3334'+path,{headers:{Authorization:`Bearer ${token}`,Accept:'application/json'},signal:AbortSignal.timeout(15000)});await response.arrayBuffer();assert.equal(response.status,200,path);return performance.now()-start;}
try{
  const {Scrypt}=legacyRequire('@adonisjs/hash/drivers/scrypt');await resetFixture(f.db,f.schema,await new Scrypt({}).make(fixturePassword));
  await f.db.query("INSERT INTO public_users(email,member_id,account_status,created_at,updated_at) SELECT 'benchmark-'||n||'@example.test',lpad(n::text,8,'0'),'no_account','2024-01-01','2024-01-01' FROM generate_series(1,100) n");
  await f.db.query("INSERT INTO profiles(user_id,name,gender,created_at,updated_at) SELECT id,'Benchmark member '||id,'F','2024-01-01','2024-01-01' FROM public_users");
  const build=spawnSync('node',['ace','build'],{cwd:legacy,env:testEnvironment({PGOPTIONS:'-c default_transaction_read_only=on'}),encoding:'utf8',timeout:60000});
  writeFileSync(resolve(root,'.artifacts/performance-adonis-build.log'),(build.stdout??'')+(build.stderr??''));assert.equal(build.status,0,'Build Adonis for representative compiled comparison');
  assert.equal(spawnSync('go',['build','-o',resolve(root,'.artifacts/admin-api'),'./cmd/api'],{cwd:root,stdio:'inherit'}).status,0);
  restore=await borrowWorkspacePort(3334,process.argv.includes('--borrow-workspace'));
  // Reverse order on the second pass to reduce one-sided warm-cache effects.
  for(const [round,order] of [[1,['adonis','go']],[2,['go','adonis']]])for(const backend of order){
    server=await startServer(backend,f.schema,`performance-${round}-${backend}`,{compiled:true});
    for(const [,path] of workloads)for(let i=0;i<5;i++)await request(path);
    const memoryBefore=rss(server.child.pid),measurements=[];
    for(const [name,path,count] of workloads){
      const durations=[],start=performance.now();let next=0;
      await Promise.all(Array.from({length:4},async()=>{while(next++<count)durations.push(await request(path));}));
      const elapsed=performance.now()-start;durations.sort((a,b)=>a-b);
      measurements.push({name,path,requests:count,concurrency:4,p50_ms:durations[Math.ceil(count*.50)-1],p95_ms:durations[Math.ceil(count*.95)-1],min_ms:durations[0],max_ms:durations.at(-1),elapsed_ms:elapsed,requests_per_second:count*1000/elapsed});
    }
    results.push({round,backend,rss_before_mib:memoryBefore,rss_after_mib:rss(server.child.pid),measurements});
    await server.stop();server=undefined;
  }
  const report={status:'passed',observed_at:new Date().toISOString(),schema:f.schema,versions:{node:process.version,go:spawnSync('go',['version'],{encoding:'utf8'}).stdout.trim()},notes:['Both implementations used compiled code, NODE_ENV=test, the same host and remote PostgreSQL fixture.','Four concurrent requests, 220 measured and 20 warm-up requests per backend per round; two rounds in reversed order.','RSS measures the API process including its runtime, not database memory. These bounded observations do not predict production capacity.'],results};
  writeFileSync(resolve(root,'.artifacts/performance.json'),JSON.stringify(report,null,2));
  console.log(JSON.stringify(results.map(r=>({round:r.round,backend:r.backend,rss_mib:r.rss_after_mib,latency:r.measurements.map(m=>({name:m.name,p50_ms:m.p50_ms,p95_ms:m.p95_ms}))})),null,2));
}finally{
  const failures=[];for(const cleanup of [()=>server?.stop(),()=>f.db.end(),()=>restore?.()]){try{await cleanup();}catch(error){failures.push(error);}}
  if(failures.length)throw new AggregateError(failures,'Performance cleanup failed');
}
