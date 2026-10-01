import { test, expect, api, login, evidence } from './fixture.mjs';
import { legacyRequire, fixtureKey } from '../../scripts/fixture-db.mjs';

async function participant(method,path,body) {
  const token=legacyRequire('jsonwebtoken').sign({userId:2,email:'requester@example.test'},fixtureKey,{expiresIn:'15m'});
  const response=await fetch('http://localhost:3334/v2'+path,{method,headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});
  expect(response.ok,await response.clone().text()).toBe(true);
  return (await response.json()).data;
}

test('self assessment: keyboard, saved progress, review, submission, replacement and super admin result',async({page,fixture},testInfo)=>{
  test.setTimeout(240000);
  const errors=[];page.on('pageerror',error=>errors.push(error.message));
  await login(page,'requester@example.test');
  await page.goto('/profile');
  await expect(page.getByRole('button',{name:'Mulai Asesmen',exact:true})).toBeVisible();
  const primaryColor=await page.getByRole('button',{name:'Mulai Asesmen',exact:true}).evaluate(button=>getComputedStyle(button).backgroundColor);
  expect(primaryColor).toBe('rgb(20, 123, 164)');
  await evidence(page,testInfo,'profile');
  await page.getByRole('button',{name:'Mulai Asesmen',exact:true}).click();
  await expect(page.getByText('Jawab sesuai diri Anda sehari-hari')).toBeVisible();
  await page.getByRole('button',{name:'Mulai Asesmen',exact:true}).click();
  await expect(page.getByRole('button',{name:'Berikutnya',exact:true})).toBeDisabled();
  const definition=await participant('GET','/talent-assessment/definition');
  await expect(page.locator('#talent-description')).toHaveText(definition.questions[0].description);
  await expect(page.getByRole('radiogroup')).toHaveAttribute('aria-describedby','talent-description');
  const agree=page.getByRole('radio',{name:'Setuju',exact:true});
  await agree.focus();await page.keyboard.press('Space');
  await expect(agree).toBeChecked();
  await page.getByRole('button',{name:'Berikutnya',exact:true}).click();
  await expect(page.getByText('Pernyataan 2 dari 170',{exact:true})).toBeVisible();
  await expect(page.locator('#talent-description')).toHaveText(definition.questions[1].description);
  await page.getByRole('button',{name:'Sebelumnya',exact:true}).click();
  await expect(agree).toBeChecked();
  await page.getByRole('radio',{name:'Sangat Setuju',exact:true}).check();
  await expect(page.getByRole('status')).toHaveText('Semua perubahan tersimpan');
  const contrast=await page.getByRole('button',{name:'Berikutnya',exact:true}).evaluate(button=>{
    const style=getComputedStyle(button);
    const luminance=color=>{
      const rgb=color.match(/[\d.]+/g).slice(0,3).map(Number).map(value=>value/255).map(value=>value<=0.04045?value/12.92:((value+0.055)/1.055)**2.4);
      return rgb[0]*0.2126+rgb[1]*0.7152+rgb[2]*0.0722;
    };
    const a=luminance(style.color),b=luminance(style.backgroundColor);
    return (Math.max(a,b)+0.05)/(Math.min(a,b)+0.05);
  });
  expect(contrast).toBeGreaterThanOrEqual(4.5);
  await evidence(page,testInfo,'assessment');
  expect(await page.locator('body').evaluate(body=>body.scrollWidth<=window.innerWidth)).toBe(true);
  await page.getByRole('button',{name:'Simpan & Keluar',exact:true}).click();
  await expect(page).toHaveURL(/\/profile$/);
  await expect(page.getByRole('button',{name:'Lanjutkan Asesmen',exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Lanjutkan Asesmen',exact:true}).click();
  await expect(page.getByRole('radio',{name:'Sangat Setuju',exact:true})).toBeChecked();
  await page.reload();
  await expect(page.getByRole('radio',{name:'Sangat Setuju',exact:true})).toBeChecked();
  // Complete every statement through the actual radio and explicit Next controls.
  for(let number=1;number<=170;number++) {
    await expect(page.getByText(`Pernyataan ${number} dari 170`,{exact:true})).toBeVisible();
    await expect(page.locator('#talent-description')).toHaveText(definition.questions[number-1].description);
    await page.getByRole('radio',{name:'Sangat Setuju',exact:true}).check();
    await page.getByRole('button',{name:number===170?'Periksa Jawaban':'Berikutnya',exact:true}).first().click();
  }
  await expect(page.getByText('Periksa sebelum mengirim',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Tinjau Jawaban',exact:true}).click();
  await page.getByRole('button',{name:'Pernyataan 11, terisi',exact:true}).click();
  await page.getByRole('radio',{name:'Setuju',exact:true}).check();
  await page.getByRole('button',{name:'Periksa Jawaban',exact:true}).click();
  await page.getByRole('button',{name:'Kirim Asesmen',exact:true}).click();
  await expect(page).toHaveURL(/\/profile\/talent-assessment\/result$/);
  await expect(page.getByText('7 Bakat Menonjol',{exact:true})).toBeVisible();
  await expect(page.locator('.talent-score-list li')).toHaveCount(34);
  await evidence(page,testInfo,'results');
  const original=(await participant('GET','/talent-assessment')).result;
  // Retaking preserves the submitted result until replacement, with no history.
  await page.getByRole('button',{name:/Ulangi Asesmen/}).click();
  await page.getByRole('button',{name:'Ulangi Asesmen',exact:true}).click();
  await page.getByRole('dialog').getByRole('button',{name:'Ulangi Asesmen',exact:true}).click();
  await expect(page.getByText('Pernyataan 1 dari 170',{exact:true})).toBeVisible();
  let state=await participant('GET','/talent-assessment');expect(state.result.submission_id).toBe(original.submission_id);
  const saved=await participant('PUT','/talent-assessment/draft',{...state.draft,answers:Array(170).fill(1),current_question:170});
  await page.reload();
  await page.getByRole('button',{name:'Periksa Jawaban',exact:true}).first().click();
  await page.getByRole('button',{name:'Kirim Asesmen',exact:true}).click();
  await expect(page).toHaveURL(/\/result$/);
  state=await participant('GET','/talent-assessment');expect(state.draft).toBeNull();expect(state.result.submission_id).toBe(saved.draft_id);
  expect(state.result.talents.every(theme=>theme.score===0)).toBe(true);
  expect((await fixture.db.query('SELECT count(*)::int AS n FROM talent_assessment_results WHERE admin_user_id=2')).rows[0].n).toBe(1);
  await page.goto('/admin-users/1/talent-assessment/result');
  await expect(page.getByText('Anda tidak memiliki akses ke hasil ini.',{exact:true})).toBeVisible();
  await page.context().clearCookies();await page.goto('/login');await login(page);
  await page.goto('/admin-users');
  await expect(page.getByRole('button',{name:'Lihat Hasil Bakat'}).first()).toBeVisible();
  await page.goto('/admin-users/2/talent-assessment/result');
  await expect(page.getByText('7 Bakat Menonjol',{exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:/Ulangi Asesmen/})).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('save failure retains edits, concurrent tabs require reload, and layout works at 200 percent text',async({page},testInfo)=>{
  await login(page,'requester@example.test');
  await page.goto('/profile/talent-assessment');
  await page.getByRole('button',{name:'Mulai Asesmen',exact:true}).click();
  let rejectSave=true;
  await page.route('**/v2/talent-assessment/draft',async route=>{
    if(route.request().method()==='PUT'&&rejectSave) return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({message:'TEST_SAVE_FAILURE'})});
    return route.continue();
  });
  await page.getByRole('radio',{name:'Setuju',exact:true}).check();
  await expect(page.getByText('Jawaban belum tersimpan. Periksa koneksi, lalu coba simpan lagi.')).toBeVisible();
  await expect(page.getByRole('radio',{name:'Setuju',exact:true})).toBeChecked();
  rejectSave=false;
  await page.getByRole('button',{name:'Coba Simpan Lagi'}).click();
  await expect(page.getByRole('status')).toHaveText('Semua perubahan tersimpan');
  const state=await participant('GET','/talent-assessment');
  await participant('PUT','/talent-assessment/draft',{...state.draft,answers:state.draft.answers.map((v,i)=>i===0?6:v)});
  await page.getByRole('radio',{name:'Tidak Setuju',exact:true}).check();
  await expect(page.getByText(/Draf berubah di tab lain/)).toBeVisible();
  await expect(page.getByRole('button',{name:'Berikutnya',exact:true})).toBeDisabled();
  await page.getByRole('button',{name:'Muat Draf Terbaru'}).click();
  await page.getByRole('dialog').getByRole('button',{name:'Muat Draf',exact:true}).click();
  await expect(page.getByRole('radio',{name:'Sangat Setuju',exact:true})).toBeChecked();
  await page.evaluate(()=>{document.documentElement.style.fontSize='200%';});
  expect(await page.locator('body').evaluate(body=>body.scrollWidth<=window.innerWidth)).toBe(true);
  await evidence(page,testInfo,'assessment-text-zoom');
  await page.getByRole('button',{name:'Daftar Pertanyaan'}).click();
  await page.getByRole('button',{name:'Pernyataan 170, belum terisi',exact:true}).click();
  await expect(page.getByRole('button',{name:'Periksa Jawaban',exact:true})).toBeDisabled();
});

test('edits during a slow save are preserved and failed submission can be retried after server completion',async({page})=>{
  await login(page,'requester@example.test');
  await page.goto('/profile/talent-assessment');
  await page.getByRole('button',{name:'Mulai Asesmen',exact:true}).click();
  let release;
  const waiting=new Promise(done=>{release=done;});
  let saving=false, first=true;
  await page.route('**/v2/talent-assessment/draft',async route=>{
    if(route.request().method()==='PUT'&&first) {first=false;saving=true;await waiting;}
    await route.continue();
  });
  await page.getByRole('radio',{name:'Sangat Setuju',exact:true}).check();
  await expect.poll(()=>saving).toBe(true);
  await page.getByRole('button',{name:'Berikutnya',exact:true}).click();
  await page.getByRole('radio',{name:'Setuju',exact:true}).check();
  release();
  await expect(page.getByRole('status')).toHaveText('Semua perubahan tersimpan');
  let state=await participant('GET','/talent-assessment');
  expect(state.draft.answers.slice(0,2)).toEqual([6,5]);expect(state.draft.current_question).toBe(2);
  // Save failure should also block sidebar navigation until the user chooses.
  await page.route('**/v2/talent-assessment/draft',async route=>{
    if(route.request().method()==='PUT') return route.fulfill({status:503,contentType:'application/json',body:'{"message":"TEST_SAVE_FAILURE"}'});
    return route.continue();
  });
  await page.getByRole('radio',{name:'Tidak Setuju',exact:true}).check();
  await expect(page.getByText(/Jawaban belum tersimpan\. Periksa koneksi/)).toBeVisible();
  await page.getByRole('button',{name:/Menu akun/}).click();
  await page.getByText('Profil Saya',{exact:true}).last().click();
  await expect(page.getByRole('dialog',{name:'Jawaban belum tersimpan'})).toBeVisible();
  await page.getByRole('button',{name:'Tetap Mengisi',exact:true}).click();
  await expect(page.getByRole('radio',{name:'Tidak Setuju',exact:true})).toBeChecked();
  await page.unroute('**/v2/talent-assessment/draft');
  await page.getByRole('button',{name:'Coba Simpan Lagi'}).click();
  await expect(page.getByRole('status')).toHaveText('Semua perubahan tersimpan');
  state=await participant('GET','/talent-assessment');
  await participant('PUT','/talent-assessment/draft',{...state.draft,answers:Array(170).fill(4),current_question:170});
  await page.reload();
  await page.getByRole('button',{name:'Periksa Jawaban',exact:true}).first().click();
  let loseResponse=true;
  await page.route('**/v2/talent-assessment/submit',async route=>{
    if(loseResponse) {loseResponse=false;const response=await route.fetch();expect(response.ok()).toBe(true);return route.fulfill({status:503,contentType:'application/json',body:'{"message":"TEST_LOST_RESPONSE"}'});}
    return route.continue();
  });
  await page.getByRole('button',{name:'Kirim Asesmen',exact:true}).click();
  await expect(page.getByText(/Hasil belum berhasil dikirim/)).toBeVisible();
  const completed=(await participant('GET','/talent-assessment')).result;
  await page.getByRole('button',{name:'Kirim Asesmen',exact:true}).click();
  await expect(page).toHaveURL(/\/result$/);
  expect((await participant('GET','/talent-assessment')).result.submitted_at).toBe(completed.submitted_at);
});
