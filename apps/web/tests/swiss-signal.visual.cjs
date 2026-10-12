const { chromium } = require("playwright");
const assert = require("node:assert/strict");
const origin = process.env.SIGNAL_ORIGIN || "http://127.0.0.1:5173";
const output = process.env.SIGNAL_OUTPUT || "outputs/issue-81";
const fs = require("fs"),
  path = require("path");
fs.mkdirSync(output, { recursive: true });
(async () => {
  const browser = await chromium.launch({ headless: true, channel: "msedge" });
  const context = await browser.newContext({ reducedMotion: "reduce" });
  await context.addInitScript(() =>
    localStorage.setItem(
      "iot-pillot.auth",
      JSON.stringify({
        user_id: 81,
        access_token: "style-fixture",
        refresh_token: "style-fixture",
      }),
    ),
  );
  await context.route("**/api/v1/**", (r) =>
    r.fulfill({
      json: {
        code: 0,
        message: "ok",
        data: {
          user_id: 81,
          username: "探索者",
          name: "探索者",
          role: "member",
          detail: { class: "计算机 2026" },
        },
      },
    }),
  );
  const reference = await context.newPage(),
    current = await context.newPage(),
    results = [];
  const selectors = [
    ".masthead",
    ".auth-view",
    ".auth-poster",
    ".auth-space",
    ".form-tabs",
    ".auth-pane h2",
    ".home-poster",
    ".space-stats",
    ".direction-section",
    ".direction-panel",
    ".journey-section",
    ".journey-sheet",
    ".closing-note",
    ".signal-tape",
    ".footer",
  ];
  const measure = async (p) =>
    p.evaluate(
      (selectors) =>
        Object.fromEntries(
          selectors.map((s) => {
            const e = document.querySelector(s);
            if (!e) return [s, null];
            const r = e.getBoundingClientRect(),
              c = getComputedStyle(e);
            return [
              s,
              {
                x: Math.round(r.x),
                y: Math.round(r.y),
                width: Math.round(r.width),
                height: Math.round(r.height),
                fontSize: c.fontSize,
                padding: c.padding,
                background: c.backgroundColor,
              },
            ];
          }),
        ),
      selectors,
    );
  for (const width of [2048, 1440, 768, 390, 320]) {
    await reference.setViewportSize({ width, height: 1000 });
    await current.setViewportSize({ width, height: 1000 });
    for (const view of ["login", "register", "home"]) {
      await reference.goto(origin + "/swiss-signal.html#" + view);
      await current.goto(origin + "/" + (view === "home" ? "user/home" : ""));
      if (view === "register") await current.locator("#register-tab").click();
      await current.waitForTimeout(250);
      await reference.waitForTimeout(250);
      let record = {
        width,
        view,
        reference: await measure(reference),
        current: await measure(current),
      };
      const keys =
        view === "home"
          ? [
              ".masthead",
              ".home-poster",
              ".space-stats",
              ".direction-section",
              ".direction-panel",
              ".journey-sheet",
              ".closing-note",
            ]
          : [".masthead", ".auth-view", ".auth-poster", ".auth-space"];
      for (const key of keys) {
        for (const dimension of ["x", "y", "width", "height"])
          assert(
            Math.abs(
              record.reference[key][dimension] - record.current[key][dimension],
            ) <= 1,
            `${width} ${view} ${key} ${dimension}`,
          );
        for (const property of ["fontSize", "padding", "background"])
          assert.equal(
            record.current[key][property],
            record.reference[key][property],
            `${width} ${view} ${key} ${property}`,
          );
      }
      results.push(record);
      if (width !== 320) {
        for (const [p, name] of [
          [reference, "reference"],
          [current, "current"],
        ])
          await p.screenshot({
            path: path.join(output, `comparison-${view}-${name}-${width}.png`),
            fullPage: true,
          });
      }
      console.log(
        JSON.stringify({
          width,
          view,
          reference:
            record.reference[view === "home" ? ".home-poster" : ".auth-view"],
          current:
            record.current[view === "home" ? ".home-poster" : ".auth-view"],
        }),
      );
    }
  }
  fs.writeFileSync(
    path.join(output, "style-comparison.json"),
    JSON.stringify(results, null, 2),
  );
  await context.close();
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
