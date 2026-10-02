// CP10 browser test: human -> Agent -> inspect diff -> commit loop in a real
// Chromium via Playwright. Expects SW (base url) to be reachable.
import { chromium } from "playwright";
const BASE = process.env.BASE || "http://127.0.0.1:18080";

const repo = process.env.REPO;
const branch = process.env.BRANCH || "main";
const path = process.env.PATH || "base.txt";
const attempt = process.env.ATTEMPT || "";
const attemptBranch = process.env.ATTEMPT_BRANCH || "";

const results = [];
const check = (name, ok, detail) => { results.push({ name, ok, detail }); console.log((ok ? "PASS" : "FAIL") + " " + name + (detail ? " — " + detail : "")); };

const browser = await chromium.launch({ headless: true, executablePath: "/usr/bin/chromium-browser", args: ["--no-sandbox"] });
const page = await browser.newPage();

// sign in via API to seed the session cookie, then load the app shell
const login = await page.request.post(BASE + "/api/auth/login", { data: { username: "alice", password: "password123" } });
check("sign in", login.ok(), String(login.status()));
const jar = await page.context().cookies();
check("session cookie set", jar.some((c) => c.name === "switchyard_session"), jar.map((c) => c.name).join(","));

// 1. open the editor
const url = BASE + "/edit.html?name=" + encodeURIComponent(repo) + "&ref=" + encodeURIComponent(branch) + "&path=" + encodeURIComponent(path) + (attempt ? "&attempt=" + encodeURIComponent(attempt) : "");
await page.goto(url, { waitUntil: "networkidle" });
await page.waitForSelector(".cm-editor", { timeout: 10000 });
check("CM6 editor renders", (await page.locator(".cm-editor").count()) > 0, url);

const title = await page.locator("#editor-title").textContent();
check("editor title set", title.includes(repo) && title.includes(path), title);

// 2. type a human edit (Save != Commit)
const editorHost = page.locator(".cm-content");
await editorHost.focus();
await page.keyboard.press("Control+End");
const marker = "edited-by-human-" + Date.now();
await page.keyboard.press(process.platform === "darwin" ? "Meta+A" : "Control+A");
await page.keyboard.type(marker + "\ndraft survives reload\n");
await page.locator("#btn-save").click();
await page.waitForFunction(() => document.getElementById("draft-state").textContent.includes("draft saved"));
check("Save draft shows 'draft saved (not committed)'", true, await page.locator("#draft-state").textContent());

// 3. draft survives reload
await page.reload({ waitUntil: "networkidle" });
await page.waitForSelector(".cm-editor");
await page.waitForFunction(() => document.getElementById("draft-state").textContent.includes("draft saved"));
await page.waitForTimeout(800);
const reloaded = (await page.locator(".cm-content").textContent()) || "";
const committedOnBranch = await (await page.request.get(BASE + "/api/repos/" + encodeURIComponent(repo) + "/content?ref=" + encodeURIComponent(branch) + "&path=" + encodeURIComponent(path))).text();
const draftState = await page.locator("#draft-state").textContent();
check("draft survives reload (Save != Commit)",
      reloaded.includes("edited-by-human-") && reloaded !== committedOnBranch && draftState.includes("draft saved"),
      "editor-has-draft=" + reloaded.includes("edited-by-human-") + " editor-ne-committed=" + (reloaded !== committedOnBranch) + " state=" + draftState.slice(0, 20));

// 4. diff surface shows changes
await page.locator("#btn-save").click();
await page.waitForSelector("#diff-view");
await page.waitForFunction(() => document.getElementById("diff-view").textContent.includes("edited-by-human-"));
const diff = await page.locator("#diff-view").textContent();
check("diff surface shows draft vs committed", diff.includes("+edited-by-human-"), diff.split("\n").slice(0, 3).join(" | "));

// 5. commit via the shared substrate
page.once("dialog", (d) => d.accept("cp10 browser commit"));
await page.locator("#btn-commit").click();
await page.waitForFunction(() => document.getElementById("draft-state").textContent.startsWith("committed "), null, { timeout: 20000 });
check("Commit applies via shared substrate", true, await page.locator("#draft-state").textContent());

// verify committed content on the branch via API
const content = await page.request.get(BASE + "/api/repos/" + encodeURIComponent(repo) + "/content?ref=" + encodeURIComponent(branch) + "&path=" + encodeURIComponent(path));
const body = await content.text();
check("committed content on branch", body.includes("edited-by-human-"), body.slice(0, 40).replace(/\n/g, " "));

// 6. draft is cleared after commit (reload shows committed content, not draft)
await page.reload({ waitUntil: "networkidle" });
await page.waitForSelector(".cm-editor");
await page.waitForTimeout(300);
const afterCommit = await page.locator(".cm-content").textContent();
check("draft cleared after commit", afterCommit.includes("edited-by-human-") && !(await page.locator("#draft-state").textContent()).includes("draft saved"), (await page.locator("#draft-state").textContent()).slice(0, 30));

// 7. Agent panel: navigate to the ATTEMPT branch ATTEMPT.md (the file the
//    implementer edits), run the agent, and verify the editor + diff reflect
//    the agent's change (human -> Agent -> inspect diff -> commit loop).
if (attempt) {
  const url2 = BASE + "/edit.html?name=" + encodeURIComponent(repo) + "&ref=" + encodeURIComponent(attemptBranch) + "&path=ATTEMPT.md&attempt=" + encodeURIComponent(attempt);
  await page.goto(url2, { waitUntil: "networkidle" });
  await page.waitForSelector(".cm-editor");
  await page.waitForTimeout(500);
  const before = await page.locator(".cm-content").textContent();
  await page.locator("#btn-agent").click();
  await page.waitForFunction(() => document.getElementById("agent-panel").textContent.includes("execution"), null, { timeout: 30000 });
  const agentOut = await page.locator("#agent-panel").textContent();
  check("agent panel runs implementer", agentOut.includes("execution"), agentOut.slice(0, 60));
  // the run reloads the committed content; the agent appended a run line
  await page.waitForTimeout(1200);
  const after = (await page.locator(".cm-content").textContent()) || "";
  check("agent edit lands in editor", after.length > 0 && after !== before, "before=" + before.length + " after=" + after.length);
  // verify via API that the attempt branch ATTEMPT.md grew (2+ lines)
  const att = await page.request.get(BASE + "/api/repos/" + encodeURIComponent(repo) + "/content?ref=" + encodeURIComponent(attemptBranch) + "&path=ATTEMPT.md");
  const attText = await att.text();
  const lines = attText.split("\n").filter(Boolean).length;
  check("agent edit committed to attempt branch", lines >= 2, "ATTEMPT.md lines=" + lines);
}

await browser.close();
const pass = results.filter((r) => r.ok).length;
console.log("CP10_BROWSER " + pass + "/" + results.length);
process.exit(pass === results.length ? 0 : 1);