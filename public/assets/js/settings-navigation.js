/* Shared section navigation for account, organization and repository settings. */
(function () {
  "use strict";
  const nav = document.querySelector('.settings-nav');
  if (!nav) return;
  const links = [...nav.querySelectorAll('a[href^="#"]')];
  const panes = links.map(a => document.getElementById(a.hash.slice(1))).filter(Boolean);
  function select() {
    const active = links.find(a => a.hash === location.hash) || links[0];
    for (const link of links) {
      link.classList.toggle('active', link === active);
      if (link === active) link.setAttribute('aria-current', 'page');
      else link.removeAttribute('aria-current');
    }
    for (const pane of panes) pane.hidden = pane.id !== active.hash.slice(1);
  }
  window.addEventListener('hashchange', select);
  select();
})();
