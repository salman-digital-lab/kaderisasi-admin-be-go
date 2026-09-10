import assert from 'node:assert/strict';

export async function clubCases(h){
  await h.call('club:invalid','POST','/v2/clubs',{});
  await h.call('club:create','POST','/v2/clubs',{name:'Fixture club',is_show:true,is_registration_open:true,logo:'ignored'});
  await h.call('club:open-without-form','PUT','/v2/clubs/1',{is_registration_open:true});
  await h.call('club:publish','PUT','/v2/clubs/1',{is_show:true,club_type:'CLUB_BAHASA',start_period:'2026-01-01',end_period:null});
  await h.call('club:list','GET','/v2/clubs?club_type=CLUB_BAHASA&visibility=published&registration=closed');
  await h.call('club:information','PUT','/v2/clubs/1/registration-info',{registration_info:' Fixture instructions '});
  const schema={fields:[{section_name:'questions',fields:[{key:'motivation',label:'Motivasi',required:true,type:'text'}]}]};
  await h.call('form:create','POST','/v2/custom-forms',{formName:'Fixture form',formSchema:schema,isActive:true});
  for(const path of ['/v2/custom-forms?search=Fixture&is_active=true','/v2/custom-forms/unattached','/v2/custom-forms/1','/v2/custom-forms/available-clubs','/v2/custom-forms/available-activities'])await h.call(path,'GET',path);
  await h.call('form:attach-club','PUT','/v2/custom-forms/1/attach-club',{clubId:1});
  await h.call('form:duplicate-attachment','PUT','/v2/custom-forms/1/attach-club',{clubId:1});
  await h.call('form:by-feature','GET','/v2/custom-forms/by-feature?feature_type=club_registration&feature_id=1');
  await h.call('form:feature-missing','GET','/v2/custom-forms/by-feature');
  await h.call('form:attachment-conflict','POST','/v2/custom-forms',{formName:'Conflict',featureType:'club_registration',featureId:1});
  await h.call('club:expired-open','PUT','/v2/clubs/1',{registration_end_date:'2000-01-01',is_registration_open:true});
  await h.call('club:open','PUT','/v2/clubs/1',{registration_end_date:null,is_registration_open:true});
  await h.call('form:rename','PUT','/v2/custom-forms/1',{formName:'Renamed form',formSchema:schema});
  await h.call('form:open-structure-change','PUT','/v2/custom-forms/1',{formSchema:{fields:[]}});
  await h.call('form:open-toggle','PUT','/v2/custom-forms/1/toggle-active',{});
  await h.call('form:open-delete','DELETE','/v2/custom-forms/1');
  await h.call('form:open-detach','PUT','/v2/custom-forms/1/detach-club',{});
  await h.call('club:show-form','GET','/v2/clubs/1');
  const logo=await h.upload('club:logo','/v2/clubs/1/logo');
  await h.inspectObject(logo.data.logo,{width:640,height:480});
  await h.upload('club:replace-logo','/v2/clubs/1/logo');
  const media=await h.upload('club:image','/v2/clubs/1/media/image',{media_type:'image'});
  const image=media.data.media.items[0].media_url;
  await h.call('club:youtube','POST','/v2/clubs/1/media/youtube',{media_url:'https://youtu.be/abc_DEF-123',media_type:'video',video_source:'youtube'});
  await h.call('club:youtube-duplicate','POST','/v2/clubs/1/media/youtube',{media_url:'https://www.youtube.com/watch?v=abc_DEF-123',media_type:'video',video_source:'youtube'});
  await h.call('club:youtube-invalid','POST','/v2/clubs/1/media/youtube',{media_url:'https://invalid.example/watch?v=abc_DEF-123',media_type:'video',video_source:'youtube'});
  await h.call('club:delete-image','PUT','/v2/clubs/1/delete-media',{media_url:image});
  await h.call('club:delete-missing-image','PUT','/v2/clubs/1/delete-media',{media_url:image});
  await h.call('club:close','PUT','/v2/clubs/1',{is_registration_open:false});
  await h.call('form:toggle-off','PUT','/v2/custom-forms/1/toggle-active',{});
  await h.call('form:toggle-on','PUT','/v2/custom-forms/1/toggle-active',{});
  await h.call('form:detach-club','PUT','/v2/custom-forms/1/detach-club',{});
  await h.call('form:activity','POST','/v2/activities',{name:'Form activity'});
  await h.call('form:attach-activity','PUT','/v2/custom-forms/1/attach-activity',{activityId:1});
  await h.call('form:duplicate-activity','PUT','/v2/custom-forms/1/attach-activity',{activityId:1});
  await h.call('form:detach-activity','PUT','/v2/custom-forms/1/detach-activity',{});
  await h.call('form:delete','DELETE','/v2/custom-forms/1');
  await h.call('form:missing','GET','/v2/custom-forms/1');
  await h.call('club:missing','GET','/v2/clubs/999999');
  await h.upload('club:missing-media-type','/v2/clubs/1/media/image');
  await clubBoundaryCases(h);
}

