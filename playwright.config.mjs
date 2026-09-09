import { defineConfig } from '@playwright/test';
const output=process.env.GO_REWRITE_BROWSER_RESULTS??'.artifacts/browser';

export default defineConfig({
  testDir:process.env.GO_REWRITE_PUBLIC_BROWSER==='1'?'./tests/public-browser':'./tests/browser',
  fullyParallel:false,
  workers:1,
  retries:0,
  forbidOnly:true,
  timeout:90000,
  expect:{timeout:15000},
  outputDir:output+'/results',
  reporter:[['list'],['json',{outputFile:output+'/report.json'}],['html',{outputFolder:output+'/html',open:'never'}]],
  use:{baseURL:'http://localhost:3005',actionTimeout:15000,navigationTimeout:30000,trace:'retain-on-failure',screenshot:'only-on-failure',locale:'id-ID',timezoneId:'Asia/Jakarta'},
  projects:[
    {name:'desktop',use:{viewport:{width:1440,height:1000}}},
    {name:'mobile',use:{viewport:{width:390,height:844},isMobile:true,hasTouch:true}},
  ],
});
