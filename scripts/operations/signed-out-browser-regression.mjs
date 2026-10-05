// Permanent delayed-auth regression. Local UI assets; no server or real account required. Unauthorized API fetches are recorded,
// but the security contract is that private data remains inaccessible and invisible.
import fs from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const {chromium}=await import(process.env.SWITCHYARD_PLAYWRIGHT_MODULE||'playwright');
const root=fileURLToPath(new URL('../..',import.meta.url));
const out=process.argv[2]||'/tmp/switchyard-signed-out-regression';
await fs.mkdir(out,{recursive:true});
const browser=await chromium.launch({headless:true}),results=[];
const routes={'/':'index.html','/repositories':'repositories.html','/work':'work.html','/pulls':'pulls.html','/operations':'operations.html','/settings':'settings.html'};
try{for(const width of [1600,390]){for(const [route,file] of Object.entries(routes)){
 const page=await browser.newPage({viewport:{width,height:900}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 let release;const gate=new Promise(resolve=>release=resolve);let pending=true;const earlyPrivateRequests=[];
 await page.addInitScript(()=>{window.privateWorkspaceFlashes=[];const check=()=>{const main=document.querySelector('main');if(main){const text=main.innerText;if(/Welcome back|private-workspace-marker|sy-dogfood-|alice\//i.test(text))window.privateWorkspaceFlashes.push(text.slice(0,100));}requestAnimationFrame(check);};requestAnimationFrame(check);});
 await page.route('http://signedout.test/**',async intercepted=>{
  const url=new URL(intercepted.request().url());
  if(url.pathname==='/api/auth/me'){await gate;return intercepted.fulfill({status:401,json:{authed:false}});}
  if(url.pathname.startsWith('/api/')){
   if(pending&&/^\/api\/(work|operations|settings|pulls)(\/|$)/.test(url.pathname))earlyPrivateRequests.push(url.pathname);
   return intercepted.fulfill({status:401,json:{error:'unauthorized'}});
  }
  const name=url.pathname.startsWith('/assets/')?url.pathname.slice(1):file;
  try{return intercepted.fulfill({path:path.join(root,'public',name),contentType:name.endsWith('.js')?'text/javascript':name.endsWith('.css')?'text/css':name.endsWith('.webp')?'image/webp':'text/html'});}catch{return intercepted.fulfill({status:404,body:''});}
 });
 await page.goto('http://signedout.test'+route,{waitUntil:'domcontentloaded'});await page.waitForTimeout(350);
 assert.deepEqual(await page.evaluate(()=>window.privateWorkspaceFlashes),[],route+' flashed before auth resolved');
 pending=false;release();await page.locator('.header-signup').waitFor();await page.waitForTimeout(250);
 assert.deepEqual(await page.evaluate(()=>window.privateWorkspaceFlashes),[],route+' flashed after anonymous auth');
 if(route==='/')assert(await page.locator('#signed-in-home').isHidden());
 if(route==='/repositories')assert.match(await page.locator('h1').innerText(),/Public/);
 assert.deepEqual(errors,[],route+' browser errors');results.push({width,route,delayed_auth_no_flash:true,anonymous_no_flash:true,early_unauthorized_requests:earlyPrivateRequests});
 await page.unrouteAll({behavior:'ignoreErrors'});await page.close();
 }}await fs.writeFile(path.join(out,'results.json'),JSON.stringify(results,null,2)+'\n');console.log(JSON.stringify({passed:results.length,output:out}));}finally{await browser.close();}
