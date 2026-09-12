import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

const fixtures=JSON.parse(readFileSync(new URL('../internal/formschema/routing.fixtures.json',import.meta.url),'utf8'));

export async function sharedFormRoutingChecks({call,db,token}) {
  for(const [index,fixture] of fixtures.cases.entries()) {
    const activity=await call('go',`Create routed activity: ${fixture.name}`,'POST','/activities',{
      name:`Shared routing ${index}`,activity_type:1,
      additional_config:{allow_guest_registration:true,custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]},
    });
    const form=await call('go',`Attach routed form: ${fixture.name}`,'POST','/custom-forms',{
      formName:`Shared routing form ${index}`,featureType:'activity_registration',featureId:activity.id,
      isActive:true,formSchema:fixtures.schema,
    },{status:201});
    await db.query('UPDATE activities SET is_published=true,is_registration_open=true,registration_start=CURRENT_DATE-1,registration_end=CURRENT_DATE+1 WHERE id=$1',[activity.id]);
    const publicForm=await call('web',`Read routing metadata: ${fixture.name}`,'GET',`/custom-forms/by-feature?feature_type=activity_registration&feature_id=${activity.id}`);
    assert.deepEqual(publicForm.form_schema,fixtures.schema);
    const member=await call('web',`Member route: ${fixture.name}`,'POST','/custom-forms/register',{
      feature_type:'activity_registration',feature_id:activity.id,custom_form_data:fixture.answers,
    },{token,status:fixture.valid?201:422});
    const guest=await call('web',`Guest route: ${fixture.name}`,'POST',`/activities/${activity.slug}/guest-register`,{
      guest_data:{name:'Shared route guest',email:`route-${index}@example.test`},questionnaire_answer:fixture.answers,
    },{status:fixture.valid?200:422});
    const rows=(await db.query('SELECT questionnaire_answer FROM activity_registrations WHERE activity_id=$1 ORDER BY id',[activity.id])).rows;
    assert.equal(rows.length,fixture.valid?2:0);
    for(const row of rows)assert.deepEqual(row.questionnaire_answer,fixture.kept);
    if(fixture.valid) {
      assert.ok(member.id);assert.ok(guest.id);
      await call('web',`Require reachable answer on edit: ${fixture.name}`,'PUT',`/activities/${activity.slug}/registration`,{questionnaire_answer:{track:'detail'}},{token,status:422});
      await call('web',`Prune answer on edit: ${fixture.name}`,'PUT',`/activities/${activity.slug}/registration`,{questionnaire_answer:{track:'finish',reason:'stale',note:'stale'}},{token});
      assert.deepEqual((await db.query('SELECT questionnaire_answer FROM activity_registrations WHERE id=$1',[member.id])).rows[0].questionnaire_answer,{track:'finish'});
      const malformed=structuredClone(fixtures.schema);
      malformed.fields[1].navigation.defaultTarget={type:'section',sectionId:'missing'};
      await db.query('UPDATE custom_forms SET form_schema=$1::jsonb WHERE id=$2',[JSON.stringify(malformed),form.id]);
      await call('web',`Reject malformed stored route: ${fixture.name}`,'PUT',`/activities/${activity.slug}/registration`,{questionnaire_answer:{track:'finish'}},{token,status:422});
    }
  }
  const club=await call('go','Create routed shared club','POST','/clubs',{name:'Shared routing club'});
  const form=await call('go','Attach routed club form','POST','/custom-forms',{
    formName:'Shared routing club form',featureType:'club_registration',featureId:club.id,
    formSchema:fixtures.schema,isActive:true,
  },{status:201});
  await call('go','Open routed shared club','PUT',`/clubs/${club.id}`,{is_show:true,is_registration_open:true});
  await call('web','Reject invalid club routing choice','POST','/custom-forms/register',{
    feature_type:'club_registration',feature_id:club.id,custom_form_data:{track:'forged',reason:'Answer'},
  },{token,status:422});
  const registration=await call('web','Prune skipped club answers','POST','/custom-forms/register',{
    feature_type:'club_registration',feature_id:club.id,custom_form_data:{track:'finish',reason:'stale'},
  },{token,status:201});
  const stored=await call('go','Read pruned club answers in Go','GET',`/club-registrations/${registration.id}`);
  assert.deepEqual(stored.additional_data,{track:'finish'});
  await call('go','Close routed shared club','PUT',`/clubs/${club.id}`,{is_registration_open:false});
  const malformed=structuredClone(fixtures.schema);
  malformed.fields[1].navigation.defaultTarget={type:'section',sectionId:'missing'};
  await db.query('UPDATE custom_forms SET form_schema=$1::jsonb WHERE id=$2',[JSON.stringify(malformed),form.id]);
  await call('go','Prevent reopening a club with broken routes','PUT',`/clubs/${club.id}`,{is_registration_open:true},{status:400,message:'INVALID_FORM_SCHEMA'});
}
