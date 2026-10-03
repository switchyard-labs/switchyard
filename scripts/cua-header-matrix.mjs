import {writeFile} from 'node:fs/promises';
export async function check(tab,viewport,origin,path){
 await tab.goto(origin+path);await tab.getAXState({emit:false});
 await tab.playwright.locator('.account-chip').waitFor({state:'visible',timeoutMs:30000});
 const rows=[];
 for(const width of [1600,1280,1024,768,430,390]){
 await viewport.set({width,height:900});
 const row=await tab.playwright.evaluate(()=>{const a=document.querySelector('.account-chip'),m=document.querySelector('#hamburger'),i=a.querySelector('img'),ar=a.getBoundingClientRect(),mr=m.getBoundingClientRect();return {width:innerWidth,account:[ar.width,ar.height],menu:[mr.width,mr.height],text:a.textContent.trim(),border:getComputedStyle(i).borderWidth,image:i.getBoundingClientRect().width,overflow:[document.documentElement.scrollWidth>innerWidth,document.documentElement.scrollHeight>innerHeight]};});
 if(row.width!==width||row.account.join()!=row.menu.join()||row.text||row.border!=='0px'||row.image!==row.account[0]-2||row.overflow.some(Boolean))throw Error(path+' '+JSON.stringify(row));rows.push({path,...row});
 }return rows;
}
