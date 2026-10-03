export async function repositorySmoke(tab,stage) {
  const check=(ok,message)=>{if(!ok)throw new Error(message);};
  if(stage==='source'){
    const state=await tab.playwright.evaluate(()=>({selected:document.querySelector('#file-tree [aria-selected="true"]')?.getAttribute('data-path'),raw:document.getElementById('raw-link').getAttribute('href'),source:document.getElementById('file-view').textContent,width:innerWidth,scroll:document.documentElement.scrollWidth}));
    check(state.selected==='src/routes.mjs','Source selection missing');check(/ref=[0-9a-f]{40}/.test(state.raw),'Content is not pinned to an immutable commit');check(state.source.includes('export function connected'),'Actual source did not render');check(state.scroll<=state.width,'Source layout overflows document');
    const folder=tab.playwright.locator('#file-tree [data-path="src"]');
    await folder.press('ArrowLeft');check(await folder.getAttribute('aria-expanded')==='false','Keyboard collapse failed');
    await folder.press('ArrowRight');check(await folder.getAttribute('aria-expanded')==='true','Keyboard expansion failed');
    await folder.press('ArrowRight');await tab.getAXState();
    check(await tab.playwright.evaluate(()=>document.activeElement?.getAttribute('data-path'))==='src/network.json','Keyboard did not enter first child');
    return {stage,checks:7,...state};
  }
  if(stage==='empty'){
    check(await tab.playwright.getByText('This repository is empty',{exact:true}).isVisible(),'Real empty repository state missing');
    const state=await tab.playwright.evaluate(()=>({notice:document.getElementById('repo-notice').textContent,latest:document.getElementById('repo-latest').textContent,width:innerWidth,scroll:document.documentElement.scrollWidth}));
    check(!state.notice,'Empty repository incorrectly reported an error');check(state.latest==='No commits yet','Empty commit state incorrect');check(state.scroll<=state.width,'Empty layout overflows document');
    return {stage,checks:4,...state};
  }
  throw new Error('Unknown repository stage');
}
