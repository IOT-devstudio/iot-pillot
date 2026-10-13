// Playwright browser_run_code: evaluate this function with a browser page.
// Start Vite at http://127.0.0.1:5174 before running.
// Isolated browser contexts and API fixtures prevent real account requests.
async (page) => {
  const browser = page.context().browser();
  const results = [];
  for (const scenario of [
    { name: "desktop", width: 1440, height: 900 },
    { name: "mobile", width: 390, height: 844 },
    { name: "reduced", width: 390, height: 844, reduced: true },
    { name: "interrupted", width: 1440, height: 900, interrupt: true },
    { name: "slow-profile", width: 1440, height: 900, slow: true },
    { name: "admin", width: 1440, height: 900, admin: true },
  ]) {
    const isolated = await browser.newContext();
    try {
      page = await isolated.newPage();
      await page.setViewportSize({ width: scenario.width, height: scenario.height });
      await page.emulateMedia({ reducedMotion: scenario.reduced ? "reduce" : "no-preference" });
      await page.context().clearCookies();
      if (page.url() !== "about:blank") await page.evaluate(() => localStorage.clear());
      await page.addInitScript(({ slow, admin }) => {
        const original = window.fetch;
        window.fetch = async (input, init) => {
          const url = String(input instanceof Request ? input.url : input);
          if (!url.includes("/api/v1/")) return original(input, init);
          const login = url.endsWith("/login");
          if (!login && slow) await new Promise(resolve => setTimeout(resolve, 700));
          return new Response(JSON.stringify({ code: 0, message: "ok", data: login
            ? { user_id: 81, access_token: "fixture-access", refresh_token: "fixture-refresh" }
            : { user_id: 81, username: "signal-user", role: admin ? "admin" : "member", detail: {} } }),
            { headers: { "Content-Type": "application/json" } });
        };
      }, scenario);
      await page.goto("http://127.0.0.1:5174/login");
      await page.locator("#username").fill("signal-user");
      await page.locator("#login-password").fill("secret12");
      await page.evaluate(() => {
        window.__wipeCheck = { samples: [], done: false };
        const start = performance.now();
        let sawWipe = false;
        function sample() {
          const wipe = document.querySelector(".page-wipe");
          const ms = performance.now() - start;
          if (wipe) {
            sawWipe = true;
            const strips = [...wipe.children].filter(el => el.matches("i, .wipe-panel"));
            const offsets = strips.map(el => el.getBoundingClientRect().top);
            window.__wipeCheck.samples.push({ ms, offsets, overflow: document.documentElement.scrollWidth > innerWidth });
          } else if (sawWipe || (ms > 200 && location.pathname !== "/login")) {
            window.__wipeCheck.done = true;
            window.__wipeCheck.duration = ms;
            return;
          }
          if (ms > 4000) { window.__wipeCheck.done = true; return; }
          requestAnimationFrame(sample);
        }
        requestAnimationFrame(sample);
      });
      await page.locator("#login-pane button[type=submit]").click();
      if (scenario.interrupt) {
        await page.locator(".page-wipe").waitFor({ state: "attached" });
        await page.locator(".footer button").click();
      }
      await page.waitForURL(scenario.admin ? "**/dashboard" : "**/user/home");
      await page.waitForFunction(() => window.__wipeCheck.done);
      const check = await page.evaluate(() => window.__wipeCheck);
      if (await page.locator(".page-wipe").count()) throw new Error(scenario.name + ": overlay remains");
      if (check.samples.some(s => s.overflow)) throw new Error(scenario.name + ": horizontal overflow");
      if (!scenario.slow && check.duration > 1500) throw new Error(scenario.name + ": transition took " + Math.round(check.duration) + "ms (budget 1500ms including navigation)");
      if (!scenario.slow && !scenario.reduced && !scenario.interrupt && check.duration < 1050)
        throw new Error(scenario.name + ": transition ended before the requested 1100ms animation");
      if (!scenario.reduced && !scenario.interrupt &&
          !check.samples.some(({ offsets: [left, center, right] }) =>
            left > 50 && center < -50 && right > 50))
        throw new Error(scenario.name + ": blue panels must enter from below and yellow from above");
      for (let panel = 0; panel < 3; panel++) {
        let covered = false, departed = false;
        for (const sample of check.samples) {
          const distance = Math.abs(sample.offsets[panel]);
          if (distance < 2) covered = true;
          if (covered && distance > scenario.height * 0.8) departed = true;
          if (departed && distance < scenario.height * 0.2)
            throw new Error(scenario.name + ": outgoing panel flashed back into viewport");
        }
      }
      if (scenario.reduced && check.samples.length) throw new Error("reduced motion still shows wipe");
      results.push({ scenario: scenario.name, durationMs: Math.round(check.duration || 0), samples: check.samples.length });
    } finally {
      await isolated.close();
    }
  }
  return results;
}
