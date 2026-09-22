import {createRequire} from 'node:module';
import {readFileSync} from 'node:fs';
import {expect} from '@playwright/test';

const requireWeb = createRequire(new URL('../../../kaderisasi-web-fe/package.json', import.meta.url));

export async function auditNotificationUI(page, selector) {
  await page.evaluate(readFileSync(requireWeb.resolve('axe-core/axe.min.js'),'utf8'));
  const violations=await page.evaluate(async(selector)=>{
    const result=await window.axe.run({include:[selector]},{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}});
    return result.violations.map(({id,nodes})=>({id,nodes:nodes.map(({target,failureSummary})=>({target,failureSummary}))}));
  },selector);
  expect(violations).toEqual([]);
}

export async function verifyInboxRecovery(page, prefix, title) {
  const titleButton=page.getByRole('button',{name:title,exact:true});
  await titleButton.focus();
  let release;
  const gate=new Promise(resolve=>{release=resolve;});
  const listRoute=url=>url.pathname===prefix;
  await page.route(listRoute,async route=>{await gate;await route.continue();});
  const requested=page.waitForRequest(request=>request.url().includes(`${prefix}?`));
  await page.evaluate(()=>window.dispatchEvent(new Event('focus')));
  await requested;
  await expect(titleButton).toBeVisible();
  await expect(titleButton).toBeFocused();
  const refreshed=page.waitForResponse(response=>new URL(response.url()).pathname===prefix);
  release();
  await refreshed;
  await page.unroute(listRoute);
  await page.route(`**${prefix}/*/read`,route=>route.fulfill({status:503,json:{message:'UNAVAILABLE'}}));
  await titleButton.press('Enter');
  const dialog=page.getByRole('dialog');
  await expect(dialog).toContainText('Pengumuman tidak tersedia atau gagal dimuat.');
  await page.unroute(`**${prefix}/*/read`);
  await dialog.getByRole('button',{name:'Coba lagi',exact:true}).click();
  await expect(dialog).toContainText('<script>');
  await expect(dialog.locator('span[tabindex="-1"]')).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();
}
