import {test,expect,login,evidence,api} from './fixture.mjs';
import {legacyRequire} from '../../scripts/fixture-db.mjs';

for(const kind of ['activity','club'])test(`linked courses: ${kind} chooser and separate progress`,async({page,fixture},testInfo)=>{
 const mobile=testInfo.project.name==='mobile';
 const owner=kind==='activity'?await api('POST','/activities',{name:'Kegiatan kelas terkait',additional_config:{custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]}}):(await fixture.db.query("INSERT INTO clubs(name) VALUES('Klub kelas terkait') RETURNING id")).rows[0];
 const prefix=kind==='activity'?'activities':'clubs';
 const courses=(await fixture.db.query("INSERT INTO courses(title,summary,status,minimum_level) VALUES('Dasar kepemimpinan dan kerja sama untuk peserta kegiatan pembinaan','Mengenal dasar kepemimpinan melalui latihan kolaborasi.','published',3),('Praktik kerja sama','Latihan lanjutan untuk anggota dan pendaftar.','archived',0) RETURNING *")).rows;
 for(let i=0;i<12;i++)await fixture.db.query("INSERT INTO courses(title,summary) VALUES($1,'Materi tambahan untuk pilihan kelas.')",['Kelas tambahan '+String(i).padStart(2,'0')]);
 const lesson=(await fixture.db.query("INSERT INTO course_lessons(course_id,title,position) VALUES($1,'Materi pertama',1) RETURNING id",[courses[0].id])).rows[0];
 const member=await api('POST','/members',{name:'Peserta Selesai',email:'completed@example.test',gender:'F'});
 if(kind==='activity'){
  await api('POST',`/activities/${owner.id}/registrations`,{user_id:member.profile.id,questionnaire_answer:{}});
  await fixture.db.query("INSERT INTO activity_registrations(activity_id,guest_data,status,created_at) VALUES($1,'{\"name\":\"Peserta Tamu\",\"email\":\"guest@example.test\"}','TERDAFTAR',now())",[owner.id]);
 }else{
  await fixture.db.query("INSERT INTO club_registrations(club_id,member_id,status) VALUES($1,$2,'APPROVED'),($1,NULL,'PENDING')",[owner.id,member.user.id]);
 }
 await fixture.db.query('INSERT INTO course_lesson_progress(user_id,lesson_id,completed_at) VALUES($1,$2,now())',[member.user.id,lesson.id]);
 await fixture.db.query('UPDATE admin_users SET role_code=$1 WHERE id=1',[kind==='activity'?'activity_manager':'club_manager']);
 await login(page);
 const path=kind==='activity'?`/activity/${owner.id}?tab=courses`:`/club/${owner.id}?section=courses`;
 await page.goto(path);await expect(page.getByRole('tab',{name:'Kelas Online',exact:true})).toHaveAttribute('aria-selected','true');
 await page.getByRole('button',{name:'Pilih kelas online',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'Pilih kelas online terkait'});
 const search=dialog.getByLabel('Cari kelas online',{exact:true});
 await expect(dialog.getByText(courses[0].summary,{exact:true})).toBeVisible();
 await expect(dialog.getByText('Jenjang minimum: Aktivis',{exact:true})).toBeVisible();
 const first=dialog.getByRole('checkbox',{name:`Pilih ${courses[0].title} (#${courses[0].id})`,exact:true});
 await first.press('Space');
 await dialog.getByTitle('Halaman Berikutnya').click();
 await expect(dialog.getByRole('checkbox',{name:new RegExp('Kelas tambahan 09')})).toBeVisible();
 await expect(dialog.getByText('Kelas dipilih (1)',{exact:true})).toBeVisible();
 await search.fill('Praktik kerja sama');
 const second=dialog.getByRole('checkbox',{name:`Pilih ${courses[1].title} (#${courses[1].id})`,exact:true});
 await second.check();
 await expect(dialog.getByText(courses[1].summary,{exact:true})).toBeVisible();
 await expect(dialog.getByText('Diarsipkan',{exact:true})).toBeVisible();
 await dialog.getByRole('button',{name:`Naikkan ${courses[1].title}`,exact:true}).click();
 await page.route(`**/v2/${prefix}/${owner.id}/courses`,async route=>{if(route.request().method()==='PUT')await route.fulfill({status:503,json:{message:'UNAVAILABLE'}});else await route.continue();});
 const save=dialog.getByRole('button',{name:'Simpan kelas terkait',exact:true});await save.click();
 await expect(dialog.getByText('Perubahan belum tersimpan. Periksa koneksi lalu coba lagi.',{exact:true})).toBeVisible();
 await page.unroute(`**/v2/${prefix}/${owner.id}/courses`);
 await dialog.screenshot({path:testInfo.outputPath('course-chooser.png'),animations:'disabled'});
 await save.click();await expect(dialog).not.toBeVisible();await expect(page.getByText('Kelas terkait tersimpan',{exact:true})).toBeVisible();
 const table=kind==='activity'?'activity_courses':'club_courses',column=kind==='activity'?'activity_id':'club_id';
 expect((await fixture.db.query(`SELECT course_id FROM ${table} WHERE ${column}=$1 ORDER BY position`,[owner.id])).rows.map(row=>row.course_id)).toEqual([courses[1].id,courses[0].id]);
 await page.reload();
 const reveal=async()=>{await expect(page.getByText('Peserta Selesai',{exact:true})).toBeVisible();if(mobile){const closed=page.locator('article details:not([open]) > summary');while(await closed.count())await closed.first().click();}};
 await reveal();await expect(page.getByText('Selesai · 1/1 materi',{exact:true})).toBeVisible();await expect(page.getByText('Belum ada materi · 0/0 materi',{exact:true})).toBeVisible();await expect(page.getByText('Tidak dapat diverifikasi',{exact:true}).first()).toBeVisible();
 await page.getByText('Selesai · 1/1 materi',{exact:true}).scrollIntoViewIfNeeded();await evidence(page,testInfo,kind+'-course-progress');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
 await page.getByRole('combobox',{name:'Kelas untuk filter progres',exact:true}).click();await page.locator('.ant-select-dropdown:visible').getByText(`${courses[0].title} (#${courses[0].id})`,{exact:true}).click();
 await page.getByRole('combobox',{name:'Penyelesaian kelas',exact:true}).click();await page.locator('.ant-select-dropdown:visible').getByText('Selesai',{exact:true}).click();await reveal();
 await expect(page.locator('[data-list-id]').getByText('Tidak dapat diverifikasi',{exact:true})).toHaveCount(0);
 const downloading=page.waitForEvent('download');await page.getByRole('button',{name:'Ekspor semua peserta',exact:true}).click();const file=await downloading;const filePath=testInfo.outputPath('progress.xlsx');await file.saveAs(filePath);
 const workbook=new(legacyRequire('exceljs').Workbook)();await workbook.xlsx.readFile(filePath);const sheet=workbook.getWorksheet('Progres Kelas');expect(sheet.rowCount).toBe(3);expect(sheet.getRow(1).values).toContain(`${courses[0].title} (#${courses[0].id}) - Status`);
 await page.route(`**/v2/${prefix}/${owner.id}/course-progress?*`,route=>route.fulfill({status:503,json:{message:'UNAVAILABLE'}}));await page.getByRole('button',{name:'Muat ulang progres',exact:true}).click();await expect(page.getByText('Progres kelas gagal dimuat',{exact:true})).toBeVisible();await expect(page.getByText('Selesai · 1/1 materi',{exact:true})).toHaveCount(0);
 await page.unroute(`**/v2/${prefix}/${owner.id}/course-progress?*`);await page.getByRole('button',{name:'Coba lagi',exact:true}).click();await reveal();
 if(kind==='activity'){
  await page.goto(`/activity/${owner.id}/participants`);await reveal();await expect(page.getByText('Selesai · 1/1 materi',{exact:true})).toBeVisible();await expect(page.getByText('Belum ada materi · 0/0 materi',{exact:true})).toBeVisible();
  await page.getByText('Selesai · 1/1 materi',{exact:true}).scrollIntoViewIfNeeded();await evidence(page,testInfo,'activity-participants-courses');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  const participantDownload=page.waitForEvent('download');await page.getByRole('button',{name:'Ekspor semua peserta',exact:true}).click();const participantFile=await participantDownload;const participantPath=testInfo.outputPath('participants.xlsx');await participantFile.saveAs(participantPath);
  const participantWorkbook=new(legacyRequire('exceljs').Workbook)();await participantWorkbook.xlsx.readFile(participantPath);const participantSheet=participantWorkbook.getWorksheet('Registrations');expect(participantSheet.rowCount).toBe(3);
  const headers=participantSheet.getRow(1).values;const completionColumn=headers.indexOf(`${courses[0].title} (#${courses[0].id}) - Status`);expect(completionColumn).toBeGreaterThan(0);expect(headers[completionColumn+1]).toBe(`${courses[0].title} (#${courses[0].id}) - Materi selesai/total`);
  const completedRow=[participantSheet.getRow(2),participantSheet.getRow(3)].find(row=>row.getCell(completionColumn).value==='Selesai');expect(completedRow).toBeTruthy();expect(completedRow.getCell(completionColumn+1).value).toBe('1/1');expect([participantSheet.getRow(2).getCell(completionColumn).value,participantSheet.getRow(3).getCell(completionColumn).value]).toContain('Tidak dapat diverifikasi');
  await page.goto(`/activity/${owner.id}/setup?step=2`);await expect(page.getByRole('button',{name:'Pilih kelas online',exact:true})).toBeVisible();await expect(page.getByText(courses[0].summary,{exact:true})).toBeVisible();
 }
});
