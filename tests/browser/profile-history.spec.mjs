import {test,expect,api,login,evidence} from './fixture.mjs';

test('admin edits partial histories and renders structured registrant data',async({page,fixture},testInfo)=>{
  const errors=[];
  page.on('pageerror',error=>errors.push(error.message));
  const member=await api('POST','/members',{name:'History admin member',email:'history-admin@example.test'});
  await fixture.db.query('UPDATE profiles SET education_history=$1::jsonb,work_history=$2::jsonb WHERE id=$3',[
    JSON.stringify([null,{degree:'bachelor',institution:'ITB',major:'Physics',intake_year:'2017'}]),
    JSON.stringify([null,{job_title:'Engineer',company:'Company',start_year:'2021',end_year:null}]),
    member.profile.id,
  ]);
  await login(page);
  await page.goto('/member/'+member.profile.id);
  await expect(page.getByLabel('Tahun Masuk',{exact:true})).toHaveValue('2017');
  await page.getByRole('button',{name:/Ubah$/}).click();
  await page.getByLabel('Posisi / Jabatan',{exact:true}).fill('Senior Engineer');
  await page.getByRole('button',{name:/Simpan$/}).click();
  await expect(page.getByRole('button',{name:/Ubah$/})).toBeVisible();
  await expect.poll(async()=> (await fixture.db.query('SELECT work_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0].work_history)
    .toEqual([{job_title:'Senior Engineer',company:'Company',start_year:2021}]);
  await evidence(page,testInfo,'admin-history');
  const activity=await api('POST','/activities',{name:'History activity',additional_config:{custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]}});
  const registration=await api('POST','/activities/'+activity.id+'/registrations',{user_id:member.profile.id,questionnaire_answer:{}});
  await page.goto('/activity/'+activity.id+'/participants/'+registration.id);
  await expect(page.getByText('S1 - ITB, Physics (2017)',{exact:true}).first()).toBeVisible();
  await expect(page.getByText('Senior Engineer - Company - 2021 - Sekarang',{exact:true})).toBeVisible();
  await evidence(page,testInfo,'registrant-history');
  await api('POST','/custom-forms',{
    formName:'History profile fields',featureType:'activity_registration',featureId:activity.id,isActive:true,
    formSchema:{fields:[{section_name:'profile_data',fields:[
      {key:'name',label:'Nama',type:'text',required:true},
      {key:'education_history',label:'Riwayat Pendidikan',type:'education_history',required:false},
      {key:'work_history',label:'Riwayat Pekerjaan',type:'text',required:false},
    ]}]},
  });
  const guest=(await fixture.db.query(
    "INSERT INTO activity_registrations(activity_id,user_id,status,questionnaire_answer,guest_data,created_at,updated_at) VALUES($1,NULL,'TERDAFTAR','{}',$2::jsonb,now(),now()) RETURNING id",
    [activity.id,JSON.stringify({name:'History guest',education_history:[null,{institution:'Guest University',intake_year:'2020'}],work_history:[null,{job_title:'Researcher',company:'Lab'}]})],
  )).rows[0];
  await page.goto('/activity/'+activity.id+'/participants/'+guest.id);
  await expect(page.getByText('Guest University (2020)',{exact:true})).toBeVisible();
  await expect(page.getByText('Researcher - Lab',{exact:true})).toBeVisible();
  await evidence(page,testInfo,'guest-history');
  expect(errors).toEqual([]);
});
