import {test,expect,login,api} from './fixture.mjs';

async function evidence(page,testInfo,name) {
  const path=testInfo.outputPath(name+'.png');
  await page.screenshot({path,fullPage:true,animations:'disabled'});
  await testInfo.attach(name,{path,contentType:'image/png'});
}

async function noOverflow(page) {
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
}

test('Super Admin confirms activity deletion, cancellation and stale-name failure',async({page,fixture},testInfo)=>{
  const activity=await api('POST','/activities',{name:'Kegiatan Uji Penghapusan'});
  await login(page);
  await page.goto(`/activity/${activity.id}`);
  await expect(page.getByRole('button',{name:'Hapus Kegiatan',exact:true})).toBeVisible();
  await page.goto(`/activity/${activity.id}/setup`);
  const trigger=page.getByRole('button',{name:'Hapus Kegiatan',exact:true});
  await expect(trigger).toBeVisible();
  await evidence(page,testInfo,'activity-before-deletion');
  await trigger.click();
  const dialog=page.getByRole('dialog');
  const remove=dialog.getByRole('button',{name:'Hapus Kegiatan Permanen',exact:true});
  await expect(remove).toBeDisabled();
  await dialog.getByLabel('Nama kegiatan',{exact:true}).focus();
  await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();
  await trigger.focus();
  await page.keyboard.press('Enter');
  await expect(dialog).toBeVisible();
  await dialog.getByLabel('Nama kegiatan',{exact:true}).fill('wrong');
  await expect(remove).toBeDisabled();
  await dialog.getByRole('button',{name:'Batal',exact:true}).click();
  await expect(dialog).not.toBeVisible();
  await trigger.click();
  await expect(dialog.getByLabel('Nama kegiatan',{exact:true})).toHaveValue('');
  await dialog.getByLabel('Nama kegiatan',{exact:true}).fill(activity.name);
  await expect(remove).toBeEnabled();
  await evidence(page,testInfo,'activity-confirmation');
  await noOverflow(page);
  await fixture.db.query('UPDATE activities SET name=$1 WHERE id=$2',['Nama terbaru',activity.id]);
  const rejected=page.waitForResponse(r=>r.request().method()==='DELETE');
  await remove.click();
  expect((await rejected).status()).toBe(422);
  await expect(dialog.getByText(/Nama konfirmasi tidak cocok/)).toBeVisible();
  await evidence(page,testInfo,'activity-deletion-error');
  await dialog.getByRole('button',{name:'Batal',exact:true}).click();
  await page.reload();
  await trigger.click();
  await dialog.getByLabel('Nama kegiatan',{exact:true}).fill('Nama terbaru');
  await remove.click();
  await expect(page).toHaveURL(/\/activity$/);
  expect((await fixture.db.query('SELECT id FROM activities WHERE id=$1',[activity.id])).rowCount).toBe(0);
});

test('Asisten Manager deletes club while preserving its activities and members',async({page,fixture},testInfo)=>{
  const club=await api('POST','/clubs',{name:'Klub Uji Penghapusan',club_type:'UNIT'});
  const activity=await api('POST','/activities',{name:'Kegiatan yang Dipertahankan',club_id:club.id});
  await api('POST','/members',{name:'Anggota Uji',email:'deletion-member@example.test',gender:'F'});
  await fixture.db.query("INSERT INTO club_registrations(club_id,member_id,status,created_at) VALUES($1,1,'APPROVED',now())",[club.id]);
  await login(page);
  await fixture.db.query("UPDATE admin_users SET role_code='admin' WHERE id=1");
  await page.goto(`/club/${club.id}`);
  const trigger=page.getByRole('button',{name:'Hapus Klub',exact:true});
  await expect(trigger).toBeVisible();
  await evidence(page,testInfo,'club-before-deletion');
  await trigger.click();
  const dialog=page.getByRole('dialog');
  const remove=dialog.getByRole('button',{name:'Hapus Klub Permanen',exact:true});
  await expect(remove).toBeDisabled();
  await dialog.getByLabel('Nama klub',{exact:true}).fill(club.name.toLowerCase());
  await expect(remove).toBeDisabled();
  await dialog.getByLabel('Nama klub',{exact:true}).fill(club.name);
  await evidence(page,testInfo,'club-confirmation');
  await noOverflow(page);
  await remove.click();
  await expect(page).toHaveURL(/\/club$/);
  expect((await fixture.db.query('SELECT id FROM clubs WHERE id=$1',[club.id])).rowCount).toBe(0);
  expect((await fixture.db.query('SELECT club_id FROM activities WHERE id=$1',[activity.id])).rows[0].club_id).toBe(null);
  expect((await fixture.db.query('SELECT id FROM public_users WHERE id=1')).rowCount).toBe(1);
  expect((await fixture.db.query('SELECT id FROM club_registrations WHERE club_id=$1',[club.id])).rowCount).toBe(0);
});

test('program and club managers cannot see deletion controls',async({page,fixture},testInfo)=>{
  const activity=await api('POST','/activities',{name:'Kegiatan Terbatas'});
  const club=await api('POST','/clubs',{name:'Klub Terbatas',club_type:'UNIT'});
  await login(page);
  for(const role of ['activity_manager','achievement_manager']) {
    await fixture.db.query('UPDATE admin_users SET role_code=$1 WHERE id=1',[role]);
    await page.goto(`/activity/${activity.id}/setup`);
    await expect(page.getByRole('heading',{name:activity.name,exact:true})).toBeVisible();
    await expect(page.getByRole('button',{name:'Hapus Kegiatan',exact:true})).toHaveCount(0);
  }
  await fixture.db.query("UPDATE admin_users SET role_code='club_manager' WHERE id=1");
  await page.goto(`/club/${club.id}`);
  await expect(page.getByRole('heading',{name:club.name,exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:'Hapus Klub',exact:true})).toHaveCount(0);
  await evidence(page,testInfo,'club-manager-controls');
});
