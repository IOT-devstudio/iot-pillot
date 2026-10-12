/* Run against a local Vite server. API responses are intercepted; no account,
 * email, profile or recruitment writes reach the real backend. */
const { chromium } = require("playwright");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const origin = process.env.SIGNAL_ORIGIN || "http://127.0.0.1:5173";
const output = path.resolve(process.env.SIGNAL_OUTPUT || "outputs/issue-81");
fs.mkdirSync(output, { recursive: true });

async function recordReview(browser, profile, report) {
  for (const mobile of [false, true]) {
    const viewport = mobile
      ? { width: 390, height: 844 }
      : { width: 1440, height: 1000 };
    const ctx = await browser.newContext({
      viewport,
      isMobile: mobile,
      hasTouch: mobile,
      recordVideo: { dir: path.join(output, "clips"), size: viewport },
    });
    await ctx.route("**/api/v1/**", (route) => {
      const isMe = new URL(route.request().url()).pathname.endsWith("/me");
      return route.fulfill({
        json: {
          code: 0,
          message: "ok",
          data: isMe
            ? {
                ...profile,
                name: "接口测试用户",
                role: "member",
                detail: { class: "计算机 2026", email: "signal@example.test" },
              }
            : {
                user_id: 81,
                access_token: "intercepted-access",
                refresh_token: "intercepted-refresh",
              },
        },
      });
    });
    const page = await ctx.newPage();
    page.on("pageerror", (e) => report.errors.push(e.message));
    await page.goto(origin);
    await page.waitForTimeout(1100);
    await page.locator("#register-tab").click();
    await page.waitForTimeout(850);
    await page.locator("#login-tab").click();
    await page.waitForTimeout(650);
    await page.locator("#username").fill("signal-user");
    await page.locator("#login-password").fill("secret12");
    await page.locator("#login-pane button[type=submit]").click();
    await page.waitForURL("**/user/home");
    await page.waitForTimeout(1300);
    await page.locator(".direction-rail").scrollIntoViewIfNeeded();
    if (mobile) {
      const cdp = await ctx.newCDPSession(page),
        rect = await page.locator(".direction-rail").boundingBox();
      const y = rect.y + Math.min(rect.height / 2, 180),
        from = rect.x + rect.width - 30,
        to = rect.x + 20;
      await cdp.send("Input.dispatchTouchEvent", {
        type: "touchStart",
        touchPoints: [{ x: from, y }],
      });
      for (let i = 1; i <= 8; i++) {
        await cdp.send("Input.dispatchTouchEvent", {
          type: "touchMove",
          touchPoints: [{ x: from + ((to - from) * i) / 8, y }],
        });
        await page.waitForTimeout(30);
      }
      await cdp.send("Input.dispatchTouchEvent", {
        type: "touchEnd",
        touchPoints: [],
      });
      await page.waitForTimeout(850);
      assert(
        (await page
          .locator(".direction-rail")
          .evaluate((el) => el.scrollLeft)) > 100,
      );
      assert(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      );
      report.checks.push(
        "native touch swipe moves only the mobile direction rail",
      );
    } else {
      await page
        .getByRole("button", { name: "下一个方向", exact: true })
        .click();
      await page.waitForTimeout(750);
    }
    await page.locator(".direction-panel").nth(1).getByRole("button").click();
    await page.waitForTimeout(650);
    const shot = mobile
      ? "direction-dialog-mobile.png"
      : "direction-dialog-viewport.png";
    await page.screenshot({ path: path.join(output, shot) });
    report.screenshots.push(shot);
    const rect = await page.locator("dialog").boundingBox();
    assert(rect.width <= viewport.width);
    await page.keyboard.press("Escape");
    await page.waitForTimeout(250);
    await page.locator(".journey-summary .plain-link").first().click();
    await page.waitForTimeout(650);
    await page.mouse.click(5, 5);
    await page.waitForTimeout(250);
    assert.equal(await page.locator("dialog[open]").count(), 0);
    report.checks.push(
      "dialog backdrop click closes and mobile dialog fits viewport",
    );
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.locator(".journey-summary .plain-link").first().click();
    await page.waitForTimeout(450);
    await page.screenshot({
      path: path.join(
        output,
        mobile ? "reduced-dialog-mobile.png" : "reduced-dialog-desktop.png",
      ),
    });
    await page.keyboard.press("Escape");
    await page.waitForTimeout(150);
    const video = page.video();
    await ctx.close();
    await video.saveAs(
      path.join(output, mobile ? "review-mobile.webm" : "review-desktop.webm"),
    );
  }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
(async () => {
  const browser = await chromium.launch({
    headless: true,
    channel: process.env.BROWSER_CHANNEL || "msedge",
  });
  const report = {
    backend: "intercepted fixtures",
    checks: [],
    screenshots: [],
    errors: [],
  };
  let context;
  try {
    context = await browser.newContext({
      viewport: { width: 1440, height: 1000 },
      recordVideo: {
        dir: path.join(output, "video"),
        size: { width: 1440, height: 1000 },
      },
    });
    let role = "member",
      expire = false,
      failProfile = false;
    let profile = {
      user_id: 81,
      username: "signal-user",
      name: "接口测试用户",
      role,
      detail: {
        class: "计算机 2026",
        student_id: 20260081,
        qq: "123456789",
        direction: "front-end",
        email: "signal@example.test",
      },
    };
    const calls = [];
    await context.route("**/api/v1/**", async (route) => {
      const request = route.request(),
        pathname = new URL(request.url()).pathname,
        method = request.method();
      const payload = request.postDataJSON();
      calls.push({ pathname, method, payload });
      if (expire && (pathname.endsWith("/me") || pathname.endsWith("/refresh")))
        return route.fulfill({
          status: 401,
          json: { code: 1001, message: "会话已失效" },
        });
      if (
        failProfile &&
        pathname.endsWith("/me") &&
        calls.filter((c) => c.pathname.endsWith("/me")).length > 1
      )
        return route.fulfill({
          status: 500,
          json: { code: 1, message: "资料加载失败" },
        });
      if (pathname.endsWith("/me")) {
        if (method === "PUT") {
          assert(!("name" in payload));
          assert(!("email" in payload));
          assert(!("user_id" in payload));
          profile.detail = { ...profile.detail, ...payload };
        }
        return route.fulfill({
          json: { code: 0, message: "ok", data: { ...profile, role } },
        });
      }
      if (pathname.endsWith("/login") || pathname.endsWith("/register")) {
        await sleep(450);
        return route.fulfill({
          json: {
            code: 0,
            message: "ok",
            data: {
              user_id: 81,
              access_token: "intercepted-access",
              refresh_token: "intercepted-refresh",
            },
          },
        });
      }
      if (
        pathname.endsWith("/send-verify-code") ||
        pathname.endsWith("/logout")
      )
        return route.fulfill({ json: { code: 0, message: "ok" } });
      return route.fulfill({
        json: {
          code: 0,
          message: "ok",
          data: { items: [], total: 0, page: 1, page_size: 20 },
        },
      });
    });
    await context.route("**/health", (route) =>
      route.fulfill({ json: { status: "ok" } }),
    );
    const page = await context.newPage();
    page.on("pageerror", (e) => report.errors.push(e.message));
    const check = (name) => {
      report.checks.push(name);
      console.log("PASS", name);
    };
    const shot = async (name) => {
      await page.screenshot({
        path: path.join(output, name + ".png"),
        fullPage: true,
      });
      report.screenshots.push(name + ".png");
    };
    const overflow = async () => {
      const d = await page.evaluate(() => ({
        width: innerWidth,
        scroll: document.documentElement.scrollWidth,
      }));
      assert(d.scroll <= d.width, JSON.stringify(d));
    };
    const settle = async () => {
      await page.waitForTimeout(1250);
    };
    const login = async () => {
      await page.locator("#username").fill("signal-user");
      await page.locator("#login-password").fill("secret12");
      await page.locator("#login-pane button[type=submit]").click();
      await page.waitForURL("**/user/home");
      await settle();
    };
    await page.goto(origin);
    await settle();
    const scanner = page.locator(".auth-poster .signal-scanner");
    const scannerBefore = await scanner.evaluate((el) => {
      const c = getComputedStyle(el);
      return {
        translate: c.translate,
        background: c.backgroundImage,
        position: c.position,
        height: el.getBoundingClientRect().height,
        animation: c.animationName,
      };
    });
    assert(scannerBefore.background.includes("115deg"));
    assert.equal(scannerBefore.position, "absolute");
    assert(scannerBefore.height > 0);
    assert.equal(scannerBefore.animation, "swiss-signal-scan");
    await page.waitForTimeout(180);
    assert.notEqual(
      await scanner.evaluate((el) => getComputedStyle(el).translate),
      scannerBefore.translate,
    );
    await page.getByRole("button", { name: "暂停动效 ⏸", exact: true }).click();
    assert.equal(
      await scanner.evaluate((el) => getComputedStyle(el).animationPlayState),
      "paused",
    );
    await page.getByRole("button", { name: "恢复动效 ▶", exact: true }).click();
    check("diagonal halo uses reference gradient, moves, pauses and resumes");
    await shot("login-desktop");
    const reference = await context.newPage();
    await reference.goto(origin + "/swiss-signal.html");
    await reference.waitForTimeout(1500);
    await reference.screenshot({
      path: path.join(output, "reference-login-desktop.png"),
      fullPage: true,
    });
    await reference.close();
    for (const width of [320, 390, 768, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      await overflow();
      await page.locator("#register-tab").click();
      await settle();
      await overflow();
      assert.equal(await page.locator("#login-pane").getAttribute("inert"), "");
      if (width === 390 || width === 1440) await shot("register-" + width);
      await page.locator("#login-tab").click();
      await settle();
      if (width === 390) await shot("login-mobile");
    }
    check("login/register have no document overflow at 320/390/768/1440");
    await page.locator("#login-pane button[type=submit]").click();
    await page.getByText("请输入用户名", { exact: true }).waitFor();
    assert.equal(calls.filter((c) => c.pathname.endsWith("/login")).length, 0);
    check("empty login cannot call API");
    await page.locator("#username").fill("signal-user");
    await page.locator("#login-password").fill("secret12");
    await page
      .getByRole("button", { name: "显示密码", exact: true })
      .first()
      .click();
    assert.equal(
      await page.locator("#login-password").getAttribute("type"),
      "text",
    );
    await page
      .getByRole("button", { name: "隐藏密码", exact: true })
      .first()
      .click();
    for (let n = 0; n < 9; n++)
      await page.locator(n % 2 ? "#login-tab" : "#register-tab").click();
    await page.locator("#login-tab").click();
    assert.equal(await page.locator("#username").inputValue(), "signal-user");
    assert.equal(
      await page.locator("#register-pane").getAttribute("inert"),
      "",
    );
    check(
      "rapid switching preserves forms; inactive pane is inert; password toggle works",
    );
    await page.locator("#register-tab").click();
    await page.locator("#email").fill("signal@example.test");
    await page.getByRole("button", { name: "获取验证码", exact: true }).click();
    await page.getByRole("button", { name: /后重发/ }).waitFor();
    assert.equal(
      calls.filter((c) => c.pathname.endsWith("send-verify-code")).length,
      1,
    );
    check("verification code calls API and enters cooldown");
    await page.locator("#name").fill("接口测试用户");
    await page.locator("#register-password").fill("secret12");
    await page.locator("#confirm-password").fill("secret12");
    await page.locator("#code").fill("123456");
    await page.locator("#register-pane button[type=submit]").click();
    await page.locator("#register-pane form").evaluate((el) => {
      el.dispatchEvent(
        new Event("submit", { bubbles: true, cancelable: true }),
      );
      el.dispatchEvent(
        new Event("submit", { bubbles: true, cancelable: true }),
      );
    });
    await page.waitForURL("**/user/home");
    await settle();
    assert.equal(
      calls.filter((c) => c.pathname.endsWith("/register")).length,
      1,
    );
    check(
      "registration saves session, routes by role and blocks repeated submit",
    );
    assert(
      (await page.locator(".home-poster-top").innerText()).includes(
        "接口测试用户",
      ),
    );
    assert.equal(
      await page
        .locator(".profile-readout")
        .getByText("计算机 2026", { exact: true })
        .count(),
      1,
    );
    check("home displays API profile rather than generated identity");
    for (const width of [320, 390, 768, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      await overflow();
      if (width === 390 || width === 1440) await shot("home-" + width);
    }
    check("home has no document overflow at all four widths");
    await page.setViewportSize({ width: 390, height: 900 });
    await page
      .getByRole("button", { name: "切换演示申请记录", exact: true })
      .click();
    assert.equal(await page.locator(".application-picker button").count(), 3);
    await page.locator(".application-picker button").first().click();
    await page.setViewportSize({ width: 1440, height: 1000 });
    check(
      "mobile status control exposes all fixture records without changing chapter layout",
    );
    await page.locator(".direction-rail").focus();
    await page.keyboard.press("End");
    await page.waitForTimeout(750);
    assert(
      (await page.locator(".rail-controls").innerText()).includes("03 / 03"),
    );
    await page.keyboard.press("Home");
    await page.waitForTimeout(750);
    await page.getByRole("button", { name: "下一个方向", exact: true }).click();
    await page.waitForTimeout(750);
    assert(
      (await page.locator(".rail-controls").innerText()).includes("02 / 03"),
    );
    check("rail keyboard and arrow controls select matching direction");
    const trigger = page.locator(".direction-panel").nth(1).getByRole("button");
    await trigger.click();
    await page.locator("dialog[open]").waitFor();
    await page.waitForTimeout(650);
    await shot("direction-dialog");
    const before = calls.length;
    await page.getByRole("button", { name: /^体验报名意向/ }).click();
    await page
      .getByText("已选择本次体验方向，未提交到后端。", { exact: true })
      .waitFor();
    assert.equal(calls.length, before);
    await page.keyboard.press("Escape");
    await page.waitForTimeout(350);
    assert.equal(await page.locator("dialog[open]").count(), 0);
    assert(await trigger.evaluate((el) => document.activeElement === el));
    check(
      "demo intent makes no request; Escape closes dialog and returns focus",
    );
    await page.locator(".journey-summary .plain-link").first().click();
    await page.waitForTimeout(650);
    await shot("application-dialog");
    await page.getByRole("button", { name: "关闭详情", exact: true }).click();
    await page.waitForTimeout(350);
    check("application detail opens and closes");
    await page
      .getByRole("button", { name: "查看全部 3 条演示记录 ↗", exact: true })
      .click();
    await page.locator("dialog[open]").waitFor();
    assert.equal(await page.locator(".application-picker button").count(), 3);
    await page.locator(".application-picker button").nth(2).click();
    await page.waitForTimeout(200);
    assert(
      (await page.locator(".journey-summary").innerText()).includes("Agent"),
    );
    check(
      "all fixture applications remain reachable in the single-sheet chapter",
    );
    await page.getByRole("link", { name: "个人资料", exact: true }).click();
    await page.waitForURL("**/user/settings");
    await page.locator(".settings").waitFor();
    assert.equal(
      await page
        .locator(".settings__readonly")
        .getByText("接口测试用户", { exact: true })
        .count(),
      1,
    );
    assert.equal(await page.locator(".settings__readonly input").count(), 0);
    await shot("settings-desktop");
    for (const width of [320, 390, 768, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      await overflow();
      if (width === 390) await shot("settings-mobile");
    }
    check("settings readonly name/email and all four responsive widths");
    await page.getByLabel("班级", { exact: true }).fill("真实契约测试班");
    await page.getByRole("button", { name: "保存", exact: true }).click();
    await page.waitForTimeout(500);
    const put = calls.find(
      (c) => c.pathname.endsWith("/me") && c.method === "PUT",
    );
    assert.equal(put.payload.class, "真实契约测试班");
    check("settings uses PUT /me with only editable fields");
    await page.goto(origin + "/user/home");
    await settle();
    await page.getByRole("button", { name: "signal-user 的菜单" }).click();
    await page.getByRole("menuitem", { name: "退出登录", exact: true }).click();
    await page.waitForURL(origin + "/");
    assert.equal(
      await page.evaluate(() => localStorage.getItem("iot-pillot.auth")),
      null,
    );
    check("logout requests API, clears session and returns to login");
    await page.goto(origin + "/user/settings");
    await page.waitForURL(
      (url) => url.pathname === "/" && url.searchParams.has("redirect"),
    );
    assert(
      new URL(page.url()).searchParams.get("redirect") === "/user/settings",
    );
    await page.locator("#username").fill("signal-user");
    await page.locator("#login-password").fill("secret12");
    await page.locator("#login-pane button[type=submit]").click();
    await page.waitForURL("**/user/settings");
    check("protected route redirect preserved after login");
    await page.goto(origin + "/login");
    await settle();
    await login();
    expire = true;
    await page.goto(origin + "/user/home");
    await page.waitForURL((url) => url.pathname === "/");
    await page.waitForTimeout(800);
    await page.waitForLoadState("networkidle");
    await page.locator("#username").waitFor();
    assert.equal(
      await page.evaluate(() => localStorage.getItem("iot-pillot.auth")),
      null,
    );
    expire = false;
    check("expired session clears tokens and sends user to auth");
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(origin);
    await page.locator("#register-tab").click();
    await page.locator("#login-tab").click();
    await login();
    assert.equal(await page.locator(".page-wipe").count(), 0);
    await shot("reduced-motion-home");
    check("reduced motion renders visible final state without residual mask");
    await page.emulateMedia({ reducedMotion: "no-preference" });
    await page.goto(origin + "/login");
    await page.getByRole("button", { name: "暂停动效 ⏸", exact: true }).click();
    await login();
    assert.equal(await page.locator(".page-wipe").count(), 0);
    await page.getByRole("button", { name: "恢复动效 ▶", exact: true }).click();
    check("pause/resume survives auth to home");
    await page.goto(origin + "/login");
    await page.locator("#username").fill("signal-user");
    await page.locator("#login-password").fill("secret12");
    await page.locator("#login-pane button[type=submit]").click();
    await page.locator(".page-wipe").waitFor();
    await page.getByRole("button", { name: "暂停动效 ⏸", exact: true }).click();
    await page.waitForURL("**/user/home");
    await page.waitForTimeout(500);
    assert.equal(await page.locator(".page-wipe").count(), 0);
    check("pausing during mask transition releases navigation");
    // Visibility events exercise suspension without depending on a headed OS window.
    await page.getByRole("button", { name: "恢复动效 ▶", exact: true }).click();
    await page.evaluate(() => {
      Object.defineProperty(document, "hidden", {
        configurable: true,
        value: true,
      });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    assert(
      await page
        .locator(".swiss-signal")
        .evaluate((el) => el.classList.contains("motion-suspended")),
    );
    assert.equal(
      await page.evaluate(
        () =>
          document.getAnimations().filter((a) => a.playState === "running")
            .length,
      ),
      0,
    );
    await page.evaluate(() => {
      delete document.hidden;
      document.dispatchEvent(new Event("visibilitychange"));
    });
    check(
      "visibility suspension stops active animations and resumes safely (event simulation)",
    );
    await page.getByRole("link", { name: "关于我们 ↗", exact: true }).click();
    await page.waitForURL("**/user/about");
    assert.equal(await page.locator(".swiss-signal").count(), 0);
    await page.goBack();
    await page.waitForURL("**/user/home");
    await settle();
    assert.equal(await page.locator(".swiss-signal").count(), 1);
    check("SPA leave/reenter mounts one shell without a residual overlay");
    // Fail only the view's profile read; the role gate still succeeds.
    let reads = 0;
    await page.route("**/api/v1/me", async (route) => {
      reads++;
      if (reads === 2 || reads === 3)
        return route.fulfill({
          status: 500,
          json: { code: 1, message: "资料加载失败" },
        });
      return route.fulfill({
        json: { code: 0, message: "ok", data: { ...profile, role } },
      });
    });
    await page.goto(origin + "/user/home");
    await page.locator('.profile-readout [role="alert"]').waitFor();
    assert(
      (await page.locator(".profile-readout").innerText()).includes(
        "资料加载失败",
      ),
    );
    await page
      .locator(".profile-readout")
      .getByRole("button", { name: "重试 ↗" })
      .click();
    await page
      .locator(".profile-readout")
      .getByText("真实契约测试班", { exact: true })
      .waitFor();
    await page.unroute("**/api/v1/me");
    check("profile errors retain retry; no generated profile is substituted");
    profile = { ...profile, name: undefined, detail: undefined };
    await page.goto(origin + "/user/home");
    await settle();
    assert(
      (await page.locator(".profile-readout").innerText()).includes("未填写"),
    );
    assert(
      (await page.locator(".home-poster-top").innerText()).includes(
        "signal-user",
      ),
    );
    check("empty API details render honest empty fields");
    role = "admin";
    await page.goto(origin + "/login");
    await page.locator("#username").fill("signal-user");
    await page.locator("#login-password").fill("secret12");
    await page.locator("#login-pane button[type=submit]").click();
    await page.waitForURL("**/dashboard");
    assert.equal(await page.locator(".swiss-signal").count(), 0);
    check("admin lands in existing dashboard without Swiss shell styles");
    role = "member";
    await page.goto(origin + "/login");
    await page.route("**/api/v1/login", async (route) => {
      await sleep(17000);
      await route.abort().catch(() => {});
    });
    await page.locator("#username").fill("signal-user");
    await page.locator("#login-password").fill("secret12");
    await page.locator("#login-pane button[type=submit]").click();
    await page
      .getByText("请求超时，请检查网络后重试", { exact: true })
      .waitFor({ timeout: 18000 });
    assert.equal(
      await page.locator("#login-pane button[type=submit]").isDisabled(),
      false,
    );
    await page.unroute("**/api/v1/login");
    check("stalled login aborts in 15 seconds and unlocks submit");
    assert.equal(report.errors.length, 0, report.errors.join("\n"));
    check("no uncaught browser errors");
    fs.writeFileSync(
      path.join(output, "browser-report.json"),
      JSON.stringify(report, null, 2),
    );
    const video = page.video();
    await context.close();
    await video.saveAs(path.join(output, "verification-full.webm"));
    context = null;
    await recordReview(browser, profile, report);
    fs.writeFileSync(
      path.join(output, "browser-report.json"),
      JSON.stringify(report, null, 2),
    );
  } finally {
    if (context) await context.close();
    await browser.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
