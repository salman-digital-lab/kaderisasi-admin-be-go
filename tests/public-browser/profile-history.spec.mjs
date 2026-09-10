import {test,expect,api,evidence} from '../browser/fixture.mjs';
import {fixturePassword} from '../../scripts/fixture-db.mjs';

test('profile history editing, validation, cancellation and persistence',async({page,fixture},testInfo)=>{
  const errors=[];
  page.on('pageerror',error=>errors.push(error.message));
  const member=await api('POST','/members',{name:'History browser member',email:'history-browser@example.test',password:fixturePassword});
  await api('PUT','/profiles/'+member.profile.id,{
    education_history:[{degree:'bachelor',institution:'ITB',major:'Physics',intake_year:2017}],
    work_history:[{job_title:'Engineer',company:'Company',start_year:2021}],
  });
  await page.goto('http://localhost:3000/login?redirect=/profile');
  await page.getByRole('textbox',{name:'Email',exact:true}).fill('history-browser@example.test');
  await page.getByPlaceholder('Password Anda',{exact:true}).fill(fixturePassword);
  await page.getByRole('button',{name:'Masuk',exact:true}).click();
  await expect(page).toHaveURL('http://localhost:3000/profile');
  await expect(page.getByText('S1 • ITB / Physics • 2017',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'+ Tambah Pendidikan',exact:true}).click();
  await page.getByRole('button',{name:'Batal edit pendidikan',exact:true}).click();
  await expect(page.getByText('Pendidikan 2',{exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'+ Tambah Pekerjaan / Aktivitas',exact:true}).click();
  await page.getByRole('button',{name:'Batal edit pekerjaan',exact:true}).click();
  await expect(page.getByText('Pekerjaan / Aktivitas 2',{exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'Edit pekerjaan',exact:true}).click();
  await page.getByLabel('Tahun Selesai',{exact:true}).fill('2020');
  await page.getByRole('button',{name:'Simpan pekerjaan',exact:true}).click();
  await expect(page.getByText('Tahun selesai tidak boleh lebih kecil dari tahun mulai',{exact:true})).toBeVisible();
  await page.getByLabel('Tahun Selesai',{exact:true}).fill('');
  await page.getByLabel('Posisi / Jabatan',{exact:true}).fill('Senior Engineer');
  await page.getByRole('button',{name:'Simpan pekerjaan',exact:true}).click();
  await expect(page.getByText('Senior Engineer - Company • 2021 - Sekarang',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Ubah Data Diri',exact:true}).click();
  await expect.poll(async()=> (await fixture.db.query('SELECT work_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0].work_history)
    .toEqual([{job_title:'Senior Engineer',company:'Company',start_year:2021}]);
  await page.reload();
  await expect(page.getByText('Senior Engineer - Company • 2021 - Sekarang',{exact:true})).toBeVisible();
  await evidence(page,testInfo,'profile-history');
  await page.getByRole('button',{name:'Hapus pendidikan',exact:true}).click();
  await page.getByRole('button',{name:'Hapus pekerjaan',exact:true}).click();
  await page.getByRole('button',{name:'Ubah Data Diri',exact:true}).click();
  await expect.poll(async()=> (await fixture.db.query('SELECT education_history,work_history FROM profiles WHERE id=$1',[member.profile.id])).rows[0])
    .toEqual({education_history:[],work_history:[]});
  expect(errors).toEqual([]);
});
