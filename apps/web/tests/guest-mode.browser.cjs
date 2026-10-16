// All API traffic is intercepted. Run against Vite; never writes production data.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const assert = require("node:assert/strict");
const base = process.env.TEST_BASE_URL || "http://127.0.0.1:5186";
const screenshots = process.env.TEST_SCREENSHOT_DIR;
const session = { user_id: 7, access_token: "test-only", refresh_token: "test-only" };
const directions = ["front-end", "back-end", "agent"].map((code, i) => ({
  id: i + 1, code, name: ["前端开发", "后端开发", "智能体"][i],
  open: i !== 2, sort_order: i, form: { id: 1, share_token: "test-form", is_open: true },
}));
const application = { id: 42, user_id: 7, round_id: 1, direction_code: "front-end", status: "applied",
  name: "测试姓名", created_at: "2026-10-16T00:00:00Z", updated_at: "2026-10-16T00:00:00Z" };

async function fixture(browser, { width = 1440, role = "member", signedIn = false, rejectedPath, status = 401, networkFailure = false, reducedMotion = "reduce" } = {}) {
  const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion });
  if (signedIn) await context.addInitScript(s => localStorage.setItem("iot-pillot.auth", JSON.stringify(s)), session);
  const page = await context.newPage();
  const requests = [];
  const errors = [];
  let submitted = false;
  let rejected = false;
  page.on("pageerror", e => errors.push(e.message));
  await context.route("**/api/v1/**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    requests.push({ path, method: request.method(), auth: request.headers().authorization });
    const ok = data => route.fulfill({ contentType: "application/json", body: JSON.stringify({ code: 0, message: "ok", data }) });
    if (path === rejectedPath && !rejected) {
      rejected = true;
      if (networkFailure) return route.abort("failed");
      return route.fulfill({ status, contentType: "application/json", body: JSON.stringify({ code: 4011, message: "会话失效" }) });
    }
    if (path === "/api/v1/recruitment/directions") return ok({ items: directions });
    if (path === "/api/v1/me") return ok({ ...session, username: "fixture", name: "测试姓名", role,
      detail: { class: "测试班级", student_id: 123, qq: "12345", email: "test@example.com", direction: "front-end" } });
    if (path === "/api/v1/login") return ok(session);
    if (path === "/api/v1/recruitment/applications") {
      if (request.method() === "POST") {
        assert.equal(request.headers().authorization, "Bearer test-only");
        assert.deepEqual(request.postDataJSON(), { direction_code: "front-end" });
        submitted = true;
        return ok(application);
      }
      // Delay successful personal reads to probe stale responses after another request invalidates the session.
      if (rejectedPath && !networkFailure) await new Promise(resolve => setTimeout(resolve, 100));
      return ok({ items: submitted ? [application] : [], total: submitted ? 1 : 0, page: 1, page_size: 20 });
    }
    errors.push("Unexpected API: " + request.method() + " " + path);
    return route.abort();
  });
  return { context, page, requests, errors };
}

async function assertGuest(page) {
  await page.locator(".guest-journey").waitFor();
  assert.equal(await page.locator(".guest-stat").count(), 2);
  assert.equal(await page.locator(".sign-in-cta").count(), 1);
  assert.equal(await page.locator(".user-menu").count(), 0);
  assert.equal(await page.locator(".journey-summary").count(), 0);
}

