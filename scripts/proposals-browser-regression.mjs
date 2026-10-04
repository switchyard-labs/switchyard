// Fixture rendering certification; API lifecycle/dogfood is a separate check.
import {readFile,mkdir,writeFile} from 'node:fs/promises';
import path from 'node:path';
const {chromium}=await import(process.env.SWITCHYARD_PLAYWRIGHT_MODULE||'playwright');
const output=process.argv[2]||'/tmp/switchyard-proposal-browser';
await mkdir(output,{recursive:true});
const browser=await chromium.launch({headless:true,executablePath:process.env.SWITCHYARD_CHROMIUM_EXECUTABLE||undefined});
const proposal={id:'prop_fixture',title:'A long Proposal title '+('evidence '.repeat(30)),type:'Question',state:'open',description:'long_unbroken_evidence_'.repeat(200),author_principal:'alice',version:'1',labels:['browser'],provenance:{source:'review',source_sha:'a'.repeat(40)},history:[{kind:'created',actor:'alice',at:'2026-10-05'}]};
const results=[];
try{
 for(const width of [1600,1280,1024,768,430,390]){
  const page=await browser.newPage({viewport:{width,height:1000}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.route('http://proposal.test/**',async route=>{
   const url=new URL(route.request().url());let value;
   if(url.pathname==='/api/auth/me')value={username:'alice'};
   else if(url.pathname.endsWith('/proposal-settings'))value={can_create:true};
   else if(url.pathname.endsWith('/comments'))value={version:'1',items:[{id:'pc_fixture',author_principal:'alice',body:'Own discussion',created_at:'2026-10-05'}]};
   else if(url.pathname.endsWith('/links'))value={version:'1',items:[]};
   else if(url.pathname.endsWith('/proposals/prop_fixture'))value=proposal;
   else if(url.pathname.endsWith('/proposals'))value={items:[proposal],total:1,page:1,page_size:20};
   else if(url.pathname==='/api/repositories/alice/demo')value={can_write:true,visibility:'public'};
   else if(url.pathname.startsWith('/api/'))value={};
   if(url.pathname.startsWith('/api/')&&value!==undefined)return route.fulfill({contentType:'application/json',body:JSON.stringify(value)});
   const file=url.pathname.startsWith('/assets/')?url.pathname.slice(1):'proposals.html';
   try{const body=await readFile(path.resolve('public',file));return route.fulfill({body,contentType:file.endsWith('.css')?'text/css':file.endsWith('.js')?'text/javascript':file.endsWith('.svg')?'image/svg+xml':'text/html'});}catch{return route.fulfill({status:404,body:''});}
  });
  for(const [name,suffix] of [['list',''],['detail','/prop_fixture']]){
   await page.goto('http://proposal.test/alice/demo/proposals'+suffix);await page.locator(name==='list'?'.work-row':'#proposal-comment').waitFor();
   if(await page.locator('#proposal-error').isVisible())throw Error(await page.locator('#proposal-error').innerText());
   const bounds=await page.evaluate(()=>({width:innerWidth,height:innerHeight,scrollWidth:document.documentElement.scrollWidth,scrollHeight:document.documentElement.scrollHeight}));
   if(bounds.scrollWidth>width||bounds.scrollHeight>bounds.height)throw Error(`${width} ${name} page overflow: ${JSON.stringify(bounds)}`);
   if(name==='detail'){await page.getByRole('button',{name:'Edit',exact:true}).click();await page.getByRole('button',{name:'Cancel',exact:true}).click();}
   await page.screenshot({path:path.join(output,`${width}-${name}.png`)});results.push({width,surface:name,bounds});
  }
  if(errors.length)throw Error(errors.join('; '));await page.close();
 }
 await writeFile(path.join(output,'results.json'),JSON.stringify(results,null,2));
 console.log(JSON.stringify({passed:results.length,output}));
}finally{await browser.close();}
