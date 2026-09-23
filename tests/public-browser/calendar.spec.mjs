import {test,expect,api,evidence} from '../browser/fixture.mjs';

test('public calendar navigation, private links and refreshed changes',async({page,fixture},testInfo)=>{
  const today=new Date(Date.now()+7*3600000).toISOString().slice(0,10);
  const next=new Date(Date.parse(`${today}T00:00:00Z`)+86400000).toISOString().slice(0,10);
  const activity=(await fixture.db.query("INSERT INTO activities(name,slug,is_published) VALUES('Kegiatan rahasia','calendar-private',false) RETURNING id")).rows[0];
  const input={title:'Pertemuan BMKA',description:'Agenda bersama seluruh tim.',location:'Salman',all_day:true,starts_at:`${today}T00:00:00+07:00`,ends_at:`${next}T00:00:00+07:00`,activity_id:activity.id};
  const created=await api('POST','/admin/calendar-events',input);
  await page.goto('http://localhost:3000/activity',{waitUntil:'domcontentloaded'});
  await expect(page.getByRole('heading',{level:1})).toBeVisible();
  await evidence(page,testInfo,'related-public-activity');
  if(testInfo.project.name==='mobile'){
    await page.getByRole('button',{name:'Buka menu',exact:true}).click();
    await page.getByRole('dialog').getByRole('link',{name:'Kalender BMKA',exact:true}).click();
    await expect(page.getByRole('dialog')).not.toBeVisible();
  }else{
    await page.getByRole('button',{name:'Tentang',exact:true}).click();
    await page.getByRole('menuitem',{name:'Kalender BMKA',exact:true}).click();
  }
  await expect(page).toHaveURL('http://localhost:3000/tentang/kalender-bmka');
  await expect(page.getByRole('heading',{name:'Kalender BMKA',exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:/Pertemuan BMKA/}).first()).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
  await evidence(page,testInfo,'public-calendar-default');
  await page.getByRole('button',{name:/Pertemuan BMKA/}).first().press('Enter');
  let dialog=page.getByRole('dialog');
  await expect(dialog.getByText('Agenda bersama seluruh tim.')).toBeVisible();
  await expect(dialog.getByText('Kegiatan rahasia')).toHaveCount(0);
  await dialog.getByRole('button',{name:'Tutup',exact:true}).click();
  await page.getByText(testInfo.project.name==='mobile'?'Bulan':'Agenda',{exact:true}).click();
  await expect(page.getByRole('button',{name:/Pertemuan BMKA/}).first()).toBeVisible();
  await evidence(page,testInfo,'public-calendar-alternate');
  await fixture.db.query('UPDATE activities SET is_published=true WHERE id=$1',[activity.id]);
  await api('PUT',`/admin/calendar-events/${created.id}`,{...input,title:'Pertemuan diperbarui'});
  await page.reload();
  await page.getByRole('button',{name:/Pertemuan diperbarui/}).first().click();
  dialog=page.getByRole('dialog');
  await expect(dialog.getByRole('link',{name:'Lihat kegiatan: Kegiatan rahasia'})).toHaveAttribute('href','/activity/calendar-private');
  await dialog.getByRole('button',{name:'Tutup',exact:true}).click();
  await page.route('**/v2/calendar-events?*',route=>route.fulfill({status:503,json:{message:'UNAVAILABLE'}}));
  await page.getByRole('button',{name:'Bulan berikutnya'}).click();
  await expect(page.getByText('Kalender gagal dimuat')).toBeVisible();
  await page.unroute('**/v2/calendar-events?*');
  await page.getByRole('button',{name:'Coba lagi'}).click();
  await expect(page.getByText('Kalender gagal dimuat')).not.toBeVisible();
  await api('DELETE',`/admin/calendar-events/${created.id}`);
  await page.getByRole('button',{name:'Hari ini'}).click();
  await expect(page.getByRole('button',{name:/Pertemuan diperbarui/})).toHaveCount(0);
  if(testInfo.project.name==='mobile') {
    await expect(page.getByText('Belum ada acara pada periode ini.')).toBeVisible();
  } else {
    await expect(page.getByRole('table')).toBeVisible();
    await expect(page.getByText('Belum ada acara pada periode ini.')).toHaveCount(0);
  }
});

test('busy dates retain every event and multi-day occurrences',async({page},testInfo)=>{
  const today=new Date(Date.now()+7*3600000).toISOString().slice(0,10);
  const at=Date.parse(`${today}T00:00:00Z`);
  const before=new Date(at-86400000).toISOString().slice(0,10);
  const after=new Date(at+2*86400000).toISOString().slice(0,10);
  for(let index=1;index<=8;index++)await api('POST','/admin/calendar-events',{title:`Agenda tim ${index}`,starts_at:`${before}T00:00:00+07:00`,ends_at:`${after}T00:00:00+07:00`,all_day:true,activity_id:null});
  await page.goto('http://localhost:3000/tentang/kalender-bmka');
  if(testInfo.project.name==='mobile')await page.getByText('Bulan',{exact:true}).click();
  const day=page.locator('td').filter({has:page.locator(`time[datetime="${today}"]`)});
  await expect(day.getByRole('button')).toHaveCount(8);
  const last=day.getByRole('button',{name:/Agenda tim 8/});
  await last.scrollIntoViewIfNeeded();
  await last.press('Enter');
  await expect(page.getByRole('dialog')).toContainText('Agenda tim 8');
  await page.getByRole('dialog').getByRole('button',{name:'Tutup',exact:true}).click();
  await page.getByText('Agenda',{exact:true}).click();
  const occurrences=page.getByRole('button',{name:/Agenda tim 8/});
  await expect.poll(() => occurrences.count()).toBeGreaterThanOrEqual(2);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
});
