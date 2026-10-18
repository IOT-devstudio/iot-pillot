// 报名资料入口四态（issue #152）。所有 /api/v1 流量都被拦截，只打 fixture，不写生产数据。
// 运行：先起 Vite（默认 5186），再 PLAYWRIGHT_MODULE=<playwright 路径> node apps/web/tests/profile-entry.browser.cjs
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const assert = require("node:assert/strict");
const base = process.env.TEST_BASE_URL || "http://127.0.0.1:5186";
const session = { user_id: 7, access_token: "test-only", refresh_token: "test-only" };
const directions = [
  {
    id: 1,
    code: "front-end",
    name: "前端开发",
    open: true,
    sort_order: 0,
    form: { id: 1, share_token: "test-form", is_open: true },
  },
];
const application = {
  id: 42,
  user_id: 7,
  round_id: 1,
  direction_code: "front-end",
  status: "applied",
  name: "测试姓名",
  created_at: "2026-10-16T00:00:00Z",
  updated_at: "2026-10-16T00:00:00Z",
};

function profileWith(detail, extra = {}) {
  return {
    user_id: 7,
    username: "fixture",
    role: "member",
    name: "测试姓名",
    detail: { qq: "12345", email: "test@example.com", direction: "front-end", ...detail },
    ...extra,
  };
}

async function fixture(
  browser,
  { width = 1440, profile = profileWith({ class: "物联2301", student_id: 2023114514 }), meFails = false } = {},
) {
  const context = await browser.newContext({
    viewport: { width, height: 900 },
    reducedMotion: "reduce",
  });
  await context.addInitScript(s => localStorage.setItem("iot-pillot.auth", JSON.stringify(s)), session);
  const page = await context.newPage();
  const state = { profile, posts: [], puts: [], errors: [] };
  page.on("pageerror", error => state.errors.push(error.message));

  await context.route("**/api/v1/**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const ok = data =>
      route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({ code: 0, message: "ok", data }),
      });

    if (path === "/api/v1/recruitment/directions") return ok({ items: directions });
    if (path === "/api/v1/me") {
      if (meFails) {
        return route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({ code: 5000, message: "资料暂时不可用" }),
        });
      }
      if (request.method() === "PUT") {
        const patch = request.postDataJSON();
        state.puts.push(patch);
        state.profile = { ...state.profile, detail: { ...state.profile.detail, ...patch } };
      }
      return ok(state.profile);
    }
    if (path === "/api/v1/recruitment/applications") {
      if (request.method() === "POST") {
        state.posts.push(request.postDataJSON());
        return ok(application);
      }
      return ok({
        items: state.posts.length ? [application] : [],
        total: state.posts.length,
        page: 1,
        page_size: 20,
      });
    }
    state.errors.push("Unexpected API: " + request.method() + " " + path);
    return route.abort();
  });

  return { context, page, state };
}

async function openDialog(page) {
  await page.locator(".direction-panel").first().waitFor();
  await page.locator(".direction-panel .panel-button").first().click();
  await page.locator("dialog[open]").waitFor();
}

const profileRow = page => page.locator("dialog[open] .direction-facts");
// 报名成功不会关弹窗，只在弹窗里出提示（handleApply 的既有行为）。
const waitForSubmitSuccess = page =>
  page.getByText("报名成功，可在「我的进度」查看审核进展。").waitFor();

