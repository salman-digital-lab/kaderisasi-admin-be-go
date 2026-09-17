import {test,expect,api,login,evidence} from './fixture.mjs';
import {randomUUID} from 'node:crypto';
import {legacyRequire} from '../../scripts/fixture-db.mjs';
test('desktop standalone builder exposes branching, upload settings, preview, responses, and sharing',async({page,fixture},testInfo)=>{
 const schema={version:2,settings:{accessMode:'public'},fields:[{id:'start',section_name:'Pilihan',fields:[{key:'track',label:'Pilih jalur',type:'radio',required:true,options:[{label:'Berkas',value:'file'},{label:'Selesai',value:'done'}]}],navigation:{questionKey:'track',defaultTarget:{type:'submit'},routes:[{optionValue:'file',target:{type:'section',sectionId:'upload'}}]}},{id:'upload',section_name:'Unggah',fields:[{key:'proof',label:'Bukti',type:'file',required:true,file:{accept:'pdf_or_image',maxFiles:1,maxSizeMB:10}}]}]};
 const form=await api('POST','/custom-forms',{formName:'Form mandiri desktop',featureType:'independent_form',formSchema:schema});
 let shortLink;
 let creations=0;
 await page.route('**/v2/short-links**',async route=>{
  if(route.request().method()==='POST') {
   creations++;
   if(creations===1)return route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({message:'SHORT_CODE_TAKEN'})});
   const input=route.request().postDataJSON();shortLink={code:input.code,short_url:`http://localhost:4000/${input.code}`,original_url:input.original_url,created_at:new Date().toISOString(),visit_count:0};
   return route.fulfill({status:201,contentType:'application/json',body:JSON.stringify({data:shortLink})});
  }
  return route.fulfill({contentType:'application/json',body:JSON.stringify({data:{data:shortLink?[shortLink]:[],base_url:'http://localhost:4000',meta:{total:shortLink?1:0,current_page:1,per_page:100,last_page:1}}})});
 });
 await page.setViewportSize({width:1440,height:1000});await login(page);
 await page.goto(`/custom-form/${form.id}/edit`);
 for(const label of ['Pertanyaan','Alur','Respons','Pengaturan'])await expect(page.getByRole('tab',{name:label,exact:true})).toBeVisible();
 await page.getByRole('navigation',{name:'Daftar bagian formulir'}).getByRole('button',{name:/2\. Unggah/}).click();
 await page.locator('#builder-section-upload .builder-question-title').click();
 for(const [value,label,mime,name] of [['pdf','PDF saja','application/pdf','contoh.pdf'],['image','Gambar saja','image/png','contoh.png'],['pdf_or_image','PDF dan gambar','application/pdf','contoh.pdf']]) {
  await page.getByRole('combobox',{name:'Jenis berkas',exact:true}).click();await page.getByRole('option',{name:label,exact:true}).click();
  await page.getByRole('button',{name:'Pratinjau',exact:true}).click();const preview=page.getByRole('dialog');await preview.getByRole('radio',{name:'Berkas',exact:true}).check();await preview.getByRole('button',{name:'Lanjutkan',exact:true}).click();
  await preview.getByRole('button',{name:'Kirim (simulasi)',exact:true}).click();await expect(preview.getByText('Jawaban wajib diisi.',{exact:true})).toBeVisible();
  await expect(preview.locator('input[type=file]')).toHaveAttribute('accept',value==='pdf'?'application/pdf':value==='image'?'image/jpeg,image/png,image/webp':'application/pdf,image/jpeg,image/png,image/webp');
  await preview.locator('input[type=file]').setInputFiles({name,mimeType:mime,buffer:Buffer.from('local preview only')});await preview.getByRole('button',{name:'Kirim (simulasi)',exact:true}).click();await expect(preview.getByText('Simulasi selesai',{exact:true})).toBeVisible();await page.keyboard.press('Escape');
  await page.getByRole('button',{name:'Simpan Perubahan',exact:true}).click();await expect(page.getByRole('status',{name:'Status penyimpanan'})).toHaveText('Semua perubahan tersimpan');
  const stored=await api('GET',`/custom-forms/${form.id}`);expect(stored.form_schema.fields.find(section=>section.id==='upload').fields[0].file.accept).toBe(value);
 }
 await evidence(page,testInfo,'upload-question-1440');
 await page.getByRole('tab',{name:'Alur',exact:true}).click();
 await expect(page.getByText('Jika Berkas: Unggah',{exact:true})).toBeVisible();
 await expect(page.getByText('Jika Selesai: Kirim formulir',{exact:true})).toBeVisible();
 await evidence(page,testInfo,'standalone-flow-1440');
 await page.getByRole('button',{name:'Pratinjau',exact:true}).click();
 await page.getByRole('radio',{name:'Selesai',exact:true}).check();
 await page.getByRole('button',{name:'Kirim (simulasi)',exact:true}).click();
 await expect(page.getByText('Simulasi selesai',{exact:true})).toBeVisible();
 expect((await fixture.db.query('SELECT count(*)::int AS total FROM custom_form_responses')).rows[0].total).toBe(0);
 await page.keyboard.press('Escape');
 await page.getByRole('tab',{name:'Pengaturan',exact:true}).click();
 await expect(page.getByText('Siapa saja yang memiliki tautan',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Buka penerimaan respons',exact:true}).click();
 await expect(page.getByText('Sedang menerima respons',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Bagikan',exact:true}).click();await expect(page.getByRole('link',{name:'Buka formulir',exact:true})).toBeVisible();
 await page.getByRole('textbox',{name:'Kode khusus',exact:true}).fill('formdemo');await page.getByRole('button',{name:'Buat tautan pendek & QR',exact:true}).click();await expect(page.getByText('Kode sudah digunakan. Pilih kode lain.',{exact:true})).toBeVisible();
 await page.getByRole('textbox',{name:'Kode khusus',exact:true}).fill('formbaru');await page.getByRole('button',{name:'Buat tautan pendek & QR',exact:true}).click();await expect(page.getByRole('dialog',{name:'Kode QR tautan'})).toBeVisible();await page.keyboard.press('Escape');await expect(page.getByRole('textbox',{name:'Tautan pendek halaman'})).toHaveValue('http://localhost:4000/formbaru');
 await expect(page.getByRole('dialog',{name:'Kode QR tautan'})).not.toBeVisible();await expect(page.getByRole('textbox',{name:'Tautan pendek halaman'})).toBeFocused();
 await page.keyboard.press('Escape');await page.getByRole('button',{name:'Bagikan',exact:true}).click();await expect(page.getByRole('link',{name:'Buka tautan pendek',exact:true})).toHaveAttribute('href','http://localhost:4000/formbaru');expect(creations).toBe(2);await page.keyboard.press('Escape');
 await page.getByRole('tab',{name:'Respons',exact:true}).click();await expect(page.getByText('Belum ada respons. Buka penerimaan respons dan bagikan formulir.',{exact:true})).toBeVisible();
 await page.setViewportSize({width:1280,height:900});await evidence(page,testInfo,'standalone-responses-1280');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('standalone responses paginate, retain historical questions, and export',async({page,fixture},testInfo)=>{
 const schema={version:2,settings:{accessMode:'public'},fields:[{id:'answers',section_name:'Jawaban',fields:[{key:'answer',label:'Pertanyaan saat dikirim',type:'text',required:false}]}]};
 const form=await api('POST','/custom-forms',{formName:'Respons tersimpan',featureType:'independent_form',formSchema:schema});
 for(let index=0;index<21;index++) {
  const session=randomUUID();
  await fixture.db.query('INSERT INTO custom_form_sessions(id,form_id,token_hash,schema_hash,expires_at,completed_at) VALUES($1,$2,$3,$4,now()+interval \'1 day\',now())',[session,form.id,session,'a'.repeat(64)]);
  await fixture.db.query('INSERT INTO custom_form_responses(id,form_id,session_id,form_snapshot,answers) VALUES($1,$2,$3,$4,$5)',[randomUUID(),form.id,session,JSON.stringify({title:'Judul saat dikirim',schema}),JSON.stringify(index===20?{}:{answer:index===19?'=1+1':`Jawaban ${index+1}`})]);
 }
 await api('PUT',`/custom-forms/${form.id}`,{formName:'Judul terbaru',formSchema:{...schema,fields:[{...schema.fields[0],fields:[{...schema.fields[0].fields[0],label:'Pertanyaan terbaru'}]}]}});
 await page.setViewportSize({width:1440,height:1000});await login(page);await page.goto(`/custom-form/${form.id}/edit`);
 await page.getByRole('tab',{name:'Respons',exact:true}).click();await expect(page.getByRole('button',{name:'Lihat jawaban',exact:true})).toHaveCount(20);
 await page.locator('.ant-pagination-item-2').click();await expect(page.getByRole('button',{name:'Lihat jawaban',exact:true})).toHaveCount(1);
 await page.getByRole('button',{name:'Lihat jawaban',exact:true}).click();const detail=page.getByRole('dialog',{name:'Jawaban formulir'});
 await expect(detail.getByRole('heading',{name:'Judul saat dikirim'})).toBeVisible();await expect(detail.getByText('Pertanyaan saat dikirim',{exact:true})).toBeVisible();await expect(detail.getByText('Pertanyaan terbaru',{exact:true})).toHaveCount(0);
 await evidence(page,testInfo,'standalone-response-detail-1440');await page.keyboard.press('Escape');
 const download=page.waitForEvent('download');await page.getByRole('button',{name:'Ekspor Excel',exact:true}).click();const exported=await download;expect(exported.suggestedFilename()).toBe('respons-formulir.xlsx');
 const book=new (legacyRequire('exceljs').Workbook)();await book.xlsx.readFile(await exported.path());const sheet=book.getWorksheet('Respons');expect(sheet.rowCount).toBe(22);expect(sheet.getCell('F3').value).toBe("'=1+1");expect(sheet.getCell('A2').value).toBeTruthy();expect(sheet.getCell('F2').value).toBeNull();
});
