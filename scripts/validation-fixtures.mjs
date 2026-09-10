import {createRequire} from 'node:module';
import {readFileSync,readdirSync,writeFileSync,mkdirSync} from 'node:fs';
import {resolve} from 'node:path';
import vm from 'node:vm';
import {root,legacy} from './env.mjs';
import {inlineValidators} from './inline-validators.mjs';
const require=createRequire(resolve(legacy,'package.json'));
const ts=require('typescript');const vine=require('@vinejs/vine');
// File validators are registered by Adonis and exercised via multipart HTTP
// tests. This placeholder only permits loading modules containing JSON schemas.
vine.default.file=()=>vine.default.any();
const schemas=JSON.parse(readFileSync(resolve(root,'internal/validation/schemas.json'),'utf8'));
const roles=JSON.parse(readFileSync(resolve(root,'internal/auth/roles.json'),'utf8'));
const clubTypes=JSON.parse(readFileSync(resolve(legacy,'app/models/club.ts'),'utf8').match(/CLUB_TYPES\s*=\s*(\[[\s\S]*?\])/)[1].replaceAll("'",'"'));
const validators={};
for(const file of readdirSync(resolve(legacy,'app/validators')).filter(x=>x.endsWith('.ts'))){const exports={};const source=ts.transpileModule(readFileSync(resolve(legacy,'app/validators',file),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022,esModuleInterop:true}}).outputText;vm.runInNewContext(source,{exports,require:name=>name==='@vinejs/vine'?vine:name==='#models/club'?{CLUB_TYPES:clubTypes}:{ADMIN_ROLE_CODES:roles.map(x=>x.code)}});Object.assign(validators,exports)}
const inlineExports={};
vm.runInNewContext(ts.transpileModule(inlineValidators(),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{exports:inlineExports,vine:vine.default});
Object.assign(validators,inlineExports);
function sample(rule){
 const methods=Object.fromEntries(rule.chain.map(x=>[x.method,x.args]));
 switch(rule.kind){
 case'object':return Object.fromEntries(Object.entries(rule.args[0]).map(([k,v])=>[k,sample(v)]));
 case'array':return Array.from({length:methods.minLength?.[0]??1},()=>sample(rule.args[0]));
 case'record':return {fixture:sample(rule.args[0])};
 case'string':return methods.email?'fixture@example.test':methods.url?'https://example.test': 'x'.repeat(Math.max(3,methods.minLength?.[0]??0));
 case'number':return methods.range?methods.range[0][0]:1;
 case'boolean':return true;
 case'enum':return rule.args[0][0];
 case'literal':return rule.args[0];
 case'date':return '2026-08-01';
 case'any':return {fixture:'arbitrary JSON'};
 case'file':throw new Error('File validation belongs to multipart HTTP integration');
 default:throw new Error(rule.kind);
 }
}
// Lucid date columns preserve the local calendar date, not the UTC day of the
// underlying JavaScript Date (which can be the preceding day in Jakarta).
function normalize(value){if(value instanceof Date)return `${value.getFullYear()}-${String(value.getMonth()+1).padStart(2,'0')}-${String(value.getDate()).padStart(2,'0')}`;if(Array.isArray(value))return value.map(normalize);if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).map(([k,v])=>[k,normalize(v)]));return value}
function emptyToNull(value){if(value==='')return null;if(Array.isArray(value))return value.map(emptyToNull);if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).map(([k,v])=>[k,emptyToNull(v)]));return value}
const fixtures=[];
for(const [name,rule]of Object.entries(schemas)){
 if(JSON.stringify(rule).includes('"kind":"file"'))continue;
 const good=sample(rule);
 const cases=[['valid',good],['missing',{}],['nulls',Object.fromEntries(Object.keys(good).map(k=>[k,null]))],['unknown',{...good,unknownFixtureField:123}]];
 for(const [key,value]of Object.entries(good))if(typeof value==='string')cases.push([`invalid-${key}`,{...good,[key]:{invalid:true}}]);
 for(const [key,value]of Object.entries(good))if(typeof value==='number')for(const number of [0,-1,0.5,2147483648])cases.push([`boundary-${key}-${number}`,{...good,[key]:number}]);
 for(const [key,field]of Object.entries(rule.args[0]))if(field.kind==='number')for(const value of [true,false,[],[null],[[]],['2'],['0x2'],[1,2],{},'0x2','0b10','0o2','\ufeff2','\u00852',' 2 ',' ','Infinity','0x'])cases.push([`number-coercion-${key}-${JSON.stringify(value)}`,{...good,[key]:value}]);
 for(const [key,field]of Object.entries(rule.args[0]))if(field.kind==='date')for(const value of ['2026-03-01 00:30:00','2026-03-01T00:30:00+07:00','2026-03-01T00:30:00.000Z','2026-03-01 00:30:00.000','2026-02-29','2024-02-29','2026-3-1','2026-03-01 1:02:03'])cases.push([`date-format-${key}-${value}`,{...good,[key]:value}]);
 for(const [label,input]of cases){try{const value=await validators[name].validate(emptyToNull(structuredClone(input)));fixtures.push({name,label,input,output:normalize(value),issues:[]})}catch(error){if(!error.messages)throw error;fixtures.push({name,label,input,issues:error.messages})}}
}
mkdirSync(resolve(root,'internal/validation/testdata'),{recursive:true});writeFileSync(resolve(root,'internal/validation/testdata/fixtures.json'),JSON.stringify(fixtures,null,2)+'\n');console.log(`Captured ${fixtures.length} validation fixtures from Vine.`);
