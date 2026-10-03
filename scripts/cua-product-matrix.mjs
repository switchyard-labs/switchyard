// C22 actual-browser gate. No mocks, injected fetches or synthetic page mutations.
import {writeFile,mkdir} from 'node:fs/promises';
export const widths=[1600,1280,1024,768,430,390];
export async function captureSurface(tab,viewport,{name,url,ready,expected,output}){
 if(await tab.url()!==url)await tab.goto(url);await tab.getAXState({emit:false});
 if(ready)await tab.playwright.locator(ready).waitFor({state:'visible',timeoutMs:60000});
 const rows=[];await mkdir(output,{recursive:true});
 for(const width of widths){await viewport.set({width,height:1000});await tab.getAXState({emit:false});const result=await tab.playwright.evaluate(()=>({width:innerWidth,height:innerHeight,documentHeight:document.documentElement.scrollHeight,documentWidth:document.documentElement.scrollWidth,text:document.body.innerText,main:!!document.querySelector('main'),header:document.querySelector('#hamburger')?.getBoundingClientRect().toJSON(),dead:[...document.querySelectorAll('main a')].filter(a=>a.getAttribute('href')==='#').map(a=>a.textContent)}));
  if(result.width!==width)throw Error(name+' viewport mismatch: requested '+width+', actual '+result.width);
  if(result.documentHeight>result.height)throw Error(name+' document scrolls vertically at '+width+': '+result.documentHeight);
  if(result.documentWidth>result.width)throw Error(name+' overflows at '+width+': '+result.documentWidth);
  if(!result.main||!result.header)throw Error(name+' missing product shell');
  if(expected&&!new RegExp(expected,'i').test(result.text))throw Error(name+' missing expected loaded content at '+width);
  if(result.dead.length)throw Error(name+' dead links: '+result.dead.join(', '));
  await writeFile(output+'/'+name+'-'+width+'.jpg',await tab.screenshot({fullPage:false}));rows.push({surface:name,width,actualWidth:result.width,documentHeight:result.documentHeight,documentWidth:result.documentWidth,header:result.header,checks:['loaded content','shell','horizontal and vertical document overflow','actual viewport','valid links']});
 }
 return rows;
}
export async function menu(tab,viewport,output){const results=[];for(const width of widths){await viewport.set({width,height:1000});await tab.playwright.locator('#hamburger').click();await tab.getAXState({emit:false});const actual=await tab.playwright.evaluate(()=>innerWidth);if(actual!==width)throw Error('Menu viewport mismatch');const open=await tab.playwright.locator('#mobile-close').evaluate(e=>e.getBoundingClientRect().toJSON());await writeFile(output+'/menu-'+width+'.jpg',await tab.screenshot({fullPage:false}));await tab.playwright.locator('#mobile-close').click();await tab.getAXState({emit:false});const closed=await tab.playwright.locator('#hamburger').evaluate(e=>e.getBoundingClientRect().toJSON());if(Math.abs(open.x-closed.x)>1||Math.abs(open.y-closed.y)>1)throw Error('Menu shifted at '+width);results.push({surface:'menu',width,checks:['opens','closes','stable rectangle']})}return results}
