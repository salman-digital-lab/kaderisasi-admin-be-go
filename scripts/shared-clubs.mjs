import assert from 'node:assert/strict';

// Go administers the club/form/review; the unchanged public backend owns
// submission, ownership, cancellation and immutable applicant answers.
export async function sharedClubChecks({call,db,token,otherToken}) {
  const club=await call('go','Create shared club','POST','/clubs',{name:'Shared club'});
  const schema={fields:[{section_name:'Answers',fields:[
    {key:'experience',label:'Experience',type:'number',required:true},
    {key:'consent',label:'Consent',type:'checkbox',required:true},
    {key:'motivation',label:'Motivation',type:'text',required:true},
  ]}]};
  const form=await call('go','Create public club form in Go','POST','/custom-forms',{formName:'Shared club form',featureType:'club_registration',featureId:club.id,formSchema:schema,isActive:true},{status:201});
  await call('go','Publish and open shared club','PUT',`/clubs/${club.id}`,{is_show:true,is_registration_open:true});
  const publicClub=await call('web','Read Go club publicly','GET',`/clubs/${club.id}`);
  assert.equal(publicClub.name,'Shared club');
  const publicForm=await call('web','Read Go form publicly','GET',`/custom-forms/by-feature?feature_type=club_registration&feature_id=${club.id}`);
  assert.deepEqual(publicForm.form_schema,schema);
  const answers={experience:0,consent:true,motivation:'Learn and contribute'};
  const register=(label,data,status=201,applicant=token,message)=>call('web',label,'POST','/custom-forms/register',{feature_type:'club_registration',feature_id:club.id,custom_form_data:data},{token:applicant,status,message});
  await register('Reject unchecked required consent',{...answers,consent:false},422,token,'INVALID_FORM_SUBMISSION');
  await register('Reject unknown submitted answer',{...answers,injected:'bad'},422,token,'INVALID_FORM_SUBMISSION');
  await register('Reject whitespace-only required answer',{...answers,motivation:' \t '},422,token,'INVALID_FORM_SUBMISSION');
  let registration=await register('Submit public club form with zero and true',answers);
  await register('Reject duplicate public club submission',answers,409,token,'ALREADY_REGISTERED');
  const fromGo=await call('go','Read public submission in Go','GET',`/club-registrations/${registration.id}`);
  assert.deepEqual(fromGo.additional_data,answers);assert.equal(fromGo.status,'PENDING');
  await call('web','Reject other applicant amendment','PUT',`/clubs/${club.id}/registration`,{additional_data:{consent:false}},{token:otherToken,status:404,message:'REGISTRATION_NOT_FOUND'});
  await call('web','Reject other applicant cancellation','DELETE',`/clubs/${club.id}/registration`,undefined,{token:otherToken,status:404,message:'REGISTRATION_NOT_FOUND'});
  await call('web','Keep pending answers immutable','PUT',`/clubs/${club.id}/registration`,{additional_data:{consent:false}},{token,status:400,message:'CANNOT_UPDATE_REGISTRATION'});
  await call('web','Allow owner to cancel pending submission','DELETE',`/clubs/${club.id}/registration`,undefined,{token,message:'CLUB_REGISTRATION_DELETED'});
  assert.equal((await db.query('SELECT id FROM club_registrations WHERE id=$1',[registration.id])).rowCount,0);
  registration=await register('Allow owner to submit again after cancellation',answers);
  await call('go','Review public club submission in Go','PUT',`/club-registrations/${registration.id}`,{status:'APPROVED'});
  const reviewed=await call('web','Read Go review through web-be','GET',`/clubs/${club.id}/registration-status`,undefined,{token});
  assert.equal(reviewed.registration.status,'APPROVED');assert.deepEqual(reviewed.registration.additional_data,answers);
  await call('web','Protect reviewed cancellation','DELETE',`/clubs/${club.id}/registration`,undefined,{token,status:400,message:'CANNOT_CANCEL_REGISTRATION'});
  await call('web','Protect reviewed answers','PUT',`/clubs/${club.id}/registration`,{additional_data:{consent:false,injected:'bad'}},{token,status:400,message:'CANNOT_UPDATE_REGISTRATION'});
  const stored=(await db.query('SELECT status,additional_data FROM club_registrations WHERE id=$1',[registration.id])).rows[0];
  assert.deepEqual(stored,{status:'APPROVED',additional_data:answers});
  await call('go','Assign shared club role','POST',`/clubs/${club.id}/member-roles`,{club_registration_id:registration.id,role_name:'Coordinator',is_primary:true},{status:201});
  const withRole=await call('web','Read Go club role through web-be','GET',`/clubs/${club.id}/registration-status`,undefined,{token});
  assert.equal(withRole.registration.roles[0].role_name,'Coordinator');
  await call('go','Close shared club in Go','PUT',`/clubs/${club.id}`,{is_registration_open:false});
  await register('Respect Go registration closure',answers,400,otherToken,'REGISTRATION_CLOSED');
  await call('go','Deactivate closed club form in Go','PUT',`/custom-forms/${form.id}/toggle-active`,{});
  // Model a stale legacy open flag. The public API must still require an active
  // form; this direct write targets only the run's isolated club record.
  await db.query('UPDATE clubs SET is_registration_open=true WHERE id=$1',[club.id]);
  await register('Reject stale open club without active form',answers,400,otherToken,'ACTIVE_CUSTOM_FORM_REQUIRED');
  await db.query('UPDATE clubs SET is_registration_open=false WHERE id=$1',[club.id]);
}
