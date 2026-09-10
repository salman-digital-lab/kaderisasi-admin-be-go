import { chromium, expect } from '@playwright/test';
import { mkdir } from 'node:fs/promises';

const output = new URL('../.artifacts/admin-profile/', import.meta.url).pathname;
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true });
try {
  for (const width of [1440, 390]) {
    const context = await browser.newContext({ viewport: { width, height: 960 } });
    const page = await context.newPage();
    const user = { id: 1, email: 'admin@example.test', display_name: 'Admin Uji', is_active: true, role: { code: 'super_admin', name: 'Super Admin' } };
    const session = { access_token: 'fixture', access_token_expires_in: 900, user, permissions: ['admin_users.read', 'admin_users.manage', 'rbac.read'], authentication_methods: ['password'], is_super_admin: true };
    const roles = [user.role, { code: 'konselor', name: 'Konselor' }];
    let account = { id: 2, email: 'konselor@example.test', display_name: 'Konselor Uji', is_active: true, role: roles[1], created_at: '2026-09-10T00:00:00Z', effective_permissions: [], authentication_methods: ['password'] };
    const queries = [];
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.route('**/*', async route => {
      const request = route.request();
      const url = new URL(request.url());
      if (request.resourceType() !== 'xhr' && request.resourceType() !== 'fetch') {
        if (url.hostname === 'localhost' && url.port === '3005') return route.continue();
        return route.abort();
      }
      let data;
      const path = url.pathname.replace(/^\/v2/, '');
      if (path === '/auth/refresh' || path === '/auth/me') data = session;
      else if (path === '/auth/profile') { user.display_name = request.postDataJSON().displayName; data = session; }
      else if (path === '/rbac/roles') data = roles;
      else if (path === '/admin-users/2' && request.method() === 'PUT') { account = { ...account, display_name: request.postDataJSON().displayName }; data = account; }
      else if (path === '/admin-users') { queries.push(url.search); data = { data: [account], meta: { current_page: 1, per_page: 10, total: 1 } }; }
      else data = [];
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ message: 'UPDATE_DATA_SUCCESS', data }) });
    });
    await page.goto('http://localhost:3005/profile');
    await page.getByLabel('Nama', { exact: true }).fill('Nama Saya Baru');
    await page.getByRole('button', { name: 'Simpan Nama' }).click();
    await expect(page.getByRole('button', { name: 'Menu akun Nama Saya Baru' })).toBeVisible();
    await page.screenshot({ path: `${output}profile-${width}.png`, fullPage: true });
    await page.goto('http://localhost:3005/admin-users');
    if (width === 390) await page.getByRole('button').filter({ hasText: 'Filter' }).click();
    await page.getByLabel('Filter role', { exact: true }).click();
    await page.getByTitle('Konselor', { exact: true }).click();
    await page.getByLabel('Filter status akun', { exact: true }).click();
    await page.getByTitle('Nonaktif', { exact: true }).click();
    await page.getByRole('button', { name: 'Terapkan', exact: true }).first().click();
    await expect.poll(() => queries.some(query => query.includes('role_code=konselor') && query.includes('is_active=false'))).toBe(true);
    await page.screenshot({ path: `${output}admin-users-${width}.png`, fullPage: true });
    if (width === 390) await page.getByText('Detail lainnya (1)', { exact: true }).click();
    await page.getByRole('button', { name: 'Ubah Akun', exact: true }).click();
    await page.getByLabel('Nama', { exact: true }).fill('Nama Konselor Baru');
    await page.getByRole('button', { name: 'OK', exact: true }).click();
    await expect(page.getByText('Nama Konselor Baru', { exact: true })).toBeVisible();
    if (errors.length) throw new Error(errors.join('\n'));
    console.log(`Profile save, immediate header update, filters, and admin name edit pass at ${width}px`);
    await context.close();
  }
} finally { await browser.close(); }
