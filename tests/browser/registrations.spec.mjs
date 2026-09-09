import {test,expect,login,evidence,api} from './fixture.mjs';
import {legacyRequire} from '../../scripts/fixture-db.mjs';

async function assertWorkbook(download,testInfo,name){
  const path=testInfo.outputPath(name+'.xlsx');await download.saveAs(path);
  const workbook=new (legacyRequire('exceljs').Workbook)();await workbook.xlsx.readFile(path);
  const rows=workbook.getWorksheet('Registrations');
  expect(rows.rowCount).toBe(2);expect(rows.getCell(2,2).value).toBe('Registration member');
  await testInfo.attach(name,{path,contentType:'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'});
}

test('activity and club registrations, form attachment, and Excel downloads',async({page,fixture},testInfo)=>{
  await api('POST','/members',{name:'Registration member',email:'registrations@example.test',gender:'F'});
  await api('POST','/activities',{name:'Browser activity',additional_config:{custom_selection_status:[],mandatory_profile_data:[],additional_questionnaire:[]}});
  await api('POST','/clubs',{name:'Browser club'});
  const formSchema={fields:[{section_name:'Motivation',fields:[{key:'motivation',label:'Motivasi',type:'text',required:false}]}]};
  await api('POST','/custom-forms',{formName:'Browser activity form',formSchema,isActive:true});
  await api('POST','/custom-forms',{formName:'Browser club form',formSchema,isActive:true});
  await login(page);
  await page.goto('/activity/1');
  await page.getByRole('tab',{name:'Form Pendaftaran',exact:true}).click();
  await page.getByRole('button',{name:/Buat atau Pilih Form$/}).click();
  let dialog=page.getByRole('dialog');
  await dialog.getByRole('combobox').click();
  await page.getByText('Browser activity form',{exact:true}).click();
  await dialog.getByRole('button',{name:'Gunakan',exact:true}).click();
  await expect(dialog).not.toBeVisible();
  expect((await fixture.db.query('SELECT feature_type,feature_id FROM custom_forms WHERE id=1')).rows[0]).toEqual({feature_type:'activity_registration',feature_id:1});
  await page.goto('/activity/1/participants');
  await page.getByRole('button',{name:/Tambah$/}).click();
  dialog=page.getByRole('dialog');
  await dialog.getByRole('row').filter({hasText:'Registration member'}).click();
  await dialog.getByRole('button',{name:'OK',exact:true}).click();
  await expect(dialog).not.toBeVisible();
  expect((await fixture.db.query('SELECT count(*)::int AS count FROM activity_registrations')).rows[0].count).toBe(1);
  const activityDownload=page.waitForEvent('download');
  await page.getByRole('button').filter({has:page.locator('svg[data-icon="download"]')}).click();
  await assertWorkbook(await activityDownload,testInfo,'activity-registrations');
  await evidence(page,testInfo,'activity-registration');
  await page.goto('/club/1?section=registration');
  await page.getByRole('button',{name:/Pilih Form yang Sudah Ada$/}).click();
  dialog=page.getByRole('dialog');
  await dialog.getByRole('combobox').click();
  await page.getByText('Browser club form',{exact:true}).click();
  await dialog.getByRole('button',{name:'Lampirkan Form',exact:true}).click();
  await expect(dialog).not.toBeVisible();
  expect((await fixture.db.query('SELECT feature_type,feature_id FROM custom_forms WHERE id=2')).rows[0]).toEqual({feature_type:'club_registration',feature_id:1});
  await page.goto('/club/1?section=people');
  await page.getByRole('button',{name:/Tambahkan Pendaftar$/}).click();
  dialog=page.getByRole('dialog');
  await dialog.getByRole('row').filter({hasText:'Registration member'}).getByRole('radio').check();
  await dialog.getByRole('button',{name:'Tambahkan',exact:true}).click();
  await expect(dialog).not.toBeVisible();
  expect((await fixture.db.query('SELECT status FROM club_registrations')).rows[0].status).toBe('PENDING');
  const clubDownload=page.waitForEvent('download');
  await page.getByRole('button',{name:/Export XLSX$/}).click();
  await assertWorkbook(await clubDownload,testInfo,'club-registrations');
  await evidence(page,testInfo,'club-registration');
});
