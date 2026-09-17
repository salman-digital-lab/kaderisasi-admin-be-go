import {readFileSync} from 'node:fs';
import {test,expect,login,evidence,api} from './fixture.mjs';
const source=JSON.parse(readFileSync(new URL('../../internal/formschema/routing.fixtures.json',import.meta.url),'utf8')).schema;
async function identity(page) {
 const preview=page.getByRole('dialog');
 await preview.getByRole('textbox',{name:'Nama',exact:true}).fill('Peserta pratinjau');
 await preview.getByRole('combobox',{name:'Jenis Kelamin',exact:true}).click();
 await page.getByRole('option',{name:'Perempuan',exact:true}).click();
 await preview.getByRole('button',{name:'Lanjutkan',exact:true}).click();
}
test('focused builder preview, branching, and draft recovery',async({page,fixture},testInfo)=>{
 const errors=[];page.on('pageerror',error=>errors.push(error.message));
 const form=await api('POST','/custom-forms',{formName:'Branching builder',formSchema:source,isActive:true});
 await login(page);await page.goto(`/custom-form/${form.id}/edit`);
 const choice=page.locator('#builder-section-choice');await choice.locator('.builder-question-title').click();
 await choice.getByRole('textbox',{name:'Judul pertanyaan',exact:true}).fill('Pilih jalur');
 await choice.getByRole('textbox',{name:'Pilihan 2',exact:true}).fill('Lewati detail');
 await choice.getByRole('button',{name:'Edit bagian',exact:true}).click();await choice.getByRole('textbox',{name:'Nama bagian',exact:true}).fill('Pilihan peserta');
 await page.getByRole('button',{name:'Pratinjau',exact:true}).click();const preview=page.getByRole('dialog');
 await preview.getByRole('button',{name:'Lanjutkan',exact:true}).click();await expect(preview.locator('#preview-name-error')).toHaveText('Jawaban wajib diisi.');await identity(page);
 await preview.getByRole('radio',{name:'Lewati detail',exact:true}).check();await preview.getByRole('button',{name:'Lanjutkan',exact:true}).click();
 await expect(preview.getByRole('heading',{name:'Penutup',exact:true})).toBeVisible();await expect(preview.getByRole('textbox',{name:'Alasan',exact:true})).toHaveCount(0);
 await preview.getByRole('button',{name:'Kembali',exact:true}).click();await preview.getByRole('radio',{name:'Detail',exact:true}).check();await preview.getByRole('button',{name:'Lanjutkan',exact:true}).click();await preview.getByRole('button',{name:'Lanjutkan',exact:true}).click();
 await expect(preview.locator('#preview-reason-error')).toHaveText('Jawaban wajib diisi.');await preview.getByRole('textbox',{name:'Alasan',exact:true}).fill('Alasan contoh');await preview.getByRole('button',{name:'Lanjutkan',exact:true}).click();await preview.getByRole('button',{name:'Kirim (simulasi)',exact:true}).click();
 await expect(preview.getByText('Simulasi selesai',{exact:true})).toBeVisible();expect((await fixture.db.query('SELECT count(*)::int AS count FROM activity_registrations')).rows[0].count).toBe(0);
 await preview.getByRole('button',{name:'Mulai ulang',exact:true}).click();await identity(page);await preview.getByRole('radio',{name:'Selesai',exact:true}).check();await preview.getByRole('button',{name:'Kirim (simulasi)',exact:true}).click();await page.keyboard.press('Escape');
 await page.getByRole('button',{name:'Simpan Perubahan',exact:true}).click();await expect(page.getByRole('status',{name:'Status penyimpanan'})).toHaveText('Semua perubahan tersimpan');
 const stored=async()=> (await fixture.db.query('SELECT form_schema FROM custom_forms WHERE id=$1',[form.id])).rows[0].form_schema;
 expect((await stored()).fields[1].navigation).toEqual(source.fields[1].navigation);expect((await stored()).fields[1].fields[0].options[1]).toEqual({label:'Lewati detail',value:'skip'});
 await choice.getByRole('textbox',{name:'Deskripsi bagian',exact:true}).fill('Draf lokal yang dipulihkan');await expect(page.getByRole('status',{name:'Status penyimpanan'})).toHaveText('Draf tersimpan di perangkat ini');
 page.once('dialog',dialog=>dialog.accept());await page.reload();await page.getByRole('button',{name:'Pulihkan draf',exact:true}).click();await choice.getByRole('button',{name:'Edit bagian',exact:true}).click();await expect(choice.getByRole('textbox',{name:'Deskripsi bagian',exact:true})).toHaveValue('Draf lokal yang dipulihkan');
 await page.getByRole('button',{name:'Simpan Perubahan',exact:true}).click();await expect(page.getByRole('status',{name:'Status penyimpanan'})).toHaveText('Semua perubahan tersimpan');
 for(const width of [1280,1440]) {await page.setViewportSize({width,height:1000});await evidence(page,testInfo,`builder-questions-${width}`);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);}
 await page.goto('/activity/new');await expect(page.getByRole('heading',{name:'Buat kegiatan',exact:true})).toBeVisible();for(const width of [1280,1440]) {await page.setViewportSize({width,height:1000});await evidence(page,testInfo,`analogous-activity-${width}`);}expect(errors).toEqual([]);
});
test('legacy values and failed saves retain recoverable drafts',async({page,fixture},testInfo)=>{
 const legacy={fields:[{section_name:'Pilihan lama',fields:[{key:'legacy',label:'Pilihan',type:'select',required:false,options:[{label:'Nilai lama'}]}]}]};
 const form=await api('POST','/custom-forms',{formName:'Legacy recovery',isActive:true});await fixture.db.query('UPDATE custom_forms SET form_schema=$1::jsonb WHERE id=$2',[JSON.stringify(legacy),form.id]);
 const stored=async()=> (await fixture.db.query('SELECT form_schema FROM custom_forms WHERE id=$1',[form.id])).rows[0].form_schema;
 await login(page);await page.goto(`/custom-form/${form.id}/edit`);await page.locator('.builder-question-title').click();await page.getByRole('textbox',{name:'Pilihan 1',exact:true}).fill('Label baru');
 await page.getByRole('tab',{name:'Pengaturan',exact:true}).click();await page.getByRole('textbox',{name:/Nama Form$/}).fill('Draf gagal disimpan');
 const endpoint=`**/v2/custom-forms/${form.id}`;await page.route(endpoint,route=>route.request().method()==='PUT'?route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({message:'GENERAL_ERROR'})}):route.continue());
 await page.getByRole('button',{name:'Simpan Perubahan',exact:true}).click();await expect(page.getByText('Perubahan belum disimpan',{exact:true})).toBeVisible();expect(await stored()).toEqual(legacy);
 await expect(page.getByRole('status',{name:'Status penyimpanan'})).toHaveText('Draf tersimpan di perangkat ini');await page.unroute(endpoint);page.once('dialog',dialog=>dialog.accept());await page.reload();await page.getByRole('button',{name:'Pulihkan draf',exact:true}).click();
 await page.getByRole('button',{name:'Simpan Perubahan',exact:true}).click();await expect(page.getByRole('status',{name:'Status penyimpanan'})).toHaveText('Semua perubahan tersimpan');expect((await stored()).fields[1].fields[0].options[0]).toEqual({label:'Label baru',value:'Nilai lama'});await evidence(page,testInfo,'legacy-recovery');
});
test('keyboard sorting, rules, validation, and empty sections',async({page,fixture},testInfo)=>{
 const form=await api('POST','/custom-forms',{formName:'Builder controls',formSchema:source,isActive:true});await login(page);await page.goto(`/custom-form/${form.id}/edit`);
 const choice=page.locator('#builder-section-choice');await choice.locator('.builder-question-title').click();const handle=choice.getByRole('button',{name:'Geser pilihan 3',exact:true});await handle.focus();await page.keyboard.press('Space');await expect(handle).toHaveAttribute('aria-pressed','true');await page.keyboard.press('ArrowUp');await expect(page.locator('[id^="DndLiveRegion"]').filter({hasText:'over droppable area track:option:skip:1.'})).toHaveCount(1);await page.keyboard.press('Space');await expect(choice.getByRole('textbox',{name:'Pilihan 2',exact:true})).toHaveValue('Selesai');
 await page.getByRole('tab',{name:'Alur',exact:true}).click();await page.getByRole('combobox',{name:'Tujuan untuk Detail',exact:true}).click();await page.getByRole('option',{name:'Detail',exact:true}).click();
 await page.getByRole('tab',{name:'Pertanyaan',exact:true}).click();const toggle=page.getByRole('button',{name:/^Daftar bagian/});if(await toggle.isVisible())await toggle.click();await page.getByRole('navigation',{name:'Daftar bagian formulir'}).getByRole('button',{name:/2\. Detail/}).click();
 const detail=page.locator('#builder-section-detail');await detail.locator('.builder-question-title').click();await detail.getByRole('button',{name:'Teks bantuan dan validasi',exact:false}).click();await detail.getByRole('spinbutton',{name:'minLength',exact:true}).fill('3');await detail.getByRole('spinbutton',{name:'maxLength',exact:true}).fill('2');await expect(detail.getByText('Batas minimum melebihi maksimum. Turunkan batas minimum atau naikkan batas maksimum.',{exact:true})).toBeVisible();
 await detail.getByRole('spinbutton',{name:'maxLength',exact:true}).fill('20');await detail.getByRole('textbox',{name:'Pola jawaban (regex)',exact:true}).fill('[');await expect(detail.getByText('Pola validasi tidak dapat dibaca. Perbaiki penulisan regex atau kosongkan pola.',{exact:true})).toBeVisible();await detail.getByRole('textbox',{name:'Pola jawaban (regex)',exact:true}).fill('^[A-Z]+$');await detail.getByRole('textbox',{name:'Pesan kesalahan',exact:true}).fill('Isi huruf kapital');
 await page.getByRole('button',{name:'Tambah bagian',exact:true}).last().click();await page.getByRole('textbox',{name:'Nama bagian',exact:true}).fill('Petunjuk akhir');await page.getByRole('textbox',{name:'Deskripsi bagian',exact:true}).fill('Terima kasih telah melengkapi formulir.');
 await page.getByRole('button',{name:'Simpan Perubahan',exact:true}).click();await expect(page.getByRole('status',{name:'Status penyimpanan'})).toHaveText('Semua perubahan tersimpan');const schema=(await fixture.db.query('SELECT form_schema FROM custom_forms WHERE id=$1',[form.id])).rows[0].form_schema;
 expect(schema.fields.at(-1).fields).toEqual([]);expect(schema.fields.at(-1).description).toBe('Terima kasih telah melengkapi formulir.');expect(schema.fields.find(item=>item.id==='detail').fields[0].validation).toEqual({minLength:3,maxLength:20,pattern:'^[A-Z]+$',customMessage:'Isi huruf kapital'});await evidence(page,testInfo,'builder-controls');
});
