// Permanent browser regression through the documented cua_repl browser API.
// Run one stage per tool call and inspect fresh AX state between navigations;
// this lets the in-app browser finish asynchronous provider requests.
import assert from 'node:assert/strict';
export async function actionsSmokeStage(tab,stage,{populated,empty}) {
 const checks=[];
 if(stage==='open'){await tab.goto(populated);await tab.getAXState();return {stage,checks};}
 if(stage==='list'){
  const count=await tab.playwright.locator('.actions-run-row').count();assert.ok(count>0,'populated fixture has no real run');checks.push('populated run list');
  await tab.playwright.locator('#actions-status').selectOption('failure');const failedCount=await tab.playwright.locator('.actions-run-row').count();assert.ok(failedCount<=count);assert.ok((await tab.playwright.locator('.actions-run-row').allTextContents({})).every(text=>/failed|failure/i.test(text)));await tab.playwright.locator('#actions-clear').click();assert.equal(await tab.playwright.locator('.actions-run-row').count(),count);checks.push('status filter and reset');
  await tab.playwright.locator('#actions-search').fill('no-such-commit');assert.equal(await tab.playwright.locator('.actions-run-row').count(),0);await tab.playwright.locator('#actions-search').fill('');checks.push('search and clear');
  await tab.playwright.locator('.actions-run-row').first().click();await tab.getAXState();checks.push('run navigation');
 }else if(stage==='detail'){
  await tab.playwright.locator('.actions-step summary').first().click();assert.ok(await tab.playwright.locator('.actions-step[open]').count()>0);checks.push('step disclosure');
  const widths=await tab.playwright.evaluate(()=>({viewport:innerWidth,document:document.documentElement.scrollWidth}));assert.ok(widths.document<=widths.viewport,'horizontal overflow');checks.push('no horizontal overflow');
  await tab.goto(empty);await tab.getAXState();
 }else if(stage==='empty'){
  assert.equal(await tab.playwright.locator('.actions-run-row').count(),0);assert.match(await tab.playwright.locator('.actions-empty h2').innerText(),/checked on every push/);checks.push('true empty state');
 }else throw new Error('Unknown smoke stage');
 return {stage,passed:checks.length,checks};
}
