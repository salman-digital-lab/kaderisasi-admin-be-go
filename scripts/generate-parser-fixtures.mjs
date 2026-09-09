import {writeFileSync,mkdirSync} from 'node:fs';
import {resolve} from 'node:path';
import {legacyRequire} from './fixture-db.mjs';
import {root} from './env.mjs';
const qs=legacyRequire('qs');
const forms=[
 'name=Fixture&country=', 'a=1&a=2', 'badges[]=LMD&badges[]=ALUMNI',
 'extra_data.preferred_name=Fixture', 'a[0][name]=First&a[0][value]=one&a[1][name]=Second',
 'a[3]=three&a[1]=one', 'a[0]=one&a[0]=two', 'a[]=one&a[0]=two',
 'a[20]=twenty&a[21]=twenty-one', 'a[01]=zero-one&a[1]=one',
 'a[b]=one&a[]=two', 'a=one&a[b]=two', 'a[b]=one&a=two',
 'a=one&a[]=two', 'a[]=one&a[b]=two', 'a.b[c]=one',
 'a[b][c][d][e][f][g]=seven', 'a.b.c.d.e.f.g=seven',
 '__proto__[polluted]=yes&name=safe','constructor[prototype][polluted]=yes&name=safe',
 'a[toString]=ignored&a[safe]=yes','a[prototype]=value',
 'empty=&nil&space=+&encoded=%26%3D&unicode=%E2%98%83', 'bad=%E0%A4%A',
 'a[]=one&a[]=two&a[]=three', 'a[0][x]=one&a[0][y]=two',
 'a[1]=one&a[3]=three&a[2]=two', 'a[2][x]=two&a[0][x]=zero',
 'a[b]=&a[c][]=one&a[c][]=', 'email=super%40example.test&password=Test%2B123',
];
const cases=forms.flatMap(raw=>[false,true].map(emptyNull=>({raw,emptyNull,expected:qs.parse(raw,{allowDots:true,...(emptyNull?{decoder(str,decoder,charset,type){const value=decoder(str,decoder,charset);return type==='value'&&value===''?null:value}}:{})})})));
const inputs=['{','{ ','{\n','{email:1}','{"a" 1}','{"a":1,}','{"a":1 x}','[1 x]','{} {}','[]x','{"a":}','[}]','[1,]','[','[1','{"a":','{"a":"unterminated','{"a":"bad\\x"}','{"a":"bad\n"}','{"a":01}','{"a":-}','{"a":1.}','{"a":1e}','{"a":tru}','{"a":undefined}','{"雪":}','{"😀":}','{\n"a":1,\n bad:2}','{"verylongkey":"some long text", "a":undefined}'];
const json=inputs.map(raw=>{try{JSON.parse(raw);throw new Error('Expected invalid fixture')}catch(error){return {raw,message:error.message}}});
mkdirSync(resolve(root,'tests/fixtures'),{recursive:true});
writeFileSync(resolve(root,'tests/fixtures/parser.json'),JSON.stringify({forms:cases,json},null,2)+'\n');
console.log(`${cases.length} qs and ${json.length} Node JSON error fixtures`);
