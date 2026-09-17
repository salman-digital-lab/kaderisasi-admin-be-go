import { createRequire } from 'node:module';
import { randomUUID } from 'node:crypto';
import { resolve } from 'node:path';
import { test, expect, evidence, api } from '../browser/fixture.mjs';
import { testEnvironment, workspace } from '../../scripts/env.mjs';
import { fixtureKey, fixturePassword, legacyRequire } from '../../scripts/fixture-db.mjs';
const require = createRequire(resolve(workspace,'kaderisasi-web-be/package.json'));
const {PDFDocument}=require('pdf-lib');
const sharp=require('sharp');
const {S3Client,DeleteObjectCommand,GetObjectCommand}=require('@aws-sdk/client-s3');
const schema={version:2,settings:{accessMode:'public'},fields:[
 {id:'choice',section_name:'Pilihan',fields:[{key:'track',label:'Jenis respons',type:'radio',required:true,options:[{label:'Dengan berkas',value:'file'},{label:'Tanpa berkas',value:'skip'}]}],navigation:{questionKey:'track',defaultTarget:{type:'submit'},routes:[{optionValue:'file',target:{type:'section',sectionId:'files'}}]}},
 {id:'files',section_name:'Berkas',fields:[{key:'proof',label:'Bukti dokumen',type:'file',required:true,file:{accept:'pdf_or_image',maxFiles:2,maxSizeMB:10}}]}
]};
async function web(method,path,body,token,extra={}) {
 const response=await fetch('http://localhost:3333/v2'+path,{method,headers:{...(body instanceof FormData?{}:{'Content-Type':'application/json'}),...(token?{Authorization:`Bearer ${token}`} : {}),...extra},body:body===undefined?undefined:body instanceof FormData?body:JSON.stringify(body)});
 return {status:response.status,body:await response.json()};
}
test('standalone responses, file ownership, optimized storage, retries, closure, and desktop submission',async({page,fixture},testInfo)=>{
 const errors=[];page.on('pageerror',error=>errors.push(error.message));
 const env=testEnvironment();
 const storage=new S3Client({endpoint:env.DRIVE_ENDPOINT,region:env.DRIVE_REGION,forcePathStyle:true,credentials:{accessKeyId:env.DRIVE_ACCESS_KEY_ID,secretAccessKey:env.DRIVE_SECRET_ACCESS_KEY}});
 const keys=[];
 try {
  const form=await api('POST','/custom-forms',{formName:'Form mandiri pengujian',featureType:'independent_form',formSchema:schema});
  expect(form.is_active).toBe(false);
  await api('PUT',`/custom-forms/${form.id}/toggle-active`,{});
  const read=await web('GET',`/custom-forms/by-feature?feature_type=independent_form&feature_id=${form.id}`);
  expect(read.status).toBe(200);
  const start=async()=>{const result=await web('POST',`/custom-forms/${form.id}/sessions`,{schema_hash:read.body.data.schema_hash});expect(result.status).toBe(201);return result.body.data.token;};
  const token=await start();const other=await start();
  const image=await sharp({create:{width:3000,height:1500,channels:4,background:{r:80,g:120,b:200,alpha:0.5}}}).png().toBuffer();
  const upload=new FormData();upload.append('file',new Blob([image],{type:'image/png'}),'bukti.png');
  const file=await web('POST',`/custom-forms/${form.id}/files/proof`,upload,undefined,{'x-form-session':token});
  expect(file.status,JSON.stringify(file.body)).toBe(201);
  keys.push((await fixture.db.query('SELECT storage_key FROM custom_form_attachments WHERE id=$1',[file.body.data.id])).rows[0].storage_key);
  expect(file.body.data).toMatchObject({mime_type:'image/webp',width:2400,height:1200,download_name:'bukti.webp'});
  const stored=await storage.send(new GetObjectCommand({Bucket:env.DRIVE_BUCKET,Key:keys[0]}));
  const metadata=await sharp(Buffer.from(await stored.Body.transformToByteArray())).metadata();expect(metadata.format).toBe('webp');expect(metadata.hasAlpha).toBe(true);
  const forged=await web('POST',`/custom-forms/${form.id}/responses`,{session_token:other,answers:{track:'file',proof:[file.body.data.id]}});expect(forged.status).toBe(422);
  const submit=()=>web('POST',`/custom-forms/${form.id}/responses`,{session_token:token,answers:{track:'file',proof:[file.body.data.id]}});
  const [saved,retried]=await Promise.all([submit(),submit()]);expect(saved.status,JSON.stringify(saved.body)).toBe(201);expect(retried.body.data.id).toBe(saved.body.data.id);
  const detail=await api('GET',`/custom-forms/${form.id}/responses/${saved.body.data.id}`);expect(detail.attachments).toHaveLength(1);
  expect((await api('GET',`/custom-forms/${form.id}/responses`)).total).toBe(1);
  const unauth=await fetch(`http://localhost:3334/v2/custom-forms/${form.id}/files/${file.body.data.id}`);expect(unauth.status).toBe(401);
  const adminToken=legacyRequire('jsonwebtoken').sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'15m'});
  const download=await fetch(`http://localhost:3334/v2/custom-forms/${form.id}/files/${file.body.data.id}`,{headers:{Authorization:`Bearer ${adminToken}`}});expect(download.status).toBe(200);expect(download.headers.get('content-type')).toBe('image/webp');
  const exportResponse=await fetch(`http://localhost:3334/v2/custom-forms/${form.id}/export-responses`,{headers:{Authorization:`Bearer ${adminToken}`}});expect(exportResponse.status).toBe(200);expect((await exportResponse.arrayBuffer()).byteLength).toBeGreaterThan(1000);
  const deletion=await fetch(`http://localhost:3334/v2/custom-forms/${form.id}`,{method:'DELETE',headers:{Authorization:`Bearer ${adminToken}`}});expect(deletion.status).toBe(409);
  const stale=await start();const changed=structuredClone(schema);changed.fields[0].fields[0].label='Pilihan pengisian';
  await api('PUT',`/custom-forms/${form.id}`,{formSchema:changed});
  expect((await web('POST',`/custom-forms/${form.id}/responses`,{session_token:stale,answers:{track:'skip'}})).status).toBe(409);
  await page.setViewportSize({width:1440,height:1000});await page.goto(`http://localhost:3000/form/${form.id}`);
  await page.getByRole('radio',{name:'Tanpa berkas',exact:true}).check();
  await page.getByRole('button',{name:'Kirim jawaban',exact:true}).click();
  await page.getByRole('button',{name:/^Ya|^Kirim$/}).last().click();
  await expect(page.getByText('Jawaban telah tersimpan',{exact:true})).toBeVisible();
  await evidence(page,testInfo,'standalone-complete-desktop');
  expect((await api('GET',`/custom-forms/${form.id}/responses`)).total).toBe(2);
  await api('PUT',`/custom-forms/${form.id}/toggle-active`,{});
  await page.reload();await expect(page.getByText('Penerimaan respons ditutup',{exact:true})).toBeVisible();
  const closed=await web('POST',`/custom-forms/${form.id}/sessions`,{schema_hash:(await web('GET',`/custom-forms/by-feature?feature_type=independent_form&feature_id=${form.id}`)).body.data.schema_hash});expect(closed.status).toBe(409);
  expect((await web('POST',`/custom-forms/${form.id}/files/proof`,upload,undefined,{'x-form-session':other})).status).toBe(409);
  expect(errors).toEqual([]);
 } finally {
  // All keys come from this test's owned schema and recorded uploaded attachment.
  const owned = (await fixture.db.query('SELECT storage_key FROM custom_form_attachments')).rows;
  for(const key of new Set([...keys, ...owned.map(row=>row.storage_key)]))await storage.send(new DeleteObjectCommand({Bucket:env.DRIVE_BUCKET,Key:key}));
  storage.destroy();
 }
});

