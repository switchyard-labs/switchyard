/* Switchyard web application island. */
(function () {
  "use strict";

  async function api(path, opts) {
    const res = await fetch(path, {
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      ...opts,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok && !(path === "/api/auth/me" && res.status === 401)) {
      throw new Error(data.error || ("HTTP " + res.status));
    }
    return data;
  }

  function el(html) {
    const t = document.createElement("template");
    t.innerHTML = html.trim();
    return t.content.firstChild;
  }

  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  }

  /* mobile menu: full-screen, Escape dismisses, focus trapped */
  const hamburger = document.getElementById("hamburger");
  const mobileMenu = document.getElementById("mobile-menu");
  const mobileClose = document.getElementById("mobile-close");
  function openMenu() {
    if (!mobileMenu) return;
    mobileMenu.hidden = false;
    document.body.classList.add("menu-open");
    hamburger.setAttribute("aria-expanded", "true");
    mobileClose.focus();
    document.addEventListener("keydown", trap);
  }
  function closeMenu() {
    if (!mobileMenu) return;
    mobileMenu.hidden = true;
    document.body.classList.remove("menu-open");
    hamburger.setAttribute("aria-expanded", "false");
    document.removeEventListener("keydown", trap);
    hamburger.focus();
  }
  function trap(e) {
    if (e.key === "Escape") closeMenu();
    if (e.key === "Tab") {
      const focusables = [...mobileMenu.querySelectorAll("a[href], button, summary")].filter((x) => !x.hasAttribute("disabled"));
      const first = focusables[0], last = focusables[focusables.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    }
  }
  if (hamburger) hamburger.addEventListener("click", openMenu);
  if (mobileClose) mobileClose.addEventListener("click", closeMenu);
  if (mobileMenu) mobileMenu.addEventListener("click", (e) => { if (e.target.closest("a[href]")) closeMenu(); });
  if (mobileMenu) {
    const here = location.pathname.replace(/\/index\.html$/, "/");
    mobileMenu.querySelectorAll("a[href]").forEach((a) => {
      try { const p = new URL(a.href, location.href).pathname.replace(/\/index\.html$/, "/"); if (p === here) a.setAttribute("aria-current", "page"); } catch (_) {}
    });
  }

  /* auth state */
  async function refreshAuth() {
    let user = "";
    try { const m = await api("/api/auth/me"); if (m.authed) user = m.user; } catch (e) { /* ignore */ }
    const state = document.getElementById("auth-state");
    const mAuth = document.getElementById("mobile-auth");
    if (state) {
      state.innerHTML = user ? '<a class="account-chip" href="/'+encodeURIComponent(user)+'"><img src="/api/avatars/user/'+encodeURIComponent(user)+'" alt=""><span>'+esc(user)+'</span></a>' : '<a href="/signin.html">Sign in</a>';
    }
    if (mAuth) mAuth.innerHTML = user
      ? '<a href="/'+encodeURIComponent(user)+'">Profile</a><a href="/settings">Settings</a><a href="#" id="logout-mobile">Sign out ('+esc(user)+')</a>'
      : '<a href="/signin.html">Sign in</a>';
    const lo = document.getElementById("logout-mobile");
    if (lo) lo.onclick = async (e) => { e.preventDefault(); await api("/api/auth/logout", { method: "POST" }); location.reload(); };
    return user;
  }

  /* dashboard */
  async function renderDashboard() {
    const reposEl = document.getElementById("repos");
    const workEl = document.getElementById("work");
    if (!reposEl && !workEl) return;
    const user = await refreshAuth();
    if (!user) { reposEl.innerHTML = '<p class="muted">Sign in to browse repositories.</p>'; workEl.innerHTML = ""; return; }
    try {
      const repos = await api("/api/repos");
      reposEl.innerHTML = repos.items.length
        ? repos.items.map((r) =>
            "<div class='list-item'><a href='/repo.html?name=" + esc(r.name) + "'>" + esc(r.name) + "</a>" +
            "<span class='muted'>" + esc(r.default_branch) + "</span>" +
            (r.registered ? "<span class='muted'>registered</span>" : "") + "</div>").join("")
        : "<p class='muted'>No repositories.</p>";
    } catch (e) { reposEl.innerHTML = "<p class='error'>" + esc(e.message) + "</p>"; }
    try {
      const work = await api("/api/work");
      workEl.innerHTML = work.items.length
        ? work.items.slice(0, 10).map((w) =>
            "<div class='list-item'><a href='#' class='work-link' data-id='" + esc(w.id) + "'>" + esc(w.title) + "</a>" +
            "<span class='muted'>" + esc(w.kind) + " · " + esc(w.status) + "</span></div>").join("")
        : "<p class='muted'>No work yet — create some from the Work page.</p>";
      document.querySelectorAll(".work-link").forEach((a) => a.onclick = (e) => e.preventDefault());
    } catch (e) { workEl.innerHTML = "<p class='error'>" + esc(e.message) + "</p>"; }
  }

  /* sign in / register */
  function renderSignin() {
    const form = document.getElementById("signin-form");
    if (!form) return;
    const err = document.getElementById("auth-error");
    const toggle = document.getElementById("toggle-mode");
    let mode = "login";
    toggle.addEventListener("click", (e) => {
      e.preventDefault();
      mode = mode === "login" ? "register" : "login";
      form.querySelector("button").textContent = mode === "login" ? "Sign in" : "Register";
      toggle.textContent = mode === "login" ? "Register" : "Sign in";
    });
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      err.hidden = true;
      const body = { username: form.username.value, password: form.password.value };
      if (mode === "register") body.display_name = form.username.value;
      try {
        await api(mode === "login" ? "/api/auth/login" : "/api/auth/register", { method: "POST", body: JSON.stringify(body) });
        location.href = "/";
      } catch (ex) { err.textContent = ex.message; err.hidden = false; }
    });
  }

  /* work page */
  function renderWork() {
    const form = document.getElementById("work-form");
    const list = document.getElementById("work-list");
    if (!form) return;
    if (form) form.addEventListener("submit", async (e) => {
      e.preventDefault();
      try {
        await api("/api/work", { method: "POST", body: JSON.stringify({ title: form.title.value, kind: form.kind.value, body: form.body ? form.body.value : "", repo: form.repo ? form.repo.value : "", assignee: form.assignee ? form.assignee.value : "" }) });
        form.title.value = "";
        await loadWork(list);
      } catch (ex) { list.innerHTML = "<p class='error'>" + esc(ex.message) + "</p>"; }
    });
    loadWork(list);
  }
  async function loadWork(list) {
    try {
      const work = await api("/api/work");
      const draw = (filter="open") => { const xs=work.items.filter(w=>filter==="all"||w.status===filter); list.innerHTML = xs.length ? xs.map((w) => "<a class='work-row' href='/work/"+encodeURIComponent(w.id)+"'><span class='status-badge "+esc(w.status)+"'>"+esc(w.status)+"</span><div><strong>" + esc(w.title) + "</strong><span>" + esc(w.kind) + " · opened by " + esc(w.owner) + "</span></div></a>").join("") : "<div class='empty-state'><strong>No "+esc(filter)+" work.</strong><span>Create a bug, feature, investigation or maintenance item without involving an Agent.</span></div>"; }; draw(); document.querySelectorAll('[data-work-filter]').forEach(b=>b.onclick=()=>{document.querySelectorAll('[data-work-filter]').forEach(x=>x.classList.toggle('active',x===b));draw(b.dataset.workFilter)});
    } catch (e) { list.innerHTML = "<p class='error'>" + esc(e.message) + "</p>"; }
  }

  /* repo browser */
  function canonicalRepoContext() {
    const params = new URLSearchParams(location.search), legacy=params.get("name");
    if (legacy) return {legacy:true,artifact:legacy,owner:"",repo:"",ref:params.get("ref")||"main",path:params.get("path")||""};
    const parts=location.pathname.split("/").filter(Boolean).map(decodeURIComponent);
    if(parts.length<2) return {legacy:true,artifact:"",owner:"",repo:"",ref:"main",path:""};
    const out={legacy:false,owner:parts[0],repo:parts[1],artifact:"",ref:"main",path:""};
    if(parts.length>=4&&(parts[2]==="blob"||parts[2]==="tree")){out.kind=parts[2];out.ref=parts[3];out.path=parts.slice(4).join("/");}
    return out;
  }
  function canonicalRepoURL(ctx,kind,ref,path){const base="/"+encodeURIComponent(ctx.owner)+"/"+encodeURIComponent(ctx.repo); if(!kind)return base; const tail=path?"/"+path.split("/").map(encodeURIComponent).join("/"):""; return base+"/"+kind+"/"+encodeURIComponent(ref||"main")+tail;}
  function repoRoot(ctx){return ctx.legacy?"/api/repos/"+encodeURIComponent(ctx.artifact):"/api/repositories/"+encodeURIComponent(ctx.owner)+"/"+encodeURIComponent(ctx.repo);}
  function rawURL(ctx,path){return repoRoot(ctx)+"/content?ref="+encodeURIComponent(ctx.ref)+"&path="+encodeURIComponent(path);}
  function renderBreadcrumbs(ctx){const el=document.getElementById("repo-breadcrumbs"); if(!el)return; const ps=(ctx.path||"").split("/").filter(Boolean); let cur=""; el.innerHTML='<a href="'+(ctx.legacy?'/repo.html?name='+encodeURIComponent(ctx.artifact):canonicalRepoURL(ctx))+ '">'+esc(ctx.repo||ctx.artifact)+'</a>'+ps.map((p,i)=>{cur+=(cur?"/":"")+p; return '<span>/</span><a href="'+(i===ps.length-1?'#':(ctx.legacy?'/repo.html?name='+encodeURIComponent(ctx.artifact)+'&ref='+encodeURIComponent(ctx.ref)+'&path='+encodeURIComponent(cur):canonicalRepoURL(ctx,'tree',ctx.ref,cur)))+'">'+esc(p)+'</a>';}).join('');}
  async function renderRepo(){
    const nameEl=document.getElementById("repo-name"),treeEl=document.getElementById("file-tree"),viewEl=document.getElementById("file-view"); if(!nameEl)return;
    const ctx=canonicalRepoContext();
    if(!ctx.legacy){const meta=await api(repoRoot(ctx));ctx.artifact=meta.artifact_name;nameEl.textContent=meta.slug||meta.display_name; document.getElementById("repo-owner").textContent=meta.owner_slug||ctx.owner; document.getElementById("repo-description").textContent=meta.description||""; const vis=document.getElementById("repo-visibility"); if(vis)vis.textContent=meta.visibility||"private"; if(!ctx.ref||ctx.ref==="main")ctx.ref=meta.default_branch||ctx.ref;} else nameEl.textContent=ctx.artifact;
    const sel=document.getElementById("repo-ref-select"); if(sel){try{const rr=await api(repoRoot(ctx)+"/refs"); const branches=Object.keys(rr.refs||{}).filter(x=>x.startsWith('refs/heads/')).map(x=>x.slice(11)); sel.innerHTML=branches.map(x=>'<option '+(x===ctx.ref?'selected':'')+'>'+esc(x)+'</option>').join('')||'<option>'+esc(ctx.ref)+'</option>'; sel.onchange=()=>{location.href=ctx.legacy?'/repo.html?name='+encodeURIComponent(ctx.artifact)+'&ref='+encodeURIComponent(sel.value):(canonicalRepoURL(ctx,'tree',sel.value,''));};}catch(e){}}
    if(!ctx.legacy){
      try{
        const ov=await api(repoRoot(ctx)+"/overview");
        const latest=ov.latest_commit||{};
        const msg=(latest.Message||latest.message||"No commits yet").split("\n")[0];
        document.getElementById("repo-latest").textContent=msg;
        const hash=latest.Hash||latest.hash||"";
        const ts=latest.Timestamp||latest.committedAt||latest.timestamp||0;
        document.getElementById("repo-latest-meta").textContent=(hash?hash.slice(0,7)+" · ":"")+(ts?new Date(Number(ts)*1000).toLocaleString():"");
        document.getElementById("repo-branches").textContent=ov.branch_count||0;
        document.getElementById("repo-tags").textContent=ov.tag_count||0;
        document.getElementById("repo-commits").textContent=ov.commit_count_sample||0;
        const dlg=document.getElementById("clone-dialog"), input=document.getElementById("clone-url"); input.value=ov.clone_https||"";
        document.getElementById("clone-toggle").onclick=()=>dlg.showModal();
        document.getElementById("clone-copy").onclick=async()=>{await navigator.clipboard.writeText(input.value); document.getElementById("clone-copy").textContent="Copied";};
        document.getElementById("tab-code").href=canonicalRepoURL(ctx);
        document.getElementById("tab-commits").href=canonicalRepoURL(ctx)+"/commits/"+encodeURIComponent(ctx.ref);
        document.getElementById("tab-prs").href=canonicalRepoURL(ctx)+"/pulls";
        document.getElementById("tab-settings").href=canonicalRepoURL(ctx)+"/settings";
      }catch(e){ const strip=document.getElementById("repo-overview-strip"); if(strip)strip.innerHTML='<p class="error">'+esc(e.message)+'</p>'; }
    } else { const strip=document.getElementById("repo-overview-strip"); if(strip)strip.hidden=true; }
    renderBreadcrumbs(ctx); loadTree(ctx,treeEl,viewEl);
  }
  async function loadTree(ctx,treeEl,viewEl){
    try{
      const root=repoRoot(ctx),t=await api(root+"/tree?ref="+encodeURIComponent(ctx.ref)),files=t.tree.map(it=>it.path).sort(); treeEl.innerHTML="";
      for(const f of files){const row=el("<div class='file' data-path='"+esc(f)+"'><span class='file-icon'>·</span>"+esc(f)+"</div>"); row.onclick=()=>location.href=ctx.legacy?"/repo.html?name="+encodeURIComponent(ctx.artifact)+"&ref="+encodeURIComponent(ctx.ref)+"&path="+encodeURIComponent(f):canonicalRepoURL(ctx,"blob",ctx.ref,f); treeEl.appendChild(row);}
      if(!ctx.path){
        const candidates=['README.md','README.markdown','README','docs/README.md'];
        const found=candidates.find(x=>files.includes(x));
        if(found){
          const rr=await fetch(rawURL(ctx,found),{credentials:'same-origin'});
          if(rr.ok){const md=await rr.text(), readme=document.getElementById('readme-view'); if(readme){
            const base=found.includes('/')?found.slice(0,found.lastIndexOf('/')+1):'';
            const resolver=(u,kind)=>{ if(/^https?:\/\//i.test(u)||u.startsWith('#')||u.startsWith('mailto:'))return u; let path=(base+u).split('/').reduce((a,x)=>{if(x==='..')a.pop();else if(x&&x!=='.')a.push(x);return a;},[]).join('/'); return kind==='image'?rawURL(ctx,path):(ctx.legacy?'/repo.html?name='+encodeURIComponent(ctx.artifact)+'&ref='+encodeURIComponent(ctx.ref)+'&path='+encodeURIComponent(path):canonicalRepoURL(ctx,'blob',ctx.ref,path)); };
            readme.hidden=false; readme.innerHTML='<div class="readme-head">README</div>'+SwitchyardMarkdown.render(md,{resolve:resolver});
          }}
        }
      }
      if(ctx.path){
        const edit=document.getElementById('edit-link'); if(edit)edit.href='/edit.html?name='+encodeURIComponent(ctx.artifact)+'&ref='+encodeURIComponent(ctx.ref)+'&path='+encodeURIComponent(ctx.path);
        document.getElementById('file-view-title').textContent=ctx.path; const raw=await fetch(rawURL(ctx,ctx.path),{credentials:'same-origin'}); if(!raw.ok)throw new Error('content request failed: '+raw.status); const ct=raw.headers.get('content-type')||''; const blob=await raw.blob(); const size=blob.size; document.getElementById('file-meta').textContent=size<1024?size+' B':(size/1024).toFixed(1)+' KB';
        document.getElementById('raw-link').href=rawURL(ctx,ctx.path); document.getElementById('copy-path').onclick=()=>navigator.clipboard&&navigator.clipboard.writeText(ctx.path);
        const lang=window.SwitchyardCode?SwitchyardCode.detectLanguage(ctx.path):'text'; document.getElementById('file-language').textContent=window.SwitchyardCode?SwitchyardCode.labelFor(lang):lang;
        const image=/^image\//.test(ct)||/\.(png|jpe?g|gif|webp|svg)$/i.test(ctx.path);
        const text=!image?await blob.text():'';
        if(image){viewEl.innerHTML='<div class="image-preview"><img alt="'+esc(ctx.path)+'" src="'+rawURL(ctx,ctx.path)+'"></div>';}
        else {viewEl.innerHTML=SwitchyardCode.lines(text,lang); document.getElementById('copy-file').onclick=()=>navigator.clipboard&&navigator.clipboard.writeText(text);}
        renderBreadcrumbs(ctx);
      }
    }catch(e){treeEl.innerHTML="<p class='error'>"+esc(e.message)+"</p>";}
  }

  async function startLive() {
    const liveEl = document.getElementById("live");
    if (!liveEl) return;
    const user = await refreshAuth();
    if (!user) { liveEl.innerHTML = '<p class="muted">Sign in to stream live events.</p>'; return; }
    liveEl.innerHTML = '<p class="muted">Connected — watching live events…</p>';
    const es = new EventSource("/api/events/stream");
    es.addEventListener("ready", () => { liveEl.innerHTML = '<p class="muted">Connected — watching live events…</p>'; });
    es.addEventListener("event", (e) => {
      try {
        const ev = JSON.parse(e.data);
        const row = el("<div class='list-item'><strong>" + esc(ev.type) + "</strong><span class='muted'>" + esc(ev.repo || "") + " · " + new Date().toLocaleTimeString() + "</span></div>");
        liveEl.prepend(row);
        while (liveEl.children.length > 20) liveEl.removeChild(liveEl.lastChild);
      } catch (err) { /* ignore */ }
    });
    es.onerror = () => { /* EventSource auto-reconnects */ };
  }

  window.refreshAuth = refreshAuth;

  document.addEventListener("DOMContentLoaded", async () => {
    startLive();
    if (document.getElementById("repos") || document.getElementById("work")) await renderDashboard();
    if (document.getElementById("signin-form")) renderSignin();
    if (document.getElementById("work-form")) renderWork();
    if (document.getElementById("repo-name")) renderRepo();
    if (!document.getElementById("repos") && !document.getElementById("work")) await refreshAuth();
  });
})();