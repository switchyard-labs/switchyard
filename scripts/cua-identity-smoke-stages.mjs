export async function identitySmoke(tab, stage) {
  const check = (ok, message) => { if (!ok) throw new Error(message); };
  const geometry = await tab.playwright.evaluate(() => ({width:innerWidth,scroll:document.documentElement.scrollWidth}));
  check(geometry.scroll <= geometry.width, 'Identity surface overflows document');
  if (stage === 'registration') {
    check(await tab.playwright.getByRole('heading',{name:'Create your Switchyard account',exact:true}).isVisible(),'Registration mode missing');
    const state = await tab.playwright.evaluate(() => ({nameVisible:!document.getElementById('registration-name').hidden,passwordMode:document.querySelector('[name=password]').autocomplete}));
    check(state.nameVisible && state.passwordMode === 'new-password','Registration fields have incorrect state');
    return {stage,checks:3,...geometry,...state};
  }
  if (stage === 'profile') {
    check(await tab.playwright.locator('#profile-pins-heading').innerText() === 'Pinned repositories','Actual pin heading missing');
    check(await tab.playwright.locator('#pinned-repos a').count() === 2,'Actual pins missing');
    await tab.playwright.getByRole('link',{name:'Repositories 2',exact:true}).click();
    await tab.playwright.getByRole('searchbox').fill('railway');
    await tab.getAXState();
    check(await tab.playwright.locator('#profile-repos a').count() === 1,'Repository search did not filter');
    check(await tab.playwright.evaluate(()=>document.getElementById('overview').hidden),'Overview remains visible in repository tab');
    return {stage,checks:5,...geometry};
  }
  if (stage === 'settings') {
    await tab.playwright.getByRole('link',{name:'Appearance',exact:true}).click();
    await tab.getAXState();
    check(await tab.playwright.evaluate(()=>document.getElementById('profile').hidden&&!document.getElementById('appearance').hidden),'Settings section selection failed');
    check(await tab.playwright.locator('.settings-nav [aria-current]').innerText() === 'Appearance','Settings current section missing');
    return {stage,checks:3,...geometry};
  }
  if (stage === 'work') {
    check(await tab.playwright.locator('#work-list a').count() === 3,'Open fixture Work count incorrect');
    await tab.playwright.getByRole('button',{name:'New work',exact:true}).click();
    await tab.getAXState();
    check(await tab.playwright.evaluate(()=>document.getElementById('work-dialog').open&&document.activeElement?.name==='title'),'Creation dialog did not focus title');
    await tab.playwright.getByRole('button',{name:'Close new work',exact:true}).click();
    await tab.playwright.getByRole('button',{name:'Closed',exact:true}).click();
    await tab.getAXState();
    check(await tab.playwright.locator('#work-list a').count() === 1,'Closed filter incorrect');
    return {stage,checks:4,...geometry};
  }
  throw new Error('Unknown identity stage');
}
