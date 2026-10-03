/* Shared inert Git diff rendering: line numbers, file disclosure and split view. */
(function (root) {
  "use strict";
  function parse(text) {
    const files = [];
    let file = null,
      old = 0,
      next = 0;
    for (const line of String(text).split("\n")) {
      if (line.startsWith("diff --git ")) {
        file = { title: line.slice(11), lines: [] };
        files.push(file);
        continue;
      }
      if (!file) {
        file = { title: "Changes", lines: [] };
        files.push(file);
      }
      const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(line);
      if (hunk) {
        old = Number(hunk[1]);
        next = Number(hunk[2]);
        file.lines.push({ kind: "hunk", text: line });
      } else if (
        line.startsWith("--- ") ||
        line.startsWith("+++ ") ||
        line.startsWith("index ") ||
        line.startsWith("new file") ||
        line.startsWith("deleted file") ||
        line.startsWith("old mode") ||
        line.startsWith("new mode")
      )
        file.lines.push({ kind: "meta", text: line });
      else if (line.startsWith("+"))
        file.lines.push({ kind: "add", text: line.slice(1), next: next++ });
      else if (line.startsWith("-"))
        file.lines.push({ kind: "remove", text: line.slice(1), old: old++ });
      else if (line.startsWith(" "))
        file.lines.push({
          kind: "context",
          text: line.slice(1),
          old: old++,
          next: next++,
        });
      else if (line) file.lines.push({ kind: "meta", text: line });
    }
    return files;
  }
  function render(container, text, split = false) {
    container.replaceChildren();
    for (const file of parse(text)) {
      const details = document.createElement("details");
      details.open = true;
      details.className = "git-diff-file";
      const summary = document.createElement("summary");
      summary.textContent = container.dataset.path || file.title;
      details.append(summary);
      const lines = document.createElement("div");
      lines.className = "git-diff-lines" + (split ? " split" : "");
      for (const line of file.lines) {
        const row = document.createElement("div");
        row.className = "git-diff-line " + line.kind;
        const number = (value) => {
          const n = document.createElement("span");
          n.className = "git-diff-number";
          n.textContent = value || "";
          return n;
        };
        const code = (value) => {
          const n = document.createElement("code");
          n.textContent = value;
          return n;
        };
        if (split && ["add", "remove", "context"].includes(line.kind)) {
          row.append(
            number(line.old),
            code(line.kind === "add" ? "" : line.text),
            number(line.next),
            code(line.kind === "remove" ? "" : line.text),
          );
        } else
          row.append(
            number(line.old),
            number(line.next),
            code(
              (line.kind === "add" ? "+" : line.kind === "remove" ? "-" : " ") +
                line.text,
            ),
          );
        lines.append(row);
      }
      details.append(lines);
      container.append(details);
    }
  }
  function enhance(pre) {
    let split = false,
      last = "";
    const wrap = document.createElement("div");
    wrap.className = "git-diff-surface";
    const controls = document.createElement("div");
    controls.className = "git-diff-controls";
    const unified = document.createElement("button"),
      side = document.createElement("button");
    unified.textContent = "Unified";
    side.textContent = "Split";
    unified.className = side.className = "btn mini";
    const output = document.createElement("div");
    controls.append(unified, side);
    wrap.append(controls, output);
    pre.after(wrap);
    function draw() {
      const text = pre.textContent;
      if (!text.includes("@@") && !text.includes("diff --git")) {
        wrap.hidden = true;
        pre.hidden = false;
        return;
      }
      pre.hidden = true;
      wrap.hidden = false;
      unified.setAttribute("aria-pressed", String(!split));
      side.setAttribute("aria-pressed", String(split));
      output.dataset.path = pre.dataset.path || "";
      render(output, text, split);
      last = text;
    }
    unified.onclick = () => {
      split = false;
      draw();
    };
    side.onclick = () => {
      split = true;
      draw();
    };
    new MutationObserver(() => {
      if (last !== pre.textContent) draw();
    }).observe(pre, { childList: true, characterData: true, subtree: true });
    draw();
  }
  if (typeof document !== "undefined")
    document.querySelectorAll("pre.diff-view").forEach(enhance);
  const api = { parse, render };
  root.SwitchyardDiff = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