for (const accept of ['pdf','image','pdf_or_image']) test(`upload ${accept}: standalone, member activity, guest activity, club, and edit ownership`, async ({page,fixture},testInfo) => {
 const env=testEnvironment();
 const storage=new S3Client({endpoint:env.DRIVE_ENDPOINT,region:env.DRIVE_REGION,forcePathStyle:true,credentials:{accessKeyId:env.DRIVE_ACCESS_KEY_ID,secretAccessKey:env.DRIVE_SECRET_ACCESS_KEY}});
 try {
  const pdf=await PDFDocument.create();pdf.addPage();const pdfBytes=Buffer.from(await pdf.save());
  const image=await sharp({create:{width:80,height:40,channels:4,background:{r:40,g:100,b:150,alpha:0.5}}}).png().toBuffer();
  const selected=accept==='pdf'?{name:'dokumen.pdf',mimeType:'application/pdf',buffer:pdfBytes}:{name:'gambar.png',mimeType:'image/png',buffer:image};
  const member=await api('POST','/members',{name:'Upload member',email:'uploads@example.test',password:fixturePassword,gender:'F'});
  const auth=await web('POST','/auth/login',{email:'uploads@example.test',password:fixturePassword});
  const memberToken=auth.body.data.token.token;
  const identityBefore=(await fixture.db.query('SELECT * FROM profiles WHERE user_id=$1',[member.user.id])).rows;
  for(const kind of ['standalone','activity','guest','club']) {
   const fields=[{id:'files',section_name:'Dokumen',fields:[{key:'proof',label:'Bukti unggahan',type:'file',required:true,file:{accept,maxFiles:1,maxSizeMB:1}}]}];
   const localSchema={version:2,settings:{accessMode:kind==='standalone'?'members':'public'},fields:[{id:'identity',section_name:'profile_data',fields:kind==='standalone'?[{key:'name',label:'Nama opsional',type:'text',required:false}]:[]},...fields]};
   let feature;
   if(kind==='club') feature=await api('POST','/clubs',{name:`Klub unggahan ${accept}`});
   if(kind==='activity'||kind==='guest') feature=await api('POST','/activities',{name:`Kegiatan unggahan ${kind} ${accept}`,activity_type:1,additional_config:{allow_guest_registration:true,custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]}});
   const featureType=kind==='standalone'?'independent_form':kind==='club'?'club_registration':'activity_registration';
   const form=await api('POST','/custom-forms',{formName:`Unggahan ${kind} ${accept}`,featureType,featureId:feature?.id,isActive:true,formSchema:localSchema});
   if(kind==='standalone')await api('PUT',`/custom-forms/${form.id}/toggle-active`,{});
   if(kind==='club')await api('PUT',`/clubs/${feature.id}`,{is_show:true,is_registration_open:true});
   if(kind==='activity'||kind==='guest')await fixture.db.query('UPDATE activities SET is_published=true,is_registration_open=true,registration_start=CURRENT_DATE-1,registration_end=CURRENT_DATE+1 WHERE id=$1',[feature.id]);
   const current=await web('GET',`/custom-forms/by-feature?feature_type=${featureType}&feature_id=${feature?.id??form.id}`);
   expect(current.status,JSON.stringify(current.body)).toBe(200);
   const identity=kind==='guest'?undefined:memberToken;
   const session=await web('POST',`/custom-forms/${form.id}/sessions`,{schema_hash:current.body.data.schema_hash},identity);expect(session.status,JSON.stringify(session.body)).toBe(201);
   const headers={'x-form-session':session.body.data.token};
   const upload=async(file,id=randomUUID())=>{const body=new FormData();body.append('file',new Blob([file.buffer],{type:file.mimeType}),file.name);return web('POST',`/custom-forms/${form.id}/files/proof`,body,identity,{...headers,'x-form-upload':id});};
   if(accept!=='pdf_or_image')expect((await upload(accept==='pdf'?{name:'spoof.pdf',mimeType:'application/pdf',buffer:image}:{name:'spoof.png',mimeType:'image/png',buffer:pdfBytes})).status).toBe(422);
   expect((await upload({name:'malformed.png',mimeType:'image/png',buffer:Buffer.from('not an image')})).status).toBe(422);
   expect((await upload({...selected,buffer:Buffer.alloc(1024*1024+1)})).status).toBe(422);
   const uploadID=randomUUID();let file=await upload(selected,uploadID);expect(file.status,JSON.stringify(file.body)).toBe(201);expect((await upload(selected,uploadID)).body.data.id).toBe(file.body.data.id);
   expect((await upload(selected)).status).toBe(422);
   const removed=await web('DELETE',`/custom-forms/${form.id}/files/${file.body.data.id}`,undefined,identity,headers);expect(removed.status).toBe(200);
   file=await upload(selected);expect(file.status).toBe(201);
   const storedRow=(await fixture.db.query('SELECT storage_key FROM custom_form_attachments WHERE id=$1',[file.body.data.id])).rows[0];
   const stored=await storage.send(new GetObjectCommand({Bucket:env.DRIVE_BUCKET,Key:storedRow.storage_key}));
   const bytes=Buffer.from(await stored.Body.transformToByteArray());
   const objectURL=new URL(`${env.DRIVE_BUCKET}/${storedRow.storage_key}`,env.DRIVE_ENDPOINT.endsWith('/')?env.DRIVE_ENDPOINT:env.DRIVE_ENDPOINT+'/');expect((await fetch(objectURL)).status).toBeGreaterThanOrEqual(400);
   if(accept==='pdf')expect(bytes.equals(pdfBytes)).toBe(true);else expect((await sharp(bytes).metadata()).format).toBe('webp');
   const answers={proof:[file.body.data.id]};
   let result;
   if(kind==='standalone') {
    expect((await web('POST',`/custom-forms/${form.id}/sessions`,{schema_hash:current.body.data.schema_hash})).status).toBe(401);
    result=await web('POST',`/custom-forms/${form.id}/responses`,{session_token:session.body.data.token,answers:{...answers,name:'Identitas respons saja'}},identity);
   } else if(kind==='guest') {
    result=await web('POST',`/activities/${feature.slug}/guest-register`,{guest_data:{name:'Tamu berkas',email:`guest-${accept}@example.test`},questionnaire_answer:answers,session_token:session.body.data.token});
   } else {
    result=await web('POST','/custom-forms/register',{feature_type:featureType,feature_id:feature.id,custom_form_data:answers,session_token:session.body.data.token},identity);
   }
   expect([200,201],JSON.stringify(result.body)).toContain(result.status);
   expect((await fixture.db.query('SELECT claimed_at FROM custom_form_attachments WHERE id=$1',[file.body.data.id])).rows[0].claimed_at).not.toBeNull();
   if(kind==='activity') {
    const edit=await web('PUT',`/activities/${feature.slug}/registration`,{questionnaire_answer:answers},identity);expect(edit.status,JSON.stringify(edit.body)).toBe(200);
    expect((await web('PUT',`/activities/${feature.slug}/registration`,{questionnaire_answer:{proof:['00000000-0000-4000-8000-000000000001']}},identity)).status).toBe(422);
   }
  }
  expect((await fixture.db.query('SELECT * FROM profiles WHERE user_id=$1',[member.user.id])).rows).toEqual(identityBefore);
  const publicSchema={version:2,settings:{accessMode:'public'},fields:[{id:'identity',section_name:'profile_data',fields:[{key:'name',label:'Nama opsional',type:'text',required:false}]},{id:'files',section_name:'Dokumen',fields:[{key:'proof',label:'Bukti unggahan',type:'file',required:true,file:{accept,maxFiles:1,maxSizeMB:1}}]}]};
  const publicForm=await api('POST','/custom-forms',{formName:`Berkas publik ${accept}`,featureType:'independent_form',formSchema:publicSchema});await api('PUT',`/custom-forms/${publicForm.id}/toggle-active`,{});
  await page.setViewportSize({width:1280,height:900});await page.goto(`http://localhost:3000/form/${publicForm.id}`);
  await expect(page.getByRole('textbox',{name:'Nama opsional',exact:true})).toBeVisible();await expect(page.getByRole('textbox',{name:'Email',exact:true})).toHaveCount(0);await page.getByRole('button',{name:'Lanjutkan',exact:true}).click();
  await expect(page.locator('input[type=file]')).toBeEnabled();await page.locator('input[type=file]').setInputFiles(selected);await expect(page.getByText(selected.name,{exact:true})).toBeVisible();
  await evidence(page,testInfo,`upload-${accept}-1280`);
  await page.getByRole('button',{name:'Hapus',exact:true}).click();await expect(page.getByText(selected.name,{exact:true})).toHaveCount(0);
  await page.locator('input[type=file]').setInputFiles(selected);await expect(page.getByText(selected.name,{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Kirim jawaban',exact:true}).click();await page.getByRole('button',{name:'Ya, Kirim',exact:true}).click();
  await expect(page.getByText('Jawaban telah tersimpan',{exact:true})).toBeVisible();
 } finally {
  const rows=(await fixture.db.query('SELECT storage_key FROM custom_form_attachments')).rows;
  for(const row of rows)await storage.send(new DeleteObjectCommand({Bucket:env.DRIVE_BUCKET,Key:row.storage_key}));
  console.log(`Form upload cleanup: removed ${rows.length} owned objects (${accept})`);storage.destroy();
 }
});
