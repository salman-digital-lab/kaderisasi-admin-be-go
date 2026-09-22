import assert from 'node:assert/strict';
import {mkdirSync,readFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import {resolve} from 'node:path';
import {parseEnv} from 'node:util';
import {chromium,expect} from '@playwright/test';
import {root,workspace} from './env.mjs';
import {fixturePassword} from './fixture-db.mjs';
import {borrowWorkspacePort,startAdminFrontend,startPublicFrontend} from './server-process.mjs';

const {PDFDocument}=createRequire(resolve(workspace,'kaderisasi-web-be/package.json'))('pdf-lib');

export async function runCertificateScoresBrowser({certificate,activity,registration,guest:guestRegistration,call,fixture,participantToken,record,restorations}) {
  const directory=resolve(root,'.artifacts/certificate-scores');
  mkdirSync(directory,{recursive:true});
  let adminFE,publicFE,browser;
  const errors=[];
  const code=certificate.certificate.certificate_code;
  const axePath=createRequire(resolve(workspace,'kaderisasi-web-fe/package.json')).resolve('axe-core/axe.min.js');
  async function accessibility(page,selector,label) {
    await page.evaluate(readFileSync(axePath,'utf8'));
    const violations=await page.evaluate(async selector=>(await window.axe.run(selector,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa','wcag22aa']}})).violations,selector);
    assert.deepEqual(violations.map(({id,nodes})=>({id,targets:nodes.map(node=>node.target)})),[],`${label}: accessibility violations`);
  }
  async function evidence(page,label) {
    const first=page.getByRole('tab',{name:/1\. Sertifikat$/});
    const second=page.getByRole('tab',{name:/2\. Hasil penilaian$/});
    await first.click();
    await page.evaluate(()=>scrollTo(0,0));
    const button=page.getByRole('button',{name:/Unduh PDF/});
    await expect(button).toBeInViewport();
    assert.ok((await button.boundingBox()).height>=44,`${label}: download target too small`);
    await page.screenshot({path:resolve(directory,`${label}-artwork.png`),fullPage:true,animations:'disabled'});
    await first.focus();
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Enter');
    await expect(second).toBeFocused();
    await expect(second).toHaveAttribute('aria-selected','true');
    assert.ok((await second.boundingBox()).height>=44,`${label}: tab target too small`);
    await expect(page.getByRole('heading',{name:'Hasil penilaian',exact:true})).toBeVisible();
    await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
    const metrics=await page.locator('[data-certificate-score-details] dd').evaluateAll(nodes=>nodes.map(node=>{
      const rect=node.getBoundingClientRect();
      return {font:parseFloat(getComputedStyle(node).fontSize),left:rect.left,right:rect.right,width:innerWidth};
    }));
    assert.ok(metrics.length>0&&metrics.every(metric=>metric.font>=14&&metric.left>=0&&metric.right<=metric.width),`${label}: unreadable score metrics`);
    await accessibility(page,'[role="tabpanel"]:not([hidden]):not([style*="display: none"])',label);
    const sheet=page.locator('[data-certificate-score-sheet]');
    if(await sheet.count()) {
      const fits=await sheet.evaluate(sheet=>{
        const body=sheet.querySelector('[data-score-sheet-body]').getBoundingClientRect();
        const footer=sheet.querySelector('footer').getBoundingClientRect();
        return body.bottom<footer.top;
      });
      assert.ok(fits,`${label}: print score sheet overlaps footer`);
    }
    await page.evaluate(()=>scrollTo(0,0));
    await page.screenshot({path:resolve(directory,`${label}.png`),fullPage:!label.includes('dense'),animations:'disabled'});
  }
  async function failureAndRetry(page,url,label) {
    await page.route(url,route=>route.fulfill({status:503,json:{message:'Temporary fixture failure'}}));
    await page.getByRole('button',{name:/Unduh PDF/}).click();
    await expect(page.getByRole('alert').filter({hasText:'Unduhan belum berhasil'})).toBeVisible();
    await page.screenshot({path:resolve(directory,`${label}-retry.png`),fullPage:true,animations:'disabled'});
    await page.unroute(url);
    await download(page,`${label}-retry-two-pages`,2,/Coba unduh lagi/);
    await expect(page.getByRole('alert').filter({hasText:'Unduhan belum berhasil'})).toHaveCount(0);
  }
  async function download(page,label,pageCount=2,buttonName=/Unduh PDF/) {
    const path=resolve(directory,`${label}.pdf`);
    let downloaded;
    try {
      [downloaded]=await Promise.all([
        page.waitForEvent('download',{timeout:60000}),
        page.getByRole('button',{name:buttonName}).click(),
      ]);
    } catch(error) {
      await page.screenshot({path:resolve(directory,`${label}-failure.png`),fullPage:true});
      console.error('Certificate download state:',await page.locator('body').innerText());
      throw error;
    }
    await downloaded.saveAs(path);
    await expect.poll(()=>page.getByRole('button',{name:/Unduh PDF/}).evaluate(node=>!node.disabled&&!node.hasAttribute('data-loading')&&!node.classList.contains('ant-btn-loading'))).toBe(true);
    const pdf=await PDFDocument.load(readFileSync(path));
    assert.equal(pdf.getPageCount(),pageCount,`${label}: unexpected PDF page count`);
    const [front,scores]=pdf.getPages();
    assert.ok(front.getWidth()>front.getHeight());
    if(scores)assert.ok(scores.getHeight()>scores.getWidth());
    return path;
  }
  try {
    const reusePublic=process.argv.includes('--reuse-public-frontend');
    for(const port of reusePublic?[3005]:[3005,3000])restorations.push(await borrowWorkspacePort(port,process.argv.includes('--borrow-workspace')));
    adminFE=await startAdminFrontend({...process.env,...parseEnv(readFileSync(resolve(workspace,'docs/.env.test.admin-fe'),'utf8'))});
    if(reusePublic) {
      const env=parseEnv(readFileSync(resolve(workspace,'kaderisasi-web-fe/.env.local'),'utf8'));
      assert.match(env.SERVER_BE_API||env.NEXT_PUBLIC_BE_API,/^http:\/\/(localhost|127\.0\.0\.1):3333\/v2$/,'Reused frontend must read the local fixture API');
      assert.equal((await fetch('http://localhost:3000')).status,200);
    } else publicFE=await startPublicFrontend({...process.env,...parseEnv(readFileSync(resolve(workspace,'docs/.env.test.web-fe'),'utf8'))});
    browser=await chromium.launch();
    const adminContext=await browser.newContext({viewport:{width:1440,height:1000}});
    const admin=await adminContext.newPage();
    admin.on('pageerror',error=>errors.push(error.message));
    await admin.goto('http://localhost:3005/login');
    await admin.getByLabel('Email',{exact:true}).fill('super@example.test');
    await admin.getByLabel('Password',{exact:true}).fill(fixturePassword);
    await admin.getByRole('button',{name:/\bLogin$/}).click();
    await expect(admin).toHaveURL(/\/dashboard$/);
    await admin.goto(`http://localhost:3005/certificate-preview/${certificate.certificate.id}`);
    await evidence(admin,'admin-desktop');
    await download(admin,'admin-two-pages');
    await admin.setViewportSize({width:390,height:844});
    await evidence(admin,'admin-mobile');
    await accessibility(admin,'main','admin preview');
    const apiPath=`**/v2/certificates/${certificate.certificate.id}`;
    await failureAndRetry(admin,apiPath,'admin');
    await admin.setViewportSize({width:320,height:844});
    await evidence(admin,'admin-narrow');
    const dense=structuredClone(certificate);
    dense.participant.name='Peserta uji dengan nama panjang untuk memeriksa tata letak sertifikat';
    dense.participant.scoring_result.rubric.groups=Array.from({length:10},(_,group)=>({id:`group${group}`,name:`Kelompok penilaian ${group+1}`,criteria:Array.from({length:10},(_,criterion)=>({id:`c${group}-${criterion}`,name:`Aspek penilaian ${group+1}.${criterion+1}`,maximum:100,weight:1}))}));
    dense.participant.scoring_result.result.criteria=dense.participant.scoring_result.rubric.groups.flatMap(group=>group.criteria.map(criterion=>({criterion_id:criterion.id,score:0,normalized:0,grade:'B'})));
    dense.participant.scoring_result.result.total=0;
    dense.participant.scoring_result.result.grade='B';
    dense.participant.scoring_result.note='Catatan uji Unicode: العربية 日本語.\nSemua aspek memiliki nilai nol.';
    await admin.route(apiPath,route=>route.fulfill({json:{data:dense}}));
    await admin.reload();
    await admin.setViewportSize({width:1440,height:1000});
    await evidence(admin,'admin-dense');
    await expect(admin.locator('[data-certificate-score-sheet] tbody')).toHaveCount(100);
    await expect(admin.locator('[data-certificate-score-details] li')).toHaveCount(100);
    await download(admin,'admin-dense-two-pages');
    await admin.unroute(apiPath);
    const legacy=structuredClone(certificate);
    delete legacy.participant.scoring_result;
    await admin.route(apiPath,route=>route.fulfill({json:{data:legacy}}));
    await admin.reload();
    await expect(admin.locator('[data-certificate-score-sheet]')).toHaveCount(0);
    await expect(admin.getByRole('tab')).toHaveCount(0);
    await expect(admin.getByText('PDF berisi 1 halaman.',{exact:false})).toBeVisible();
    await download(admin,'legacy-one-page',1);
    await admin.unroute(apiPath);

    const ownerContext=await browser.newContext({viewport:{width:1440,height:1000}});
    await ownerContext.addCookies([{name:'session',value:participantToken,domain:'localhost',path:'/'}]);
    const owner=await ownerContext.newPage();
    owner.on('pageerror',error=>errors.push(error.message));
    await owner.goto(`http://localhost:3000/certificate/${code}`);
    await evidence(owner,'owner-desktop');
    await expect(owner.locator('[data-certificate-score-details]')).toContainText('81,6 / 100');
    await download(owner,'owner-two-pages');
    await owner.setViewportSize({width:390,height:844});
    await evidence(owner,'owner-mobile');
    await failureAndRetry(owner,`**/api/certificates/${code}/download`,'owner');
    await owner.setViewportSize({width:320,height:844});
    await evidence(owner,'owner-narrow');

    await call('admin','UX fixture eligible guest without published scores','PUT','/activity-registrations',{registrations_id:[guestRegistration.id],status:'LULUS KEGIATAN'});
    await admin.setViewportSize({width:1440,height:1000});
    await admin.goto(`http://localhost:3005/activity/${activity.id}/certificates`);
    await admin.getByRole('button',{name:/Lanjut ke penerima/}).click();
    await expect(admin.getByText('Ingin menyertakan nilai pada halaman kedua?')).toBeVisible();
    await admin.screenshot({path:resolve(directory,'issuance-guidance-desktop.png'),fullPage:true});
    await admin.setViewportSize({width:390,height:844});
    await expect.poll(()=>admin.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
    await admin.screenshot({path:resolve(directory,'issuance-guidance-mobile.png'),fullPage:true});
    await admin.getByRole('button',{name:/Tinjau 1 sertifikat/}).click();
    await expect(admin.getByText('Contoh: Peserta Tamu · 1 halaman')).toBeVisible();
    await admin.screenshot({path:resolve(directory,'issuance-single-page-warning.png'),fullPage:true});
    await admin.getByRole('button',{name:'Kembali',exact:true}).click();
    await admin.getByRole('link',{name:'Buka penilaian kegiatan'}).click();
    await expect(admin).toHaveURL(new RegExp(`/activity/${activity.id}\\?tab=scoring$`));
    await expect(admin.getByRole('heading',{name:'Penilaian kegiatan',exact:true})).toBeVisible();

    const scoringPath=`/activities/${activity.id}/scoring`;
    const entries=await call('admin','read scoring revision for profile UX','GET',scoringPath);
    const entry=entries.entries.find(entry=>entry.registration_id===registration.id);
    await call('admin','republish fixture for profile UX','POST',`${scoringPath}/publish`,{rubric_revision:1,selections:[{registration_id:registration.id,revision:entry.scoring_data.revision}]});
    await fixture.db.query("UPDATE activities SET is_published=true,description='Kegiatan pengembangan karakter' WHERE id=$1",[activity.id]);
    await owner.goto(`http://localhost:3000/profile/activity/${activity.slug}`);
    await expect(owner.getByRole('heading',{name:'Hasil penilaian',exact:true})).toBeVisible();
    await expect(owner.getByText('81,6 / 100 · A',{exact:true})).toBeVisible();
    for(const width of [390,1440]) {
      await owner.setViewportSize({width,height:1000});
      await expect.poll(()=>owner.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
      await owner.screenshot({path:resolve(directory,`activity-result-${width}.png`),fullPage:true});
    }
    record.checks.push({label:'Certificate publication guidance, single-page warning, scoring navigation and consistent activity result layout',screenshots:directory});

    const guestContext=await browser.newContext({viewport:{width:390,height:844}});
    const guest=await guestContext.newPage();
    await guest.goto(`http://localhost:3000/certificate/${code}`);
    await expect(guest.getByRole('link',{name:'Masuk untuk unduh'})).toBeVisible();
    await expect(guest.locator('[data-certificate-score-sheet]')).toHaveCount(0);
    await expect(guest.locator('[data-certificate-score-details]')).toHaveCount(0);
    await expect(guest.getByRole('tab')).toHaveCount(0);
    assert.ok(!(await guest.content()).includes('scoring_result'),'Scores leaked in public HTML/RSC');
    await guest.screenshot({path:resolve(directory,'public-mobile.png'),fullPage:true});
    assert.deepEqual(errors,[],'Certificate browser errors');
    record.checks.push({label:'Certificate UX: keyboard page tabs, readable scores at 320/390/1440px, accessible score panels, visible downloads, retry recovery, two-page/legacy PDFs, public privacy',screenshots:directory});
  } finally {await browser?.close();await publicFE?.stop();await adminFE?.stop();}
}
