import assert from 'node:assert/strict';

export async function certificateCases(h){
  const templateData={backgroundUrl:null,canvasWidth:800,canvasHeight:566,elements:[{id:'participant',type:'variable-text',variable:'{{name}}',x:40,y:100,width:720,height:100}]};
  await h.call('certificate:template','POST','/v2/certificate-templates',{name:'Issuance fixture',templateData});
  await h.call('certificate:publish','POST','/v2/certificate-templates/1/publish',{expectedVersion:1});
  await h.call('certificate:activity','POST','/v2/activities',{name:'Snapshot activity',activity_start:'2026-02-28',certificate_template_id:1});
  await h.call('certificate:member','POST','/v2/members',{name:'Member recipient',email:'recipient@example.test',gender:'F'});
  await h.seed("INSERT INTO activity_registrations(activity_id,user_id,status,created_at,updated_at) VALUES(1,1,'LULUS KEGIATAN','2024-01-01','2024-01-01')");
  await h.seed("INSERT INTO activity_registrations(activity_id,status,guest_data,created_at,updated_at) SELECT 1,CASE WHEN n=4 THEN 'TERDAFTAR' ELSE 'LULUS KEGIATAN' END,jsonb_build_object('name','Guest '||n,'email','guest-'||n||'@example.test','gender','M'),'2024-01-01'::timestamptz+n*interval '1 day','2024-01-01'::timestamptz FROM generate_series(2,4) n");
  await h.call('certificate:preview-single','POST','/v2/certificates/generate-single',{registration_id:1});
  await h.call('certificate:preview-all','POST','/v2/certificates/generate',{activity_id:1});
  await h.call('certificate:ineligible','POST','/v2/certificates/generate-single',{registration_id:4});
  await h.call('certificate:missing-registration','POST','/v2/certificates/generate-single',{registration_id:999999});
  await h.call('certificate:recipients','GET','/v2/certificates/activities/1/recipients?sort_order=asc&per_page=2');
  await h.call('certificate:prepare','POST','/v2/certificates/prepare-issuance',{activity_id:1,registration_ids:[1,2,4,999999]});
  const issued=await h.call('certificate:issue-single','POST','/v2/certificates/issue-single',{registration_id:1});
  assert.equal(issued.data.participant.activity_date,'28 Februari 2026');
  const code=issued.data.certificate.certificate_code,id=issued.data.certificate.id;
  await h.call('certificate:repeat','POST','/v2/certificates/issue-single',{registration_id:1});
  await h.call('certificate:show','GET',`/v2/certificates/${id}`);
  await h.call('certificate:code','GET',`/v2/certificates/code/${code.toLowerCase()}`);
  await h.call('certificate:verify','GET',`/v2/certificates/verify/${code}`);
  await h.call('certificate:edit-activity','PUT','/v2/activities/1',{name:'Edited after issuance',activity_start:'2027-01-01'});
  await h.call('certificate:edit-member','PUT','/v2/profiles/1',{name:'Edited recipient'});
  await h.call('certificate:archive','POST','/v2/certificate-templates/1/archive',{expectedVersion:2});
  const snapshot=await h.call('certificate:immutable','GET',`/v2/certificates/${id}`);
  assert.deepEqual(snapshot.data,issued.data,'persisted snapshots remain immutable');
  await h.call('certificate:revoke','POST',`/v2/certificates/${id}/revoke`,{reason:'Synthetic revocation'});
  await h.call('certificate:repeat-revoke','POST',`/v2/certificates/${id}/revoke`,{reason:'Repeat'});
  await h.call('certificate:verify-revoked','GET',`/v2/certificates/verify/${code}`);
  await h.call('certificate:protect-registration','DELETE','/v2/activity-registrations/1');
  await h.call('certificate:republish','POST','/v2/certificate-templates/1/publish',{expectedVersion:3});
  await h.call('certificate:bulk-pause','POST','/v2/certificates/issue-bulk',{registration_ids:[2,3],expected:{activity_id:1,template_id:1,template_version:2}});
  const expected={activity_id:1,template_id:1,template_version:4};
  await h.call('certificate:bulk-resume','POST','/v2/certificates/issue-bulk',{registration_ids:[1,2,3,4,999999],expected,response_mode:'compact'});
  await h.call('certificate:repeat-bulk-full','POST','/v2/certificates/issue-bulk',{registration_ids:[1,2,3,4],expected});
  await h.call('certificate:prepare-issued','POST','/v2/certificates/prepare-issuance',{activity_id:1});
  await h.call('certificate:recipient-states','GET','/v2/certificates/activities/1/recipients?state=issued_active&search=Guest&per_page=1');
  await h.call('certificate:list','GET','/v2/certificates?activity_id=1&per_page=1');
  await h.call('certificate:lookup','POST','/v2/certificates/lookup',{activity_id:1,registration_ids:[1,2,1,999999]});
  for(const count of [1,20,21,50,100,200]){
    const query=new URLSearchParams({activity_id:'1',per_page:'100'});
    for(let n=0;n<count;n++)query.append('registration_ids[]',String(1+n%3));
    await h.call('certificate:query-array:'+count,'GET','/v2/certificates?'+query);
  }
  for(const query of ['registration_ids[0]=1&registration_ids[20]=2','registration_ids[]=invalid','registration_ids[unexpected]=1','page=invalid','per_page=101'])await h.call('certificate:invalid-query:'+query,'GET','/v2/certificates?'+query);
  for(const count of [100,101])await h.call('certificate:lookup-boundary:'+count,'POST','/v2/certificates/lookup',{registration_ids:Array(count).fill(1)});
  await h.call('certificate:invalid-bulk','POST','/v2/certificates/issue-bulk',{registration_ids:[]});
  await h.call('certificate:invalid-revoke','POST','/v2/certificates/2/revoke',{});
  await h.call('certificate:missing-code','GET','/v2/certificates/verify/MISSING');
  await h.call('certificate:invalid-id','GET','/v2/certificates/0');
  await h.call('certificate:missing','GET','/v2/certificates/999999');
  await h.call('certificate:missing-recipients','GET','/v2/certificates/activities/999999/recipients');
  for(const value of [2147483648,4294967297,9007199254740991,1.5]){
    await h.call(`certificate:boundary-show:${value}`,'GET',`/v2/certificates/${value}`);
    await h.call(`certificate:boundary-activity:${value}`,'GET',`/v2/certificates?activity_id=${value}`);
    await h.call(`certificate:boundary-lookup:${value}`,'POST','/v2/certificates/lookup',{registration_ids:[1,value]});
    await h.call(`certificate:boundary-preview:${value}`,'POST','/v2/certificates/generate-single',{registration_id:value});
    await h.call(`certificate:boundary-prepare:${value}`,'POST','/v2/certificates/prepare-issuance',{activity_id:value});
    await h.call(`certificate:boundary-bulk:${value}`,'POST','/v2/certificates/issue-bulk',{registration_ids:[1,value]});
  }
  await certificateBoundaryCases(h);
}

