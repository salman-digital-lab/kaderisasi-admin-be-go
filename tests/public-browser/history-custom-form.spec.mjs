import {test,expect,api,evidence} from '../browser/fixture.mjs';
import {fixturePassword} from '../../scripts/fixture-db.mjs';

test('current education selection preserves edits to the full education history',async({page,fixture},testInfo)=>{
  const errors=[];
  page.on('pageerror',error=>errors.push(error.message));
  const member=await api('POST','/members',{name:'History form member',email:'history-form@example.test',password:fixturePassword});
  await api('PUT','/profiles/'+member.profile.id,{
    education_history:[
      {degree:'bachelor',institution:'University A',major:'Physics',intake_year:2017},
      {degree:'master',institution:'University B',major:'Mathematics',intake_year:2021},
    ],
  });
  const activity=await api('POST','/activities',{name:'History form activity',additional_config:{custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]}});
  await api('POST','/custom-forms',{
    formName:'Education form',featureType:'activity_registration',featureId:activity.id,isActive:true,
    formSchema:{fields:[
      {section_name:'profile_data',fields:[
        {key:'education_history',label:'Riwayat Pendidikan',type:'education_history',required:false},
        {key:'current_education',label:'Pendidikan Sekarang',type:'current_education',required:true},
      ]},
      {section_name:'Other',fields:[{key:'reason',label:'Alasan',type:'text',required:false}]},
    ]},
  });
  await page.goto('http://localhost:3000/login?redirect=/custom-form/activity/'+activity.id);
  await page.getByRole('textbox',{name:'Email',exact:true}).fill('history-form@example.test');
  await page.getByPlaceholder('Password Anda',{exact:true}).fill(fixturePassword);
  await page.getByRole('button',{name:'Masuk',exact:true}).click();
  await expect(page).toHaveURL('http://localhost:3000/custom-form/activity/'+activity.id);
  await page.getByLabel('Jurusan',{exact:true}).nth(0).fill('Updated Physics');
  await page.getByRole('radio').nth(0).check();
  await page.getByRole('button',{name:'Lanjutkan',exact:true}).click();
  await expect(page.getByLabel('Alasan',{exact:true})).toBeVisible();
  await expect.poll(async()=> (await fixture.db.query('SELECT education_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0].education_history)
    .toEqual([
      {degree:'master',institution:'University B',faculty:'',major:'Mathematics',intake_year:2021},
      {degree:'bachelor',institution:'University A',faculty:'',major:'Updated Physics',intake_year:2017},
    ]);
  await evidence(page,testInfo,'custom-form-history-preserved');
  await page.getByRole('button',{name:'Kembali',exact:true}).click();
  await expect(page.getByLabel('Jurusan',{exact:true}).nth(1)).toHaveValue('Updated Physics');
  await page.getByRole('button',{name:'Lanjutkan',exact:true}).click();
  await expect(page.getByLabel('Alasan',{exact:true})).toBeVisible();
  const saved=(await fixture.db.query('SELECT education_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0].education_history;
  expect(saved[1].major).toBe('Updated Physics');
  expect(errors).toEqual([]);
});
