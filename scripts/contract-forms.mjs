import assert from 'node:assert/strict';

export async function formBoundaryCases(h) {
  await h.call('form:club-fixture','POST','/v2/clubs',{name:'Form boundary club'});
  const schema={fields:[{section_name:'questions',fields:[{key:'value',label:'Value',required:false,type:'select',placeholder:'',helpText:'Help',description:'Detail',options:[{label:'Zero',value:0},{label:'False',value:false,disabled:false},{label:'Object',value:{nested:[1,false,null]}},{label:'Text',value:'text'}],validation:{min:0,max:10,minLength:0,maxLength:20,pattern:'test',customMessage:'Invalid'},defaultValue:null,hidden:false,disabled:false}]}]};
  const created=await h.call('form:all-fields','POST','/v2/custom-forms',{formName:' Full form ',formDescription:' ',postSubmissionInfo:' Info ',featureType:'independent_form',featureId:null,formSchema:schema,isActive:false});
  assert.ok(created.data,JSON.stringify(created));
  assert.equal(created.data.form_description,null);assert.equal(created.data.post_submission_info,'Info');
  const id=created.data.id,path=`/v2/custom-forms/${id}`;
  await h.seed("UPDATE custom_forms SET updated_at='2026-01-01T00:00:00Z' WHERE id=$1",[id]);
  for(const [label,input] of [['empty',{}],['same',{formName:'Full form',formDescription:null,postSubmissionInfo:'Info',featureType:'independent_form',featureId:null,formSchema:schema,isActive:false}]]){
    const result=await h.call('form:unchanged-'+label,'PUT',path,input);
    assert.equal(new Date(result.data.updated_at).toISOString(),'2026-01-01T00:00:00.000Z');
  }
  await h.call('form:nullable-clear','PUT',path,{formDescription:' Description ',postSubmissionInfo:''});
  await h.call('form:schema-option-types','GET',path);
  for(const identifier of ['bad','1.5','2147483648','9007199254740992','Infinity','%31','0']) {
    for(const [method,suffix,body] of [['GET',''],['PUT','',{}],['DELETE',''],['PUT','/toggle-active',{}],['PUT','/attach-club',{clubId:1}],['PUT','/detach-club',{}],['PUT','/attach-activity',{activityId:1}],['PUT','/detach-activity',{}]]){
      await h.call(`form:identifier:${identifier}:${method}:${suffix}`,method,`/v2/custom-forms/${identifier}${suffix}`,body);
    }
  }
  for(const spelling of [`0${id}`,`${id}e0`,`0x${id.toString(16)}`,`0b${id.toString(2)}`])await h.call('form:numeric-spelling:'+spelling,'PUT','/v2/custom-forms/'+spelling,{});
  for(const value of ['bad','1.5','2147483648']){
    for(const route of [`?feature_id=${value}`,`/by-feature?feature_type=club_registration&feature_id=${value}`,`/available-activities?current_form_id=${value}`,`/available-clubs?current_form_id=${value}`])await h.call('form:query-boundary:'+route,'GET','/v2/custom-forms'+route);
  }
  await h.call('form:wide-feature-create','POST','/v2/custom-forms',{formName:'Wide independent',featureType:'independent_form',featureId:2147483648});
  await h.call('form:wide-club-create','POST','/v2/custom-forms',{formName:'Wide club',featureType:'club_registration',featureId:2147483648});
  await h.call('form:wide-feature-update','PUT',path,{featureId:2147483648});
  await h.call('form:wide-club-attach','PUT',path+'/attach-club',{clubId:2147483648});
  for(const value of [null,false,0,'',true,[],[1],{},'0','false','bad','1.5','2147483648',' 1 ',1]){
    await h.seed('UPDATE custom_forms SET feature_id=null WHERE id=$1',[id]);
    await h.call('form:activity-raw:'+JSON.stringify(value),'PUT',path+'/attach-activity',{activityId:value});
  }
  await h.seed("UPDATE custom_forms SET feature_id=null,updated_at='2026-01-01T00:00:00Z' WHERE id=$1",[id]);
  const detached=await h.call('form:detached-timestamp','PUT',path+'/detach-activity',{});
  assert.notEqual(new Date(detached.data.updated_at).toISOString(),'2026-01-01T00:00:00.000Z');
  await h.seed('UPDATE custom_forms SET is_active=null,form_schema=null WHERE id=$1',[id]);
  await h.call('form:legacy-null-noop','PUT',path,{});
  const toggled=await h.call('form:legacy-null-toggle','PUT',path+'/toggle-active',{});
  assert.equal(toggled.data.is_active,true);
  await h.seed("UPDATE custom_forms SET feature_type='club_registration',feature_id=0 WHERE id=$1",[id]);
  await h.call('form:legacy-zero-club','PUT',path,{});
  await h.call('form:legacy-zero-already-attached','PUT',path+'/attach-club',{clubId:1});
  await h.call('form:legacy-zero-activity-allowed','PUT',path+'/attach-activity',{activityId:1});
  await h.call('form:delete-boundary-fixture','DELETE',path);
}
