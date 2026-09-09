export async function referenceCases(h) {
  await h.call('health','GET','/health');
  await h.call('countries','GET','/v2/countries');
  for (const [resource, body, updated] of [
    ['provinces',{name:'Fixture Province'},{name:'Updated Province'}],
    ['cities',{name:'Fixture City',province_id:1},{name:'Updated City',province_id:1}],
    ['universities',{name:'Fixture University',provinceId:1},{name:'Updated University',provinceId:1}],
  ]) {
    await h.call(`${resource}:empty`,'GET',`/v2/${resource}`);
    await h.call(`${resource}:create`,'POST',`/v2/${resource}`,body);
    await h.call(`${resource}:show`,'GET',`/v2/${resource}/1`);
    await h.call(`${resource}:slash`,'GET',`/v2/${resource}/1/`);
    await h.call(`${resource}:update`,'PUT',`/v2/${resource}/1`,updated);
    await h.call(`${resource}:list`,'GET',`/v2/${resource}`);
    await h.call(`${resource}:missing`,'GET',`/v2/${resource}/999999`);
    await h.call(`${resource}:update-missing`,'PUT',`/v2/${resource}/999999`,body);
    for (const invalid of [{},{name:null},{name:''},{name:123}]) await h.call(`${resource}:invalid:${JSON.stringify(invalid)}`,'POST',`/v2/${resource}`,invalid);
  }
  await h.call('cities:province','GET','/v2/provinces/1/cities');
  await h.call('universities:search','GET','/v2/universities?search=updated&page=1&per_page=1');
  for(const path of ['/v2/rbac/permissions','/v2/rbac/requestable-targets','/v2/rbac/roles','/v2/rbac/roles/super_admin','/v2/rbac/roles/missing','/v2/dashboard/stats','/v2/dashboard/profiles','/v2/dashboard/gender'])await h.call(path,'GET',path);
  for(const resource of ['universities','cities','provinces']) {
    await h.call(`${resource}:delete`,'DELETE',`/v2/${resource}/1`);
    await h.call(`${resource}:delete-missing`,'DELETE',`/v2/${resource}/999999`);
  }
}

export async function authorizationCases(h,routes) {
  for (const route of routes) {
    const path=route.path.replace(/:code/g,'super_admin').replace(/:[^/]+/g,'999999');
    if(route.auth)await h.call(`unauthenticated:${route.method}:${route.path}`,route.method,path,undefined,'none');
    if(route.permission)await h.call(`forbidden:${route.method}:${route.path}`,route.method,path,undefined,'requester');
  }
}

export async function authCases(h,password){
  for(const body of [undefined,{}, {email:null,password:null}, {email:123,password:123}, {email:'bad',password:'x'}, {email:'super@example.test'}, {email:'nobody@example.test',password}, {email:'inactive@example.test',password}, {email:'super@example.test',password:'wrong'}])await h.call(`login:${JSON.stringify(body)}`,'POST','/v2/auth/login',body,'none');
  for(const body of [undefined,{}, {credential:null}, {credential:3}, {credential:'small'}, {credential:'invalid-signed-credential-with-no-signature'}])await h.call(`google:${JSON.stringify(body)}`,'POST','/v2/auth/google',body,'none');
  await h.call('refresh:missing','POST','/v2/auth/refresh',{},'none');
  const login=await h.call('login:success','POST','/v2/auth/login',{email:'SUPER@example.test',password},'none');
  const first=h.lastCookie;
  await h.call('session:me','GET','/v2/auth/me',undefined,'none',{headers:{Authorization:`Bearer ${login.data.access_token}`}});
  await h.call('refresh:rotation','POST','/v2/auth/refresh',{},'none',{cookie:first});
  const second=h.lastCookie;
  await h.call('refresh:reuse','POST','/v2/auth/refresh',{},'none',{cookie:first});
  await h.call('refresh:family-revoked','POST','/v2/auth/refresh',{},'none',{cookie:second});
  await h.call('session:migrate','POST','/v2/auth/session/migrate',{});
  const third=h.lastCookie;
  await h.call('refresh:tampered','POST','/v2/auth/refresh',{},'none',{cookie:third+'tampered'});
  await h.call('logout:post','POST','/v2/auth/logout',{},'none',{cookie:third});
  await h.call('refresh:logged-out','POST','/v2/auth/refresh',{},'none',{cookie:third});
  await h.call('logout:put','PUT','/v2/auth/logout',{},'none');
  await h.call('session:invalid-bearer','GET','/v2/auth/me',undefined,'none',{headers:{Authorization:'Bearer malformed'}});
  await h.call('origin:untrusted','POST','/v2/auth/refresh',{},'none',{headers:{Origin:'https://untrusted.example.test'}});
}
