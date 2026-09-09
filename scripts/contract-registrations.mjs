export async function registrationCases(h){
  const member=await h.call('registration:member','POST','/v2/members',{name:'Registrant fixture',email:'registrant@example.test'});
  await h.call('registration:activity','POST','/v2/activities',{name:'Registration fixture',activity_type:2,badge:'Fixture SSC',additional_config:{custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[{name:'motivation',label:'Motivasi',type:'text'}]}});
  const body={user_id:member.data.profile.id,questionnaire_answer:{motivation:'Learn'}};
  await h.call('registration:create','POST','/v2/activities/1/registrations',body);
  await h.call('registration:duplicate','POST','/v2/activities/1/registrations',body);
  await h.call('registration:show','GET','/v2/activity-registrations/1');
  await h.call('registration:by-user','GET',`/v2/activity-registrations/user/${member.data.user.id}`);
  await h.seed("INSERT INTO activity_registrations(activity_id,guest_data,status,questionnaire_answer,created_at,updated_at) VALUES(1,$1,'TERDAFTAR',$2,'2024-01-01','2024-01-01')",[{name:'Guest fixture',email:'guest@example.test',whatsapp:'081234567890',country:'Indonesia',current_education:{institution:'Guest University',intake_year:2024}},{motivation:['Learn','Share']}]);
  await h.call('registration:list','GET','/v2/activities/1/registrations');
  await h.call('registration:guest-search','GET','/v2/activities/1/registrations?search=Guest&sort_by=name&sort_order=asc');
  await h.call('registration:graduate','PUT','/v2/activity-registrations',{registrations_id:[1,2],status:'LULUS KEGIATAN'});
  await h.call('registration:repeat-graduate','PUT','/v2/activity-registrations',{registrations_id:[1],status:'LULUS KEGIATAN'});
  await h.call('registration:by-email','PUT','/v2/activities/1/registrations/status-by-email',{emails:['registrant@example.test'],status:'DITERIMA'});
  await h.call('registration:missing-email','PUT','/v2/activities/1/registrations/status-by-email',{emails:['missing@example.test'],status:'DITERIMA'});
  await h.call('registration:bulk-status','PUT','/v2/activities/1/registrations',{current_status:'DITERIMA',new_status:'TERDAFTAR'});
  await h.call('registration:statistics','GET','/v2/activities/1/registrations/statistics');
  await h.call('registration:export','GET','/v2/activities/1/registrations-export');
  await h.call('registration:delete','DELETE','/v2/activity-registrations/1');
  await h.call('registration:delete-missing','DELETE','/v2/activity-registrations/1');
  await h.call('registration:missing-show','GET','/v2/activity-registrations/999999');
  await h.call('registration:missing-profile','POST','/v2/activities/1/registrations',{user_id:999999,questionnaire_answer:{}});
  await h.call('registration:invalid','POST','/v2/activities/1/registrations',{});
  await h.call('registration:invalid-status','PUT','/v2/activity-registrations',{registrations_id:['wrong'],status:3});
}
