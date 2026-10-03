/* Switchyard shared code-surface language registry + lightweight highlighter.
   Read-only source, Markdown code fences, diffs and editor status all share
   these language identities. No runtime CDN dependency. */
(function (root) {
  "use strict";

  const languages = [
    { id: "go", label: "Go", ext: ["go"], keywords: "break default func interface select case defer go map struct chan else goto package switch const fallthrough if range type continue for import return var" },
    { id: "cpp", label: "C / C++", ext: ["c","cc","cpp","cxx","h","hh","hpp","hxx"], keywords: "alignas alignof auto bool break case catch char class const constexpr continue default delete do double else enum explicit export extern false float for friend if inline int long namespace new noexcept nullptr operator private protected public register reinterpret_cast return short signed sizeof static struct switch template this throw true try typedef typename union unsigned using virtual void volatile while" },
    { id: "rust", label: "Rust", ext: ["rs"], keywords: "as async await break const continue crate dyn else enum extern false fn for if impl in let loop match mod move mut pub ref return self Self static struct super trait true type unsafe use where while" },
    { id: "javascript", label: "JavaScript", ext: ["js","mjs","cjs"], keywords: "async await break case catch class const continue debugger default delete do else export extends false finally for from function get if import in instanceof let new null of return set static super switch this throw true try typeof undefined var void while with yield" },
    { id: "typescript", label: "TypeScript", ext: ["ts"], keywords: "abstract any as asserts async await boolean break case catch class const constructor continue declare default delete do else enum export extends false finally for from function get if implements import in infer instanceof interface keyof let module namespace never new null number object of override private protected public readonly return satisfies set static string super switch symbol this throw true try type typeof undefined unknown var void while with yield" },
    { id: "jsx", label: "JSX", ext: ["jsx"] },
    { id: "tsx", label: "TSX", ext: ["tsx"] },
    { id: "html", label: "HTML", ext: ["html","htm"] },
    { id: "css", label: "CSS", ext: ["css","scss","sass","less"] },
    { id: "json", label: "JSON", ext: ["json","jsonc"] },
    { id: "yaml", label: "YAML", ext: ["yml","yaml"] },
    { id: "toml", label: "TOML", ext: ["toml"] },
    { id: "markdown", label: "Markdown", ext: ["md","markdown","mdx"], names: ["README","README.md","AGENTS.md","HANDOVER.md"] },
    { id: "shell", label: "Shell", ext: ["sh","bash","zsh","fish"], names: ["Dockerfile"] },
    { id: "python", label: "Python", ext: ["py","pyw"], keywords: "and as assert async await break class continue def del elif else except False finally for from global if import in is lambda None nonlocal not or pass raise return True try while with yield" },
    { id: "sql", label: "SQL", ext: ["sql"], keywords: "SELECT FROM WHERE INSERT UPDATE DELETE CREATE DROP ALTER TABLE INDEX INTO VALUES JOIN LEFT RIGHT INNER OUTER ON AS AND OR NOT NULL PRIMARY KEY FOREIGN REFERENCES GROUP BY ORDER LIMIT OFFSET HAVING UNION ALL DISTINCT CASE WHEN THEN ELSE END" },
    { id: "nift", label: "Nift", ext: ["nift","nix"], names: ["template.html"], keywords: "if else for fn function return break continue import script true false null" },
    { id: "strut", label: "Strut", ext: ["p","strut"], keywords: "function return if else for while struct class enum export private public const async await new true false null map set vector string int bool float double" },
    { id: "text", label: "Plain text", ext: [] }
  ];

  const byId = Object.fromEntries(languages.map((l) => [l.id, l]));
  const byExt = {};
  const byName = {};
  languages.forEach((l) => {
    (l.ext || []).forEach((e) => { byExt[e] = l.id; });
    (l.names || []).forEach((n) => { byName[n.toLowerCase()] = l.id; });
  });

  function escapeHTML(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
  }

  function detectLanguage(path) {
    path = String(path || "");
    const name = path.split("/").pop() || "";
    const lower = name.toLowerCase();
    if (byName[lower]) return byName[lower];
    if (/^dockerfile(?:\.|$)/i.test(name)) return "shell";
    if (/^makefile$/i.test(name)) return "shell";
    const dot = lower.lastIndexOf(".");
    const ext = dot >= 0 ? lower.slice(dot + 1) : "";
    return byExt[ext] || "text";
  }

  const aliases={sh:"shell",bash:"shell",js:"javascript",ts:"typescript",py:"python",md:"markdown",yml:"yaml",c:"cpp",cxx:"cpp"};
  function labelFor(id) { return (byId[aliases[id]||id] || byId.text).label; }

  function keywordSet(id) {
    const src = (byId[id] && byId[id].keywords) || (id === "jsx" ? byId.javascript.keywords : id === "tsx" ? byId.typescript.keywords : "");
    return new Set(String(src || "").split(/\s+/).filter(Boolean));
  }

  function lexGeneric(source, id) {
    const kws = keywordSet(id);
    const out = [];
    let i = 0;
    const push = (cls, text) => out.push(cls ? '<span class="tok '+cls+'">'+escapeHTML(text)+'</span>' : escapeHTML(text));
    while (i < source.length) {
      const rest = source.slice(i);
      let m;
      if ((m = rest.match(/^\/\/[^\n]*/)) || (m = rest.match(/^#[^\n]*/)) && ["python","shell","yaml","toml","nift","strut"].includes(id)) {
        push("comment", m[0]); i += m[0].length; continue;
      }
      if ((m = rest.match(/^\/\*[\s\S]*?\*\//))) { push("comment", m[0]); i += m[0].length; continue; }
      if ((m = rest.match(/^`(?:\\.|[^`])*`/)) || (m = rest.match(/^"(?:\\.|[^"\\])*"/)) || (m = rest.match(/^'(?:\\.|[^'\\])*'/))) {
        push("string", m[0]); i += m[0].length; continue;
      }
      if ((m = rest.match(/^\b(?:0x[0-9a-fA-F]+|\d+(?:\.\d+)?)\b/))) { push("number", m[0]); i += m[0].length; continue; }
      if ((m = rest.match(/^[A-Za-z_$][A-Za-z0-9_$]*/))) {
        push(kws.has(m[0]) || kws.has(m[0].toUpperCase()) ? "keyword" : "", m[0]); i += m[0].length; continue;
      }
      push("", source[i]); i++;
    }
    return out.join("");
  }

  function highlight(source, id) {
    source = String(source == null ? "" : source);
    id = aliases[id] || id || "text";
    if (id === "text" || source.length > 800000) return escapeHTML(source);
    if (id === "json") {
      return escapeHTML(source)
        .replace(/(&quot;(?:\\.|[^&])*?&quot;)(\s*:)/g, '<span class="tok property">$1</span>$2')
        .replace(/\b(true|false|null)\b/g, '<span class="tok keyword">$1</span>')
        .replace(/\b(-?\d+(?:\.\d+)?)\b/g, '<span class="tok number">$1</span>');
    }
    if (id === "html") {
      return escapeHTML(source).replace(/(&lt;\/?)([A-Za-z][\w:-]*)/g, '$1<span class="tok tag">$2</span>').replace(/\s([A-Za-z_:][-\w:.]*)(=)/g, ' <span class="tok property">$1</span>$2');
    }
    if (id === "css") {
      return escapeHTML(source).replace(/([.#]?[A-Za-z_-][\w-]*)(\s*\{)/g, '<span class="tok tag">$1</span>$2').replace(/([\w-]+)(\s*:)/g, '<span class="tok property">$1</span>$2');
    }
    if (id === "markdown") {
      return escapeHTML(source).replace(/^(#{1,6}\s.*)$/gm, '<span class="tok heading">$1</span>').replace(/(`[^`]+`)/g, '<span class="tok string">$1</span>').replace(/(\*\*[^*]+\*\*)/g, '<span class="tok keyword">$1</span>');
    }
    if (id === "yaml" || id === "toml") {
      return lexGeneric(source, id).replace(/^([\w.-]+)(\s*[:=])/gm, '<span class="tok property">$1</span>$2');
    }
    return lexGeneric(source, id);
  }

  function lines(source, id) {
    const highlighted = highlight(source, id).split("\n");
    return highlighted.map((html, idx) => '<div class="code-line" id="L'+(idx+1)+'"><a class="line-no" href="#L'+(idx+1)+'" aria-label="Line '+(idx+1)+'">'+(idx+1)+'</a><span class="line-code">'+(html || " ")+'</span></div>').join("");
  }

  const api = { languages, detectLanguage, labelFor, highlight, lines, escapeHTML };
  root.SwitchyardCode = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
