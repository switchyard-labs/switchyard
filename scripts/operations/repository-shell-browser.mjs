import {chromium} from '/home/nick/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs';
import fs from 'node:fs/promises';import assert from 'node:assert/strict';import {fileURLToPath} from 'node:url';
const root=fileURLToPath(new URL('../..',import.meta.url)).replace(/\/$/,''),out=root+'/docs/evidence/ui-shell-c48';await fs.mkdir(out,{recursive:true});const browser=await chromium.launch({headless:true}),results=[];
const maps={'':'index.html',proposals:'proposals.html',pages:'pages.html',actions:'actions.html',settings:'repo-settings.html',pulls:'pulls.html',work:'work.html',releases:'releases.html',commits:'history.html'};
try{for(const width of [1600,1280,1024,768,430,390]){const ctx=await browser.newContext({viewport:{width,height:1000}});const page=await ctx.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
await page.route('https://switchyard.cx/**',async route=>{const req=route.request(),url=new URL(req.url()),p=url.pathname;
 if(p.startsWith('/assets/')&&!p.startsWith('/assets/images/')){try{await route.fulfill({path:root+'/public'+p});return;}catch{}}
 if(req.isNavigationRequest()){let file=p==='/'?'index.html':p==='/signup'?'signin.html':p==='/alice/demo-semantic'?'repo.html':maps[p.split('/')[3]];if(file){try{await route.fulfill({path:root+'/public/'+file,contentType:'text/html'});return;}catch{}}}
 if(p==='/api/repositories/alice/demo-semantic/settings'){await route.fulfill({json:{repository:{slug:'demo-semantic',visibility:'public',default_branch:'main'},settings:{},collaborators:[],protected_refs:[]}});return;}
 if(p.endsWith('/proposal-settings')){await route.fulfill({json:{can_configure:true,can_create:true,intake:'writers',version:1}});return;}
 if(p==='/api/repositories/alice/demo-semantic'){await route.fulfill({json:{owner_slug:'alice',owner_id:'alice',owner_type:'user',slug:'demo-semantic',artifact_name:'demo-semantic',full_name:'alice/demo-semantic',visibility:'public',default_branch:'main',can_write:true}});return;}
 await route.continue();});
 for(const section of ['', 'work','proposals','pulls','actions','commits','releases','pages','settings']){errors.length=0;console.log('Checking',width,section||'code');const path='/alice/demo-semantic'+(section?'/'+section:'');await page.goto('https://switchyard.cx'+path,{waitUntil:'domcontentloaded'});await page.waitForTimeout(1200);
 const nav=page.locator('nav[aria-label=Repository]:visible');assert.equal(await nav.count(),1,section+' nav');assert.deepEqual(await nav.locator('a').allTextContents(),['Code','Work','Proposals','Pull requests','Actions','Commits','Releases','Pages','Settings']);assert.equal(await nav.locator('[aria-current=page]').count(),1,section+' active');
 assert.deepEqual(errors,[],section);const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);assert(!overflow,section+' horizontal overflow '+width);
 if(section==='settings'){const y=await page.locator('#general').boundingBox();const head=await page.locator('.repository-shell').boundingBox();assert(y.y-head.y-head.height<(width<=768?110:70),JSON.stringify({y,head}));}
 if(section==='proposals'){await page.getByText('No proposals match these filters.').waitFor();assert.equal(await page.locator('#proposal-body .loading-state').count(),0);assert(await page.locator('#proposal-previous').isHidden());}
 if(section==='pages'){await page.getByText('No deployments on this page.').waitFor();assert.equal(await page.locator('#pages-history .loading-state').count(),0);}
 if([1600,390].includes(width)&&['','settings','proposals','pages','actions'].includes(section))await page.screenshot({path:out+'/'+width+'-'+(section||'code')+'.png'});
 results.push({width,section:section||'code',passed:true});}
 await page.goto('https://switchyard.cx/',{waitUntil:'domcontentloaded'});await page.locator('.header-signup').waitFor();await page.locator('.header-signup').click();await page.getByRole('heading',{name:'Create your Switchyard account'}).waitFor();results.push({width,section:'signed-out signup',passed:true});await page.unrouteAll({behavior:'ignoreErrors'});await ctx.close();}
 await fs.writeFile(out+'/browser.json',JSON.stringify(results,null,2));console.log('Passed',results.length,'local UI checks with live public read APIs and isolated maintainer fixtures');}finally{await browser.close();}
