// Run with an authenticated cua_repl tab; all assertions inspect rendered UI.
export async function editor(tab) {
 const result=[];
 const check=(name,ok)=>{if(!ok)throw Error(name);result.push(name)};
 const dom=await tab.playwright.evaluate(()=>({tree:!!document.querySelector('[role=tree]'),selected:!!document.querySelector('[role=treeitem][aria-selected=true]'),editor:!!document.querySelector('.cm-content'),language:document.querySelector('#status-language')?.textContent,tabs:document.querySelectorAll('#editor-tabs button').length,width:document.documentElement.scrollWidth,viewport:innerWidth,terminal:!!document.querySelector('input[placeholder*=terminal],.terminal-input'),links:[...document.querySelectorAll('a')].map(a=>({text:a.textContent.trim(),href:a.getAttribute('href')}))}));
 check('Hierarchical explorer rendered',dom.tree);
 check('Current file selected',dom.selected);
 check('CodeMirror editor rendered',dom.editor);
 check('Language-aware editor',dom.language!=='Plain text');
 check('Multiple tab controls',dom.tabs>=2);
 check('No interactive terminal',!dom.terminal);
 check('Repository return link',dom.links.some(a=>a.text==='Repository'&&a.href.startsWith('/alice/railway')));
 check('No document overflow',dom.width<=dom.viewport);
 return result;
}
export async function search(tab,query) {
 await tab.playwright.locator('.workbench-modes button[data-mode=search]').click();
 await tab.playwright.locator('#repo-search').fill(query);
 await tab.playwright.locator('#search-run').click();
}
