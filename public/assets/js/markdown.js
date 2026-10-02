/* Sanitized Markdown renderer for repository/profile READMEs.
   Intentionally does not pass raw HTML through. */
(function(root){
  "use strict";
  const esc=(s)=>window.SwitchyardCode?SwitchyardCode.escapeHTML(s):String(s).replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
  const safeHref=(href)=>{
    href=String(href||"").trim();
    if(/^javascript:/i.test(href)||/^data:/i.test(href)) return "#";
    return href;
  };
  function inline(s,resolve){
    s=esc(s);
    s=s.replace(/!\[([^\]]*)\]\(([^)]+)\)/g,(_,alt,src)=>'<img loading="lazy" alt="'+alt+'" src="'+esc(resolve(safeHref(src),'image'))+'">');
    s=s.replace(/\[([^\]]+)\]\(([^)]+)\)/g,(_,txt,href)=>'<a href="'+esc(resolve(safeHref(href),'link'))+'">'+txt+'</a>');
    s=s.replace(/`([^`]+)`/g,'<code>$1</code>');
    s=s.replace(/\*\*([^*]+)\*\*/g,'<strong>$1</strong>');
    s=s.replace(/\*([^*]+)\*/g,'<em>$1</em>');
    return s;
  }
  function slug(s){return String(s).toLowerCase().replace(/<[^>]+>/g,'').replace(/[^a-z0-9\s-]/g,'').trim().replace(/\s+/g,'-').slice(0,80);}
  function render(md,opts){
    opts=opts||{}; const resolve=opts.resolve||((u)=>u); const lines=String(md||"").replace(/\r\n?/g,'\n').split('\n'); let out=[], inCode=false, code=[], lang='text', inList=false, inQuote=false;
    const closeList=()=>{if(inList){out.push('</ul>');inList=false;}};
    for(let i=0;i<lines.length;i++){
      const line=lines[i];
      let m;
      if((m=line.match(/^```\s*([\w+-]*)/))){ if(!inCode){closeList();inCode=true;lang=m[1]||'text';code=[];} else {const id=(SwitchyardCode.languages.find(x=>x.id===lang)||{}).id||({js:'javascript',ts:'typescript',sh:'shell',py:'python'}[lang]||'text'); out.push('<pre class="md-code code-surface"><code>'+SwitchyardCode.highlight(code.join('\n'),id)+'</code></pre>');inCode=false;} continue; }
      if(inCode){code.push(line);continue;}
      if(!line.trim()){closeList();out.push('');continue;}
      if((m=line.match(/^(#{1,6})\s+(.+)/))){closeList();const lvl=m[1].length,txt=inline(m[2],resolve),id=slug(m[2]);out.push(`<h${lvl} id="${id}"><a class="heading-anchor" href="#${id}">#</a>${txt}</h${lvl}>`);continue;}
      if((m=line.match(/^[-*]\s+(.+)/))){if(!inList){out.push('<ul>');inList=true;}out.push('<li>'+inline(m[1],resolve)+'</li>');continue;}
      if((m=line.match(/^>\s?(.*)/))){closeList();out.push('<blockquote>'+inline(m[1],resolve)+'</blockquote>');continue;}
      if(/^\|.*\|\s*$/.test(line)&&i+1<lines.length&&/^\|?\s*:?-+/.test(lines[i+1])){closeList(); const heads=line.split('|').slice(1,-1).map(x=>x.trim()); i++; const rows=[]; while(i+1<lines.length&&/^\|.*\|\s*$/.test(lines[i+1])) rows.push(lines[++i].split('|').slice(1,-1).map(x=>x.trim())); out.push('<div class="table-scroll"><table><thead><tr>'+heads.map(x=>'<th>'+inline(x,resolve)+'</th>').join('')+'</tr></thead><tbody>'+rows.map(r=>'<tr>'+r.map(x=>'<td>'+inline(x,resolve)+'</td>').join('')+'</tr>').join('')+'</tbody></table></div>');continue;}
      closeList(); out.push('<p>'+inline(line,resolve)+'</p>');
    }
    closeList(); if(inCode) out.push('<pre class="md-code code-surface"><code>'+SwitchyardCode.highlight(code.join('\n'),lang)+'</code></pre>');
    return out.join('\n');
  }
  root.SwitchyardMarkdown={render,safeHref}; if(typeof module!=="undefined"&&module.exports)module.exports=root.SwitchyardMarkdown;
})(typeof globalThis!=="undefined"?globalThis:window);
