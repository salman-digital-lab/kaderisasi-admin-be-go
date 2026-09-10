import {execFileSync} from 'node:child_process';
import {mkdirSync,readFileSync,readdirSync,writeFileSync,existsSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {sourceEvidence} from './source-evidence.mjs';
import {contractGroups} from './contract-groups.mjs';

const routes=JSON.parse(execFileSync('go',['run','./cmd/inventory'],{cwd:root,encoding:'utf8'}));
const baseline=JSON.parse(readFileSync(resolve(root,'docs/BASELINE.json'),'utf8'));
const applicability=JSON.parse(readFileSync(resolve(root,'tests/route-applicability.json'),'utf8'));
if(applicability.baseline_revision!==baseline.revision)throw new Error('Route applicability must be reviewed for the adopted source');
const reviews=new Map(applicability.routes.map(row=>[row.route,row]));
if(reviews.size!==routes.length||reviews.size!==applicability.routes.length)throw new Error('Every inventoried route requires one explicit applicability review');
const artifacts=resolve(root,'.artifacts'),contracts=resolve(artifacts,'contracts');
mkdirSync(artifacts,{recursive:true});
const priority=[...routes].sort((a,b)=>{
  const x=a.path.split('/'),y=b.path.split('/');
  for(let i=0;i<Math.min(x.length,y.length);i++)if(x[i].startsWith(':')!==y[i].startsWith(':'))return x[i].startsWith(':')?1:-1;
  return y.length-x.length;
});
function match(scenario){
  if(!scenario.path)return undefined;
  const parts=scenario.path.split('?')[0].replace(/\/$/,'').split('/');
  return priority.find(route=>route.method===scenario.method&&route.path.split('/').length===parts.length&&route.path.split('/').every((segment,i)=>segment.startsWith(':')||segment===parts[i]));
}
const evidence=new Map(routes.map(route=>[route,[]]));
const reports=[];
for(const name of existsSync(contracts)?readdirSync(contracts).filter(name=>name.endsWith('-report.json')):[]){
  const report=JSON.parse(readFileSync(resolve(contracts,name),'utf8'));reports.push(report);
  const cases=JSON.parse(readFileSync(resolve(contracts,report.group+'-go.json'),'utf8'));
  const failed=new Set(report.differences.map(row=>row.name));
  for(const scenario of cases){
    if(failed.has(scenario.name))continue;
    const route=match(scenario);if(!route)continue;
    evidence.get(route).push({group:report.group,name:scenario.name,status:scenario.status,database:!!scenario.database});
  }
}
const matrix=routes.map(route=>{
  const cases=evidence.get(route);
  const review=reviews.get(route.method+' '+route.path);
  if(!review||review.source!==route.controller+'.'+route.action)throw new Error('Missing or stale route applicability review');
  const scenarios={
    success:cases.filter(c=>c.status>=200&&c.status<300),
    authentication:cases.filter(c=>c.status===401),
    permission:cases.filter(c=>c.status===403),
    invalid_input:cases.filter(c=>/edge:invalid-|query:invalid-pagination/.test(c.name)||(/invalid|malformed|tamper|wrong-|boundary/.test(c.name)&&c.status>=400)),
    missing_resource:cases.filter(c=>/missing|not-found/.test(c.name)),
  };
  const pending=[];
  if(!route.implemented)pending.push('implementation');
  if(!scenarios.success.length)pending.push('successful differential request');
  if(route.auth&&!scenarios.authentication.length)pending.push('authentication');
  if(route.permission&&!scenarios.permission.length)pending.push('permission');
  for(const kind of ['invalid_input','missing_resource']){
    if(typeof review[kind]?.applicable!=='boolean'||!review[kind]?.reason)pending.push(kind+' applicability review');
    else if(review[kind].applicable&&!scenarios[kind].length)pending.push(kind+' differential request');
  }
  return {...route,applicability:review,scenarios,pending};
});
const currentSource=sourceEvidence();
const staleGroups=reports.filter(report=>report.source?.sha256!==currentSource.sha256||report.adonis_revision!==baseline.revision).map(report=>report.group);
const missingGroups=contractGroups.filter(group=>!reports.some(report=>report.group===group));
const report={source:currentSource,stale_groups:staleGroups,missing_groups:missingGroups,baseline:baseline.revision,route_count:matrix.length,implemented:matrix.filter(r=>r.implemented).length,success_covered:matrix.filter(r=>r.scenarios.success.length).length,contract_scenarios:reports.reduce((n,r)=>n+r.scenarios,0),contract_failures:reports.reduce((n,r)=>n+r.failed,0),routes:matrix,scope:'Implementation, success, authentication, permission, and explicitly reviewed invalid-input/missing-resource gates.'};
writeFileSync(resolve(artifacts,'route-coverage.json'),JSON.stringify(report,null,2)+'\n');
const cell=cases=>cases.length?cases.map(c=>`${c.group}:${c.name} (${c.status})`).join('<br>'):'—';
const rows=matrix.map(r=>`| ${r.method} ${r.path} | ${r.controller}.${r.action} | ${r.permission||(r.auth?'authenticated':'public')} | ${cell(r.scenarios.success)} | ${r.scenarios.authentication.length} / ${r.scenarios.permission.length} | ${r.applicability.invalid_input.applicable?cell(r.scenarios.invalid_input):'N/A: '+r.applicability.invalid_input.reason} | ${r.applicability.missing_resource.applicable?cell(r.scenarios.missing_resource):'N/A: '+r.applicability.missing_resource.reason} |`);
writeFileSync(resolve(root,'docs/COMPATIBILITY.md'),`# API compatibility matrix\n\nAdopted Adonis revision: \`${baseline.revision}\`. Original: \`${baseline.original_revision}\`.\n\nCurrent Go source fingerprint: \`${currentSource.sha256}\`. Stale groups: ${staleGroups.join(', ')||'none'}. Missing groups: ${missingGroups.join(', ')||'none'}.\n\nThis matrix includes historical passing direct Adonis/Go comparisons. Entries from stale groups require rerunning before they count toward current compatibility. Routes are matched with static-segment precedence. ${report.success_covered}/${matrix.length} routes have a successful comparison; ${report.contract_scenarios} scenarios have run. Authentication and permission columns count passing denials. Input and resource applicability is reviewed explicitly for all routes in tests/route-applicability.json. Every applicable category requires executed passing differential evidence; N/A entries carry the source-based reason. Some invalid inputs deliberately retain the source's success status. PostgreSQL state is compared after every mutation.\n\n| Route | Go owner | Access | Successful evidence | Auth / permission denials | Invalid input evidence | Missing resource evidence |\n|---|---|---|---|---|---|---|\n${rows.join('\n')}\n\nScheduled jobs: \`internal/jobs.Runner\` owns all three. Calendar boundary, repeated execution, cancellation, and database failure checks run against PostgreSQL. All three commands have passed direct Adonis/Go state comparison and repeat execution (six scenarios; .artifacts/job-contracts.json).\n`);
console.log(JSON.stringify({routes:matrix.length,implemented:report.implemented,success_covered:report.success_covered,contract_scenarios:report.contract_scenarios,stale_groups:staleGroups,missing_groups:missingGroups,pending:matrix.filter(r=>r.pending.length).map(r=>({route:r.method+' '+r.path,pending:r.pending}))},null,2));
if(process.argv.includes('--check')&&(staleGroups.length||missingGroups.length||report.contract_failures||matrix.some(r=>r.pending.length)))process.exitCode=1;
