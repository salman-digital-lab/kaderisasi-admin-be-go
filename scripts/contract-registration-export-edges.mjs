export async function registrationExportEdges(h,profileId,formId){
  const path='/v2/activities/1/registrations-export';
  const call=label=>h.call('registration:export-boundary:'+label,'GET',path);
  const config={additional_questionnaire:[{name:'motivation',label:'Motivation'}]};
  const form={fields:[{section_name:'questions',fields:[{key:'motivation',label:'Motivation'}]}]};
  for(const [label,value] of [['null-config',null],['missing-questions',{}],['null-questions',{additional_questionnaire:null}],['object-questions',{additional_questionnaire:{}}],['null-question',{additional_questionnaire:[null]}],['empty-questions',{additional_questionnaire:[]}]]){
    await h.seed('UPDATE activities SET additional_config=$1 WHERE id=1',[JSON.stringify(value)]);
    await call(label);
  }
  await h.seed('UPDATE activities SET additional_config=$1 WHERE id=1',[JSON.stringify(config)]);
  await h.seed('UPDATE custom_forms SET is_active=true WHERE id=$1',[formId]);
  for(const [label,value] of [['null-form',null],['missing-sections',{}],['null-sections',{fields:null}],['object-sections',{fields:{}}],['null-section',{fields:[null]}],['missing-fields',{fields:[{}]}],['null-fields',{fields:[{fields:null}]}],['null-field',{fields:[{fields:[null]}]}],['empty-fields',{fields:[]}]]){
    await h.seed('UPDATE custom_forms SET form_schema=$1 WHERE id=$2',[JSON.stringify(value),formId]);
    await call(label);
  }
  await h.seed('UPDATE custom_forms SET form_schema=$1,is_active=false WHERE id=$2',[JSON.stringify(form),formId]);
  for(const value of [null,3,false,'text',[]]){
    await h.seed('UPDATE activity_registrations SET questionnaire_answer=$1 WHERE activity_id=1',[JSON.stringify(value)]);
    await call('answers-'+JSON.stringify(value));
  }
  await h.seed('UPDATE activity_registrations SET questionnaire_answer=NULL WHERE activity_id=1');
  await call('sql-null-answers');
  await h.seed('UPDATE activities SET additional_config=$1 WHERE id=1',[JSON.stringify({additional_questionnaire:[]})]);
  await call('no-questions-null-answers');
  await h.seed('UPDATE activity_registrations SET questionnaire_answer=$1 WHERE activity_id=1',[JSON.stringify({motivation:'Boundary fixture'})]);
  await h.seed('UPDATE activities SET additional_config=$1 WHERE id=1',[JSON.stringify(config)]);
  for(const field of ['education_history','work_history']){
    for(const [label,value] of [['null-entry',[null]],['primitive-entries',[3,false,'text']],['truthy-text',[{degree:'0',institution:'false',faculty:'0',major:'false',intake_year:'0',job_title:'0',company:'false',start_year:'0',end_year:'false'}]],['false-values',[{degree:0,institution:false,faculty:0,major:false,intake_year:0,job_title:0,company:false,start_year:0,end_year:false}]]]){
      await h.seed(`UPDATE profiles SET ${field}=$1 WHERE id=$2`,[JSON.stringify(value),profileId]);
      await call(field+'-'+label);
    }
    await h.seed(`UPDATE profiles SET ${field}='[]'::jsonb WHERE id=$1`,[profileId]);
  }
  for(const [label,value] of [['primitive-current-education',{current_education:3}],['truthy-current-education',{current_education:{institution:'0',faculty:'false',major:'0',intake_year:'false'},intake_year:'0'}],['null-education-entry',{education_history:[null]}]]){
    await h.seed('UPDATE activity_registrations SET guest_data=guest_data||$1::jsonb WHERE id=2',[JSON.stringify(value)]);
    await call(label);
  }
  await h.seed("UPDATE activity_registrations SET guest_data=guest_data||'{\"education_history\":[]}'::jsonb WHERE id=2");
  for(const id of ['invalid','1.5','4294967297'])await h.call('registration:export-invalid-id:'+id,'GET',`/v2/activities/${id}/registrations-export`);
}
