import assert from 'node:assert/strict';
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

  await h.call('registration:empty-status-ids','PUT','/v2/activity-registrations',{registrations_id:[],status:'DITERIMA'});
  const other=await h.call('registration:other-member','POST','/v2/members',{name:'Unregistered fixture',email:'unregistered@example.test'});
  await h.call('registration:email-without-registration','PUT','/v2/activities/1/registrations/status-by-email',{emails:['unregistered@example.test'],status:'DITERIMA'});
  const memberReg=await h.call('registration:register-again','POST','/v2/activities/1/registrations',body);
  await h.seed("UPDATE profiles SET level=1,badges='[]',updated_at='2020-01-01' WHERE user_id=ANY($1::int[])",[[member.data.user.id,other.data.user.id]]);
  await h.seed("UPDATE activity_registrations SET updated_at='2020-01-01' WHERE activity_id=1");
  await h.call('registration:email-upgrades-selected-users','PUT','/v2/activities/1/registrations/status-by-email',{emails:['registrant@example.test','unregistered@example.test'],status:'LULUS KEGIATAN'});
  const upgraded=await h.seed('SELECT level,badges FROM profiles WHERE user_id=ANY($1::int[]) ORDER BY id',[[member.data.user.id,other.data.user.id]]);
  assert.deepEqual(upgraded,[{level:3,badges:['Fixture SSC']},{level:3,badges:['Fixture SSC']}]);
  assert.ok((await h.seed('SELECT updated_at FROM activity_registrations WHERE activity_id=1')).every(row=>row.updated_at.toISOString()==='2020-01-01T00:00:00.000Z'),'status updates preserve registration timestamps');
  await h.seed("UPDATE profiles SET updated_at='2020-01-01' WHERE user_id=ANY($1::int[])",[[member.data.user.id,other.data.user.id]]);
  await h.call('registration:repeat-preserves-profile-time','PUT','/v2/activities/1/registrations/status-by-email',{emails:['registrant@example.test','unregistered@example.test'],status:'LULUS KEGIATAN'});
  assert.ok((await h.seed('SELECT updated_at FROM profiles WHERE user_id=ANY($1::int[])',[[member.data.user.id,other.data.user.id]])).every(row=>row.updated_at.toISOString()==='2020-01-01T00:00:00.000Z'),'unchanged badge preserves profile timestamps');
  const incompatible=await h.call('registration:type-four-activity','POST','/v2/activities',{name:'Type four fixture',activity_type:4,badge:'Should not be applied',additional_config:{custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]}});
  const incompatibleReg=await h.call('registration:type-four-create','POST',`/v2/activities/${incompatible.data.id}/registrations`,body);
  await h.call('registration:type-four-rolls-back','PUT','/v2/activity-registrations',{registrations_id:[incompatibleReg.data.id,memberReg.data.id],status:'LULUS KEGIATAN'});
  assert.equal((await h.seed('SELECT status FROM activity_registrations WHERE id=$1',[incompatibleReg.data.id]))[0].status,'TERDAFTAR');
  await h.call('registration:first-activity-controls-upgrade','PUT','/v2/activity-registrations',{registrations_id:[memberReg.data.id,incompatibleReg.data.id],status:'LULUS KEGIATAN'});
  const high=await h.call('registration:minimum-level-activity','POST','/v2/activities',{name:'Minimum level fixture',minimum_level:10});
  await h.call('registration:unmatched-level','POST',`/v2/activities/${high.data.id}/registrations`,body);
  const [orphan]=await h.seed("INSERT INTO profiles(name,level,created_at,updated_at) VALUES('Orphan profile',1,'2020-01-01','2020-01-01') RETURNING id");
  await h.call('registration:null-user-duplicate','POST','/v2/activities/1/registrations',{user_id:orphan.id,questionnaire_answer:{}});
  const [emptyActivity]=await h.seed("INSERT INTO activities(name,slug,minimum_level,created_at,updated_at) VALUES('Orphan activity','orphan-activity',0,'2020-01-01','2020-01-01') RETURNING id");
  await h.call('registration:null-user-create','POST',`/v2/activities/${emptyActivity.id}/registrations`,{user_id:orphan.id,questionnaire_answer:{}});
  await h.seed('UPDATE activity_registrations SET status=NULL WHERE activity_id=$1',[emptyActivity.id]);
  const nullStats=await h.call('registration:null-status-statistics','GET',`/v2/activities/${emptyActivity.id}/registrations/statistics`);
  assert.deepEqual(nullStats.data,{total:1,by_status:{null:1}});
  await h.call('registration:fractional-profile','POST','/v2/activities/1/registrations',{user_id:1.5,questionnaire_answer:{}});
  await h.call('registration:wide-profile','POST','/v2/activities/1/registrations',{user_id:4294967297,questionnaire_answer:{}});
  await h.call('registration:fractional-status-id','PUT','/v2/activity-registrations',{registrations_id:[1.5],status:'DITERIMA'});
  await h.call('registration:wide-status-id','PUT','/v2/activity-registrations',{registrations_id:[4294967297],status:'DITERIMA'});
  await h.call('registration:invalid-bulk-name','PUT','/v2/activities/1/registrations',{name:'Ignored name',new_status:'DITERIMA'});

  for(const column of ['created_at','name','email','status','level','university_id','province_id','intake_year','major','whatsapp','unknown']) {
    for(const direction of ['asc','desc']) await h.call(`registration:sort:${column}:${direction}`,'GET',`/v2/activities/1/registrations?sort_by=${column}&sort_order=${direction}&per_page=1`);
  }
  for(const query of ['page=2&per_page=1','page=1&per_page=0','page=invalid&per_page=invalid','page=1&per_page=-1','page=0&per_page=2','page=1.5&per_page=1.5','page=9007199254740991&per_page=9007199254740991','intake_year=invalid','search=fixture&status=LULUS&intake_year=invalid']) await h.call(`registration:list-filter:${query}`,'GET','/v2/activities/1/registrations?'+query);
  await h.seed("UPDATE profiles SET birth_date='2000-01-02',extra_data=$1,education_history=$2,work_history=$3 WHERE id=$4",[{favorite:'reading'},JSON.stringify([{institution:'Synthetic college',intake_year:2020}]),JSON.stringify([{job_title:'Engineer',company:'Synthetic company'}]),member.data.profile.id]);

  const [province]=await h.seed("INSERT INTO provinces(name) VALUES('Export province') RETURNING id");
  const [city]=await h.seed("INSERT INTO cities(name,province_id) VALUES('Export city',$1) RETURNING id",[province.id]);
  const [university]=await h.seed("INSERT INTO universities(name,province_id) VALUES('Export university',$1) RETURNING id",[province.id]);
  await h.seed("UPDATE profiles SET province_id=$1,city_id=$2,origin_province_id=$1,origin_city_id=$2,university_id=$3,major='Engineering',intake_year=2020,country='Indonesia' WHERE id=$4",[province.id,city.id,university.id,member.data.profile.id]);
  await h.seed("UPDATE activity_registrations SET guest_data=guest_data||$1::jsonb WHERE id=2",[{province_id:' '+province.id+' ',city_id:String(city.id),origin_province_id:'0x'+province.id.toString(16),origin_city_id:city.id,university_id:university.id,birth_date:'2001-02-03',intake_year:2022,major:'Biology',education_history:[{degree:'S1',institution:'Guest university',faculty:'Science',major:'Biology',intake_year:2022}]}]);
  const fullExport=await h.call('registration:export-complete-profile-and-guest','GET','/v2/activities/1/registrations-export');
  const exportMember=fullExport.sheets[0].rows.find(row=>row[1]==='Registrant fixture');
  const birthInstant={UTC:'2000-01-02T00:00:00.000Z','Asia/Jakarta':'2000-01-01T17:00:00.000Z','America/Los_Angeles':'2000-01-02T08:00:00.000Z'}[h.timezone];
  assert.ok(birthInstant,'known fixture timezone');
  assert.equal(exportMember[7],birthInstant.slice(0,10));
  assert.equal(exportMember[12],'Export province');
  const exportGuest=fullExport.sheets[0].rows.find(row=>row[1]==='Guest fixture');
  assert.equal(exportGuest[12],'Export province');assert.equal(exportGuest[15],'Export province');
  const schema={fields:[{section_name:'profile_data',fields:[{key:'name',label:'Skip profile duplicate',required:true,type:'text'}]},{section_name:'questions',fields:[{key:'motivation',label:'Custom motivation',required:true,type:'text'},{key:'consent',label:'Consent',required:false,type:'checkbox'}]}]};
  const customForm=await h.call('registration:export-custom-form','POST','/v2/custom-forms',{formName:'Registration export fixture',featureType:'activity_registration',featureId:1,formSchema:schema,isActive:true});
  await h.seed("UPDATE activity_registrations SET questionnaire_answer=questionnaire_answer||'{\"consent\":true}'::jsonb WHERE activity_id=1");
  const customExport=await h.call('registration:export-custom-schema-precedence','GET','/v2/activities/1/registrations-export');
  assert.deepEqual(customExport.sheets[0].rows[0].slice(30),['Custom motivation','Consent']);
  await h.call('registration:export-deactivate-form','PUT',`/v2/custom-forms/${customForm.data.id}/toggle-active`,{});
  await h.call('registration:export-inactive-schema-fallback','GET','/v2/activities/1/registrations-export');
  const profileFields=['name','id','user_id','birth_date','updated_at','created_at','badges','education_history','work_history','extra_data','picture','country'];
  await h.seed("UPDATE activities SET additional_config=jsonb_set(additional_config,'{mandatory_profile_data}',$1) WHERE id=1",[JSON.stringify(profileFields.map(name=>({name,label:name})))]);
  const projected=await h.call('registration:dynamic-profile-fields','GET','/v2/activities/1/registrations?sort_by=created_at&sort_order=asc');
  assert.ok(projected.data.data.some(row=>row.name===null&&row.id===null),'profile fields override guest name and registration id');
  assert.equal(projected.data.data.find(row=>row.name==='Registrant fixture').birth_date,birthInstant);
  await h.seed("UPDATE activities SET additional_config=jsonb_set(additional_config,'{mandatory_profile_data}','[{\"name\":\"does_not_exist\"}]') WHERE id=1");
  await h.call('registration:unknown-profile-column','GET','/v2/activities/1/registrations');
  for(const [label,config] of [['missing',{}],['null-fields',{mandatory_profile_data:null}],['object-fields',{mandatory_profile_data:{}}],['null-config',null]]) {
    await h.seed('UPDATE activities SET additional_config=$1 WHERE id=1',[JSON.stringify(config)]);
    await h.call(`registration:invalid-profile-config:${label}`,'GET','/v2/activities/1/registrations');
  }
  for(const identifier of ['invalid','1.5','4294967297']) {
    await h.call(`registration:invalid-path-show:${identifier}`,'GET',`/v2/activity-registrations/${identifier}`);
    await h.call(`registration:invalid-path-create:${identifier}`,'POST',`/v2/activities/${identifier}/registrations`,body);
    await h.call(`registration:invalid-path-statistics:${identifier}`,'GET',`/v2/activities/${identifier}/registrations/statistics`);
  }
}
