(function(){
  'use strict';
  function infer(text, cls){
    cls=String(cls||'');
    let m=cls.match(/language-([\w+-]+)/); if(m) return alias(m[1]);
    const s=String(text||'').trim();
    if(/^\s*[\[{]/.test(s) && /[\]}]\s*$/.test(s)) return 'json';
    if(/\b(function|const|let|var|async|await)\b/.test(s) && /[{};]/.test(s)) return 'javascript';
    if(/^(git|go |ssh |curl |export |SWITCHYARD_|systemctl |nift |\$ )/m.test(s) || /^#\s*(clone|build|run|copy|start|create|push|fetch)/mi.test(s)) return 'shell';
    if(/^(SELECT|INSERT|UPDATE|DELETE|CREATE TABLE)\b/im.test(s)) return 'sql';
    if(/^\s*<\/?[A-Za-z]/.test(s)) return 'html';
    return 'text';
  }
  function alias(x){return ({js:'javascript',ts:'typescript',sh:'shell',bash:'shell',py:'python',md:'markdown',yml:'yaml'}[x]||x||'text');}
  function icon(){return '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="8" y="8" width="10" height="10" rx="1.5"></rect><path d="M6 16H5.5A1.5 1.5 0 0 1 4 14.5v-9A1.5 1.5 0 0 1 5.5 4h9A1.5 1.5 0 0 1 16 5.5V6"></path></svg>'}
  function enhance(pre){
    if(!pre || pre.dataset.codeEnhanced==='1') return;
    const code=pre.querySelector('code'); if(!code) return;
    pre.dataset.codeEnhanced='1';
    const raw=code.textContent;
    const lang=infer(raw, code.className);
    if(window.SwitchyardCode && lang!=='text') code.innerHTML=window.SwitchyardCode.highlight(raw,lang);
    code.classList.add('code-surface');
    pre.classList.add('enhanced-code');
    const bar=document.createElement('div'); bar.className='code-toolbar';
    const label=document.createElement('span'); label.className='code-language'; label.textContent=(window.SwitchyardCode?window.SwitchyardCode.labelFor(lang):lang);
    const btn=document.createElement('button'); btn.type='button'; btn.className='code-copy'; btn.setAttribute('aria-label','Copy code'); btn.innerHTML=icon();
    btn.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(raw); btn.classList.add('copied'); btn.setAttribute('aria-label','Copied'); setTimeout(()=>{btn.classList.remove('copied');btn.setAttribute('aria-label','Copy code')},1400);}catch(_){}});
    bar.append(label,btn); pre.parentNode.insertBefore(bar,pre); bar.appendChild(pre);
  }
  function scan(root){(root||document).querySelectorAll('pre').forEach(enhance)}
  if(document.readyState==='loading') document.addEventListener('DOMContentLoaded',()=>scan(document)); else scan(document);
  new MutationObserver(ms=>ms.forEach(m=>m.addedNodes.forEach(n=>{if(n.nodeType===1){if(n.matches&&n.matches('pre')) enhance(n); scan(n)}}))).observe(document.documentElement,{subtree:true,childList:true});
})();
