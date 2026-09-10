export async function queryEdgeCases(h){
  await h.call('query:province','POST','/v2/provinces',{name:'Query province'});
  await h.call('query:university','POST','/v2/universities',{name:'Query university',provinceId:1});
  await h.call('query:member','POST','/v2/members',{name:'Query member',email:'query@example.test'});
  await h.call('query:activity','POST','/v2/activities',{name:'Query activity'});
  await h.call('query:club','POST','/v2/clubs',{name:'Query club'});
  await h.call('query:form','POST','/v2/custom-forms',{formName:'Query form'});
  await h.call('query:template','POST','/v2/certificate-templates',{name:'Query template'});
  const paths=['admin-users','access-requests','tickets/review','universities','profiles','activities','activities/1/registrations','ruang-curhat','achievements','leaderboards/monthly','leaderboards/lifetime','clubs','clubs/1/members','clubs/1/registrations','custom-forms','custom-forms/unattached','certificate-templates','certificates','certificates/activities/1/recipients'];
  for(const path of paths)for(const value of ['invalid','0','-1','1.5','9007199254740991','0x2','%202%20','%EF%BB%BF2','10000000000000000000']){
    await h.call(`query:invalid-pagination:${path}:${value}`,'GET',`/v2/${path}?page=${value}&per_page=${value}&limit=${value}`);
  }
  for(const [path,body] of [
    ['certificates/prepare-issuance',{activity_id:999999}],
    ['certificates/issue-single',{registration_id:999999}],
    ['certificates/issue-bulk',{registration_ids:[999999]}],
    ['certificates/generate',{activity_id:999999}],
    ['certificates/generate-single',{registration_id:999999}],
    ['certificates/lookup',{registration_ids:[999999]}],
  ])await h.call(`query:missing-resource:${path}`,'POST','/v2/'+path,body);
  await h.seed("UPDATE profiles SET education_history='[{\"institution\":\"Fixture College\"}]',badges='[\"LMD\"]'");
  await h.seed("UPDATE public_users SET member_id='QUERY-FIXTURE' WHERE id=1");
  for(const query of ['search=Query','member_id=QUERY-FIXTURE','education_institution=Fixture','badge=LMD','search=Query&member_id=QUERY-FIXTURE&education_institution=Fixture&badge=LMD']){
    await h.call('query:invalid-profile-pagination:'+query,'GET','/v2/profiles?per_page=-1&'+query);
  }

}
