// Run through the supported CUA browser binding on a loaded, real profile.
export async function verifyProfile(tab) {
 const result=await tab.playwright.evaluate(()=>{
  const account=document.querySelector('.account-chip'),menu=document.querySelector('#hamburger'),grid=document.querySelector('#pinned-repos');
  const size=e=>{const r=e.getBoundingClientRect();return [r.width,r.height,r.y]};
  const cells=[...document.querySelectorAll('.contribution-grid .contribution-cell:not(.blank)')];
  return {account:account?size(account):null,menu:size(menu),accountText:account?.textContent.trim(),accountLabel:account?.getAttribute('aria-label'),organizations:[...document.querySelectorAll('#profile-org-grid a')].map(a=>({name:a.title,href:a.getAttribute('href'),alt:a.querySelector('img')?.alt})),repositoryScroll:getComputedStyle(grid).overflowY,repositories:grid.querySelectorAll('.repo-card').length,cells:cells.length,dates:cells.map(c=>c.title.match(/\d{4}-\d{2}-\d{2}/)?.[0]),cardBottoms:[...grid.querySelectorAll('.repo-card')].map(e=>e.getBoundingClientRect().bottom),contributionTop:document.querySelector('#profile-contributions').getBoundingClientRect().top,width:innerWidth,documentWidth:document.documentElement.scrollWidth,documentHeight:document.documentElement.scrollHeight,height:innerHeight};
 });
 if(result.account&&(JSON.stringify(result.account)!==JSON.stringify(result.menu)||result.accountText||!result.accountLabel?.startsWith('View profile for ')))throw Error('Account control differs from menu or lacks accessible identity');
 if(result.organizations.some(o=>!o.name||!o.alt||!o.href||o.href==='/undefined'))throw Error('Unnamed organization tile');
 if(result.repositoryScroll!=='visible'||result.repositories>4)throw Error('Overview has nested repository scrolling or unbounded cards');
 if(result.cells!==365||new Set(result.dates).size!==365)throw Error('Contribution calendar lacks 365 unique dates');
 if(result.cardBottoms.some(y=>y>result.contributionTop))throw Error('Repository cards overlap contributions');
 if(result.documentWidth>result.width||result.documentHeight>result.height)throw Error('Profile document overflow');
 return result;
}
