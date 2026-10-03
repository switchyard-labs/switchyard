// Run through the authorized cua browser runtime; no shell browser driver.
export async function shellSmoke(tab) {
  await tab.playwright.getByRole('button',{name:'Open navigation',exact:true}).click();
  await tab.getAXState();
  const open = await tab.playwright.evaluate(() => {
    const rect = id => { const r=document.getElementById(id).getBoundingClientRect();return [r.x,r.y,r.width,r.height]; };
    return {open:rect('hamburger'),close:rect('mobile-close'),focus:document.activeElement.id,inert:document.querySelector('main').hasAttribute('inert'),width:innerWidth,scroll:document.documentElement.scrollWidth};
  });
  if(open.open.some((v,i)=>Math.abs(v-open.close[i])>.5)||open.focus!=='mobile-close'||!open.inert||open.scroll>open.width)throw new Error('Navigation geometry or focus failed');
  await tab.playwright.getByRole('button',{name:'Close navigation',exact:true}).press('Escape');
  await tab.getAXState();
  const closed=await tab.playwright.evaluate(()=>({hidden:document.getElementById('mobile-menu').hidden,inert:document.querySelector('main').hasAttribute('inert'),focus:document.activeElement.id}));
  if(!closed.hidden||closed.inert||closed.focus!=='hamburger')throw new Error('Navigation did not restore focus/background');
  return {open,closed,checks:7};
}
