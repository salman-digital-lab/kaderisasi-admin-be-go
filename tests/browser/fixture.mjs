import { test as base, expect } from '@playwright/test';
import { fixtureDatabase,resetFixture,legacyRequire,fixturePassword,fixtureKey } from '../../scripts/fixture-db.mjs';

const {Scrypt}=legacyRequire('@adonisjs/hash/drivers/scrypt');
const passwordHash=await new Scrypt({}).make(fixturePassword);
export const test=base.extend({
  fixture:[async({},use)=>{
    if(process.env.GO_REWRITE_BROWSER!=='1')throw new Error('Run through scripts/browser.mjs to isolate fixture resources');
    const fixture=await fixtureDatabase('candidate');
    try{await resetFixture(fixture.db,fixture.schema,passwordHash);await use(fixture);}finally{await fixture.db.end();}
  },{auto:true}],
});
export {expect};
export async function api(method,path,body){
  const token=legacyRequire('jsonwebtoken').sign({userId:1,email:'super@example.test'},fixtureKey,{expiresIn:'15m'});
  const response=await fetch('http://localhost:3334/v2'+path,{method,headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});
  const data=await response.json();
  expect(response.ok,JSON.stringify(data)).toBe(true);
  return data.data;
}
export async function login(page,email='super@example.test'){
  await page.goto('/login',{waitUntil:'domcontentloaded'});
  await page.getByLabel('Email',{exact:true}).fill(email);
  await page.getByLabel('Password',{exact:true}).fill(fixturePassword);
  const response=page.waitForResponse(response=>response.url().endsWith('/auth/login')&&response.request().method()==='POST');
  await page.getByRole('button',{name:/\bLogin$/}).click();
  expect((await response).status()).toBe(200);
  await expect(page).toHaveURL(email==='super@example.test'?/\/dashboard$/:/\/my-requests$/);
}
export async function evidence(page,testInfo,name){
  const path=testInfo.outputPath(name+'.png');
  await page.screenshot({path,fullPage:true});
  await testInfo.attach(name,{path,contentType:'image/png'});
}