async function clubBoundaryCases(h){
  const media={items:[{media_url:'https://www.youtube.com/embed/fixture12',media_type:'video',video_source:'youtube'}]};
  const created=await h.call('club:create-complete','POST','/v2/clubs',{name:'Complete club',club_type:'CLUB_KEPROFESIAN',description:' Description ',short_description:' Summary ',media,start_period:'2026-03-01 00:30:00',end_period:'2027-02-28',registration_end_date:'2027-02-01',is_show:true,is_registration_open:true});
  assert.ok(created.data,JSON.stringify(created));
  const id=created.data.id,path=`/v2/clubs/${id}`;
  assert.equal(created.data.is_show,false);assert.equal(created.data.is_registration_open,false);
  assert.equal(Object.hasOwn(created.data,'logo'),false);assert.equal(Object.hasOwn(created.data,'registration_info'),false);
  await h.seed("UPDATE clubs SET updated_at='2026-01-01T00:00:00Z' WHERE id=$1",[id]);
  const unchanged=await h.call('club:empty-update-preserves-timestamp','PUT',path,{});
  assert.equal(new Date(unchanged.data.updated_at).toISOString(),'2026-01-01T00:00:00.000Z');
  const same=await h.call('club:same-values-preserve-timestamp','PUT',path,{name:'Complete club',club_type:'CLUB_KEPROFESIAN',media,is_show:false,logo:'ignored'});
  assert.equal(new Date(same.data.updated_at).toISOString(),'2026-01-01T00:00:00.000Z');
  await h.call('club:dates-cleared','PUT',path,{start_period:null,end_period:null,registration_end_date:null,description:'',short_description:''});
  await h.call('club:date-update','PUT',path,{start_period:'2026-03-01 00:30:00',end_period:'2027-02-28',registration_end_date:'2027-02-01'});
  for(const date of ['2026-03-01T00:30:00+07:00','2026-03-01 00:30:00.000','2026-02-29','2026-3-1'])await h.call('club:invalid-date-format:'+date,'PUT',path,{start_period:date});
  await h.call('club:show-complete','GET',path);
  const duplicate={items:[{media_url:'same',media_type:'image'},{media_url:' same ',media_type:'video'}]};
  await h.call('club:duplicate-create','POST','/v2/clubs',{name:'Duplicate media',media:duplicate});
  await h.call('club:duplicate-update','PUT',path,{media:duplicate});
  await h.call('club:duplicate-before-missing','PUT','/v2/clubs/999999',{media:duplicate});
  for(const type of ['UNIT','CLUB_KEPROFESIAN','CLUB_BAHASA','AVISMAN_REGIONAL','invalid'])await h.call('club:filter:'+type,'GET',`/v2/clubs?club_type=${type}`);
  for(const query of ['visibility=draft&registration=closed','search=Complete','search=not-existing','visibility=invalid&registration=invalid','club_type=invalid&per_page=-1'])await h.call('club:filtered:'+query,'GET','/v2/clubs?'+query);
  await h.seed("INSERT INTO custom_forms(form_name,feature_type,feature_id,is_active,form_schema,created_at,updated_at) VALUES ('Older active','club_registration',$1,true,'{}','2026-01-01','2026-01-01'),('Newer inactive','club_registration',$1,false,'{\"fields\":[]}','2026-01-02','2026-01-02')",[id]);
  const detail=await h.call('club:latest-form-includes-inactive','GET',path);
  assert.equal(detail.data.attachedCustomForm.form_name,'Newer inactive');assert.equal(detail.data.attachedCustomForm.is_active,false);
  await h.call('club:open-with-older-active-form','PUT',path,{is_registration_open:true});
  await h.call('club:close-with-expired-date','PUT',path,{is_registration_open:false,registration_end_date:'2000-01-01'});
  await h.call('club:expired-unchanged-unless-reopening','PUT',path,{name:'Renamed closed club'});
  await h.call('club:expired-stored-date-reopening','PUT',path,{is_registration_open:true});
  for(const identifier of ['2147483648','999999999999999999999999','1.5','bad','NaN','Infinity','%32','%2f','%252f']){
    await h.call('club:invalid-show-identifier:'+identifier,'GET','/v2/clubs/'+identifier);
    await h.call('club:invalid-update-identifier:'+identifier,'PUT','/v2/clubs/'+identifier,{});
    await h.call('club:invalid-info-identifier:'+identifier,'PUT',`/v2/clubs/${identifier}/registration-info`,{registration_info:'test'});
  }
  for(const identifier of [`0${id}`,`${id}e0`,`0x${id.toString(16)}`,`0b${id.toString(2)}`,`%20${id}%20`])await h.call('club:numeric-update-identifier:'+identifier,'PUT','/v2/clubs/'+identifier,{});
}
