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
      state.textContent = user ? "Signed in as " + user : "";
      if (!user) state.innerHTML = '<a href="/signin.html">Sign in</a>';
    }
    if (mAuth) mAuth.innerHTML = user
      ? "<a href='#' id='logout-mobile'>Sign out (" + esc(user) + ")</a>"
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
        await api("/api/work", { method: "POST", body: JSON.stringify({ title: form.title.value, kind: form.kind.value }) });
        form.title.value = "";
        await loadWork(list);
      } catch (ex) { list.innerHTML = "<p class='error'>" + esc(ex.message) + "</p>"; }
    });
    loadWork(list);
  }
  async function loadWork(list) {
    try {
      const work = await api("/api/work");
      list.innerHTML = work.items.length
        ? work.items.map((w) =>
            "<div class='list-item'><strong>" + esc(w.title) + "</strong>" +
            "<span class='muted'>" + esc(w.id) + " · " + esc(w.kind) + " · " + esc(w.status) + "</span></div>").join("")
        : "<p class='muted'>No work yet.</p>";
    } catch (e) { list.innerHTML = "<p class='error'>" + esc(e.message) + "</p>"; }
  }

  /* repo browser */
  function canonicalRepoContext() {
    const params = new URLSearchParams(location.search);
    const legacy = params.get("name");
    if (legacy) return { legacy: true, artifact: legacy, owner: "", repo: "", ref: params.get("ref") || "main", path: params.get("path") || "" };
    const parts = location.pathname.split("/").filter(Boolean).map(decodeURIComponent);
    if (parts.length < 2) return { legacy: true, artifact: "", owner: "", repo: "", ref: "main", path: "" };
    const out = { legacy: false, owner: parts[0], repo: parts[1], artifact: "", ref: "main", path: "" };
    if (parts.length >= 4 && (parts[2] === "blob" || parts[2] === "tree")) {
      out.kind = parts[2]; out.ref = parts[3]; out.path = parts.slice(4).join("/");
    }
    return out;
  }
  function canonicalRepoURL(ctx, kind, ref, path) {
    const base = "/" + encodeURIComponent(ctx.owner) + "/" + encodeURIComponent(ctx.repo);
    if (!kind) return base;
    const tail = path ? "/" + path.split("/").map(encodeURIComponent).join("/") : "";
    return base + "/" + kind + "/" + encodeURIComponent(ref || "main") + tail;
  }
  async function renderRepo() {
    const nameEl = document.getElementById("repo-name");
    const treeEl = document.getElementById("file-tree");
    const viewEl = document.getElementById("file-view");
    if (!nameEl) return;
    const ctx = canonicalRepoContext();
    if (!ctx.legacy) {
      const meta = await api("/api/repositories/" + encodeURIComponent(ctx.owner) + "/" + encodeURIComponent(ctx.repo));
      ctx.artifact = meta.artifact_name;
      nameEl.textContent = meta.full_name;
      const desc = document.getElementById("repo-description"); if (desc) desc.textContent = meta.description || "";
    } else {
      nameEl.textContent = ctx.artifact;
    }
    document.getElementById("repo-ref").textContent = "ref: " + ctx.ref;
    loadTree(ctx, treeEl, viewEl);
  }
  async function loadTree(ctx, treeEl, viewEl) {
    try {
      const root = ctx.legacy
        ? "/api/repos/" + encodeURIComponent(ctx.artifact)
        : "/api/repositories/" + encodeURIComponent(ctx.owner) + "/" + encodeURIComponent(ctx.repo);
      const t = await api(root + "/tree?ref=" + encodeURIComponent(ctx.ref));
      const files = t.tree.map((it) => it.path).sort();
      treeEl.innerHTML = "";
      for (const f of files) {
        const row = el("<div class='file' data-path='" + esc(f) + "'>" + esc(f) + "</div>");
        row.onclick = () => {
          location.href = ctx.legacy
            ? "/repo.html?name=" + encodeURIComponent(ctx.artifact) + "&ref=" + encodeURIComponent(ctx.ref) + "&path=" + encodeURIComponent(f)
            : canonicalRepoURL(ctx, "blob", ctx.ref, f);
        };
        treeEl.appendChild(row);
      }
      if (ctx.path) {
        const editLink = document.getElementById("edit-link");
        if (editLink) editLink.href = "/edit.html?name=" + encodeURIComponent(ctx.artifact) + "&ref=" + encodeURIComponent(ctx.ref) + "&path=" + encodeURIComponent(ctx.path);
        const titleEl = document.getElementById("file-view-title"); if (titleEl) titleEl.textContent = ctx.path;
        const raw = await fetch(root + "/content?ref=" + encodeURIComponent(ctx.ref) + "&path=" + encodeURIComponent(ctx.path), { credentials: "same-origin" });
        if (!raw.ok) throw new Error("content request failed: " + raw.status);
        viewEl.textContent = await raw.text();
      }
    } catch (e) { treeEl.innerHTML = "<p class='error'>" + esc(e.message) + "</p>"; }
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