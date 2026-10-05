/* Switchyard web application island. */
(function () {
  "use strict";

  const iconPaths = {
 menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
 close: '<path d="m6 6 12 12M6 18 18 6"/>',
    repository: '<path d="M4 3h12a2 2 0 0 1 2 2v14H6a2 2 0 0 1-2-2V3Z"/><path d="M4 15h14M8 7h6M8 10h4"/>',
    pullRequest: '<circle cx="6" cy="5" r="2"/><circle cx="6" cy="19" r="2"/><circle cx="18" cy="19" r="2"/><path d="M6 7v10M18 17V9a4 4 0 0 0-4-4h-3m3-3-3 3 3 3"/>',
    issue: '<circle cx="12" cy="12" r="9"/><path d="M12 7v6m0 3v1"/>',
    star: '<path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9L12 3Z"/>'
  };
  function icon(name) { return '<svg class="ui-icon" viewBox="0 0 24 24" aria-hidden="true">'+(iconPaths[name]||'')+'</svg>'; }
  window.SwitchyardIcons = { icon };
  function mountRepositoryIdentity(owner, repo, section, metadata = {}) {
    const main = document.getElementById('main-content');
    if (!main || !owner || !repo || main.querySelector('.repository-shell') || main.querySelector('.repository-page')) return;
    const base = '/' + encodeURIComponent(owner) + '/' + encodeURIComponent(repo);
    const shell = document.createElement('header'); shell.className = 'repository-shell';
    const identity = document.createElement('nav'); identity.className = 'repository-context'; identity.setAttribute('aria-label', 'Repository location');
    const ownerLink = document.createElement('a'); ownerLink.href = '/' + encodeURIComponent(owner);
    const image = document.createElement('img'); image.width = 24; image.height = 24; image.alt = '';
    ownerLink.append(image, document.createTextNode(owner));
    const repoLink = document.createElement('a'); repoLink.href = base; repoLink.textContent = repo;
    const badge = document.createElement('span'); badge.className = 'badge'; badge.hidden = true;
    identity.append(ownerLink, document.createTextNode('/'), repoLink, badge);
    if (section) { const current = document.createElement('span'); current.className = 'repository-section'; current.textContent = section; current.setAttribute('aria-current','page'); identity.append(document.createTextNode('/'),current); }
    const nav = document.createElement('nav'); nav.className = 'tabs repository-nav'; nav.setAttribute('aria-label','Repository');
    const tabs = [['Code',''],['Work','work'],['Proposals','proposals'],['Pull requests','pulls'],['Actions','actions'],['Commits','commits/main'],['Releases','releases'],['Pages','pages'],['Settings','settings']];
    const active = location.pathname.split('/')[3];
    for (const [label,path] of tabs) { const link = document.createElement('a'); link.textContent = label; link.href = base + (path ? '/' + path : ''); if (path.split('/')[0] === active || (active === 'pull' && path === 'pulls')) { link.className = 'active'; link.setAttribute('aria-current','page'); } nav.append(link); }
    shell.append(identity,nav); main.prepend(shell);
    function update(meta) { image.src = '/api/avatars/' + (meta.owner_type === 'org' ? 'org' : 'user') + '/' + encodeURIComponent(meta.owner_id || owner); if(meta.visibility){badge.textContent=meta.visibility;badge.hidden=false;} nav.querySelectorAll('a')[5].href=base+'/commits/'+encodeURIComponent(meta.default_branch||'main'); }
    update(metadata);
    if (!metadata.visibility) api('/api/repositories/' + encodeURIComponent(owner) + '/' + encodeURIComponent(repo)).then(update).catch(() => {});
  }
  window.SwitchyardIdentity = {mount: mountRepositoryIdentity};
 document.querySelectorAll(".hamburger").forEach(button=>button.innerHTML=icon("menu"));
  async function api(path, opts) {
    const res = await fetch(path, {
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      ...opts,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok && !(path === "/api/auth/me" && res.status === 401)) {
      throw new Error(data.message || data.error || ("HTTP " + res.status));
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
      links.innerHTML = '<a href="/">Explore</a><a href="/repositories">Public repositories</a><a href="/organizations">Organizations</a>';
      header.querySelector(".header-right").before(links);
    }
    const main = document.querySelector("main");
    const contextLinks = [...document.querySelectorAll(".header-right > a")].filter(link => !link.hidden);
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
        <details class="menu-group" open><summary>Explore</summary><div class="menu-links"><a href="/repositories">Public repositories</a><a href="/organizations">Organizations</a></div></details>
        <details class="menu-group" data-workspace-menu hidden open><summary>Your workspace</summary><div class="menu-links"><a href="/">Home</a><a href="/repositories.html">Repositories</a><a href="/work.html">Work</a><a href="/pulls.html">Pull Requests</a></div></details>
        <details class="menu-group" data-workspace-menu hidden open><summary>Coordination</summary><div class="menu-links"><a href="/operations#attention">Needs Attention</a><a href="/operations#queue">Integration Queue</a><a href="/operations#workflows">Workflows</a><a href="/operations#agents">Agents</a></div></details>
        <details class="menu-group" open><summary>Account</summary><div class="menu-links"><div class="mobile-auth" id="mobile-auth"></div><a href="/organizations">Organizations</a><a href="/settings" data-workspace-menu hidden>Settings</a></div></details>
      </div>
      <div class="menu-footer"><span>Switchyard · Code, work and review in one place.</span><div id="menu-session-actions" class="menu-session-actions"></div><p id="menu-session-error" class="error" role="alert" hidden></p></div>`;
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
    document.querySelectorAll("[data-workspace-menu]").forEach(el => el.hidden = !user);
    const primary = document.querySelector(".site-nav");
    if(primary) primary.innerHTML = user ? '<a href="/">Dashboard</a><a href="/repositories">Repositories</a><a href="/work">Work</a><a href="/pulls">Pull requests</a>' : '<a href="/">Explore</a><a href="/repositories">Public repositories</a><a href="/organizations">Organizations</a>';
    document.querySelectorAll("[data-auth-content]").forEach(el=>el.hidden=!user && !/^\/[^/]+\/[^/]+\/pulls/.test(location.pathname));
    document.querySelectorAll("[data-auth-prompt]").forEach(el=>el.hidden=!!user || /^\/[^/]+\/[^/]+\/pulls/.test(location.pathname));
    window.dispatchEvent(new CustomEvent("switchyard-auth", {detail:{user}}));
    const state = document.getElementById("auth-state");
    const mAuth = document.getElementById("mobile-auth");
    if (state) {
      state.innerHTML = user ? '<a class="account-chip" href="/'+encodeURIComponent(user)+'" aria-label="View profile for '+esc(user)+'" title="'+esc(user)+'"><img src="/api/avatars/user/'+encodeURIComponent(user)+'" alt=""></a>' : '<a class="btn header-signin" href="/signin">Sign in</a><a class="btn header-signup" href="/signup">Sign up</a>';
    }
    if (mAuth) mAuth.innerHTML = user ? '<a href="/'+encodeURIComponent(user)+'">Profile</a>' : '';
    const session = document.getElementById("menu-session-actions");
    if (session) session.innerHTML = user
      ? '<span>Signed in as <strong>'+esc(user)+'</strong></span><button class="btn" id="logout-mobile" type="button">Sign out</button>'
      : '<a class="btn" href="/signin">Sign in</a><a class="btn" href="/signup">Sign up</a>';
    const lo = document.getElementById("logout-mobile");
    if (lo) lo.onclick = async () => {
      const error = document.getElementById("menu-session-error");
      lo.disabled = true; error.hidden = true;
      try { await api("/api/auth/logout", { method: "POST" }); location.reload(); }
      catch { error.textContent = "Sign out could not be completed. Please try again."; error.hidden = false; lo.disabled = false; }
    };
    return user;
  }

  /* Workspace overview: ordinary repositories, work and pull requests first. */
  async function renderDashboard() {
    const reposEl=document.getElementById("repos"),workEl=document.getElementById("work"),pulls=document.getElementById("home-pulls");
    if(!reposEl||!workEl)return;
    const user=await refreshAuth();
    if(!user) return;
    document.getElementById('public-home').hidden=true;
    document.getElementById('signed-in-home').hidden=false;
    document.getElementById('home-username').textContent=user;
    document.getElementById('home-account-description').textContent='Your personal workspace';
    document.getElementById('home-account').href='/'+encodeURIComponent(user);
    document.getElementById('home-avatar').src='/api/avatars/user/'+encodeURIComponent(user);
    document.getElementById('home-greeting').textContent='Welcome back, '+user;
    let repositories=[];
    try{repositories=(await api('/api/repositories')).items||[];reposEl.innerHTML=repositories.slice(0,12).map(r=>'<a href="/'+encodeURIComponent(r.owner_slug)+'/'+encodeURIComponent(r.slug)+'"><span class="home-repo-icon" aria-hidden="true">'+icon('repository')+'</span><span>'+esc(r.full_name)+'</span></a>').join('')||'<p class="muted">No repositories yet.</p>';}catch{reposEl.innerHTML='<p class="error">Repositories could not be loaded.</p>';}
    await Promise.all([
      (async()=>{try{const items=((await api('/api/work')).items||[]).sort((a,b)=>String(b.updated_at||'').localeCompare(String(a.updated_at||''))).slice(0,6);workEl.innerHTML=items.map(w=>'<a class="home-feed-row" href="/work/'+encodeURIComponent(w.id)+'"><span class="home-work-icon '+esc(w.status)+'" aria-hidden="true">'+icon('issue')+'</span><div><strong>'+esc(w.title)+'</strong><small>'+esc(w.kind)+' · '+esc(w.owner)+' · '+esc(w.status)+'</small></div></a>').join('')||'<div class="empty-state"><strong>No work yet</strong><span>Start with a problem, idea or task.</span><a href="/work">Create work</a></div>';}catch{workEl.innerHTML='<div class="error-state">Work could not be loaded.</div>';}})(),
      (async()=>{try{const items=((await api('/api/prs')).items||[]).filter(p=>p.status==='open').slice(0,5);pulls.innerHTML=items.map(p=>{const repo=repositories.find(r=>r.artifact_name===p.repo||r.full_name===p.repo);const href=repo?'/'+encodeURIComponent(repo.owner_slug)+'/'+encodeURIComponent(repo.slug)+'/pull/'+encodeURIComponent(p.id):'/pulls';return '<a class="home-feed-row" href="'+href+'"><span class="home-pr-icon" aria-hidden="true">'+icon('pullRequest')+'</span><div><strong>'+esc(p.title)+'</strong><small>'+esc(repo?.full_name||p.repo)+' · '+esc(p.branch)+' → '+esc(p.base)+'</small></div></a>';}).join('')||'<div class="empty-state"><strong>No open pull requests</strong><span>Changes ready for review will appear here.</span></div>';}catch{pulls.innerHTML='<div class="error-state">Pull requests could not be loaded.</div>';}})()
    ]);
  }

  /* sign in / register */
  function renderSignin() {
    const form = document.getElementById("signin-form");
    if (!form) return;
    const err = document.getElementById("auth-error");
    const toggle = document.getElementById("toggle-mode");
    let mode = /\/(signup|register)\/?$/.test(location.pathname) || location.hash === "#register" ? "register" : "login";
    let registrationAllowed = false;
    const submit = form.querySelector("button[type=submit]");
    const password = form.elements.namedItem("password");
    const email = form.elements.namedItem("email"), confirm = form.elements.namedItem("confirm_password");
    function drawMode() {
      const registering = mode === "register";
      document.getElementById("forgot-password-link").hidden = registering;
      document.getElementById("registration-email").hidden = !registering;
      document.getElementById("registration-confirm").hidden = !registering;
      email.disabled = confirm.disabled = !registering;
      email.required = confirm.required = registering;
      password.autocomplete = registering ? "new-password" : "current-password";
      password.minLength = registering ? 8 : 1;
      document.getElementById("auth-title").textContent = registering ? "Create your Switchyard account" : "Sign in to Switchyard";
      document.querySelector(".auth-copy").textContent = registering ? "Choose a username and enter your email and password. Your email stays private." : "Access repositories, Work, Pull Requests, organizations and the editor.";
      submit.textContent = registering ? "Create account" : "Sign in";
      submit.disabled = registering && !registrationAllowed;
      toggle.textContent = registering ? "Back to sign in" : "No account? Register";
      toggle.hidden = !registering && !registrationAllowed;
    }
    toggle.addEventListener("click", () => {
      mode = mode === "login" ? "register" : "login";
      history.replaceState(null,"",mode === "register" ? "/signup" : "/signin");
      err.hidden = true; drawMode();
    });
    drawMode();
    api("/api/auth/registration").then(policy => {
      registrationAllowed = policy.mode === "open";drawMode();
      if (mode === "register" && !registrationAllowed) { err.textContent = "New account registration is currently disabled.";err.hidden = false; }
    }).catch(() => { err.textContent = "Registration availability could not be checked. Please try again.";err.hidden = false; });
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      err.hidden = true;
      const body = { username: form.username.value, password: form.password.value };
      if (mode === "register") {
        if (!registrationAllowed) return;
        if (password.value !== confirm.value) { err.textContent = "Passwords do not match.";err.hidden = false;confirm.focus();return; }
        body.email = email.value;body.confirm_password = confirm.value;
      }
      submit.disabled = true; toggle.disabled = true;
      try {
        const result = await api(mode === "login" ? "/api/auth/login" : "/api/auth/register", { method: "POST", body: JSON.stringify(body) });
        location.href = mode === "register" ? "/settings.html?verify=email&delivery=" + encodeURIComponent(result.email_verification || "unavailable") + "#security" : "/";
      } catch (ex) {
        const messages = { invalid_credentials: "Username or password is incorrect.", username_taken: "This username is already in use.", email_taken: "This email is already in use.", email_invalid: "Enter a valid email address.", password_confirmation_mismatch: "Passwords do not match.", username_invalid: "Use 3–39 letters, numbers or hyphens; this name may be reserved.", username_or_password_too_short: "Use a username of at least 3 characters and a password of at least 8 characters.", password_too_long: "Use a password of at most 72 bytes.", registration_disabled: "New account registration is currently disabled.", session_persistence_failed: "Your account was created, but sign-in could not complete. Try signing in.", registration_unavailable: "Registration is temporarily unavailable. Please try again.", registration_rate_limited: "Too many registration attempts. Please try again later.", login_rate_limited: "Too many sign-in attempts. Please try again later.", rate_limited: "Too many attempts. Please try again later." };
        err.textContent = messages[ex.message] || "Your account could not be accessed. Check the details and try again.";
        err.hidden = false;
      } finally { submit.disabled = mode === "register" && !registrationAllowed; toggle.disabled = false; }
    });
  }

  /* Work list and creation dialog. */
  function renderWork() {
    const form=document.getElementById('work-form'),list=document.getElementById('work-list'),dialog=document.getElementById('work-dialog'),query=new URLSearchParams(location.search);const workRoute=location.pathname.split('/').filter(Boolean).map(decodeURIComponent);if(workRoute.length===3&&workRoute[2]==='work')query.set('repo',workRoute[0]+'/'+workRoute[1]);
    if(!form||!list)return;
    let filter='open',items=[];
    function draw(){const search=document.getElementById('work-search').value.toLowerCase(),repo=query.get('repo');const xs=items.filter(w=>(filter==='all'||w.status===filter)&&(!repo||w.repo===repo)&&(!search||(w.title+' '+w.kind+' '+w.owner).toLowerCase().includes(search)));document.getElementById('work-count').textContent=xs.length+' item'+(xs.length===1?'':'s');list.innerHTML=xs.map(w=>'<a class="work-row" href="/work/'+encodeURIComponent(w.id)+'"><span class="home-work-icon '+esc(w.status)+'" aria-hidden="true">'+icon('issue')+'</span><div><strong>'+esc(w.title)+'</strong><span>'+esc(w.kind)+' · opened by '+esc(w.owner)+(w.repo?' · '+esc(w.repo):'')+'</span></div><span class="badge">'+esc(w.status)+'</span></a>').join('')||'<div class="empty-state"><strong>No '+esc(filter==='all'?'matching':filter)+' work</strong><span>Try another filter or create a new item.</span></div>';}
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
    const route = location.pathname.split('/').filter(Boolean).map(decodeURIComponent);
    const sections = {tree:'Code',blob:'Source',commits:'Commits',commit:'Commit',pull:'Pull request',pulls:'Pull requests',actions:'Actions',proposals:'Proposals',pages:'Pages',releases:'Releases',settings:'Settings',work:'Work',branches:'Branches',tags:'Tags'};
    const reservedOwners = new Set(['api','assets','organizations','work','settings','operations','repositories','pulls','signin']);
    if (route.length >= 3 && !reservedOwners.has(route[0]) && sections[route[2]]) mountRepositoryIdentity(route[0], route[1], sections[route[2]]);
    renderDemoBanner();
    startLive();
    if (document.getElementById("repos") || document.getElementById("work")) await renderDashboard();
    if (document.getElementById("signin-form")) renderSignin();
    if (document.getElementById("work-form")) renderWork();
    if (!document.getElementById("repos") && !document.getElementById("work")) await refreshAuth();
  });
})();