async function certificateBoundaryCases(h){
  const expected={activity_id:1,template_id:1,template_version:4};
  for(const field of ['activity_id','template_id','template_version'])for(const value of [2147483648,1e30])await h.call(`certificate:expected-wide:${field}:${value}`,'POST','/v2/certificates/issue-bulk',{registration_ids:[1,999999],expected:{...expected,[field]:value}});
  for(const value of [2147483648,1e30]){
    await h.call('certificate:huge-full:'+value,'POST','/v2/certificates/issue-bulk',{registration_ids:[1,value,value],response_mode:'full'});
    await h.call('certificate:huge-compact:'+value,'POST','/v2/certificates/issue-bulk',{registration_ids:[1,value],response_mode:'compact'});
    await h.call('certificate:prepare-wide-selection:'+value,'POST','/v2/certificates/prepare-issuance',{activity_id:1,registration_ids:[1,value]});
    await h.call('certificate:prepare-missing-precedes-wide-selection:'+value,'POST','/v2/certificates/prepare-issuance',{activity_id:999999,registration_ids:[value]});
    await h.call('certificate:recipient-wide-precedes-invalid:'+value,'GET',`/v2/certificates/activities/${value}/recipients?per_page=invalid`);
    await h.call('certificate:wide-revoke-invalid-body:'+value,'POST',`/v2/certificates/${value}/revoke`,{});
  }
  for(const query of ['page=2147483648','page=1e30','page=1e300','activity_id=0x1','registration_ids[]=2147483648'])await h.call('certificate:list-numeric:'+query,'GET','/v2/certificates?'+query);
  for(const query of ['page=2147483648','page=1e30','page=1e300','registration_ids[]=2147483648','registration_ids[0]=1&registration_ids[1]=1'])await h.call('certificate:recipient-numeric:'+query,'GET','/v2/certificates/activities/1/recipients?'+query);
  const [guest]=await h.seed("INSERT INTO activity_registrations(activity_id,status,guest_data,created_at,updated_at) VALUES(1,'LULUS KEGIATAN','{}','2024-02-01','2024-02-01') RETURNING id");
  for(const value of [1,'1','01',0,-1,1.5,2147483648,1e30,'2147483648','bad','1e0',true,null]){
    await h.seed('UPDATE activity_registrations SET guest_data=$1::jsonb WHERE id=$2',[JSON.stringify({name:10,email:false,gender:' F ',university_id:value}),guest.id]);
    await h.call('certificate:guest-university:'+JSON.stringify(value),'POST','/v2/certificates/generate-single',{registration_id:guest.id});
  }
  await h.seed('UPDATE activity_registrations SET guest_data=$1::jsonb WHERE id=$2',[JSON.stringify({name:' Typed guest ',email:' typed@example.test ',university:' Direct university ',university_id:2147483648}),guest.id]);
  await h.call('certificate:guest-direct-university','POST','/v2/certificates/generate-single',{registration_id:guest.id});
  for(const value of [1,'1',0,'0',-1,1.5,2147483648,true,[],[1],{}]){
    await h.seed('UPDATE activities SET certificate_template_id=null,additional_config=jsonb_build_object(\'certificate_template_id\',$1::jsonb) WHERE id=1',[JSON.stringify(value)]);
    await h.call('certificate:legacy-template:'+JSON.stringify(value),'POST','/v2/certificates/generate-single',{registration_id:guest.id});
    await h.call('certificate:legacy-template-prepare:'+JSON.stringify(value),'POST','/v2/certificates/prepare-issuance',{activity_id:1,registration_ids:[guest.id]});
    await h.call('certificate:legacy-template-expectation:'+JSON.stringify(value),'POST','/v2/certificates/issue-bulk',{registration_ids:[1],expected});
  }
  await h.seed("UPDATE activities SET certificate_template_id=1,additional_config='{}' WHERE id=1");
  await h.seed('UPDATE activity_registrations SET created_at=null WHERE id=$1',[guest.id]);
  await h.call('certificate:recipient-null-date','GET','/v2/certificates/activities/1/recipients?sort_order=asc');
  await h.call('certificate:missing-generate','POST','/v2/certificates/generate',{activity_id:999999});
  await h.call('certificate:empty-status','POST','/v2/certificates/generate',{activity_id:1,status:'nonexistent'});
  await h.seed("UPDATE issued_certificates SET activity_snapshot='null'::jsonb,participant_snapshot=participant_snapshot-'gender' WHERE id=1");
  await h.call('certificate:legacy-snapshot-fallback','GET','/v2/certificates/1');
  await h.seed("UPDATE issued_certificates SET participant_snapshot=participant_snapshot-'email'-'name' WHERE id=1");
  await h.call('certificate:legacy-list-omitted-fields','GET','/v2/certificates');
}
