/* Switchyard browser editor (CP10): CM6, draft Save != Commit, diff surface,
   findings, hideable Agent panel, commit via the shared substrate. */
(function () {
  "use strict";
  const params = new URLSearchParams(location.search);
  const name = params.get("name");
  const ref = params.get("ref") || "main";
  const path = params.get("path");
  const attempt = params.get("attempt") || "";
  if (!name || !path) {
    document.getElementById("editor-title").textContent = "Missing ?name=&path=";
    return;
  }
  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  async function api(p, opts) {
    const r = await fetch(p, Object.assign({ credentials: "same-origin", headers: { "Content-Type": "application/json" } }, opts));
    const body = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(body.error || r.statusText);
    return body;
  }

  let committedText = "";
  let draftSaved = false;
  let view = null;

  $("editor-title").textContent = name + " · " + ref + " · " + path;

  async function loadCommitted() {
    const raw = await fetch("/api/repos/" + encodeURIComponent(name) + "/content?ref=" + encodeURIComponent(ref) + "&path=" + encodeURIComponent(path), { credentials: "same-origin" });
    committedText = raw.ok ? await raw.text() : "";
    return committedText;
  }

  function makeEditor(initial) {
    view = new CM6.EditorView({
      state: CM6.EditorState.create({
        doc: initial,
        extensions: [
          CM6.basicSetup,
          CM6.keymap.of([...CM6.defaultKeymap, CM6.indentWithTab]),
          CM6.markdown(),
          CM6.oneDark
        ]
      }),
      parent: $("editor")
    });
  }

  async function loadDraft() {
    try {
      const d = await api("/api/drafts/" + encodeURIComponent(name) + "/" + encodeURIComponent(ref) + "/" + encodeURIComponent(path));
      return d.content;
    } catch (e) {
      return null;
    }
  }

  async function refreshDraftState() {
    const d = await loadDraft();
    draftSaved = d !== null && d !== "";
    $("draft-state").textContent = draftSaved ? "draft saved (not committed)" : "";
  }

  async function showDiff() {
    const newText = view.state.doc.toString();
    if (newText === committedText) { $("diff-view").textContent = "(no changes)"; return; }
    try {
      const d = await api("/api/diff", { method: "POST", body: JSON.stringify({ path, old: committedText, new: newText }) });
      $("diff-view").textContent = d.diff || "(no diff)";
      $("diff-state").textContent = "draft vs committed";
    } catch (e) {
      $("diff-view").textContent = "diff failed: " + e.message;
    }
  }

  async function loadFindings() {
    if (!attempt) return;
    try {
      const f = await api("/api/findings");
      const mine = f.items.filter((x) => x.target === attempt && x.file === path);
      $("findings").innerHTML = mine.length ? mine.map((x) =>
        "<div class='finding " + esc(x.severity) + "'><span class='sev'>" + esc(x.severity) + "</span> " + esc(x.message) + " <span class='muted'>(" + esc(x.status) + ")</span></div>"
      ).join("") : "<p class='muted'>No findings for this file.</p>";
    } catch (e) { $("findings").textContent = "findings unavailable"; }
  }

  $("btn-save").addEventListener("click", async () => {
    try {
      const r = await api("/api/drafts", { method: "POST", body: JSON.stringify({ repo: name, branch: ref, path, content: view.state.doc.toString() }) });
      draftSaved = true;
      $("draft-state").textContent = "draft saved (not committed)";
      showDiff();
    } catch (e) { alert("save failed: " + e.message); }
  });

  $("btn-commit").addEventListener("click", async () => {
    const message = prompt("Commit message:", "edit " + path + " via Switchyard editor");
    if (message === null) return;
    try {
      const saved = await api("/api/drafts", { method: "POST", body: JSON.stringify({ repo: name, branch: ref, path, content: view.state.doc.toString() }) });
      const r = await api("/api/drafts/" + saved.id + "/commit", { method: "POST", body: JSON.stringify({ message }) });
      committedText = view.state.doc.toString();
      draftSaved = false;
      $("draft-state").textContent = "committed " + String(r.new_sha).slice(0, 7);
      showDiff();
    } catch (e) { alert("commit failed: " + e.message); }
  });

  $("btn-agent").addEventListener("click", async () => {
    const panel = $("agent-panel");
    panel.textContent = "running implementer on attempt " + attempt + " …";
    try {
      const r = await api("/api/attempts/" + attempt + "/run");
      panel.innerHTML = "<div>execution " + esc(r.execution) + " → " + esc(r.status) + "</div><pre>" + esc(r.output || "") + "</pre>";
      committedText = await loadCommitted();
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: committedText } });
      showDiff();
    } catch (e) { panel.textContent = "agent run failed: " + e.message; }
  });

  (async function init() {
    await refreshAuth();
    const committed = await loadCommitted();
    const draft = await loadDraft();
    makeEditor(draft !== null && draft !== "" ? draft : committed);
    await refreshDraftState();
    loadFindings();
    $("editor-title").textContent = name + " · " + ref + " · " + path;
  })();
})();