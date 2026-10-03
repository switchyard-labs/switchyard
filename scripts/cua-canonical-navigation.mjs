// Run with the documented CUA tab API against the disposable local fixture.
async function ready(tab, locator) {
  // The desktop backend can cap individual selector waits below the requested
  // timeout. Inspect loading state between bounded retries instead of sleeping.
  for (let attempt = 0; attempt < 10; attempt++) {
    try { await locator.waitFor({state: 'visible', timeoutMs: 30000}); return; }
    catch (error) {
      const snapshot = await tab.playwright.domSnapshot();
      if (await locator.isVisible()) return;
      if (!snapshot.includes('Loading') || attempt === 9) throw error;
    }
  }
}
export async function verifyCanonicalNavigation(tab, origin, owner, repo) {
  const base = `${origin}/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}`;
  const results = [];
  await tab.goto(base + '/branches');
  await ready(tab, tab.playwright.getByText('Back to code', {exact: true}));
  results.push({surface: 'branches', url: await tab.url(), links: await tab.playwright.locator('.repository-ref-row').allTextContents({})});
  await tab.playwright.getByRole('link', {name: 'Commits', exact: true}).click();
  await ready(tab, tab.playwright.locator('.commit-row').first());
  await tab.playwright.locator('.commit-row').first().click();
  await ready(tab, tab.playwright.locator('#commit-detail h2'));
  const commitURL = await tab.url();
  if (!commitURL.includes('/commit/')) throw new Error('Missing canonical commit URL');
  await tab.reload();
  await ready(tab, tab.playwright.locator('#commit-detail h2'));
  results.push({surface: 'commit-refresh', url: await tab.url(), detail: await tab.playwright.locator('#commit-detail').innerText()});
  await tab.back();
  if (!(await tab.url()).includes('/commits/')) throw new Error('Back did not restore history');
  await tab.forward();
  await ready(tab, tab.playwright.locator('#commit-detail h2'));
  if (await tab.url() !== commitURL) throw new Error('Forward lost commit identity');
  results.push({surface: 'history-back-forward', passed: true});
  await tab.goto(base + '/tags');
  await ready(tab, tab.playwright.getByText('Back to code', {exact: true}));
  results.push({surface: 'tags', url: await tab.url(), text: await tab.playwright.locator('#repository-directory').innerText()});
  await tab.playwright.getByRole('link', {name: 'Work', exact: true}).nth(1).click();
  await ready(tab, tab.playwright.locator('#work-count').getByText(/items?/));
  results.push({surface: 'repository-work', url: await tab.url(), text: await tab.playwright.locator('#work-list').innerText()});
  results.push({surface: 'console', entries: await tab.dev.logs({levels: ['error', 'warn'], limit: 50})});
  return results;
}

// Call on an authenticated disposable editor branch with known committed paths.
export async function verifyCanonicalEditor(tab, origin, owner, repo, ref, firstPath, nextFile) {
 const base=`${origin}/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/edit/${encodeURIComponent(ref)}/`;
 await tab.goto(base+firstPath.split('/').map(encodeURIComponent).join('/'));
 await ready(tab,tab.playwright.getByRole('treeitem',{name:nextFile,exact:true}));
 await tab.playwright.getByRole('treeitem',{name:nextFile,exact:true}).click();
 const url=await tab.url();
 if(!url.startsWith(base)||url.includes('?path=')||!url.endsWith('/'+encodeURIComponent(nextFile)))throw Error('File navigation lost canonical editor path');
 await tab.reload();
 await ready(tab,tab.playwright.getByRole('textbox',{name:'File editor',exact:true}));
 const title=await tab.playwright.locator('#editor-title').innerText();
 if(!title.endsWith(nextFile))throw Error('Editor refresh changed selected file');
 const ownerHref=await tab.playwright.getByRole('navigation',{name:'Repository location'}).getByRole('link',{name:owner,exact:true}).getAttribute('href');
 return {url:await tab.url(),title,ownerHref,console:await tab.dev.logs({levels:['error','warn'],limit:20})};
}

// The repository browser owns one identity row; deeper breadcrumbs contain only ref/path.
export async function verifyRepositoryHeading(tab, origin, owner, repo, ref, path) {
 const base=`${origin}/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}`;
 const target=base+'/blob/'+encodeURIComponent(ref)+'/'+path.split('/').map(encodeURIComponent).join('/');
 if(await tab.url()!==target)await tab.goto(target);
 await ready(tab,tab.playwright.locator('#file-view .code-line').first());
 const result=await tab.playwright.evaluate(()=>({
  duplicate:document.querySelectorAll('.repository-context').length,
  owner:document.querySelector('#repo-owner').getAttribute('href'),
  repo:document.querySelector('#repo-name a').getAttribute('href'),
  path:[...document.querySelectorAll('#repo-breadcrumbs a')].map(a=>a.textContent),
  visibility:document.querySelectorAll('#repo-visibility').length
 }));
 if(result.duplicate||result.owner!=='/'+encodeURIComponent(owner)||result.repo!==new URL(base).pathname||result.path.join('/')!==ref+'/'+path||result.visibility!==1)throw Error('Duplicate or incorrect repository heading: '+JSON.stringify(result));
 return result;
}
