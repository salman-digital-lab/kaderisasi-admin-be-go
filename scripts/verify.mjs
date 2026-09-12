import {spawn} from 'node:child_process';
import {mkdirSync,writeFileSync,readFileSync} from 'node:fs';
import {once} from 'node:events';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {sourceEvidence} from './source-evidence.mjs';
const source=sourceEvidence();

const directory=resolve(root,'.artifacts/verify',new Date().toISOString().replace(/[:.]/g,'-'));
mkdirSync(directory,{recursive:true});
const results=[];
async function run(name,command,args){
  const started=Date.now(),child=spawn(command,args,{cwd:root,stdio:['ignore','pipe','pipe']});let output='';
  for(const stream of [child.stdout,child.stderr])stream.on('data',chunk=>{output+=chunk;process.stdout.write(chunk);});
  const [code]=await once(child,'exit');
  writeFileSync(resolve(directory,name+'.log'),output);
  const skipped=output.split('\n').flatMap(line=>{try{const event=JSON.parse(line);return event.Action==='skip'&&event.Test?[event.Test]:[];}catch{return [];}});
  const result={name,command:[command,...args],status:code===0&&!skipped.length?'passed':'failed',exit_code:code,duration_ms:Date.now()-started,skipped};results.push(result);
  writeFileSync(resolve(directory,'results.json'),JSON.stringify({status:'running',results},null,2));
  return result.status==='passed';
}
try{
  if(!await run('check','node',['scripts/check.mjs']))throw new Error('Foundation checks failed');
  await run('unit','go',['test','-json','-race','-count=1','./...']);
  await run('package','node',['scripts/package.mjs']);
  if(!await run('fixtures','node',['scripts/ensure-fixtures.mjs']))throw new Error('Fixture preparation failed');
  await run('fixture-disconnect','node',['scripts/test-fixture-disconnect.mjs']);
  await run('source-club-workflows','node',['scripts/source-club-workflows.mjs']);
  await run('integration','node',['scripts/test-go.mjs','-json','./...']);
  await run('contracts','node',['scripts/contracts-all.mjs',...process.argv.slice(2)]);
  await run('session-transfer','node',['scripts/session-transfer.mjs',...process.argv.slice(2)]);
  await run('shared-database','node',['scripts/shared-database.mjs',...process.argv.slice(2)]);
  await run('jobs','node',['scripts/contract-jobs.mjs']);
  await run('admin-browser','node',['scripts/browser.mjs',...process.argv.slice(2)]);
  await run('public-browser','node',['scripts/browser.mjs','--public',...process.argv.slice(2)]);
  await run('courses','node',['scripts/course-workflows.mjs','--browser',...process.argv.slice(2)]);
  await run('existing-applications','node',['scripts/baseline.mjs']);
  await run('performance','node',['scripts/performance.mjs',...process.argv.slice(2)]);
  await run('feature-deletion','node',['scripts/feature-deletion.mjs','--browser',...process.argv.slice(2)]);
  await run('route-coverage','node',['scripts/coverage.mjs','--check']);
}catch(error){results.push({name:'orchestration',status:'failed',error:error.message});}
finally{
  await run('cleanup','node',['scripts/cleanup.mjs']);
  const review=JSON.parse(readFileSync(resolve(root,'docs/completion-review.json'),'utf8'));
  const unresolved=review.criteria.filter(criterion=>criterion.status!=='passed');
  const sourceUnchanged=source.sha256===sourceEvidence().sha256;
  if(!sourceUnchanged)results.push({name:'source-integrity',status:'failed',error:'Source changed while verification was running'});
  const status=results.every(result=>result.status==='passed')&&!unresolved.length?'passed':'incomplete';
  const report={status,source,finished_at:new Date().toISOString(),results,unresolved_review:unresolved};
  writeFileSync(resolve(directory,'results.json'),JSON.stringify(report,null,2));
  writeFileSync(resolve(root,'.artifacts/verify/latest.json'),JSON.stringify({directory,status},null,2));
  if(status!=='passed')process.exitCode=1;
  console.log(`Verification ${status}; evidence: ${directory}`);
}
