import assert from 'node:assert/strict';
import {once} from 'node:events';
import {randomUUID} from 'node:crypto';
import {writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {fixtureDatabase} from './fixture-db.mjs';
import {acquireFixtureLease} from './fixture-lease.mjs';

acquireFixtureLease();
const victim=await fixtureDatabase('candidate');
let control;
try {
  control=await fixtureDatabase('cross');
  const application='go-rewrite-disconnect-'+randomUUID();
  await victim.db.query("SELECT set_config('application_name',$1,false)",[application]);
  const identity=(await victim.db.query("SELECT pid,backend_start::text FROM pg_stat_activity WHERE pid=pg_backend_pid() AND application_name=$1",[application])).rows[0];assert.ok(identity);
  // Only this run's explicitly tagged, still-live connection is eligible. The
  // query cannot terminate another application or a reused backend PID.
  const failed=once(victim.db,'error',{signal:AbortSignal.timeout(5000)});
  failed.catch(()=>{}); // Preserve teardown if the control query itself fails.
  const stopped=await control.db.query('SELECT pg_terminate_backend(pid) AS stopped FROM pg_stat_activity WHERE pid=$1 AND backend_start=$2::timestamptz AND application_name=$3 AND usename=current_user AND datname=current_database()',[identity.pid,identity.backend_start,application]);
  assert.equal(stopped.rows.length,1);assert.equal(stopped.rows[0].stopped,true);
  const [failure]=await failed;assert.equal(failure.code,'57P01');
  assert.throws(()=>victim.assertHealthy(),/connection was lost/);
  await assert.rejects(()=>victim.db.query('SELECT 1'),/connection error|not queryable|closed|terminated/i);
  await control.db.query('SELECT 1');control.assertHealthy();
  writeFileSync(resolve(root,'.artifacts/fixture-disconnect.json'),JSON.stringify({status:'passed',error_code:failure.code,own_connection_terminated:true,future_queries_reject:true,control_connection_healthy:true,at:new Date().toISOString()},null,2)+'\n');
  console.log('Owned PostgreSQL disconnect is handled; future queries reject and cleanup remains reachable');
}finally{
  const closed=await Promise.allSettled([victim.db.end(),control?.db.end()]);for(const result of closed)if(result.status==='rejected')throw result.reason;
}