(async () => {
  const browser = await chromium.launch({ channel: "msedge", headless: true });
  try {
    for (const width of [1440, 320, 375, 390, 430]) {
      const { context, page, requests, errors } = await fixture(browser, { width });
      await page.goto(base + "/user/home?source=guest#directions");
      await page.locator(".direction-panel").first().waitFor();
      await assertGuest(page);
      assert.equal(await page.locator(".direction-panel").count(), 3);
      assert.match(await page.locator(".space-stats").innerText(), /02/);
      assert.equal(await page.getByRole("button", { name: "暂未开放" }).isDisabled(), true);
      assert(requests.every(r => r.path === "/api/v1/recruitment/directions" && !r.auth), "guests only request public directions");
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "no page overflow at " + width);
      if (screenshots && [1440, 390].includes(width)) {
        await page.screenshot({ path: screenshots + "/guest-home-" + width + ".png", fullPage: true });
      }
      await page.getByRole("button", { name: "了解并报名" }).first().click();
      const dialog = page.getByRole("dialog", { name: "需要登录" });
      await dialog.waitFor();
      if (screenshots && width === 390) await page.screenshot({ path: screenshots + "/guest-sign-in-390.png" });
      await dialog.getByRole("button", { name: "继续浏览" }).click();
      assert.equal(await page.locator("dialog[open]").count(), 0);
      // Every personal entry shares the same prompt.
      for (const name of ["我的进度", "邀请与结果", "完善我的资料"]) {
        await page.getByRole("link", { name: new RegExp("^" + name) }).click();
        await dialog.waitFor();
        await page.keyboard.press("Escape");
        await dialog.waitFor({ state: "detached" });
      }
      await page.getByRole("button", { name: "了解并报名" }).first().click();
      await dialog.getByRole("button", { name: "去登录", exact: true }).click();
      await page.waitForURL(url => url.pathname === "/");
      assert.equal(new URL(page.url()).searchParams.get("redirect"), "/user/home?source=guest#directions");
      if (width === 1440) {
        await page.locator("#username").fill("fixture");
        await page.locator("#login-password").fill("Password123!");
        await page.locator("#login-pane button[type=submit]").click();
        await page.waitForURL(url => url.pathname === "/user/home");
        assert.equal(new URL(page.url()).hash, "#directions");
        await page.locator(".user-menu").waitFor();
        assert.equal(await page.locator(".sign-in-cta").count(), 0);
      }
      await page.evaluate(() => localStorage.clear());
      requests.length = 0;
      await page.goto(base + "/user/about");
      await page.locator(".about").waitFor();
      assert.equal(requests.length, 0, "guest about must not load private APIs");
      for (const target of ["/user/settings", "/user/invitations", "/recruitment/applications", "/dashboard", "/f/test-form"]) {
        await page.goto(base + target);
        await page.waitForURL(url => url.pathname === "/");
        assert.equal(new URL(page.url()).searchParams.get("redirect"), target);
      }
      await page.goto(base + "/");
      if (screenshots && [1440, 390].includes(width)) {
        await page.screenshot({ path: screenshots + "/guest-login-" + width + ".png", fullPage: true });
      }
      await page.getByRole("link", { name: "先逛逛，了解工作室", exact: true }).click();
      await page.waitForURL(url => url.pathname === "/user/home");
      await assertGuest(page);
      assert.deepEqual(errors, []);
      console.log("PASS guest browsing / prompts / redirects / protected routes: " + width + "px");
      await context.close();
    }
    for (const status of [401, 403]) {
      for (const rejectedPath of ["/api/v1/me", "/api/v1/recruitment/applications", "/api/v1/recruitment/directions"]) {
        const { context, page, errors } = await fixture(browser, { signedIn: true, rejectedPath, status });
        await page.goto(base + "/user/home");
        await assertGuest(page);
        await page.locator(".direction-panel").first().waitFor();
        await page.waitForTimeout(200);
        assert.equal(await page.evaluate(() => localStorage.getItem("iot-pillot.auth")), null);
        assert.equal(new URL(page.url()).pathname, "/user/home");
        assert.equal(await page.locator('[role="alert"]').count(), 0);
        assert.deepEqual(errors, []);
        console.log("PASS rejected session stays guest: " + status + " " + rejectedPath);
        await context.close();
      }
    }
    for (const role of ["admin", "member"]) {
      const { context, page, errors } = await fixture(browser, { signedIn: true, role });
      await page.goto(base + "/user/home");
      await page.locator(".user-menu").waitFor();
      await page.getByRole("button", { name: "了解并报名" }).first().click();
      await page.getByRole("button", { name: "报名 · 无需填表" }).click();
      await page.getByText("报名成功，可在「我的进度」查看审核进展。").waitFor();
      assert.equal(await page.getByRole("button", { name: "已报名", exact: true }).isDisabled(), true);
      assert.equal(await page.getByRole("button", { name: "面试 · 填写该方向表单" }).isEnabled(), true);
      await page.keyboard.press("Escape");
      await page.locator(".journey-summary").waitFor();
      assert.match(await page.locator(".space-stats").innerText(), /01/);
      assert.deepEqual(errors, []);
      console.log("PASS authenticated application flow: " + role);
      await context.close();
    }
    {
      const { context, page, errors } = await fixture(browser, { signedIn: true });
      // Hold the storage notification back: the request must recheck storage itself.
      await context.addInitScript(() => window.addEventListener("storage", event => event.stopImmediatePropagation(), true));
      let release;
      let requested;
      const pending = new Promise(resolve => { requested = resolve; });
      await page.route("**/api/v1/recruitment/directions", async route => {
        requested();
        await new Promise(resolve => { release = resolve; });
        await route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: 4011, message: "失效" }) });
      });
      await page.goto(base + "/user/home");
      await pending;
      const other = await context.newPage();
      await other.goto(base + "/");
      await other.evaluate(() => localStorage.setItem("iot-pillot.auth", JSON.stringify({ user_id: 8, access_token: "new-account", refresh_token: "new-refresh" })));
      await page.waitForTimeout(100);
      release();
      await page.locator('[role="alert"]').waitFor();
      assert.equal(await page.evaluate(() => JSON.parse(localStorage.getItem("iot-pillot.auth")).user_id), 8);
      assert.equal(await page.locator(".guest-journey").count(), 0);
      assert.deepEqual(errors, []);
      console.log("PASS stale directions rejection preserves newer account");
      await context.close();
    }
    {
      const { context, page, errors } = await fixture(browser, { signedIn: true });
      let release;
      let requested;
      const pending = new Promise(resolve => { requested = resolve; });
      await page.route("**/api/v1/recruitment/applications", async route => {
        if (route.request().method() !== "POST") return route.fallback();
        requested();
        await new Promise(resolve => { release = resolve; });
        await route.fulfill({ contentType: "application/json", body: JSON.stringify({ code: 0, data: application }) });
      });
      await page.goto(base + "/user/home");
      await page.getByRole("button", { name: "了解并报名" }).first().click();
      await page.getByRole("button", { name: "报名 · 无需填表" }).click();
      await pending;
      const other = await context.newPage();
      await other.goto(base + "/");
      await other.evaluate(() => localStorage.setItem("iot-pillot.auth", JSON.stringify({ user_id: 8, access_token: "new-account", refresh_token: "new-refresh" })));
      await page.waitForFunction(() => !document.querySelector("dialog[open]"));
      await page.getByRole("button", { name: "了解并报名" }).first().click();
      release();
      await page.waitForTimeout(150);
      assert.equal(await page.getByText("报名成功，可在「我的进度」查看审核进展。").count(), 0);
      assert.deepEqual(errors, []);
      console.log("PASS stale application response cannot publish to newer account");
      await context.close();
    }
    {
      const { context, page, errors } = await fixture(browser, { width: 390, reducedMotion: "no-preference" });
      await page.goto(base + "/user/home");
      await assertGuest(page);
      const other = await context.newPage();
      await other.goto(base + "/user/about");
      await other.evaluate(s => localStorage.setItem("iot-pillot.auth", JSON.stringify(s)), session);
      await page.locator(".user-menu").waitFor();
      await page.waitForFunction(() => !document.querySelector(".guest-journey"));
      await page.goto(base + "/user/about");
      await page.locator(".user-menu__name").filter({ hasText: "测试姓名" }).waitFor({ state: "attached" });
      const reloadProfile = page.waitForRequest("**/api/v1/me");
      await other.evaluate(() => localStorage.setItem("iot-pillot.auth", JSON.stringify({ user_id: 8, access_token: "new-account", refresh_token: "new-refresh" })));
      await reloadProfile;
      await page.locator(".user-menu__name").filter({ hasText: "测试姓名" }).waitFor({ state: "attached" });
      await page.goto(base + "/user/home");
      await other.evaluate(() => localStorage.clear());
      await assertGuest(page);
      assert.deepEqual(errors, []);
      console.log("PASS reactive cross-tab sign-in/sign-out with full motion");
      await context.close();
    }
    const { context, page, errors } = await fixture(browser, { signedIn: true, rejectedPath: "/api/v1/me", networkFailure: true });
    await page.goto(base + "/user/home");
    await page.getByRole("alert").waitFor();
    assert.notEqual(await page.evaluate(() => localStorage.getItem("iot-pillot.auth")), null);
    assert.equal(await page.locator(".guest-journey").count(), 0);
    assert.deepEqual(errors, []);
    console.log("PASS transient network failure preserves session");
    await context.close();
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });
