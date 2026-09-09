import {generateKeyPairSync} from 'node:crypto';
import {createServer} from 'node:http';
import {once} from 'node:events';
import assert from 'node:assert/strict';
import {legacyRequire} from './fixture-db.mjs';
const jwt=legacyRequire('jsonwebtoken');

export async function googleFixture(){
  const {privateKey,publicKey}=generateKeyPairSync('rsa',{modulusLength:2048});
  const jwk={...publicKey.export({format:'jwk'}),kid:'synthetic-key',use:'sig',alg:'RS256'};
  const counts={pem:0,jwks:0};
  const server=createServer((request,response)=>{
    const pem=request.url==='/pem';counts[pem?'pem':'jwks']++;
    response.writeHead(200,{'Content-Type':'application/json','Cache-Control':'public, max-age=600'});
    response.end(JSON.stringify(pem?{'synthetic-key':publicKey.export({type:'spki',format:'pem'})}:{keys:[jwk]}));
  });
  server.listen(0,'127.0.0.1');await once(server,'listening');
  return{counts,url:`http://127.0.0.1:${server.address().port}/jwks`,close:()=>new Promise((resolve,reject)=>server.close(error=>error?reject(error):resolve())),token(overrides={},options={}){
    const now=Math.floor(Date.now()/1000);
    const claims={iss:'https://accounts.google.com',aud:'synthetic-client',sub:'synthetic-google-subject',email:'google@example.test',email_verified:true,name:'Google fixture',iat:now-10,exp:now+3600,...overrides};
    for(const key of Object.keys(claims))if(claims[key]===null)delete claims[key];
    return jwt.sign(claims,options.key??privateKey,{algorithm:'RS256',keyid:options.kid??'synthetic-key'});
  }};
}

export async function googleCases(h,keys){
  const call=(name,claims={},options={})=>h.call('google:'+name,'POST','/v2/auth/google',{credential:keys.token(claims,options)},'none');
  const created=await call('new-account');assert.equal(created.data.user.email,'google@example.test');
  await call('repeat-subject',{email:'changed-google@example.test'});
  const linked=await call('link-password',{sub:'synthetic-super-subject',email:'SUPER@example.test'});assert.equal(linked.data.is_super_admin,true);
  for(const [name,claims] of [
    ['wrong-issuer',{iss:'https://invalid.example.test'}],['wrong-audience',{aud:'wrong'}],
    ['unverified',{sub:'unverified',email_verified:false}],['missing-email',{email:null}],['missing-subject',{sub:null}],
    ['inactive',{sub:'inactive-subject',email:'inactive@example.test'}],
    ['legacy-issuer',{iss:'accounts.google.com'}],
    ['expired',{exp:Math.floor(Date.now()/1000)-360}],['expired-within-skew',{exp:Math.floor(Date.now()/1000)-60}],
    ['future-issued',{iat:Math.floor(Date.now()/1000)+360}],['missing-issued',{iat:null}],['missing-expiry',{exp:null}],
  ])await call(name,claims);
  await call('unknown-key',{}, {kid:'unknown-fixture-key'});
  const other=generateKeyPairSync('rsa',{modulusLength:2048});
  await call('forged',{}, {key:other.privateKey});
  await h.call('google:malformed','POST','/v2/auth/google',{credential:'malformed'},'none');
}