(async () => {
  const browser = await chromium.launch({ channel: "msedge", headless: true });
  try {
    // 1. 资料完整：入口是「查看资料」，报名直接提交。
    {
      const { context, page, state } = await fixture(browser);
      await page.goto(base + "/user/home");
      await openDialog(page);
      const text = await profileRow(page).innerText();
      assert.match(text, /资料/);
      assert.match(text, /查看资料/);
      assert.doesNotMatch(text, /完善资料/);
      await page.locator("dialog[open] .action-button").filter({ hasText: "报名" }).click();
      await waitForSubmitSuccess(page);
      assert.equal(state.posts.length, 1);
      assert.deepEqual(state.errors, []);
      console.log("PASS complete profile shows 查看资料 and submits");
      await context.close();
    }

    // 2. 缺学号：两个报名动作都拦、都不发 POST；落地页提示 + 自动进编辑态；保存后回到「查看资料」。
    {
      const { context, page, state } = await fixture(browser, {
        profile: profileWith({ class: "物联2301", student_id: 0 }),
      });
      await page.goto(base + "/user/home");
      await openDialog(page);
      assert.match(await profileRow(page).innerText(), /完善资料/);

      await page.locator("dialog[open] .action-button").filter({ hasText: "报名" }).click();
      await page.waitForURL(/\/user\/settings\?from=apply/);
      assert.equal(state.posts.length, 0, "拦截时不得提交报名");

      const hint = page.locator(".apply-hint");
      await hint.waitFor();
      assert.match(await hint.innerText(), /报名前请先补全/);
      assert.match(await hint.innerText(), /学号/);
      assert.doesNotMatch(await hint.innerText(), /班级/, "只提示缺的那一项");
      // 自动进编辑态：表单输入框已渲染
      await page.locator('[placeholder="如：物联网 2301"]').waitFor();

      await page.locator('[placeholder="如：2023114514"]').fill("2023114514");
      await page.getByRole("button", { name: "保存" }).click();
      await page.waitForFunction(() => !document.querySelector(".apply-hint"));
      assert.equal(state.puts.length, 1);
      assert.equal(state.puts[0].student_id, 2023114514);

      await page.goBack();
      await page.locator(".direction-panel").first().waitFor();
      await openDialog(page);
      assert.match(await profileRow(page).innerText(), /查看资料/);
      await page.locator("dialog[open] .action-button").filter({ hasText: "报名" }).click();
      await waitForSubmitSuccess(page);
      assert.equal(state.posts.length, 1);
      assert.deepEqual(state.errors, []);
      console.log("PASS incomplete profile blocks both paths, saves, then shows 查看资料");
      await context.close();
    }

    // 3. 面试路径同样被拦（同一条申请、同一份快照）。
    {
      const { context, page, state } = await fixture(browser, {
        profile: profileWith({ class: "", student_id: 0 }),
      });
      await page.goto(base + "/user/home");
      await openDialog(page);
      await page.locator("dialog[open] .action-button").filter({ hasText: "面试" }).click();
      await page.waitForURL(/\/user\/settings\?from=apply/);
      assert.equal(state.posts.length, 0);
      assert.doesNotMatch(page.url(), /\/f\//);
      const hint = await page.locator(".apply-hint").innerText();
      assert.match(hint, /班级/);
      assert.match(hint, /学号/);
      assert.deepEqual(state.errors, []);
      console.log("PASS interview path is gated by the same rule");
      await context.close();
    }

    // 4. 创始管理员：不渲染入口，也不拦报名。
    {
      const { context, page, state } = await fixture(browser, {
        profile: profileWith({ class: "", student_id: 0 }, { is_founding: true }),
      });
      await page.goto(base + "/user/home");
      await openDialog(page);
      const row = await profileRow(page).innerText();
      assert.match(row, /创始账号/);
      assert.doesNotMatch(row, /查看资料|完善资料|个人资料/);
      await page.locator("dialog[open] .action-button").filter({ hasText: "报名" }).click();
      await waitForSubmitSuccess(page);
      assert.equal(state.posts.length, 1, "创始管理员豁免");
      assert.deepEqual(state.errors, []);
      console.log("PASS founding admin hides the entry and stays exempt");
      await context.close();
    }

    // 5. 资料读取失败：中性文案，fail-open 不拦报名。
    {
      const { context, page, state } = await fixture(browser, { meFails: true });
      await page.goto(base + "/user/home");
      await openDialog(page);
      const row = await profileRow(page).innerText();
      assert.match(row, /个人资料/);
      assert.doesNotMatch(row, /查看资料|完善资料/);
      await page.locator("dialog[open] .action-button").filter({ hasText: "报名" }).click();
      await waitForSubmitSuccess(page);
      assert.equal(state.posts.length, 1, "资料未知时 fail-open");
      console.log("PASS unknown profile fails open with neutral wording");
      await context.close();
    }

    // 6. 手机端 390：提示可读、无横向溢出。
    {
      const { context, page, state } = await fixture(browser, {
        width: 390,
        profile: profileWith({ class: "物联2301", student_id: 0 }),
      });
      await page.goto(base + "/user/home");
      await openDialog(page);
      await page.locator("dialog[open] .action-button").filter({ hasText: "报名" }).click();
      await page.waitForURL(/\/user\/settings\?from=apply/);
      await page.locator(".apply-hint").waitFor();
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth),
        false,
        "手机端出现横向溢出",
      );
      assert.deepEqual(state.errors, []);
      console.log("PASS 390px landing page has no horizontal overflow");
      await context.close();
    }
  } finally {
    await browser.close();
  }
})().catch(error => {
  console.error(error);
  process.exit(1);
});
