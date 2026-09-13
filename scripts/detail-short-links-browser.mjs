import {expect} from '@playwright/test';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';

export async function detailShortLinksBrowser(page, db, schema, artifacts) {
 const activity={id:91001,name:'Kegiatan QR',slug:'kegiatan-qr',is_published:0,is_registration_open:false,additional_config:{},images:[]};
 const club={id:91002,name:'Club QR',club_type:'UNIT',is_show:false,is_registration_open:false,media:{items:[]}};
 await page.route('**/v2/activities/91001',r=>r.fulfill({json:{data:activity}}));
 await page.route('**/v2/activities/91001/readiness',r=>r.fulfill({json:{data:{can_publish:false,can_open_registration:false,issues:[],actions:{can_edit:false,can_publish:false,can_manage_registration:false}}}}));
 await page.route('**/v2/clubs/91002',r=>r.fulfill({json:{data:club}}));
 for(const [kind,path,destination] of [['activity','/activity/91001','http://127.0.0.1:3000/activity/kegiatan-qr'],['club','/club/91002','http://127.0.0.1:3000/clubs/91002']]) {
  await page.setViewportSize({width:1280,height:900});await page.goto('http://localhost:3005'+path);
  const card=page.locator('.ant-card').filter({has:page.getByText('Tautan Pendek & QR',{exact:true})});
  await expect(card.getByText('Halaman belum tayang',{exact:true})).toBeVisible();
  await expect(card.getByText('Alamat tujuan: '+destination,{exact:true})).toBeVisible();
  await card.getByRole('button',{name:'Buat tautan pendek & QR',exact:true}).click();
  const dialog=page.getByRole('dialog');await expect(dialog).toBeVisible();
  await dialog.getByRole('button',{name:'Tutup',exact:true}).click();
  const short=await card.getByLabel('Tautan pendek halaman').inputValue();assert.match(short,/^http:\/\/localhost:4000\/[A-Za-z0-9]{6}$/);
  const rows=await db.query(`SELECT original_url FROM "${schema}".urls WHERE id=$1`,[short.split('/').pop()]);assert.equal(rows.rows[0].original_url,destination);
  await page.reload();await expect(card.getByLabel('Tautan pendek halaman')).toHaveValue(short);
  await expect(card.getByRole('button',{name:'Buat tautan pendek & QR',exact:true})).toHaveCount(0);
  await card.getByRole('button',{name:'Salin tautan',exact:true}).click();assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),short);
  await card.scrollIntoViewIfNeeded();await page.screenshot({path:resolve(artifacts,kind+'-short-link-desktop.png'),animations:'disabled'});
  await page.setViewportSize({width:390,height:844});await card.scrollIntoViewIfNeeded();await page.waitForTimeout(350);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  await page.screenshot({path:resolve(artifacts,kind+'-short-link-mobile.png'),animations:'disabled'});
  await card.getByRole('button',{name:'Kode QR',exact:true}).click();await expect(dialog.getByText(short,{exact:true})).toBeVisible();
  const download=page.waitForEvent('download');await dialog.getByRole('button',{name:'Unduh PNG',exact:true}).click();assert.equal((await download).suggestedFilename(),`short-link-${short.split('/').pop()}.png`);
  await dialog.getByRole('button',{name:'Tutup',exact:true}).press('Escape');await expect(dialog).not.toBeVisible();
 }
 console.log('Kegiatan and Club short-link creation, persistence, copy and QR passed');
}
