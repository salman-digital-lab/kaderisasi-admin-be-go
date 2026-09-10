import assert from 'node:assert/strict';
export async function clubMemberCases(h){
  await h.call('membership:club','POST','/v2/clubs',{name:'Membership fixture'});
  await h.call('membership:member','POST','/v2/members',{name:'Club member',email:'club-member@example.test'});
  await h.call('membership:member-two','POST','/v2/members',{name:'Second member',email:'club-second@example.test'});
  await h.call('membership:invalid','POST','/v2/clubs/1/registrations',{});
  await h.call('membership:create','POST','/v2/clubs/1/registrations',{member_id:1,additional_data:{motivation:'Learn',choice:['A','B'],consent:true}});
  await h.call('membership:duplicate','POST','/v2/clubs/1/registrations',{member_id:1});
  await h.call('membership:create-two','POST','/v2/clubs/1/registrations',{member_id:2});
  await h.seed("UPDATE club_registrations SET created_at='2024-01-01 00:00:00+00'::timestamptz+id*interval '1 day'");
  await h.call('membership:list','GET','/v2/clubs/1/registrations?status=PENDING&sort_order=asc&limit=1&page=2');
  await h.call('membership:show','GET','/v2/club-registrations/1');
  await h.call('role:pending-denied','POST','/v2/clubs/1/member-roles',{club_registration_id:1,role_name:'Leader'});
  await h.call('membership:approve','PUT','/v2/club-registrations/1',{status:'APPROVED'});
  await h.call('role:invalid','POST','/v2/clubs/1/member-roles',{club_registration_id:1,role_name:'x'});
  await h.call('role:create','POST','/v2/clubs/1/member-roles',{club_registration_id:1,role_name:' Leader ',is_primary:true,start_date:'2026-01-01'});
  await h.call('role:replace-primary','POST','/v2/clubs/1/member-roles',{club_registration_id:1,role_name:'Secretary',is_primary:true,sort_order:2});
  await h.call('role:update','PUT','/v2/club-registrations/member-roles/1',{role_name:'Chair',is_primary:true,sort_order:-1,end_date:'2026-12-31'});
  await h.call('role:list','GET','/v2/clubs/1/member-roles');
  await h.call('role:suggestions','GET','/v2/clubs/1/member-role-suggestions');
  await h.call('membership:approved-search','GET','/v2/clubs/1/members?search=Club&sort_order=asc');
  await h.call('membership:show-roles','GET','/v2/club-registrations/1');
  await h.call('membership:bulk-duplicate','PUT','/v2/club-registrations/bulk-update',{registrations:[{id:1,status:'REJECTED'},{id:1,status:'APPROVED'}]});
  await h.call('membership:bulk-missing','PUT','/v2/club-registrations/bulk-update',{registrations:[{id:1,status:'REJECTED'},{id:999999,status:'APPROVED'}]});
  await h.call('membership:bulk-empty','PUT','/v2/club-registrations/bulk-update',{registrations:[]});
  await h.call('membership:bulk-update','PUT','/v2/club-registrations/bulk-update',{registrations:[{id:2,status:'APPROVED',additional_data:{motivation:{value:'Help',label:'Helping'}}},{id:1,status:'REJECTED'}]});
  await h.call('membership:form','POST','/v2/custom-forms',{formName:'Member questions',isActive:true,featureType:'club_registration',featureId:1,formSchema:{fields:[{section_name:'Info',fields:[{key:'motivation',label:'Motivasi',required:true,type:'text'}]}]}});
  await h.seed("UPDATE club_registrations SET created_at=CASE WHEN id=1 THEN NULL ELSE '2026-02-28T17:00:00Z'::timestamptz END,additional_data=additional_data||'{\"experience\":0,\"consent\":false}'::jsonb");
  const exported=await h.call('membership:export','GET','/v2/clubs/1/registrations/export');
  const sheet=exported.sheets[0],headers=sheet.rows[0];
  assert.ok(sheet.rows.slice(1).some(row=>row[11]==='2026-03-01 00:00:00'),'registration dates use Jakarta independently of process TZ');
  assert.ok(sheet.rows.slice(1).some(row=>row[11]===''),'missing legacy registration dates export as empty cells');
  assert.equal(sheet.rows[1][headers.indexOf('Experience')],0);
  assert.equal(sheet.rows[1][headers.indexOf('Consent')],'Tidak');
  await h.call('membership:approved-list','GET','/v2/clubs/1/members');
  await h.call('role:approved-list','GET','/v2/clubs/1/member-roles');
  await h.call('role:delete','DELETE','/v2/club-registrations/member-roles/2');
  await h.call('role:delete-missing','DELETE','/v2/club-registrations/member-roles/2');
  await h.call('membership:delete','DELETE','/v2/club-registrations/1');
  await h.call('membership:missing','GET','/v2/club-registrations/999999');
  await h.call('membership:missing-member','POST','/v2/clubs/1/registrations',{member_id:999999});
  await h.call('membership:missing-club','GET','/v2/clubs/999999/members');
  await memberBoundaryCases(h);
}

