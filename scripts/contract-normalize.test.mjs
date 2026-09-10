import assert from 'node:assert/strict';
import {test} from 'node:test';
import {ContractNormalizer} from './contract-normalize.mjs';

const sample=(rows,path='/v2/activities/1/registrations-export')=>({method:'GET',path,status:200,body:{sheets:[{name:'Registrations',rows:[['No','Name','Email'],...rows]}]}});
const normalize=result=>new ContractNormalizer(Date.now()).response(result);
const alice=[1,'Alice fixture','alice@example.test'],bob=[2,'Bob fixture','bob@example.test'];

test('unordered registration export retains values and verifies visible numbering',()=>{
  const original=sample([alice,bob]);
  const reordered=sample([[1,...bob.slice(1)],[2,...alice.slice(1)]]);
  assert.deepEqual(normalize(original),normalize(reordered));
  assert.deepEqual(original.body.sheets[0].rows[1],alice,'normalization must not mutate evidence');
  assert.throws(()=>normalize(sample([[2,...alice.slice(1)]])),/actual row positions/);
  assert.notDeepEqual(normalize(original),normalize(sample([alice,[2,'Changed name','bob@example.test']])));
  assert.notDeepEqual(normalize(original),normalize(sample([alice])));
  const changedHeader=structuredClone(original);changedHeader.body.sheets[0].rows[0][1]='Changed column';
  assert.notDeepEqual(normalize(original),normalize(changedHeader));
});

test('ordered club exports remain sensitive to row order',()=>{
  const path='/v2/clubs/1/registrations-export';
  assert.notDeepEqual(normalize(sample([alice,bob],path)),normalize(sample([[1,...bob.slice(1)],[2,...alice.slice(1)]],path)));
});

test('development diagnostics preserve identity while validating runtime frames',()=>{
  const source={method:'GET',path:'/v2/admin-users/bad',status:500,body:{message:'select query - invalid integer',name:'error',status:500,frames:[{filePath:'/app/node_modules/pg-protocol/parser.ts',callee:'Parser.parse',line:42,isApp:false,isModule:true,isNative:false}]}};
  const candidate=structuredClone(source);
  candidate.body.frames=[{filePath:'/app/internal/httpapi/admin.go',callee:'kaderisasi/admin/internal/httpapi.(*Server).adminReply',line:22,isApp:true,isModule:false,isNative:false}];
  assert.deepEqual(normalize(source),normalize(candidate));
  candidate.body.message='Different diagnostic';
  assert.notDeepEqual(normalize(source),normalize(candidate));
  candidate.body.frames=[];
  assert.throws(()=>normalize(candidate),/development diagnostic frames/);
});

test('export diagnostic normalization preserves the error and requires actual call sites',()=>{
  const source={method:'GET',path:'/v2/activities/1/registrations-export',status:500,body:{message:'GENERAL_ERROR',error:'TypeError: invalid stored field\n    at ActivityRegistrationsController.export (/app/controller.ts:12)'}};
  const candidate=structuredClone(source);
  candidate.body.error='TypeError: invalid stored field\n    at kaderisasi/admin/internal/httpapi.(*Server).exportRegistrations (/app/export.go:20)';
  assert.deepEqual(normalize(source),normalize(candidate));
  candidate.body.error=candidate.body.error.replace('invalid stored field','different error');
  assert.notDeepEqual(normalize(source),normalize(candidate));
  candidate.body.error='TypeError: invalid stored field\n    at unrelated (/app/other.go:20)';
  assert.throws(()=>normalize(candidate),/actual registration export or database call site/);
});
