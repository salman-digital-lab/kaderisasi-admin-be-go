import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {readFileSync,writeFileSync,rmSync} from 'node:fs';
import {resolve} from 'node:path';
import {root} from './env.mjs';
import {sourceEvidence} from './source-evidence.mjs';

const source=sourceEvidence();
const report={source,status:'running',cleanup:'pending',checks:[]};
const path=resolve(root,'.artifacts/calendar.json');
const cases=resolve(root,'.artifacts/calendar-cases.json');
const save=()=>writeFileSync(path,JSON.stringify(report,null,2)+'\n');
rmSync(cases,{force:true});save();
try {
  assert.equal(spawnSync('node',['scripts/test-go.mjs','./internal/httpapi','-run','TestCalendarWorkflow','-v'],{cwd:root,stdio:'inherit'}).status,0,'Calendar database and authorization tests');
  report.checks=JSON.parse(readFileSync(cases,'utf8'));
  if(process.argv.includes('--browser')) {
    const options=process.argv.filter(arg=>arg==='--borrow-workspace');
    for(const scope of [[],['--public']])assert.equal(spawnSync('node',['scripts/browser.mjs','calendar.spec.mjs',...scope,...options],{cwd:root,stdio:'inherit'}).status,0,'Calendar desktop and mobile tests');
  }
  assert.equal(sourceEvidence().sha256,source.sha256,'Source changed during calendar verification');
  report.status='passed';report.cleanup='complete';
} catch(error) {
  report.status='failed';report.failure=error.message;throw error;
} finally { save(); }
