import {test,expect,login,evidence,api} from './fixture.mjs';
import {legacyRequire,fixtureKey} from '../../scripts/fixture-db.mjs';

async function courseApi(path,body){
  const token=legacyRequire('jsonwebtoken').sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'15m',audience:'kaderisasi-admin'});
  const response=await fetch('http://localhost:3334/v2'+path,{method:'POST',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(body)});
  const data=await response.json();expect(response.ok,JSON.stringify(data)).toBe(true);return data.data;
}

test('linked courses: setup, progress, filters, preferences, export and retry',async({page,fixture},testInfo)=>{
  const mobile=testInfo.project.name==='mobile';
  const revealProgress=async()=>{
    if(!mobile)return;
    await expect(page.getByRole('checkbox',{name:'Pilih Peserta Selesai',exact:true})).toBeVisible();
    const closed=page.locator('article details:not([open]) > summary');
    while(await closed.count())await closed.first().click();
  };
  const activity=await api('POST','/activities',{name:'Kegiatan dengan kelas terkait',additional_config:{custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]}});
  const first=await courseApi('/courses',{title:'Dasar kepemimpinan dan kerja sama untuk peserta kegiatan pembinaan',summary:'',description:'',minimum_level:0,status:'draft'});
  const second=await courseApi('/courses',{title:'Praktik kerja sama',summary:'',description:'',minimum_level:0,status:'draft'});
  await fixture.db.query("UPDATE courses SET status='archived' WHERE id=$1",[second.id]);
  const lesson=await courseApi(`/courses/${first.id}/lessons`,{title:'Materi pertama',youtube_url:'https://youtu.be/dQw4w9WgXcQ'});
  const member=await api('POST','/members',{name:'Peserta Selesai',email:'completed@example.test',gender:'F'});
  await api('POST',`/activities/${activity.id}/registrations`,{user_id:member.profile.id,questionnaire_answer:{}});
  await fixture.db.query('INSERT INTO course_lesson_progress(user_id,lesson_id,completed_at) VALUES($1,$2,now())',[member.user.id,lesson.id]);
  await fixture.db.query("INSERT INTO activity_registrations(activity_id,guest_data,status,created_at,updated_at) VALUES($1,'{\"name\":\"Peserta Tamu\",\"email\":\"guest@example.test\"}','TERDAFTAR',now(),now())",[activity.id]);
  await fixture.db.query("UPDATE admin_users SET role_code='activity_manager' WHERE id=1");
  await login(page);
  await page.goto(`/activity/${activity.id}/setup?step=2`);
  const picker=page.getByRole('combobox',{name:'Kelas online terkait',exact:true});
  await expect(picker).toBeEnabled();
  await Promise.all([
    page.waitForResponse(response=>new URL(response.url()).pathname.endsWith('/activities/course-options') && new URL(response.url()).searchParams.get('search')==='Dasar kepemimpinan'),
    picker.fill('Dasar kepemimpinan'),
  ]);
  await expect(page.locator('.ant-select-dropdown:visible .ant-select-item-option')).toHaveCount(1);
  await expect(page.locator('.ant-select-dropdown:visible').getByText(new RegExp(first.title))).toBeVisible();
  await picker.press('Enter');await picker.press('Escape');
  await expect(page.locator('.ant-select-selection-item').filter({hasText:first.title})).toBeVisible();
  await expect(page.getByRole('button',{name:'Simpan kelas terkait',exact:true})).toBeEnabled();
  await page.getByRole('link',{name:'← Daftar kegiatan',exact:true}).click();
  await expect(page.getByRole('dialog',{name:'Perubahan belum disimpan'})).toBeVisible();
  await page.getByRole('button',{name:'Lanjutkan mengisi',exact:true}).click();
  await expect(page.getByRole('dialog',{name:'Perubahan belum disimpan'})).not.toBeVisible();
  await picker.click();
  await picker.fill('Praktik kerja sama');
  await expect(page.locator('.ant-select-dropdown:visible').getByText(/Praktik kerja sama/)).toBeVisible();
  await page.locator('.ant-select-dropdown:visible').getByText(/Praktik kerja sama/).click();
  await picker.press('Escape');
  const save=page.getByRole('button',{name:'Simpan kelas terkait',exact:true});
  await page.route(`**/v2/activities/${activity.id}/courses`,async route=>{
    if(route.request().method()==='PUT')await route.fulfill({status:503,contentType:'application/json',body:'{"message":"UNAVAILABLE"}'});
    else await route.continue();
  });
  await save.click();
  await expect(page.getByText('Perubahan belum tersimpan. Periksa koneksi lalu coba lagi.',{exact:true})).toBeVisible();
  await page.unroute(`**/v2/activities/${activity.id}/courses`);
  await save.click();await expect(page.getByText('Kelas terkait tersimpan',{exact:true})).toBeVisible();
  expect((await fixture.db.query('SELECT course_id FROM activity_courses WHERE activity_id=$1 ORDER BY position',[activity.id])).rows.map(row=>row.course_id)).toEqual([first.id,second.id]);
  await page.reload();await expect(save).toBeDisabled();
  await expect(page.getByText(new RegExp(first.title)).first()).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await evidence(page,testInfo,'linked-courses-setup');

  await page.goto(`/activity/${activity.id}/participants`);
  await revealProgress();
  await expect(page.getByText('Selesai · 1/1 materi',{exact:true})).toBeVisible();
  await expect(page.getByText('Belum ada materi · 0/0 materi',{exact:true})).toBeVisible();
  await expect(page.getByText('Tidak dapat diverifikasi',{exact:true}).first()).toBeVisible();
  await page.getByRole('button',{name:/Atur Kolom$/}).click();
  const column=page.getByRole('dialog').getByRole('checkbox',{name:`${first.title} (#${first.id})`,exact:true});
  await column.uncheck();await page.getByRole('dialog').getByRole('button',{name:'Simpan',exact:true}).click();
  await page.reload();await expect(page.getByText('Selesai · 1/1 materi',{exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:/Atur Kolom$/}).click();await column.check();await page.getByRole('dialog').getByRole('button',{name:'Simpan',exact:true}).click();
  if(mobile)await page.getByRole('button',{name:/Filter/}).click();
  const filter=page.getByRole('combobox',{name:'Kelas untuk filter progres',exact:true});
  await filter.click();await page.locator('.ant-select-dropdown:visible').getByText(`${first.title} (#${first.id})`,{exact:true}).click();
  const completion=page.getByRole('combobox',{name:'Penyelesaian kelas',exact:true});
  await completion.click();await page.locator('.ant-select-dropdown:visible').getByText('Selesai',{exact:true}).click();
  if(mobile)await page.getByRole('button',{name:'Terapkan',exact:true}).click();
  await revealProgress();
  await expect(page.getByText('Peserta Tamu',{exact:true})).toHaveCount(0);
  await expect(page.getByText('Peserta Selesai',{exact:true})).toBeVisible();
  const downloading=page.waitForEvent('download');await page.getByRole('button',{name:'Ekspor semua peserta',exact:true}).click();
  const file=await downloading;const path=testInfo.outputPath('linked-courses.xlsx');await file.saveAs(path);
  const workbook=new (legacyRequire('exceljs').Workbook)();await workbook.xlsx.readFile(path);
  const sheet=workbook.getWorksheet('Registrations');expect(sheet.rowCount).toBe(3);
  expect(sheet.getRow(1).values).toContain(`${first.title} (#${first.id}) - Status`);
  expect(sheet.getRow(1).values).toContain(`${second.title} (#${second.id}) - Status`);
  await page.getByText('Selesai · 1/1 materi',{exact:true}).scrollIntoViewIfNeeded();
  await evidence(page,testInfo,'linked-courses-participants');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await page.route(`**/v2/activities/${activity.id}/registrations?*`,route=>route.fulfill({status:503,contentType:'application/json',body:'{"message":"UNAVAILABLE"}'}));
  await page.getByRole('button',{name:'Muat ulang peserta',exact:true}).click();
  await expect(page.getByText('Peserta dan progres kelas gagal dimuat',{exact:true})).toBeVisible();
  await expect(page.getByText('Selesai · 1/1 materi',{exact:true})).toHaveCount(0);
  await page.unroute(`**/v2/activities/${activity.id}/registrations?*`);
  await page.getByRole('button',{name:'Coba lagi',exact:true}).click();
  await revealProgress();
  await expect(page.getByText('Selesai · 1/1 materi',{exact:true})).toBeVisible();
  await api('PUT',`/activities/${activity.id}/courses`,{course_ids:[]});
  await page.getByRole('button',{name:'Muat ulang peserta',exact:true}).click();
  await expect(page.getByRole('checkbox',{name:'Pilih Peserta Tamu',exact:true})).toBeVisible();
  await expect(page.getByText('Peserta dan progres kelas gagal dimuat',{exact:true})).toHaveCount(0);
  await expect(page.getByText('Selesai · 1/1 materi',{exact:true})).toHaveCount(0);
  expect((await fixture.db.query('SELECT count(*)::integer AS count FROM course_lesson_progress')).rows[0].count).toBe(1);
});
