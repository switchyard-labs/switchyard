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

  // One navigation renderer serves generated and hand-written product pages.
  function ensureGlobalMenu() {
    const btn = document.getElementById("hamburger");
    if (!btn) return;
    btn.type = "button";
    btn.innerHTML = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16M4 12h16M4 18h16"/></svg>';
    const header = document.querySelector(".header-inner");
    if (header && !header.querySelector(".site-nav")) {
      const links = document.createElement("nav");
      links.className = "site-nav";
      links.setAttribute("aria-label", "Primary");
      links.innerHTML = '<a href="/">Dashboard</a><a href="/repositories">Repositories</a><a href="/work">Work</a><a href="/pulls">Pull requests</a>';
      header.querySelector(".header-right").before(links);
    }
    const main = document.querySelector("main");
    const contextLinks = [...document.querySelectorAll(".header-right > a")];
    if (main && contextLinks.length) {
      const context = document.createElement("div"); context.className = "page-context";
      contextLinks.forEach(link => context.append(link)); main.prepend(context);
    }
    document.getElementById("mobile-menu")?.remove();
    const nav = document.createElement("nav");
    nav.className = "mobile-menu";
    nav.id = "mobile-menu";
    nav.setAttribute("aria-label", "Switchyard");
    nav.hidden = true;
    nav.innerHTML = `
      <div class="mobile-menu-head">
        <a class="brand" href="/"><img class="brand-mark-img" src="/assets/images/favicon.webp" alt="" width="26" height="26"><span class="brand-name">Switchyard</span></a>
        <button class="hamburger" id="mobile-close" type="button" aria-label="Close navigation"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg></button>
      </div>
      <div class="menu-groups">
        <details class="menu-group" open><summary>Your workspace</summary><div class="menu-links"><a href="/">Home</a><a href="/repositories.html">Repositories</a><a href="/work.html">Work</a><a href="/pulls.html">Pull Requests</a></div></details>
        <details class="menu-group" open><summary>Coordination</summary><div class="menu-links"><a href="/operations#attention">Needs Attention</a><a href="/operations#queue">Integration Queue</a><a href="/operations#workflows">Workflows</a><a href="/operations#agents">Agents</a></div></details>
        <details class="menu-group" open><summary>Account</summary><div class="menu-links"><div class="mobile-auth" id="mobile-auth"></div><a href="/organizations">Organizations</a><a href="/settings">Settings</a></div></details>
      </div>
      <div class="menu-footer">Switchyard · Code, work and review in one place.</div>`;
    document.body.insertBefore(nav, document.querySelector("main") || document.body.firstChild);
    btn.setAttribute("aria-controls", "mobile-menu");
    btn.setAttribute("aria-expanded", "false");
  }
  ensureGlobalMenu();

  /* mobile menu: full-screen, Escape dismisses, focus trapped */
  const hamburger = document.getElementById("hamburger");
  const mobileMenu = document.getElementById("mobile-menu");
  const mobileClose = document.getElementById("mobile-close");
  const menuInertState = new Map();
  function openMenu() {
    if (!mobileMenu) return;
    mobileMenu.hidden = false;
    for (const child of document.body.children) {
      if (child === mobileMenu || child.tagName === "SCRIPT") continue;
      menuInertState.set(child, child.inert);
      child.inert = true;
    }
    document.body.classList.add("menu-open");
    hamburger.setAttribute("aria-expanded", "true");
    mobileClose.focus();
    document.addEventListener("keydown", trap);
  }
  function closeMenu() {
    if (!mobileMenu) return;
    mobileMenu.hidden = true;
    for (const [child, wasInert] of menuInertState) child.inert = wasInert;
    menuInertState.clear();
    document.body.classList.remove("menu-open");
    hamburger.setAttribute("aria-expanded", "false");
    document.removeEventListener("keydown", trap);
    hamburger.focus();
  }
  function trap(e) {
    if (e.key === "Escape") { e.preventDefault(); closeMenu(); return; }
    if (e.key === "Tab") {
      const focusables = [...mobileMenu.querySelectorAll("a[href], button, summary")].filter((x) => !x.hasAttribute("disabled") && x.getClientRects().length);
      const first = focusables[0], last = focusables[focusables.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    }
  }
  if (hamburger) hamburger.addEventListener("click", openMenu);
  if (mobileClose) mobileClose.addEventListener("click", closeMenu);
  if (mobileMenu) mobileMenu.addEventListener("click", (e) => { if (e.target.closest("a[href]")) closeMenu(); });
  if (mobileMenu) {
    const here = location.pathname.replace(/\/index\.html$/, "/").replace(/\.html$/, "");
    document.querySelectorAll(".mobile-menu a[href], .site-nav a[href]").forEach((a) => {
      try { const p = new URL(a.href, location.href).pathname.replace(/\/index\.html$/, "/").replace(/\.html$/, ""); if (p === here) a.setAttribute("aria-current", "page"); } catch (_) {}
    });
  }

  /* auth state */
  let authRequest;
  async function refreshAuth() {
    let user = "";
    try { const m = await (authRequest ||= api("/api/auth/me")); if (m.authed) user = m.user; } catch (e) { /* ignore */ }
    const state = document.getElementById("auth-state");
    const mAuth = document.getElementById("mobile-auth");
    if (state) {
      state.innerHTML = user ? '<a class="account-chip" href="/'+encodeURIComponent(user)+'"><img src="/api/avatars/user/'+encodeURIComponent(user)+'" alt=""><span>'+esc(user)+'</span></a>' : '<a class="btn header-signin" href="/signin.html">Sign in</a>';
    }
    if (mAuth) mAuth.innerHTML = user
      ? '<a href="/'+encodeURIComponent(user)+'">Profile</a><a href="#" id="logout-mobile">Sign out ('+esc(user)+')</a>'
      : '<a href="/signin.html">Sign in</a>';
    const lo = document.getElementById("logout-mobile");
    if (lo) lo.onclick = async (e) => { e.preventDefault(); await api("/api/auth/logout", { method: "POST" }); location.reload(); };
    return user;
  }

  /* Workspace overview: ordinary repositories, work and pull requests first. */
  async function renderDashboard() {
    const reposEl=document.getElementById("repos"),workEl=document.getElementById("work"),pulls=document.getElementById("home-pulls");
    if(!reposEl||!workEl)return;
    const user=await refreshAuth();
    if(!user){reposEl.innerHTML='<p class="muted">Your repositories appear after sign in.</p>';workEl.innerHTML='<div class="empty-state"><strong>Keep your work together</strong><span>Track issues and review changes in your workspace.</span><a class="btn primary" href="/signin">Sign in</a></div>';pulls.innerHTML='<div class="empty-state"><strong>Review changes in one place</strong><span>Sign in to see pull requests that you can access.</span></div>';return;}
    document.getElementById('home-username').textContent=user;
    document.getElementById('home-account-description').textContent='Your personal workspace';
    document.getElementById('home-account').href='/'+encodeURIComponent(user);
    document.getElementById('home-avatar').src='/api/avatars/user/'+encodeURIComponent(user);
    document.getElementById('home-greeting').textContent='Welcome back, '+user;
    let repositories=[];
    try{repositories=(await api('/api/repositories')).items||[];reposEl.innerHTML=repositories.slice(0,12).map(r=>'<a href="/'+encodeURIComponent(r.owner_slug)+'/'+encodeURIComponent(r.slug)+'"><span class="home-repo-icon" aria-hidden="true">▣</span><span>'+esc(r.full_name)+'</span></a>').join('')||'<p class="muted">No repositories yet.</p>';}catch{reposEl.innerHTML='<p class="error">Repositories could not be loaded.</p>';}
    await Promise.all([
      (async()=>{try{const items=((await api('/api/work')).items||[]).sort((a,b)=>String(b.updated_at||'').localeCompare(String(a.updated_at||''))).slice(0,6);workEl.innerHTML=items.map(w=>'<a class="home-feed-row" href="/work/'+encodeURIComponent(w.id)+'"><span class="home-work-icon '+esc(w.status)+'" aria-hidden="true">○</span><div><strong>'+esc(w.title)+'</strong><small>'+esc(w.kind)+' · '+esc(w.owner)+' · '+esc(w.status)+'</small></div></a>').join('')||'<div class="empty-state"><strong>No work yet</strong><span>Start with a problem, idea or task.</span><a href="/work">Create work</a></div>';}catch{workEl.innerHTML='<div class="error-state">Work could not be loaded.</div>';}})(),
      (async()=>{try{const items=((await api('/api/prs')).items||[]).filter(p=>p.status==='open').slice(0,5);pulls.innerHTML=items.map(p=>{const repo=repositories.find(r=>r.artifact_name===p.repo||r.full_name===p.repo);const href=repo?'/'+encodeURIComponent(repo.owner_slug)+'/'+encodeURIComponent(repo.slug)+'/pull/'+encodeURIComponent(p.id):'/pulls';return '<a class="home-feed-row" href="'+href+'"><span class="home-pr-icon" aria-hidden="true">⑂</span><div><strong>'+esc(p.title)+'</strong><small>'+esc(repo?.full_name||p.repo)+' · '+esc(p.branch)+' → '+esc(p.base)+'</small></div></a>';}).join('')||'<div class="empty-state"><strong>No open pull requests</strong><span>Changes ready for review will appear here.</span></div>';}catch{pulls.innerHTML='<div class="error-state">Pull requests could not be loaded.</div>';}})()
    ]);
  }

  /* sign in / register */
  function renderSignin() {
    const form = document.getElementById("signin-form");
    if (!form) return;
    const err = document.getElementById("auth-error");
    const toggle = document.getElementById("toggle-mode");
    let mode = "login";
    const submit = form.querySelector("button[type=submit]");
    const password = form.elements.namedItem("password");
    const displayName = document.getElementById("registration-name");
    toggle.addEventListener("click", () => {
      mode = mode === "login" ? "register" : "login";
      err.hidden = true;
      displayName.hidden = mode !== "register";
      password.autocomplete = mode === "register" ? "new-password" : "current-password";
      password.minLength = mode === "register" ? 8 : 1;
      const title = document.getElementById("auth-title");
      const copy = document.querySelector(".auth-copy");
      submit.textContent = mode === "login" ? "Sign in" : "Create account";
      toggle.textContent = mode === "login" ? "Create an account" : "Back to sign in";
      if (title) title.textContent = mode === "login" ? "Sign in to Switchyard" : "Create your Switchyard account";
      if (copy) copy.textContent = mode === "login" ? "Access repositories, Work, Pull Requests, organizations and the editor." : "Create an account with a username and password. You can add your profile and avatar afterwards.";
    });
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      err.hidden = true;
      const body = { username: form.username.value, password: form.password.value };
      if (mode === "register") body.display_name = form.elements.namedItem("display_name").value || form.username.value;
      submit.disabled = true; toggle.disabled = true;
      try {
        await api(mode === "login" ? "/api/auth/login" : "/api/auth/register", { method: "POST", body: JSON.stringify(body) });
        location.href = "/";
      } catch (ex) {
        const messages = { invalid_credentials: "Username or password is incorrect.", username_taken: "This username is already in use.", rate_limited: "Too many attempts. Please try again later." };
        err.textContent = messages[ex.message] || "Your account could not be accessed. Check the details and try again.";
        err.hidden = false;
      } finally { submit.disabled = false; toggle.disabled = false; }
    });
  }

  /* Work list and creation dialog. */
  function renderWork() {
    const form=document.getElementById('work-form'),list=document.getElementById('work-list'),dialog=document.getElementById('work-dialog'),query=new URLSearchParams(location.search);
    if(!form||!list)return;
    let filter='open',items=[];
    function draw(){const search=document.getElementById('work-search').value.toLowerCase(),repo=query.get('repo');const xs=items.filter(w=>(filter==='all'||w.status===filter)&&(!repo||w.repo===repo)&&(!search||(w.title+' '+w.kind+' '+w.owner).toLowerCase().includes(search)));document.getElementById('work-count').textContent=xs.length+' item'+(xs.length===1?'':'s');list.innerHTML=xs.map(w=>'<a class="work-row" href="/work/'+encodeURIComponent(w.id)+'"><span class="home-work-icon '+esc(w.status)+'" aria-hidden="true">○</span><div><strong>'+esc(w.title)+'</strong><span>'+esc(w.kind)+' · opened by '+esc(w.owner)+(w.repo?' · '+esc(w.repo):'')+'</span></div><span class="badge">'+esc(w.status)+'</span></a>').join('')||'<div class="empty-state"><strong>No '+esc(filter==='all'?'matching':filter)+' work</strong><span>Try another filter or create a new item.</span></div>';}
    async function load(){try{items=((await api('/api/work')).items||[]).sort((a,b)=>String(b.updated_at||'').localeCompare(String(a.updated_at||'')));draw();}catch{list.innerHTML='<div class="error-state">Sign in to see your work. <a href="/signin">Sign in</a></div>';}}
    document.getElementById('new-work').onclick=()=>{form.repo.value=query.get('repo')||'';document.getElementById('work-create-error').hidden=true;dialog.showModal();form.elements.namedItem('title').focus();};document.getElementById('work-dialog-close').onclick=()=>dialog.close();document.getElementById('work-search').oninput=draw;
    document.querySelectorAll('[data-work-filter]').forEach(b=>b.onclick=()=>{filter=b.dataset.workFilter;document.querySelectorAll('[data-work-filter]').forEach(x=>{x.classList.toggle('active',x===b);x.setAttribute('aria-pressed',String(x===b));});draw();});
    form.onsubmit=async e=>{e.preventDefault();const button=form.querySelector('[type=submit]'),error=document.getElementById('work-create-error');button.disabled=true;error.hidden=true;try{const created=await api('/api/work',{method:'POST',body:JSON.stringify({title:form.elements.namedItem('title').value,kind:form.kind.value,body:form.body.value,repo:form.repo.value,assignee:form.assignee.value})});dialog.close();location.href='/work/'+encodeURIComponent(created.id);}catch{error.hidden=false;error.textContent='Work could not be created. Check repository access and try again.';}finally{button.disabled=false;}};load();if(query.has('new'))document.getElementById('new-work').click();
  }

  async function startLive() {
    const liveEl = document.getElementById("live");
    if (!liveEl) return;
    const user = await refreshAuth();
    if (!user) { liveEl.innerHTML = '<p class="muted">Sign in to stream live events.</p>'; return; }
    liveEl.innerHTML = '<p class="muted">No recent repository events.</p>';
    const es = new EventSource("/api/events/stream");
    es.addEventListener("ready", () => { liveEl.innerHTML = '<p class="muted">No recent repository events.</p>'; });
    es.addEventListener("event", (e) => {
      try {
        const ev = JSON.parse(e.data);
        const row = el("<div class='list-item'><strong>" + esc(ev.type) + "</strong><span class='muted'>" + esc(ev.repo || "") + " · " + new Date().toLocaleTimeString() + "</span></div>");
        liveEl.prepend(row);
        while (liveEl.children.length > 20) liveEl.removeChild(liveEl.lastChild);
      } catch (err) { /* ignore */ }
    });
    es.onerror = () => { /* EventSource auto-reconnects */ };
    window.addEventListener("pagehide",()=>es.close(),{once:true});
  }

  window.refreshAuth = refreshAuth;

  async function renderDemoBanner(){ try{const d=await api("/api/demo"); if(d.enabled){let b=document.getElementById("demo-banner");if(!b){b=document.createElement("div");b.id="demo-banner";b.className="demo-banner";document.body.prepend(b)}b.textContent=d.guest?"Public demo · read-only guest browsing":"Demo environment";b.hidden=false}}catch(_){}}

  document.addEventListener("DOMContentLoaded", async () => {
    renderDemoBanner();
    startLive();
    if (document.getElementById("repos") || document.getElementById("work")) await renderDashboard();
    if (document.getElementById("signin-form")) renderSignin();
    if (document.getElementById("work-form")) renderWork();
    if (!document.getElementById("repos") && !document.getElementById("work")) await refreshAuth();
  });
})();