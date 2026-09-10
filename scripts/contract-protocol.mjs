import {gzipSync,deflateSync} from 'node:zlib';

export async function protocolCases(h,password){
  const login=JSON.stringify({email:'super@example.test',password});
  const raw=async(name,path,body,headers={},actor='super',method='POST')=>h.call(`protocol:${name}`,method,path,undefined,actor,{rawBody:body,headers});
  for(const type of ['application/json','application/json-patch+json','application/vnd.api+json','application/csp-report','application/json; charset=utf-8'])
    await raw(`login-${type}`,'/v2/auth/login',login,{'Content-Type':type},'none');
  for(const [name,body] of Object.entries({truncated:'{',trailing:'{} {}',comma:'{"email":}',key:'{email:1}',array:'[]',null:'null',number:'123',string:'"hello"',true:'true',whitespace:'  \n',empty:'',duplicate:'{"email":"wrong@example.test","email":"super@example.test","password":'+JSON.stringify(password)+'}'}))
    await raw(`json-${name}`,'/v2/auth/login',body,{},'none');
  for(const [name,path,method] of [['create','/v2/members','POST'],['edit','/v2/profiles/1','PUT'],['delete','/v2/profiles/1','DELETE'],['logout','/v2/auth/logout','POST'],['logout-put','/v2/auth/logout','PUT'],['migrate','/v2/auth/session/migrate','POST'],['image','/v2/activities/1/images','POST']]){
    await raw(`malformed-before-auth-${name}`,path,'{',{},'none',method);
    await raw(`malformed-authenticated-${name}`,path,'{',{},'super',method);
  }
  for(const type of ['text/plain','application/octet-stream','application/problem+json'])
    await raw(`unparsed-${type}`,'/v2/auth/login',login,{'Content-Type':type},'none');
  await raw('oversized-json','/v2/auth/login',JSON.stringify({extra:'x'.repeat(1024*1024)}),{},'none');
  await raw('oversized-text','/v2/auth/login','x'.repeat(1024*1024+1),{'Content-Type':'text/plain'},'none');
  await raw('gzip-json','/v2/auth/login',gzipSync(login),{'Content-Encoding':'gzip'},'none');
  await raw('deflate-json','/v2/auth/login',deflateSync(login),{'Content-Encoding':'deflate'},'none');
  await raw('unsupported-encoding','/v2/auth/login',login,{'Content-Encoding':'compress'},'none');
  await raw('form-login','/v2/auth/login',new URLSearchParams({email:'super@example.test',password}).toString(),{'Content-Type':'application/x-www-form-urlencoded'},'none');
  await raw('form-empty','/v2/auth/login','email=&password=',{'Content-Type':'application/x-www-form-urlencoded'},'none');
  await raw('form-duplicate','/v2/auth/login','email=one&email=two&password=wrong',{'Content-Type':'application/x-www-form-urlencoded'},'none');
  await raw('form-member','/v2/members','name=Form+Member&country=&gender=M',{'Content-Type':'application/x-www-form-urlencoded'});
  await raw('form-nested','/v2/profiles/1','badges[]=LMD&badges[]=ALUMNI&extra_data.preferred_name=Form&education_history[0][degree]=bachelor&education_history[0][institution]=Fixture&education_history[0][faculty]=Science&education_history[0][major]=Physics&education_history[0][intake_year]=2020',{'Content-Type':'application/x-www-form-urlencoded'},'super','PUT');
  await raw('query-create','/v2/members?name=Query+Member','{}');
  await raw('query-over-body','/v2/members?name=Ignored','{"name":"Body Member"}');
  await raw('prototype-fields','/v2/members','{"name":"Prototype member","__proto__":{"polluted":true},"constructor":{"prototype":{"polluted":true}}}');
  const form=new FormData();form.append('name','Ignored multipart');
  await h.call('protocol:multipart-unprocessed','POST','/v2/members',undefined,'super',{form});
}
