// All API traffic is mocked; this test never submits production data.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const assert = require("node:assert/strict");
const base = process.env.TEST_BASE_URL || "http://127.0.0.1:5189";

(async () => {
  const browser = await chromium.launch({ channel: "msedge", headless: true });
  try {
    for (const role of ["admin", "member"]) {
      for (const redirect of ["/user/home?source=guest#directions", "/user/about"]) {
        const context = await browser.newContext({ reducedMotion: "reduce" });
        const page = await context.newPage();
        const errors = [];
        page.on("pageerror", error => errors.push(error.message));
        await context.route("**/api/v1/**", route => {
          const path = new URL(route.request().url()).pathname;
          const session = { user_id: 9, access_token: "fixture", refresh_token: "fixture" };
          let data = { items: [], total: 0, page: 1, page_size: 20 };
          if (path === "/api/v1/login") data = session;
          if (path === "/api/v1/me") data = {
            user_id: 9, username: "fixture", name: "测试姓名", role,
            detail: { class: "测试班级", student_id: 123, qq: "12345", email: "fixture@example.com" },
          };
          return route.fulfill({ contentType: "application/json", body: JSON.stringify({ code: 0, data }) });
        });
        await page.goto(base + "/?redirect=" + encodeURIComponent(redirect));
        await page.locator("#username").fill("fixture");
        await page.locator("#login-password").fill("Password123!");
        await page.locator("#login-pane button[type=submit]").click();
        const expected = role === "admin" ? "/dashboard" : redirect;
        await page.waitForURL(url => url.pathname + url.search + url.hash === expected);
        // A fresh About navigation must load its real module and matching exports.
        await page.goto(base + "/user/about");
        await page.locator(".about__heading").waitFor();
        assert.deepEqual(errors, [], `${role}: ${redirect}`);
        console.log(`PASS ${role}: ${redirect} -> ${expected}; About loads without page errors`);
        await context.close();
      }
    }
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
