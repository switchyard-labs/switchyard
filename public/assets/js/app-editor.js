/* Switchyard workbench: multi-file navigation, draft CAS, shared language ids,
   diff/findings and distinct interactive-vs-formal Agent modes. */
(function () {
  "use strict";
  const q = new URLSearchParams(location.search),
    route = location.pathname.split('/').filter(Boolean).map(decodeURIComponent),
    canonical = route.length >= 4 && route[2] === 'edit',
    repositoryIdentity = canonical ? route[0] + '/' + route[1] : q.get("name"),
    ref = canonical ? route[3] : q.get("ref") || "main";
  let name = repositoryIdentity;
  let path = canonical ? route.slice(4).join('/') : q.get("path") || "";
  const attempt = q.get("attempt") || "";
  const $ = (id) => document.getElementById(id),
    esc = (s) =>
      String(s == null ? "" : s).replace(
        /[&<>"']/g,
        (c) =>
          ({
            "&": "&amp;",
            "<": "&lt;",
            ">": "&gt;",
            '"': "&quot;",
            "'": "&#39;",
          })[c],
      );
  async function api(p, o) {
    const r = await fetch(
      p,
      Object.assign(
        {
          credentials: "same-origin",
          headers: { "Content-Type": "application/json" },
        },
        o,
      ),
    );
    const b = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(b.error || r.statusText);
    return b;
  }
  let committedText = "",
    savedText = "",
    view = null,
    currentRevision = "",
    baseSHA = "",
    files = [],
    committedFiles = new Set(),
    tabs = [],
    dirty = false;
  const buffers = new Map();
  let navigationVersion = 0,
    repositorySHA = "",
    canonicalRoot = "",
    canonicalBase = "",
    canWrite = false;
  const storageKey = "switchyard-editor:" + name + ":" + ref;
  let expanded = new Set(),
    searchVersion = 0;
  try {
    const stored = JSON.parse(sessionStorage.getItem(storageKey) || "{}");
    tabs = (stored.tabs || [])
      .filter((p) => typeof p === "string")
      .slice(0, 20);
    expanded = new Set(stored.expanded || []);
    path = path || stored.active || "";
  } catch {}
  function remember() {
    try {
      sessionStorage.setItem(
        storageKey,
        JSON.stringify({ tabs, expanded: [...expanded], active: path }),
      );
    } catch {}
  }
  function validPath(p) {
    return (
      p &&
      !p.startsWith("/") &&
      !p.includes("\\") &&
      !p
        .split("/")
        .some(
          (x) => !x || x === "." || x === ".." || x.toLowerCase() === ".git",
        ) &&
      p.length < 1024
    );
  }
  function unsavedAffected(p) {
    stash();
    return [...buffers.entries()].some(
      ([f, b]) =>
        (f === p || f.startsWith(p + "/")) && (b.dirty || b.pendingDraft),
    );
  }
  function pane(mode) {
    document
      .querySelectorAll(".mobile-workbench-switcher button")
      .forEach((b) => b.classList.toggle("active", b.dataset.pane === mode));
    document
      .querySelectorAll(".workbench>[data-pane]")
      .forEach((n) =>
        n.classList.toggle(
          "mobile-pane-active",
          n.dataset.pane === (mode === "search" ? "files" : mode),
        ),
      );
    if (mode === "search") setMode("search");
  }
  function setMode(mode) {
    document
      .querySelectorAll(".workbench-modes button")
      .forEach((b) => b.classList.toggle("active", b.dataset.mode === mode));
    $("sidebar-tools")?.toggleAttribute("hidden", mode !== "files");
    $("workspace-tree").hidden = mode !== "files";
    $("workspace-search").hidden = mode !== "search";
    $("workspace-source").hidden = mode !== "source";
    if (mode === "search") $("repo-search").focus();
  }
  function dialog(title, fields, submitLabel, help) {
    return new Promise((resolve) => {
      const d = document.createElement("dialog"),
        f = document.createElement("form"),
        h = document.createElement("h2");
      f.className = "form-stack";
      h.textContent = title;
      f.append(h);
      if (help) {
        const p = document.createElement("p");
        p.className = "muted";
        p.textContent = help;
        f.append(p);
      }
      const inputs = {};
      for (const [name, label, value] of fields) {
        const l = document.createElement("label"),
          i = document.createElement("input");
        l.textContent = label;
        i.value = value || "";
        i.required = true;
        l.append(i);
        f.append(l);
        inputs[name] = i;
      }
      const actions = document.createElement("div");
      actions.className = "dialog-actions";
      const cancel = document.createElement("button"),
        submit = document.createElement("button");
      cancel.type = "button";
      cancel.textContent = "Cancel";
      submit.type = "submit";
      submit.textContent = submitLabel;
      submit.className = "btn primary";
      actions.append(cancel, submit);
      f.append(actions);
      d.append(f);
      document.body.append(d);
      const finish = (value) => {
        d.close();
        d.remove();
        resolve(value);
      };
      cancel.onclick = () => finish(null);
      d.oncancel = (e) => {
        e.preventDefault();
        finish(null);
      };
      f.onsubmit = (e) => {
        e.preventDefault();
        finish(
          Object.fromEntries(
            Object.entries(inputs).map(([k, i]) => [k, i.value.trim()]),
          ),
        );
      };
      d.showModal();
      Object.values(inputs)[0]?.focus();
    });
  }
  function stash() {
    if (view && path)
      buffers.set(path, {
        state: view.state,
        committedText,
        savedText,
        currentRevision,
        baseSHA,
        dirty,
        pendingDraft: buffers.get(path)?.pendingDraft || false,
        scrollTop: view.scrollDOM.scrollTop,
      });
  }
  function restore(b) {
    committedText = b.committedText;
    savedText = b.savedText ?? b.committedText;
    currentRevision = b.currentRevision;
    baseSHA = b.baseSHA;
    dirty = b.dirty;
    makeEditor(b.state);
    view.scrollDOM.scrollTop = b.scrollTop || 0;
    renderTabs();
  }
  function status(msg, kind) {
    const e = $("status-message");
    e.textContent = msg || "";
    e.className = kind ? "status-" + kind : "";
  }
  function contentURL(p) {
    return (
      canonicalRoot +
      "/workspace/content?" +
      new URLSearchParams({ sha: repositorySHA, path: p })
    );
  }
  function editorURL(p) {
    return canonicalBase + '/edit/' + encodeURIComponent(ref) + (p ? '/' + p.split('/').map(encodeURIComponent).join('/') : '') + (attempt ? '?' + new URLSearchParams({attempt}) : '');
  }
  function detect() {
    const id = SwitchyardCode.detectLanguage(path);
    $("status-language").textContent = SwitchyardCode.labelFor(id);
    return id;
  }
  function renderTabs() {
    const element = $("editor-tabs");
    element.replaceChildren();
    for (const file of tabs) {
      const tab = document.createElement("div");
      tab.className = "editor-tab" + (file === path ? " active" : "");
      const open = document.createElement("button"),
        close = document.createElement("button");
      open.textContent =
        file.split("/").pop() +
        ((file === path ? dirty : buffers.get(file)?.dirty) ? " ●" : "");
      open.title = file;
      open.setAttribute("aria-pressed", String(file === path));
      close.textContent = "×";
      close.className = "tab-close";
      close.setAttribute("aria-label", "Close " + file);
      open.onclick = () => openFile(file);
      close.onclick = () => closeTab(file);
      tab.append(open, close);
      element.append(tab);
    }
    remember();
  }
  function closeTab(file) {
    stash();
    if (
      buffers.get(file)?.dirty &&
      !confirm("Discard unsaved changes in " + file + "?")
    )
      return;
    const index = tabs.indexOf(file);
    tabs = tabs.filter((x) => x !== file);
    buffers.delete(file);
    if (path === file) {
      view?.destroy();
      view = null;
      path = "";
      const next = tabs[index] || tabs[index - 1];
      if (next) openFile(next);
      else {
        $("editor").innerHTML =
          '<div class="editor-empty">Select a file from the explorer.</div>';
        $("editor-title").textContent = "No file selected";
      }
    }
    renderTabs();
  }
  function setPath(p) {
    path = p;
    q.set("path", p);
    history.replaceState(null, "", editorURL(path));
    if (!tabs.includes(p)) tabs.push(p);
    renderTabs();
    $("editor-title").textContent = p || "New file";
    $("editor-context").textContent =
      (canonicalBase.slice(1) || name) + " · " + ref;
    $("status-branch").textContent = ref;
    detect();
    renderTree(files);
    remember();
  }
  async function loadCommitted(p) {
    if (!committedFiles.has(p)) return "";
    const r = await fetch(contentURL(p), { credentials: "same-origin" });
    if (!r.ok && r.status !== 404)
      throw new Error("Unable to load committed file");
    return r.ok ? await r.text() : "";
  }
  async function loadRefSHA() {
    try {
      const rr = await api("/api/repos/" + encodeURIComponent(name) + "/refs");
      baseSHA = (rr.refs || {})["refs/heads/" + ref] || "";
      repositorySHA = baseSHA;
      if (!baseSHA) throw Error("Select a writable branch with a commit.");
    } catch (e) {
      throw e;
    }
  }
  async function loadDraft(p) {
    const r = await fetch(
      "/api/drafts/" +
        encodeURIComponent(name) +
        "/" +
        encodeURIComponent(ref) +
        "/" +
        encodeURIComponent(p),
      { credentials: "same-origin" },
    );
    if (r.status === 404) return null;
    const d = await r.json();
    if (!r.ok) throw new Error(d.error || "Unable to load draft");
    return d;
  }
  function makeEditor(initial) {
    if (view) view.destroy();
    view = new CM6.EditorView({
      state:
        typeof initial === "string"
          ? CM6.EditorState.create({
              doc: initial,
              extensions: [
                CM6.basicSetup,
                CM6.language(detect()),
                CM6.EditorState.readOnly.of(!canWrite),
                CM6.EditorView.contentAttributes.of({
                  "aria-label": "File editor",
                }),
                CM6.keymap.of([...CM6.defaultKeymap, CM6.indentWithTab]),
                CM6.EditorView.updateListener.of((u) => {
                  if (u.docChanged) {
                    dirty = u.state.doc.toString() !== savedText;
                    renderTabs();
                  }
                  if (u.selectionSet || u.docChanged) {
                    const h = u.state.selection.main.head,
                      line = u.state.doc.lineAt(h);
                    $("status-position").textContent =
                      `Ln ${line.number}, Col ${h - line.from + 1}`;
                  }
                }),
              ],
            })
          : initial,
      parent: $("editor"),
    });
    const h = view.state.selection.main.head,
      line = view.state.doc.lineAt(h);
    $("status-position").textContent =
      `Ln ${line.number}, Col ${h - line.from + 1}`;
  }
  async function openFile(p, line = 0, column = 1) {
    if (!validPath(p)) return;
    if (p === path && view) {
      if (line) {
        const selected = view.state.doc.line(
            Math.min(line, view.state.doc.lines),
          ),
          position = Math.min(
            selected.to,
            selected.from + Math.max(0, column - 1),
          );
        view.dispatch({
          selection: { anchor: position },
          effects: CM6.EditorView.scrollIntoView(position, { y: "center" }),
        });
        view.focus();
        pane("editor");
      }
      return;
    }
    stash();
    if (view) {
      view.destroy();
      view = null;
    }
    const generation = ++navigationVersion;
    setPath(p);
    try {
      if (buffers.has(p)) {
        restore(buffers.get(p));
      } else {
        const [committed, d] = await Promise.all([
          loadCommitted(p),
          loadDraft(p),
        ]);
        if (generation !== navigationVersion) return;
        committedText = committed;
        currentRevision = d && d.revision ? String(d.revision) : "";
        baseSHA = d && !d.committed && d.base_sha ? d.base_sha : repositorySHA;
        savedText = d && !d.committed ? d.content : committed;
        makeEditor(savedText);
        dirty = false;
        stash();
        buffers.get(p).pendingDraft = !!d && !d.committed;
      }
      renderTabs();
      $("draft-state").textContent = currentRevision
        ? "draft rev " + currentRevision
        : "";
      await showDiff();
      loadFindings();
      if (line) {
        const selected = view.state.doc.line(
            Math.min(line, view.state.doc.lines),
          ),
          position = Math.min(
            selected.to,
            selected.from + Math.max(0, column - 1),
          );
        view.dispatch({
          selection: { anchor: position },
          effects: CM6.EditorView.scrollIntoView(position, { y: "center" }),
        });
        view.focus();
      }
      pane("editor");
    } catch (e) {
      status("File load failed: " + e.message, "warning");
    }
  }
  async function loadTree() {
    try {
      const t = await api(
        canonicalRoot +
          "/workspace/tree?sha=" +
          encodeURIComponent(repositorySHA),
      );
      committedFiles = new Set(t.tree.map((x) => x.path));
      files = [...new Set([...committedFiles, ...buffers.keys()])].sort();
      renderTree(files);
    } catch (e) {
      $("workspace-tree").innerHTML =
        '<p class="error">' + esc(e.message) + "</p>";
    }
  }
  function renderTree(list) {
    const tree = $("workspace-tree");
    if (!list.length) {
      tree.innerHTML = '<p class="muted">No files match.</p>';
      return;
    }
    SwitchyardTree.mount(tree, SwitchyardTree.build(list), {
      selected: path,
      url: editorURL,
      onSelect: openFile,
      expanded,
      onExpand: remember,
    });
    tree.querySelectorAll("[data-path]").forEach((item) => {
      const file = item.dataset.path;
      item.draggable = canWrite;
      item.ondragstart = (e) => {
        if (unsavedAffected(file)) {
          e.preventDefault();
          status("Save and commit affected drafts before moving.", "warning");
          return;
        }
        e.dataTransfer.setData("text/switchyard-path", file);
        e.dataTransfer.effectAllowed = "move";
      };
      item.ondragover = (e) => {
        if (item.hasAttribute("aria-expanded")) {
          e.preventDefault();
          item.classList.add("drop-target");
          e.dataTransfer.dropEffect = "move";
        }
      };
      item.ondragleave = () => item.classList.remove("drop-target");
      item.ondrop = (e) => {
        e.preventDefault();
        item.classList.remove("drop-target");
        const source = e.dataTransfer.getData("text/switchyard-path");
        if (source)
          fileOperation("move", source, file + "/" + source.split("/").pop());
      };
      item.oncontextmenu = (e) => {
        e.preventDefault();
        contextMenu(e, file, item.hasAttribute("aria-expanded"));
      };
    });
    tree.ondragover = (e) => {
      if (e.target === tree) e.preventDefault();
    };
    tree.ondrop = (e) => {
      if (e.target !== tree) return;
      e.preventDefault();
      const source = e.dataTransfer.getData("text/switchyard-path");
      if (source) fileOperation("move", source, source.split("/").pop());
    };
  }
  function contextMenu(event, file, directory) {
    document.querySelector(".editor-context-menu")?.remove();
    const menu = document.createElement("div");
    menu.className = "editor-context-menu";
    menu.setAttribute("role", "menu");
    menu.style.left = Math.min(event.clientX, innerWidth - 195) + "px";
    menu.style.top = Math.min(event.clientY, innerHeight - 220) + "px";
    for (const [label, action] of [
      [
        "Open",
        () =>
          directory
            ? document
                .querySelector(
                  '#workspace-tree [data-path="' + CSS.escape(file) + '"]',
                )
                .click()
            : openFile(file),
      ],
      ["New file", () => createFile(directory ? file + "/" : "")],
      ["New directory", () => createDirectory(directory ? file + "/" : "")],
      ["Rename or move", () => fileOperation("move", file, file)],
      ["Delete", () => fileOperation("delete", file)],
    ]) {
      const b = document.createElement("button");
      b.textContent = label;
      b.setAttribute("role", "menuitem");
      b.disabled = label !== "Open" && !canWrite;
      b.onclick = () => {
        menu.remove();
        action();
      };
      menu.append(b);
    }
    document.body.append(menu);
    menu.querySelector("button").focus();
    const dismiss = (e) => {
      if (e.type === "keydown" && e.key !== "Escape") return;
      if (e.type === "pointerdown" && menu.contains(e.target)) return;
      menu.remove();
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("keydown", dismiss);
    };
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("keydown", dismiss);
  }
  async function createFile(prefix = "") {
    if (!canWrite) return;
    const data = await dialog(
      "Create file",
      [["path", "Repository path", prefix]],
      "Open draft",
      "The file is saved as a draft first. Commit explicitly to add it to Git.",
    );
    if (!data) return;
    if (!validPath(data.path) || files.includes(data.path)) {
      status("Choose a new valid repository path.", "warning");
      return;
    }
    files.push(data.path);
    await openFile(data.path);
    dirty = true;
    stash();
    renderTabs();
    renderTree(files);
  }
  async function createDirectory(prefix = "") {
    if (!canWrite) return;
    const data = await dialog(
      "Create directory",
      [["path", "Directory path", prefix]],
      "Create draft",
      "Git stores files, so an empty directory starts with a .gitkeep draft.",
    );
    if (!data) return;
    const file = data.path + "/.gitkeep";
    if (
      !validPath(file) ||
      files.some((p) => p === data.path || p.startsWith(data.path + "/"))
    ) {
      status("Choose a new directory path.", "warning");
      return;
    }
    files.push(file);
    await openFile(file);
    dirty = true;
    stash();
    renderTabs();
    renderTree(files);
  }
  async function fileOperation(operation, source = path, target = path) {
    if (!canWrite || !source) return;
    if (unsavedAffected(source)) {
      status(
        "Save and commit affected drafts before " +
          (operation === "move" ? "moving" : "deleting") +
          ".",
        "warning",
      );
      return;
    }
    const fields =
      operation === "move"
        ? [
            ["target", "Destination path", target],
            ["message", "Commit message", "Move " + source],
          ]
        : [["message", "Commit message", "Delete " + source]];
    const input = await dialog(
      operation === "move" ? "Move in Git" : "Delete from Git",
      fields,
      "Commit " + operation,
      "This operation creates a Git commit at the current branch head. Saved drafts are preserved; affected uncommitted drafts block it.",
    );
    if (!input) return;
    if (operation === "move" && !validPath(input.target)) {
      status("Invalid destination path.", "warning");
      return;
    }
    try {
      const result = await api(canonicalRoot + "/workspace/mutate", {
        method: "POST",
        body: JSON.stringify({
          branch: ref,
          expected_sha: repositorySHA,
          source,
          target: input.target || "",
          operation,
          message: input.message,
        }),
      });
      repositorySHA = result.new_sha;
      const affected = tabs.filter(
        (p) => p === source || p.startsWith(source + "/"),
      );
      affected.forEach((p) => {
        buffers.delete(p);
        tabs = tabs.filter((x) => x !== p);
      });
      if (affected.includes(path)) {
        view?.destroy();
        view = null;
        path = "";
      }
      await loadTree();
      if (operation === "move")
        await openFile(
          input.target +
            (files.includes(input.target)
              ? ""
              : "/" +
                (
                  files.find((p) => p.startsWith(input.target + "/")) || ""
                ).slice(input.target.length + 1)),
        );
      else if (!view && files[0]) await openFile(files[0]);
      renderTabs();
      status(
        "File operation committed at " + result.new_sha.slice(0, 7),
        "success",
      );
    } catch (e) {
      const reasons = {
        affected_draft_present:
          "An affected saved draft must be committed first.",
        branch_moved: "The branch moved. Reload to review the latest commit.",
        invalid_file_operation:
          "The destination exists or the operation is invalid.",
        move_content_limit: "This move exceeds the safe file or byte limit.",
        protected_ref_requires_review:
          "This branch requires a pull request or integration queue.",
        repository_archived: "This repository is archived.",
      };
      status(
        reasons[e.message] ||
          "The file operation failed. Your drafts are preserved.",
        "warning",
      );
    }
  }
  async function search() {
    const query = $("repo-search").value.trim(),
      generation = ++searchVersion;
    if (!query) {
      $("search-results").replaceChildren();
      return;
    }
    const button = $("search-run");
    button.disabled = true;
    $("search-summary").textContent = "Searching current commit…";
    try {
      const result = await api(
        canonicalRoot +
          "/workspace/search?" +
          new URLSearchParams({
            sha: repositorySHA,
            q: query,
            case: $("search-case").checked ? "1" : "0",
            regex: $("search-regex").checked ? "1" : "0",
          }),
      );
      if (generation !== searchVersion) return;
      $("search-summary").textContent =
        result.items.length +
        " matches · " +
        result.scanned_files +
        " files scanned" +
        (result.truncated ? " · limit reached" : "") +
        (result.skipped_files
          ? " · " + result.skipped_files + " files skipped"
          : "");
      $("search-results").replaceChildren();
      for (const item of result.items) {
        const row = document.createElement("button");
        row.className = "search-result";
        const title = document.createElement("strong"),
          preview = document.createElement("small");
        title.textContent = item.path + (item.line ? ":" + item.line : "");
        preview.textContent = item.preview;
        row.append(title, preview);
        row.onclick = () => openFile(item.path, item.line, item.column);
        $("search-results").append(row);
      }
    } catch (e) {
      $("search-summary").textContent =
        e.message === "invalid_search_pattern"
          ? "The regular expression is invalid."
          : "Search could not be completed. Try a narrower query.";
    } finally {
      button.disabled = false;
    }
  }
  async function saveDraft() {
    if (!view || !baseSHA)
      throw new Error("Load the repository head before saving");
    const savePath = path,
      content = view.state.doc.toString(),
      revision = currentRevision,
      base = baseSHA;
    stash();
    const r = await api("/api/drafts", {
      method: "POST",
      body: JSON.stringify({
        repo: name,
        branch: ref,
        path: savePath,
        content,
        expected_revision: revision,
        base_sha: base,
      }),
    });
    const buffer = buffers.get(savePath);
    if (buffer) {
      buffer.savedText = content;
      buffer.currentRevision = String(r.revision);
      buffer.pendingDraft = true;
      buffer.dirty = buffer.state.doc.toString() !== content;
    }
    if (path === savePath) {
      currentRevision = String(r.revision);
      savedText = content;
      dirty = view.state.doc.toString() !== content;
      stash();
      renderTabs();
      $("draft-state").textContent = "draft rev " + currentRevision;
      status(
        dirty ? "Draft saved; newer edits remain unsaved" : "Draft saved",
        "success",
      );
      await showDiff();
    }
    return { ...r, path: savePath, content };
  }
  let savingAll = false;
  async function saveAllDrafts() {
    if (savingAll || !canWrite) return;
    savingAll = true;
    stash();
    let savedCount = 0;
    try {
      for (const [savePath, buffer] of buffers) {
        if (!buffer.dirty) continue;
        const content = buffer.state.doc.toString();
        const result = await api("/api/drafts", { method: "POST", body: JSON.stringify({repo:name, branch:ref, path:savePath, content, expected_revision:buffer.currentRevision, base_sha:buffer.baseSHA}) });
        const latest = buffers.get(savePath);
        if (latest) { latest.savedText = content; latest.currentRevision = String(result.revision); latest.pendingDraft = true; latest.dirty = latest.state.doc.toString() !== content; }
        if (path === savePath) { currentRevision = String(result.revision); savedText = content; dirty = view.state.doc.toString() !== content; stash(); }
        savedCount++;
      }
      renderTabs(); status(savedCount + " drafts saved", "success");
    } catch (error) { status(savedCount + " drafts saved; remaining drafts need attention: " + error.message, "error"); }
    finally { savingAll = false; }
  }
  async function showDiff() {
    if (!view) return;
    const diffPath = path;
    const now = view.state.doc.toString();
    if (now === committedText) {
      $("diff-view").textContent = "(no changes)";
      return;
    }
    try {
      const d = await api("/api/diff", {
        method: "POST",
        body: JSON.stringify({ path, old: committedText, new: now }),
      });
      if (path !== diffPath || !view || view.state.doc.toString() !== now)
        return;
      $("diff-view").dataset.path = diffPath;
      $("diff-view").textContent = d.diff || "(no diff)";
      $("diff-state").textContent = "draft vs committed";
    } catch (e) {
      $("diff-view").textContent = "diff unavailable: " + e.message;
    }
  }
  async function loadFindings() {
    if (!attempt) {
      $("findings").innerHTML = '<p class="muted">No Attempt context.</p>';
      return;
    }
    try {
      const f = await api("/api/findings"),
        mine = f.items.filter(
          (x) => x.target === attempt && (!x.file || x.file === path),
        );
      $("findings").innerHTML = mine.length
        ? mine
            .map(
              (x) =>
                '<div class="finding ' +
                esc(x.severity) +
                '"><span class="sev">' +
                esc(x.severity) +
                "</span> " +
                esc(x.message) +
                "</div>",
            )
            .join("")
        : '<p class="muted">No findings for this file.</p>';
    } catch (e) {}
  }
  $("btn-save").onclick = async () => {
    try {
      await saveDraft();
    } catch (e) {
      if (String(e.message).includes("draft_stale")) {
        status("Draft is stale; reload before saving.", "warning");
      } else status("Save failed: " + e.message, "warning");
    }
  };
  $("btn-commit").onclick = () => {
    $("commit-message").value = "edit " + path + " via Switchyard";
    $("commit-modal").showModal();
    $("commit-message").focus();
  };
  $("commit-cancel").onclick = () => $("commit-modal").close();
  $("commit-form").onsubmit = async (e) => {
    e.preventDefault();
    try {
      const saved = await saveDraft();
      const r = await api("/api/drafts/" + saved.id + "/commit", {
        method: "POST",
        body: JSON.stringify({
          message: $("commit-message").value,
          expected_revision: String(saved.revision),
        }),
      });
      if (r.status !== "ok") throw new Error("Branch moved; draft preserved");
      repositorySHA = r.new_sha;
      const b = buffers.get(saved.path);
      if (b) {
        b.committedText = saved.content;
        b.pendingDraft = r.draft_cleanup !== "complete";
        if (r.draft_cleanup === "complete") {
          b.baseSHA = r.new_sha;
          b.currentRevision = String(r.revision);
        }
      }
      if (path === saved.path) {
        committedText = saved.content;
        if (r.draft_cleanup === "complete") {
          baseSHA = r.new_sha;
          currentRevision = String(r.revision);
        }
        dirty = view.state.doc.toString() !== saved.content;
        stash();
        $("draft-state").textContent =
          "committed " + String(r.new_sha).slice(0, 7);
        renderTabs();
        showDiff();
      }
      $("commit-modal").close();
      status(
        r.draft_cleanup === "complete"
          ? "Committed"
          : "Committed; draft retained because cleanup raced or failed. Reload before saving.",
        "success",
      );
    } catch (err) {
      status("Commit failed: " + err.message, "warning");
    }
  };
  $("file-filter").oninput = (e) =>
    renderTree(
      files.filter((p) =>
        p.toLowerCase().includes(e.target.value.toLowerCase()),
      ),
    );
  $("repo-search").onkeydown = (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      search();
    }
  };
  $("search-run").onclick = search;
  document
    .querySelectorAll(".workbench-modes button")
    .forEach((b) => (b.onclick = () => setMode(b.dataset.mode)));
  $("new-file").onclick = () => createFile();
  $("new-folder").onclick = () => createDirectory();
  $("rename-file").onclick = () => fileOperation("move");
  $("move-file").onclick = () => fileOperation("move");
  $("delete-file").onclick = () => fileOperation("delete");
  $("toggle-agent").onclick = () => {
    const open = $("toggle-agent").getAttribute("aria-expanded") !== "true";
    $("toggle-agent").setAttribute("aria-expanded", String(open));
    $("workbench").classList.toggle("agent-open", open);
    document.querySelector(".side-col").hidden = !open;
    if (open && innerWidth <= 800) pane("agent");
  };
  $("btn-agent-draft").onclick = async () => {
    const prompt = $("agent-prompt").value.trim();
    if (!prompt) {
      status("Describe the change you want the Agent to propose.", "warning");
      return;
    }
    try {
      const agentHeaders = await SwitchyardAgentSelection("implementer");
 if (!agentHeaders) return;
 let saved = await saveDraft();
      const proposedPath = saved.path;
      $("agent-panel").textContent =
        "Agent is preparing a proposal against draft rev " +
        currentRevision +
        "…";
      const r = await api("/api/drafts/" + saved.id + "/agent-propose", {
        method: "POST", headers:{"Content-Type":"application/json",...agentHeaders},
        body: JSON.stringify({
          prompt,
          expected_revision: String(saved.revision),
        }),
      });
      if (
        path !== proposedPath ||
        view.state.doc.toString() !== saved.content
      ) {
        const b = buffers.get(proposedPath);
        if (b && b.state.doc.toString() === saved.content) {
          b.state = b.state.update({
            changes: { from: 0, to: b.state.doc.length, insert: r.content },
          }).state;
          b.currentRevision = String(r.revision);
          b.pendingDraft = true;
          b.dirty = false;
        }
        if (b) { b.currentRevision = String(r.revision); b.pendingDraft = true; }
        if (path === proposedPath) currentRevision = String(r.revision);
        $("agent-panel").textContent = "Proposal preserved on the server. Your newer editor text remains open; saving it replaces this proposal using its revision.";
        status(
          "Agent proposal saved; your active edits were preserved. Save your text to keep it instead of the proposal.",
          "warning",
        );
        return;
      }
      currentRevision = String(r.revision);
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: r.content },
      });
      dirty = false;
      renderTabs();
      $("draft-state").textContent =
        "Agent proposal rev " + currentRevision + " (not committed)";
      $("agent-panel").innerHTML =
        "<strong>Proposal " +
        esc(r.execution) +
        "</strong><p>Applied to draft only. Review the diff, edit/revert as needed, then commit manually.</p>";
      await showDiff();
      stash();
      buffers.get(path).pendingDraft = true;
      status("Agent proposal applied to draft", "success");
    } catch (e) {
      if (String(e.message).includes("draft_stale"))
        status(
          "Agent proposal is stale because the draft changed. Your newer draft was preserved.",
          "warning",
        );
      else status("Agent proposal failed: " + e.message, "warning");
    }
  };
  $("btn-agent").onclick = async () => {
    if (!attempt) {
      status("Formal Agent run requires an Attempt context.", "warning");
      return;
    }
    $("agent-panel").textContent = "Running formal Attempt…";
    try {
      const agentHeaders = await SwitchyardAgentSelection("implementer");
 if (!agentHeaders) return;
 const r = await api("/api/attempts/" + attempt + "/run", {
 headers:{"Content-Type":"application/json",...agentHeaders},
        method: "POST",
        body: "{}",
      });
      $("agent-panel").innerHTML =
        "<strong>" +
        esc(r.status) +
        "</strong><pre>" +
        esc(r.output || "") +
        "</pre>";
      stash();
      if (![...buffers.values()].some((b) => b.dirty || b.pendingDraft)) {
        await loadRefSHA();
        buffers.clear();
        await loadTree();
        const refreshedPath = path;
        path = "";
        await openFile(refreshedPath);
      } else status("Attempt completed. Your drafts remain open; review its changes in the repository.", "warning");
    } catch (e) {
      $("agent-panel").textContent = "Agent run failed: " + e.message;
    }
  };
  document.querySelectorAll(".mobile-workbench-switcher button").forEach(
    (b) =>
      (b.onclick = () => {
        if (b.dataset.pane === "agent") {
          document.querySelector(".side-col").hidden = false;
        }
        pane(b.dataset.pane);
      }),
  );
  window.addEventListener("beforeunload", (e) => {
    stash();
    if ([...buffers.values()].some((b) => b.dirty)) {
      e.preventDefault();
      e.returnValue = "";
    }
  });
  window.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      if (e.shiftKey) saveAllDrafts(); else $("btn-save").click();
    }
  });
  (async () => {
    try {
      await window.refreshAuth();
      const repos = (await api("/api/repositories")).items || [],
        repository = repos.find(
          (r) => r.artifact_name === name || r.full_name === name,
        );
      if (!repository) throw Error("Repository access is required.");
      name = repository.artifact_name;
      canonicalBase =
        "/" +
        encodeURIComponent(repository.owner_slug) +
        "/" +
        encodeURIComponent(repository.slug);
      canonicalRoot = "/api/repositories" + canonicalBase;
      const metadata = await api(canonicalRoot);
      window.SwitchyardIdentity.mount(repository.owner_slug, repository.slug, 'Editor', metadata);
      canWrite = metadata.can_write === true;
      [
        "btn-save",
        "btn-commit",
        "new-file",
        "new-folder",
        "rename-file",
        "move-file",
        "delete-file",
        "btn-agent-draft",
      ].forEach((id) => ($(id).disabled = !canWrite));
      $("btn-agent").disabled = !canWrite || !attempt;
      await loadRefSHA();
      await loadTree();
      await openFile(path || files[0] || "README.md");
      $("back-to-repo").href = canonicalBase;
      remember();
    } catch (e) {
      $("editor-title").textContent = "Repository editor";
      if (canonicalBase) $("back-to-repo").href = canonicalBase;
      $("editor-context").textContent = "A committed branch is required to open files.";
      status(e.message, "warning");
      $("workspace-tree").textContent = "Repository could not be opened.";
      [
        "btn-save",
        "btn-commit",
        "new-file",
        "new-folder",
        "rename-file",
        "move-file",
        "delete-file",
        "btn-agent-draft",
        "btn-agent",
      ].forEach((id) => ($(id).disabled = true));
    }
  })();
})();