async function memberBoundaryCases(h){
  const created=await h.call('membership:boundary-member','POST','/v2/members',{name:'Boundary club member',email:'boundary-club@example.test'});
  const userID=created.data.user.id;
  const registration=await h.call('membership:boundary-registration','POST','/v2/clubs/1/registrations',{member_id:userID,additional_data:{zero:0,boolean:false,empty:[],nested:{a:null}}});
  const id=registration.data.id,path=`/v2/club-registrations/${id}`;
  await h.seed("UPDATE club_registrations SET created_at='2026-03-02T17:00:00Z',updated_at='2026-01-01T00:00:00Z' WHERE id=$1",[id]);
  const unchanged=await h.call('membership:unchanged','PUT',path,{status:'PENDING'});
  assert.equal(new Date(unchanged.data.updated_at).toISOString(),'2026-01-01T00:00:00.000Z');
  await h.call('membership:replace-answers','PUT',path,{status:'APPROVED',additional_data:{}});
  const role=await h.call('role:boundary-create','POST','/v2/clubs/1/member-roles',{club_registration_id:id,role_name:'Boundary chair',is_primary:true});
  const roleID=role.data.id,rolePath=`/v2/club-registrations/member-roles/${roleID}`;
  await h.seed("UPDATE club_member_roles SET updated_at='2026-01-01T00:00:00Z' WHERE id=$1",[roleID]);
  const same=await h.call('role:unchanged','PUT',rolePath,{});
  assert.equal(new Date(same.data.updated_at).toISOString(),'2026-01-01T00:00:00.000Z');
  for(const identifier of ['bad','1.5','2147483648','%31']){
    for(const [method,route,body] of [['GET',`/v2/clubs/${identifier}/registrations`],['GET',`/v2/clubs/${identifier}/members`],['GET',`/v2/clubs/${identifier}/registrations/export`],['POST',`/v2/clubs/${identifier}/registrations`,{member_id:userID}],['GET',`/v2/club-registrations/${identifier}`],['PUT',`/v2/club-registrations/${identifier}`,{status:'APPROVED'}],['DELETE',`/v2/club-registrations/${identifier}`],['GET',`/v2/clubs/${identifier}/member-roles`],['GET',`/v2/clubs/${identifier}/member-role-suggestions`],['POST',`/v2/clubs/${identifier}/member-roles`,{club_registration_id:id,role_name:'Test role'}],['PUT',`/v2/club-registrations/member-roles/${identifier}`,{}],['DELETE',`/v2/club-registrations/member-roles/${identifier}`]])await h.call(`membership:id:${method}:${route}`,method,route,body);
  }
  for(const value of [0.5,2147483648]){
    await h.call('membership:member-number:'+value,'POST','/v2/clubs/1/registrations',{member_id:value});
    await h.call('membership:batch-number:'+value,'PUT','/v2/club-registrations/bulk-update',{registrations:[{id,status:'REJECTED'},{id:value,status:'APPROVED'}]});
    await h.call('role:registration-number:'+value,'POST','/v2/clubs/1/member-roles',{club_registration_id:value,role_name:'Wide role'});
    await h.call('role:sort-number:'+value,'PUT',rolePath,{sort_order:value});
    await h.call('role:failed-create-clears-primary:'+value,'POST','/v2/clubs/1/member-roles',{club_registration_id:id,role_name:'Invalid sort',is_primary:true,sort_order:value});
  }
  for(const suffix of ['registrations','members'])for(const query of ['limit=-1','limit=0','page=-1','page=1.5&limit=2','search=Boundary&limit=-1','status=APPROVED&limit=-1'])await h.call(`membership:pagination:${suffix}:${query}`,'GET',`/v2/clubs/1/${suffix}?${query}`);
  await h.seed('UPDATE club_registrations SET additional_data=null WHERE id=$1',[id]);
  await h.call('membership:null-answers','PUT',path,{status:'APPROVED'});
  await h.call('membership:empty-batch-answers','PUT','/v2/club-registrations/bulk-update',{registrations:[{id,status:'APPROVED',additional_data:{}}]});
  await h.call('membership:boundary-export','GET','/v2/clubs/1/registrations/export');
}
