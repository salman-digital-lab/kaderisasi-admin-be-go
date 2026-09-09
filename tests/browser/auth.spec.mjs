import {test,expect,login,evidence} from './fixture.mjs';

test('login, cookie refresh, logout, and restricted navigation',async({page,fixture},testInfo)=>{
  await login(page);
  const cookies=await page.context().cookies();
  expect(cookies.some(cookie=>cookie.name==='admin_refresh_token'&&cookie.httpOnly)).toBe(true);
  const refresh=page.waitForResponse(response=>response.url().endsWith('/auth/refresh'));
  await page.reload();
  expect((await refresh).status()).toBe(200);
  await expect(page).toHaveURL(/\/dashboard$/);
  await evidence(page,testInfo,'dashboard-after-refresh');
  await page.locator('.ant-avatar').click();
  await page.getByText('Logout',{exact:true}).click();
  await expect(page).toHaveURL(/\/login$/);
  expect((await page.context().cookies()).some(cookie=>cookie.name==='admin_refresh_token')).toBe(false);
  const sessions=(await fixture.db.query('SELECT revoked_at FROM admin_refresh_tokens')).rows;
  expect(sessions.length).toBeGreaterThan(1);expect(sessions.every(row=>row.revoked_at!==null)).toBe(true);
  await login(page,'requester@example.test');
  await page.goto('/admin-users');
  await expect(page).toHaveURL(/\/forbidden$/);
  await evidence(page,testInfo,'restricted-navigation');
});

test('administrator requests access and reviewer approves',async({page,browser,fixture},testInfo)=>{
  await login(page,'requester@example.test');
  await page.getByRole('button',{name:/Ajukan Akses$/}).click();
  const dialog=page.getByRole('dialog');
  await dialog.getByRole('combobox').click();
  await page.getByRole('option',{name:/^Activity Manager /}).click();
  await dialog.getByLabel('2. Jelaskan Kebutuhan Akses').fill('Synthetic browser fixture: manage activity registrations.');
  await dialog.getByRole('button',{name:'Kirim Permintaan'}).click();
  await expect(dialog).not.toBeVisible();
  const ticket=(await fixture.db.query('SELECT id,status FROM tickets')).rows[0];
  expect(ticket.status).toBe('open');
  await evidence(page,testInfo,'submitted-access-request');
  const reviewContext=await browser.newContext({viewport:page.viewportSize(),baseURL:'http://localhost:3005'});
  try{
    const reviewer=await reviewContext.newPage();
    await login(reviewer);
    await reviewer.goto(`/ticket-review/${ticket.id}`);
    await reviewer.getByRole('button',{name:/Setujui Permintaan$/}).click();
    await reviewer.getByRole('dialog').getByRole('button',{name:'Ya, Setujui'}).click();
    await expect(reviewer.getByText('Permintaan telah ditutup',{exact:true})).toBeVisible();
    await evidence(reviewer,testInfo,'approved-access-request');
    const user=(await fixture.db.query('SELECT role_code FROM admin_users WHERE id=2')).rows[0];
    expect(user.role_code).toBe('activity_manager');
    await page.reload();
    await page.goto('/activity');
    await expect(page).toHaveURL(/\/activity$/);
  }finally{await reviewContext.close();}
});
