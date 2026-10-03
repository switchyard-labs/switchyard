# C30 anchored Code menu

The Code menu is a nonmodal dialog anchored to the trigger, with constrained
viewport dimensions and one internal scroll region. It repositions on resize
and scroll. HTTPS and Switchyard CLI tabs support arrow/Home/End keys. A second
trigger click, outside click, Escape and the close button dismiss it and return
focus to the trigger. `aria-expanded`, dialog labeling and tab semantics reflect
the state. SSH is intentionally absent pending C31 support research.

`clone-popover.json` records Chromium checks at all six requested widths:
anchoring, viewport bounds, keyboard mode selection, Escape/focus return,
trigger toggle and outside dismissal. The close button was also verified.
The browser displayed “Copied” after the HTTPS copy action, with no captured
console errors. However, the desktop clipboard read returned an empty string;
clipboard contents were **not certified** by that check. Full cross-browser
and real external clipboard verification remain open. No credentials were
copied or embedded in these public clone URLs.
