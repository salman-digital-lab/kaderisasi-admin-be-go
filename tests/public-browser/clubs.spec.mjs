import {test,expect,evidence,api} from '../browser/fixture.mjs';
import {fixturePassword} from '../../scripts/fixture-db.mjs';

test('club form validates zero, whitespace and checkbox answers and reflects Go review',async({page,fixture},testInfo)=>{
  await api('POST','/members',{name:'Browser club applicant',email:'club-browser@example.test',password:fixturePassword});
  const club=await api('POST','/clubs',{name:'Browser shared club'});
  await api('POST','/custom-forms',{formName:'Browser club registration',featureType:'club_registration',featureId:club.id,isActive:true,formSchema:{fields:[
    {section_name:'profile_data',fields:[]},
    {section_name:'Club answers',fields:[
      {key:'experience',label:'Experience',type:'number',required:true},
      {key:'consent',label:'Consent',type:'checkbox',required:true},
      {key:'motivation',label:'Motivation',type:'text',required:true},
    ]},
  ]}});
  await api('PUT',`/clubs/${club.id}`,{is_show:true,is_registration_open:true});
  const clubURL=`http://localhost:3000/clubs/${club.id}`;
  await page.goto('http://localhost:3000/login?redirect='+encodeURIComponent(`/clubs/${club.id}`));
  await page.getByRole('textbox',{name:'Email',exact:true}).fill('club-browser@example.test');
  await page.getByPlaceholder('Password Anda',{exact:true}).fill(fixturePassword);
  await page.getByRole('button',{name:'Masuk',exact:true}).click();
  await expect(page).toHaveURL(clubURL);
  async function submit({invalid=false}={}) {
    await page.getByRole('link',{name:'Daftar untuk Browser shared club',exact:true}).click();
    await expect(page.getByRole('heading',{name:'Browser club registration',exact:true})).toBeVisible();
    await page.getByRole('button',{name:'Lanjutkan',exact:true}).click();
    await page.getByRole('textbox',{name:'Experience',exact:false}).fill('0');
    await page.getByRole('textbox',{name:'Motivation',exact:false}).fill(invalid?'   ':'Learn and contribute');
    if(invalid) {
      await page.getByRole('button',{name:'Kirim',exact:true}).click();
      await expect(page.getByRole('dialog',{name:'Konfirmasi Pengiriman'})).toHaveCount(0);
    }
    await page.getByRole('checkbox',{name:'Consent',exact:false}).check();
    if(invalid) {
      await page.getByRole('button',{name:'Kirim',exact:true}).click();
      await expect(page.getByText('Motivation wajib diisi',{exact:true})).toBeVisible();
      expect((await fixture.db.query('SELECT id FROM club_registrations WHERE club_id=$1',[club.id])).rowCount).toBe(0);
      await evidence(page,testInfo,'club-required-answer-validation');
      await page.getByRole('textbox',{name:'Motivation',exact:false}).fill('Learn and contribute');
    }
    await page.getByRole('button',{name:'Kirim',exact:true}).click();
    const confirmation=page.getByRole('dialog',{name:'Konfirmasi Pengiriman'});
    await expect(confirmation).toBeVisible();
    await confirmation.getByRole('button',{name:'Ya, Kirim',exact:true}).click();
    await expect(page).toHaveURL(`http://localhost:3000/custom-form/club/${club.id}/success`);
    await expect.poll(async()=> (await fixture.db.query('SELECT additional_data FROM club_registrations WHERE club_id=$1',[club.id])).rows[0]?.additional_data).toEqual({experience:0,consent:true,motivation:'Learn and contribute'});
  }
  await submit({invalid:true});
  await evidence(page,testInfo,'public-club-submitted');
  await page.goto(clubURL);
  await expect(page.getByText('Pendaftaran sedang ditinjau',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Batalkan pendaftaran',exact:true}).click();
  await page.getByRole('dialog',{name:'Batalkan pendaftaran',exact:true}).getByRole('button',{name:'Ya, batalkan',exact:true}).click();
  await expect(page.getByRole('link',{name:'Daftar untuk Browser shared club',exact:true})).toBeVisible();
  await submit();
  const registration=(await fixture.db.query('SELECT id FROM club_registrations WHERE club_id=$1',[club.id])).rows[0];
  await api('PUT',`/club-registrations/${registration.id}`,{status:'APPROVED'});
  await page.goto(clubURL);
  await expect(page.getByText('Keanggotaan disetujui',{exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:'Batalkan pendaftaran',exact:true})).toHaveCount(0);
  await expect(page.getByRole('link',{name:/ubah|edit/i})).toHaveCount(0);
  const persisted=await api('GET',`/club-registrations/${registration.id}`);
  expect(persisted.additional_data).toEqual({experience:0,consent:true,motivation:'Learn and contribute'});
  await evidence(page,testInfo,'public-club-approved-by-go');
});
