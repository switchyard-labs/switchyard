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
    hamburger.setAttribute("aria-expanded", "true");
    mobileClose.focus();
    document.addEventListener("keydown", trap);
  }
  function closeMenu() {
    if (!mobileMenu) return;
    mobileMenu.hidden = true;
    hamburger.setAttribute("aria-expanded", "false");
    document.removeEventListener("keydown", trap);
    hamburger.focus();
  }
  function trap(e) {
    if (e.key === "Escape") closeMenu();
    if (e.key === "Tab") {
      const focusables = [...mobileMenu.querySelectorAll("a, button")];
      const first = focusables[0], last = focusables[focusables.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    }
  }
  if (hamburger) hamburger.addEventListener("click", openMenu);
  if (mobileClose) mobileClose.addEventListener("click", closeMenu);

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
  function renderRepo() {
    const nameEl = document.getElementById("repo-name");
    const treeEl = document.getElementById("file-tree");
    const viewEl = document.getElementById("file-view");
    if (!nameEl) return;
    const params = new URLSearchParams(location.search);
    const name = params.get("name") || "";
    const ref = params.get("ref") || "main";
    const path = params.get("path") || "";
    nameEl.textContent = name;
    document.getElementById("repo-ref").textContent = "ref: " + ref;
    loadTree(name, ref, treeEl, viewEl, path);
  }
  async function loadTree(name, ref, treeEl, viewEl, activePath) {
    try {
      const t = await api("/api/repos/" + encodeURIComponent(name) + "/tree?ref=" + encodeURIComponent(ref));
      const dirs = {}, files = [];
      for (const it of t.tree) {
        const top = it.path.split("/")[0];
        if (it.type === "tree") dirs[top] = 1; else files.push(it.path);
      }
      treeEl.innerHTML = "";
      for (const d of Object.keys(dirs)) treeEl.appendChild(el("<div class='dir'>" + esc(d) + "/</div>"));
      for (const f of files.sort()) {
        const row = el("<div class='file' data-path='" + esc(f) + "'>" + esc(f) + "</div>");
        row.onclick = () => { location.href = "/repo.html?name=" + encodeURIComponent(name) + "&ref=" + encodeURIComponent(ref) + "&path=" + encodeURIComponent(f); };
        treeEl.appendChild(row);
      }
      if (activePath) {
        const data = await api("/api/repos/" + encodeURIComponent(name) + "/content?ref=" + encodeURIComponent(ref) + "&path=" + encodeURIComponent(activePath), { headers: {} });
        // content endpoint returns raw bytes; fetch directly
        const raw = await fetch("/api/repos/" + encodeURIComponent(name) + "/content?ref=" + encodeURIComponent(ref) + "&path=" + encodeURIComponent(activePath), { credentials: "same-origin" });
        viewEl.textContent = await raw.text();
      }
    } catch (e) { treeEl.innerHTML = "<p class='error'>" + esc(e.message) + "</p>"; }
  }

  document.addEventListener("DOMContentLoaded", async () => {
    if (document.getElementById("repos") || document.getElementById("work")) await renderDashboard();
    if (document.getElementById("signin-form")) renderSignin();
    if (document.getElementById("work-form")) renderWork();
    if (document.getElementById("repo-name")) renderRepo();
    if (!document.getElementById("repos") && !document.getElementById("work")) await refreshAuth();
  });
})();