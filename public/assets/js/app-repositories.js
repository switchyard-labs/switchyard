(function(){"use strict";
const list=document.getElementById("repo-directory"), search=document.getElementById("repo-search"), count=document.getElementById("repo-count");
const esc=s=>String(s==null?"":s).replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
let repos=[];
async function api(p){const r=await fetch(p,{credentials:"same-origin"}),b=await r.json().catch(()=>({}));if(!r.ok)throw new Error(b.error||("HTTP "+r.status));return b;}
function draw(){const q=(search.value||"").trim().toLowerCase();const xs=repos.filter(r=>!q||(r.full_name||"").toLowerCase().includes(q)||(r.description||"").toLowerCase().includes(q));count.textContent=xs.length+" repositor"+(xs.length===1?"y":"ies");list.innerHTML=xs.length?xs.map(r=>`<a class="repo-directory-row" href="/${encodeURIComponent(r.owner_slug)}/${encodeURIComponent(r.slug)}"><div><strong>${esc(r.full_name)}</strong><p>${esc(r.description||"No description")}</p><span>${esc(r.default_branch||"main")}</span></div><span class="visibility-pill ${esc(r.visibility||"private")}">${esc(r.visibility||"private")}</span></a>`).join(""):'<div class="empty-state"><strong>No matching repositories.</strong><span>Try a different search, or open one of the curated demos.</span></div>';}
(async()=>{try{repos=(await api("/api/repositories")).items||[];draw();}catch(e){list.innerHTML='<div class="dashboard-empty"><strong>Sign in to browse repositories</strong><span>'+esc(e.message)+'</span><a class="btn primary mini" href="/signin.html">Sign in</a></div>';count.textContent="";}})();
search&&search.addEventListener("input",draw);
})();
