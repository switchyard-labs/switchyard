import {
  EditorView,
  lineNumbers,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  drawSelection,
  dropCursor,
  highlightSpecialChars,
} from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import {
  bracketMatching,
  indentOnInput,
  foldGutter,
  foldKeymap,
  syntaxHighlighting,
  HighlightStyle,
  StreamLanguage,
} from "@codemirror/language";
import {
  searchKeymap,
  highlightSelectionMatches,
  openSearchPanel,
} from "@codemirror/search";
import { closeBrackets, closeBracketsKeymap } from "@codemirror/autocomplete";
import { tags } from "@lezer/highlight";
import { javascript } from "@codemirror/lang-javascript";
import { html } from "@codemirror/lang-html";
import { css } from "@codemirror/lang-css";
import { markdown } from "@codemirror/lang-markdown";
import { json } from "@codemirror/lang-json";
import { python } from "@codemirror/lang-python";
import { cpp } from "@codemirror/lang-cpp";
import { go } from "@codemirror/legacy-modes/mode/go";
import { rust } from "@codemirror/legacy-modes/mode/rust";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { yaml } from "@codemirror/legacy-modes/mode/yaml";
import { toml } from "@codemirror/legacy-modes/mode/toml";
import { sql } from "@codemirror/legacy-modes/mode/sql";
const highlight = HighlightStyle.define([
  { tag: tags.keyword, color: "var(--syntax-keyword)" },
  { tag: tags.comment, color: "var(--syntax-comment)" },
  { tag: [tags.string, tags.regexp], color: "var(--syntax-string)" },
  { tag: [tags.number, tags.bool], color: "var(--syntax-number)" },
  { tag: [tags.propertyName, tags.typeName], color: "var(--syntax-property)" },
  { tag: tags.heading, color: "var(--syntax-heading)", fontWeight: "bold" },
]);
const theme = EditorView.theme(
  {
    "&": {
      height: "100%",
      color: "var(--text)",
      backgroundColor: "var(--bg-code)",
    },
    ".cm-scroller": {
      fontFamily: "var(--mono)",
      fontSize: "13px",
      lineHeight: "1.65",
      overflow: "auto",
    },
    ".cm-content": { padding: "14px 0", caretColor: "var(--focus)" },
    ".cm-line": { padding: "0 18px" },
    ".cm-gutters": {
      backgroundColor: "var(--bg-code)",
      color: "var(--text-faint)",
      border: "none",
    },
    ".cm-activeLine,.cm-activeLineGutter": {
      backgroundColor: "var(--panel-soft)",
    },
    "&.cm-focused .cm-selectionBackground,.cm-selectionBackground": {
      backgroundColor: "var(--signal-amber-soft)",
    },
    ".cm-cursor": { borderLeftColor: "var(--focus)" },
    ".cm-panels,.cm-tooltip": {
      backgroundColor: "var(--panel)",
      color: "var(--text)",
      borderColor: "var(--border)",
    },
    ".cm-searchMatch": {
      backgroundColor: "var(--signal-amber-soft)",
      outline: "1px solid var(--accent)",
    },
    ".cm-matchingBracket": {
      backgroundColor: "var(--signal-amber-soft)",
      outline: "1px solid var(--border-strong)",
    },
  },
  { dark: true },
);
const basicSetup = [
  lineNumbers(),
  highlightActiveLineGutter(),
  highlightActiveLine(),
  highlightSpecialChars(),
  drawSelection(),
  dropCursor(),
  history(),
  indentOnInput(),
  bracketMatching(),
  closeBrackets(),
  foldGutter(),
  highlightSelectionMatches(),
  syntaxHighlighting(highlight),
  theme,
  keymap.of([
    ...closeBracketsKeymap,
    ...defaultKeymap,
    ...historyKeymap,
    ...searchKeymap,
    ...foldKeymap,
    indentWithTab,
  ]),
];
function language(id) {
  switch (id) {
    case "javascript":
      return javascript();
    case "typescript":
      return javascript({ typescript: true });
    case "html":
    case "nift":
      return html();
    case "css":
      return css();
    case "markdown":
      return markdown();
    case "json":
      return json();
    case "python":
      return python();
    case "cpp":
      return cpp();
    case "go":
      return StreamLanguage.define(go);
    case "rust":
      return StreamLanguage.define(rust);
    case "shell":
      return StreamLanguage.define(shell);
    case "yaml":
      return StreamLanguage.define(yaml);
    case "toml":
      return StreamLanguage.define(toml);
    case "sql":
      return StreamLanguage.define(sql);
    default:
      return [];
  }
}
globalThis.CM6 = {
  EditorView,
  EditorState,
  basicSetup,
  keymap,
  defaultKeymap,
  indentWithTab,
  language,
  openSearchPanel,
};
