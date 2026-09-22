import {expect} from '@playwright/test';

export async function verifyNotificationPolling(page,path){
  const refreshed=page.waitForResponse(response=>response.url().includes(path)&&response.status()===200);
  await page.clock.fastForward(31000);
  await refreshed;
  await page.evaluate(()=>Object.defineProperty(document,'hidden',{configurable:true,value:true}));
  let hiddenRequests=0;
  const listener=request=>{if(request.url().includes(path))hiddenRequests++;};
  page.on('request',listener);
  await page.clock.fastForward(61000);
  expect(hiddenRequests).toBe(0);
  page.off('request',listener);
  const focused=page.waitForResponse(response=>response.url().includes(path)&&response.status()===200);
  await page.evaluate(()=>{delete document.hidden;window.dispatchEvent(new Event('focus'));});
  await focused;
  await page.clock.resume();
}
