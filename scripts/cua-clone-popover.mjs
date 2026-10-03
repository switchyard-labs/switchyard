export async function verifyClonePopover(tab, viewport) {
  const results = [];
  const trigger = tab.playwright.getByRole('button', {name: 'Code', exact: true});
  for (const width of [1600,1280,1024,768,430,390]) {
    await viewport.set({width, height:1000});
    await tab.getAXState({emit:false});
    await trigger.click();
    await tab.getAXState({emit:false});
    const position = await tab.playwright.evaluate(() => {
      const d=document.getElementById('clone-dialog'),b=document.getElementById('clone-toggle');
      return {width:innerWidth,dialog:d.getBoundingClientRect().toJSON(),trigger:b.getBoundingClientRect().toJSON(),focus:document.activeElement.id,open:d.open};
    });
    const d=position.dialog,b=position.trigger;
    if(position.width!==width||!position.open||d.left<0||d.right>width||d.top<0||d.bottom>1000||Math.min(Math.abs(d.top-b.bottom),Math.abs(b.top-d.bottom))>9) throw Error('Unanchored clone popover '+JSON.stringify(position));
    if(position.focus!=='clone-mode-https')throw Error('Missing initial focus');
    await tab.playwright.getByRole('tab',{name:'HTTPS',exact:true}).press('ArrowRight');
    if(!await tab.playwright.getByRole('tabpanel',{name:'Switchyard CLI',exact:true}).isVisible())throw Error('Keyboard mode switch failed');
    await tab.playwright.getByRole('tab',{name:'Switchyard CLI',exact:true}).press('Escape');
    if(await tab.playwright.evaluate(()=>document.getElementById('clone-dialog').open||document.activeElement.id!=='clone-toggle'))throw Error('Escape focus return failed');
    await trigger.click();await trigger.click();
    if(await tab.playwright.evaluate(()=>document.getElementById('clone-dialog').open))throw Error('Trigger toggle failed');
    await trigger.click();await tab.playwright.locator('#repo-name').click();
    if(await tab.playwright.evaluate(()=>document.getElementById('clone-dialog').open||document.activeElement.id!=='clone-toggle'))throw Error('Outside dismissal failed');
    results.push({...position,checks:['anchored','viewport bounds','keyboard mode switch','Escape','focus return','trigger toggle','outside dismissal']});
  }
  await viewport.reset();
  return results;
}
