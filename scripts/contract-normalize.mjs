import { createHash } from 'node:crypto';
import assert from 'node:assert/strict';
import { legacyRequire, fixtureKey } from './fixture-db.mjs';
const jwt=legacyRequire('jsonwebtoken');
const {Encryption}=legacyRequire('@adonisjs/encryption');
const verifier=new Encryption({secret:fixtureKey}).verifier;

// Only identified sources of nondeterminism are normalized. This object lives
// for one backend run so references retain the same identity across requests.
export class ContractNormalizer {
  constructor(started){this.started=started;this.random=new Map();this.hashes=new Map();}
  identity(value){if(!this.random.has(value))this.random.set(value,`<uuid-${this.random.size+1}>`);return this.random.get(value);}
  cookie(raw){
    const [pair,...attributes]=raw.split(';').map(x=>x.trim());
    const index=pair.indexOf('='),name=pair.slice(0,index),value=pair.slice(index+1);
    assert.equal(name,'admin_refresh_token');
    let canonicalValue=value;
    if(value){
      const signed=decodeURIComponent(value);assert.ok(signed.startsWith('s:'));
      const token=verifier.unsign(signed.slice(2),'admin_refresh_token');
      assert.match(token,/^[A-Za-z0-9_-]{64}$/);
      const hash=createHash('sha256').update(token).digest('hex');
      if(!this.hashes.has(hash))this.hashes.set(hash,`<refresh-${this.hashes.size+1}>`);
      canonicalValue=(value.startsWith('s%3A')?'s%3A':'s:')+this.hashes.get(hash);
    }
    const result={name,value:canonicalValue};
    for(const attribute of attributes){const [key,...rest]=attribute.split('=');result[key.toLowerCase()]=rest.length?rest.join('='):true;}
    return result;
  }
  normalize(value,path=''){
    if(Array.isArray(value))return value.map((item,index)=>this.normalize(item,`${path}/${index}`));
    // Object member order is not part of the JSON contract. Canonical traversal
    // also gives equivalent random references stable identities in both APIs.
    if(value&&typeof value==='object')return Object.fromEntries(Object.entries(value).sort(([a],[b])=>a.localeCompare(b)).map(([key,item])=>[key,this.normalize(item,`${path}/${key}`)]));
    if(typeof value!=='string')return value;
    if(path==='/body/error'&&value.startsWith('E_ROW_NOT_FOUND: Row not found\n')){
      const [identity,...frames]=value.split('\n');
      assert.ok(frames.length>0&&frames.every(frame=>/^\s+at .+/.test(frame)),'export diagnostic stack format');
      assert.ok(frames.some(frame=>/ActivityRegistrationsController\.export|httpapi\.\(\*Server\)\.exportRegistrations/.test(frame)),'actual registration export call site');
      // Error identity remains exact. Source paths, line numbers, and runtime
      // call sites change across builds and languages; validate then normalize.
      return identity+'\n<validated export diagnostic stack>';
    }
    if(/CERT-\d{4}-\d+-[A-F0-9]{32}/i.test(value)){
      return value.replace(/CERT-(\d{4})-(\d+)-[A-F0-9]{32}/gi,(code,year,activity)=>{
        assert.equal(Number(year),new Date().getUTCFullYear());assert.ok(Number(activity)>0);
        const identity=this.identity(code.toUpperCase());
        return code===code.toLowerCase()?identity.toLowerCase():identity.toUpperCase();
      });
    }
    if(/(?:^|:|\/)(?:activity|club|certificate)\//.test(value)){
      return value.replace(/[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}/gi,uuid=>this.identity(uuid));
    }
    if(path.endsWith('/password')&&path.includes('/database/')){
      assert.match(value,/^\$scrypt\$n=16384,r=8,p=1\$[A-Za-z0-9+/]+\$[A-Za-z0-9+/]+$/);
      return '<verified-synthetic-scrypt-password>';
    }
    if(path.endsWith('/number')&&value.startsWith('AR-')){
      assert.match(value,/^AR-[A-Z0-9]+-[A-F0-9]{6}$/);
      assert.ok(Math.abs(parseInt(value.split('-')[1],36)-Date.now())<120000,'ticket timestamp');
      return this.identity(value);
    }
    if((path.endsWith('/access_token')||path.endsWith('/token/token'))&&value){
      const decoded=jwt.verify(value,fixtureKey,{complete:true,algorithms:['HS256']});
      assert.equal(decoded.payload.exp-decoded.payload.iat,900);assert.ok(decoded.payload.iat*1000>=this.started-2000);
      return {header:decoded.header,claims:{...decoded.payload,iat:'<issued-now>',exp:'<issued-now+900>'}};
    }
    if(path.endsWith('/family_id')){assert.match(value,/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);return this.identity(value);}
    if(path.endsWith('/token_hash')){assert.ok(this.hashes.has(value),'refresh hash must come from a verified response cookie');return this.hashes.get(value);}
    const runtimeTimestamp=/\/(?:created_at|updated_at|updatedAt|expires_at|last_used_at|revoked_at|issued_at|published_at|archived_at|reviewed_at|resolved_at|cancelled_at|approved_at|requested_at)$/.test(path);
    if(runtimeTimestamp&&/^\d{4}-\d{2}-\d{2}T/.test(value)&&Date.parse(value)>=this.started-1000){
      const expires=path.endsWith('/expires_at');
      assert.ok(Date.parse(value)<=Date.now()+(expires?30*86400000:0)+5000,'unexpected future timestamp');
      // Database JSON timestamps use PostgreSQL's variable precision. HTTP
      // formatting is compared strictly; DB values are checked as instants.
      if(path.includes('/database/'))return expires?'<database-now+30-days>':'<database-now>';
      return value.replace(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/,'<runtime-time>').replace(/\.\d+/m,match=>'.'+'#'.repeat(match.length-1));
    }
    return value;
  }
}
