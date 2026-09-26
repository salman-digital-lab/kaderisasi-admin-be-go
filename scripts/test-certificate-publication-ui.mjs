import {chromium, expect} from '@playwright/test';
import {mkdirSync,writeFileSync} from 'node:fs';
const dir='.artifacts/certificate-publication-ui';mkdirSync(dir,{recursive:true});
const browser=await chromium.launch();const results=[];
try {
 for(const [name,viewport] of [['desktop',{width:1440,height:1000}],['mobile',{width:390,height:844}]]) {
  const page=await browser.newPage({viewport});const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto('http://localhost:3005/tests/browser/certificate.html?count=1');
  await page.getByRole('button',{name:'Lanjut ke penerima',exact:true}).click();
  await page.getByRole('button',{name:/Tinjau 1 sertifikat$/}).click();
  await expect(page.getByText('Admin pemberi persetujuan',{exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'Terbitkan 1 sertifikat',exact:true}).click();
  const modal=page.getByRole('dialog');await expect(modal).toBeVisible();
  await expect(modal.getByText('Oktofa Yudha Sudrajad, S.T., M.S.M., Ph.D.',{exact:true})).toBeVisible();
  await modal.getByRole('button',{name:'Batal',exact:true}).click();
  const metrics=()=>page.locator('#lab-metrics').textContent().then(JSON.parse);
  expect((await metrics()).issued).toBe(0);
  await page.getByRole('button',{name:'Terbitkan 1 sertifikat',exact:true}).click();
  await expect(modal).toBeVisible();
  await page.waitForTimeout(500);
  await page.screenshot({path:`${dir}/${name}-publish.png`,fullPage:false});
  await modal.getByRole('button',{name:'Terbitkan dan tanda tangani',exact:true}).click();
  await expect.poll(async()=>(await metrics()).issued).toBe(1);
  await expect(modal).not.toBeVisible();
  await page.getByRole('button',{name:'Kembali',exact:true}).click();
  if(name==='mobile') await page.locator('summary').click();
  await page.getByRole('button',{name:'Batalkan penerbitan',exact:true}).click();
  await expect(modal).toBeVisible();
  await modal.getByRole('button',{name:'Kembali',exact:true}).click();
  await expect(modal).not.toBeVisible();
  expect((await metrics()).issued).toBe(1);
  await page.getByRole('button',{name:'Batalkan penerbitan',exact:true}).click();
  await expect(modal).toBeVisible();
  await page.waitForTimeout(500);
  await page.screenshot({path:`${dir}/${name}-withdraw.png`,fullPage:false});
  await modal.getByRole('button',{name:'Batalkan penerbitan',exact:true}).click();
  await expect.poll(async()=>(await metrics()).issued).toBe(0);
  await expect(modal).not.toBeVisible();

  await page.screenshot({path:`${dir}/${name}-after-withdraw.png`,fullPage:false});
  await page.getByRole('button',{name:/Tinjau 1 sertifikat$/}).click();

  await page.getByRole('button',{name:'Terbitkan 1 sertifikat',exact:true}).click();
  await modal.getByRole('button',{name:'Terbitkan dan tanda tangani',exact:true}).press('Enter');
  await expect.poll(async()=>(await metrics()).issued).toBe(1);
  expect((await metrics()).calls.filter(x=>x.includes('/certificates/approvals'))).toEqual([]);
  expect(errors).toEqual([]);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  results.push({viewport:name,passed:true});await page.close();
 }
}finally{writeFileSync(`${dir}/results.json`,JSON.stringify(results,null,2));await browser.close();}
console.log(results);
