/* Shared repository hierarchy. Git paths remain data; rendering uses DOM text nodes. */
(function(root){
  'use strict';
  function build(entries){
    const tree={name:'',path:'',directory:true,children:[]},index=new Map([['',tree]]);
    for(const entry of entries){
      const path=typeof entry==='string'?entry:entry.path;
      if(!path||path.startsWith('/')||path.split('/').some(p=>!p||p==='.'||p==='..'))continue;
      const parts=path.split('/');let parent=tree,current='';
      parts.forEach((name,i)=>{current+=(current?'/':'')+name;let node=index.get(current);if(!node){node={name,path:current,directory:i<parts.length-1,children:[],type:entry.type||'blob'};index.set(current,node);parent.children.push(node);}parent=node;});
    }
    function sort(node){node.children.sort((a,b)=>Number(b.directory)-Number(a.directory)||a.name.localeCompare(b.name,undefined,{numeric:true}));node.children.forEach(sort);}
    sort(tree);return {tree,index};
  }
  const icon=directory=>directory?'<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M2 5h6l2 2h8v10H2z"/></svg>':'<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M5 2h7l4 4v12H5zM12 2v5h4"/></svg>';
  function mount(container,hierarchy,{selected='',url,onSelect}={}){
    container.replaceChildren();container.setAttribute('role','tree');container.setAttribute('aria-label','Repository files');
    const expanded=new Set();let ancestor=selected.split('/').slice(0,-1);while(ancestor.length){expanded.add(ancestor.join('/'));ancestor.pop();}
    function branch(nodes,parent,level){for(const node of nodes){const wrap=document.createElement('div'),control=document.createElement(node.directory?'button':'a');wrap.className='repo-tree-node';control.className='repo-tree-item';control.setAttribute('role','treeitem');control.setAttribute('aria-level',level);control.dataset.path=node.path;control.tabIndex=-1;control.style.setProperty('--tree-level',level-1);const glyph=document.createElement('span');glyph.className='repo-tree-glyph';glyph.innerHTML=icon(node.directory);const name=document.createElement('span');name.textContent=node.name;control.append(glyph,name);wrap.append(control);parent.append(wrap);
      if(node.directory){const group=document.createElement('div');group.setAttribute('role','group');wrap.append(group);branch(node.children,group,level+1);const set=open=>{control.setAttribute('aria-expanded',String(open));group.hidden=!open;};set(expanded.has(node.path));control.onclick=()=>set(control.getAttribute('aria-expanded')!=='true');}
      else{control.href=url(node.path,'blob');control.setAttribute('aria-selected',String(selected===node.path));if(onSelect)control.onclick=e=>{e.preventDefault();onSelect(node.path);};}
    }}
    branch(hierarchy.tree.children,container,1);
    const visible=()=>[...container.querySelectorAll('[role=treeitem]')].filter(n=>n.getClientRects().length);
    const initial=[...container.querySelectorAll('[role=treeitem]')].find(n=>n.dataset.path===selected)||container.querySelector('[role=treeitem]');if(initial)initial.tabIndex=0;
    container.addEventListener('focusin',e=>{const item=e.target.closest('[role=treeitem]');if(!item)return;container.querySelectorAll('[role=treeitem]').forEach(n=>n.tabIndex=n===item?0:-1);});
    container.onkeydown=e=>{const item=e.target.closest('[role=treeitem]');if(!item)return;const list=visible(),i=list.indexOf(item);let target;
      if(e.key==='ArrowDown')target=list[Math.min(i+1,list.length-1)];
      else if(e.key==='ArrowUp')target=list[Math.max(0,i-1)];
      else if(e.key==='Home')target=list[0];else if(e.key==='End')target=list.at(-1);
      else if(e.key==='ArrowRight'){if(item.getAttribute('aria-expanded')==='false')item.click();else if(item.hasAttribute('aria-expanded'))target=list[i+1];}
      else if(e.key==='ArrowLeft'){if(item.getAttribute('aria-expanded')==='true')item.click();else target=item.parentElement.parentElement.closest('.repo-tree-node')?.querySelector('[role=treeitem]');}
      else if(e.key==='Enter'||e.key===' '){e.preventDefault();item.click();return;}else return;
      e.preventDefault();target?.focus();
    };
    return {visible};
  }
  const api={build,mount,icon};root.SwitchyardTree=api;if(typeof module!=='undefined'&&module.exports)module.exports=api;
})(typeof globalThis!=='undefined'?globalThis:window);
