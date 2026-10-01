import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {readFileSync,writeFileSync,rmSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {sourceEvidence} from './source-evidence.mjs';

const source=sourceEvidence();
const report={source,status:'running',cleanup:'pending',checks:[]};
const path=resolve(root,'.artifacts/talent-assessment.json');
const cases=resolve(root,'.artifacts/talent-assessment-cases.json');
const save=()=>writeFileSync(path,JSON.stringify(report,null,2)+'\n');
rmSync(cases,{force:true});save();
try {
  assert.equal(spawnSync('node',['scripts/test-go.mjs','./internal/httpapi','-run','TestTalentAssessmentWorkflow','-v'],{cwd:root,stdio:'inherit'}).status,0,'Talent assessment database and authorization tests');
  report.checks=JSON.parse(readFileSync(cases,'utf8'));
  if(process.argv.includes('--browser')) {
    const options=process.argv.filter(arg=>arg==='--borrow-workspace');
    assert.equal(spawnSync('node',['scripts/browser.mjs','talent-assessment.spec.mjs',...options],{cwd:root,stdio:'inherit'}).status,0,'Talent assessment desktop and mobile tests');
  }
  assert.equal(sourceEvidence().sha256,source.sha256,'Source changed during talent-assessment verification');
  report.status='passed';report.cleanup='complete';
} catch(error) {
  report.status='failed';report.failure=error.message;throw error;
} finally { save(); }
