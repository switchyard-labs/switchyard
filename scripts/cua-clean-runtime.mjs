// Run through the documented CUA API against the final embedded runtime.
import {mkdir,writeFile} from 'node:fs/promises';
export async function captureRuntimePage(tab,{name,url,ready,expected,output}) {
 if(await tab.url()!==url)await tab.goto(url);
 await tab.getAXState({emit:false});
 for(let attempt=0;attempt<12;attempt++) {
  try {await tab.playwright.locator(ready).first().waitFor({state:'visible',timeoutMs:3000});break;}
  catch(error) {const snapshot=await tab.playwright.domSnapshot();if(snapshot.includes('- alert:')||attempt===11)throw error;}
 }
 const result=await tab.playwright.evaluate(()=>({url:location.href,width:innerWidth,height:innerHeight,documentWidth:document.documentElement.scrollWidth,documentHeight:document.documentElement.scrollHeight,text:document.body.innerText,main:!!document.querySelector('main'),brokenImages:[...document.images].filter(i=>i.complete&&!i.naturalWidth).map(i=>i.getAttribute('src'))}));
 if(!result.main||!new RegExp(expected,'i').test(result.text))throw Error(name+' not fully loaded');
 if(result.documentWidth>result.width||result.documentHeight>result.height)throw Error(name+' document overflow');
 if(result.brokenImages.length)throw Error(name+' broken images: '+result.brokenImages.join(', '));
 await mkdir(output,{recursive:true});await writeFile(output+'/'+name+'.png',await tab.screenshot({fullPage:false}));
 return {surface:name,url:result.url,width:result.width,checks:['loaded content','embedded shell','document bounds','loaded images'],console:await tab.dev.logs({levels:['error'],limit:5})};
}
