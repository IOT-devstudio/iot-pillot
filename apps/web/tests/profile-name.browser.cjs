// Playwright browser_run_code: run against Vite on :5174; all API calls are fixtures.
async (page) => {
  const results = [];
  for (const width of [1440, 320, 375, 390, 430]) {
    const context = await page.context().browser().newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" });
    try {
      const p = await context.newPage();
      const errors = [];
      async function waitFor(test) {
        try { await p.waitForFunction(test, undefined, {timeout: 5000}); }
        catch { throw new Error(test.toString() + " BODY: " + await p.locator("body").innerText()); }
      }
      p.on("pageerror", e => errors.push(e.message));
      await p.addInitScript(() => localStorage.setItem("iot-pillot.auth", JSON.stringify({ user_id: 1, access_token: "fixture-access", refresh_token: "fixture-refresh" })));
      let profile = { user_id: 1, username: "旧令牌姓名", name: "原始姓名", role: "member", detail: { class: "物联2301", student_id: 123, qq: "12345", direction: "front-end", email: "user@example.com" } };
      const writes = [];
      let fail = false;
      await p.route("**/api/v1/**", async route => {
        const req = route.request();
        if (!req.url().endsWith("/me")) return route.fulfill({ json: { code: 0, data: { items: [], total: 0 } } });
        if (req.method() === "PUT") {
          const body = req.postDataJSON();
          writes.push(body);
          if (fail) return route.fulfill({ status: 400, json: { code: 4000, message: "姓名保存失败，请重试" } });
          profile = { ...profile, name: body.name ?? "", detail: { ...profile.detail, class: body.class, student_id: body.student_id, qq: body.qq, direction: body.direction } };
        }
        return route.fulfill({ json: { code: 0, message: "ok", data: profile } });
      });
      await p.goto("http://127.0.0.1:5174/user/settings");
      const name = p.getByRole("textbox", { name: "姓名", exact: true });
      await name.waitFor();
      await name.fill("张三");
      await p.getByRole("button", { name: "保存", exact: true }).click();
      await waitFor(() => document.querySelector("#settings-name-error").textContent.includes("3–20"));
      if (writes.length) throw new Error("invalid name sent");
      await name.fill("新的姓名");
      await p.getByRole("textbox", { name: "班级", exact: true }).fill("物联2402");
      await p.getByRole("spinbutton", { name: "学号", exact: true }).fill("456");
      await p.getByRole("textbox", { name: "QQ", exact: true }).fill("99999");
      await p.getByRole("combobox", { name: "方向", exact: true }).selectOption("back-end");
      await p.getByRole("button", { name: "保存", exact: true }).click();
      await waitFor(() => document.body.innerText.includes("资料已保存"));
      const body = writes.at(-1);
      if (JSON.stringify(body) !== JSON.stringify({name:"新的姓名",class:"物联2402",student_id:456,qq:"99999",direction:"back-end"})) throw new Error("unexpected payload: " + JSON.stringify(body));
      if (await p.locator('input[type=email]').count()) throw new Error("email is editable");
      await name.fill("未保存姓名");
      await p.getByRole("button", { name: "重置", exact: true }).click();
      if (await name.inputValue() !== "新的姓名") throw new Error("reset failed");
      await p.reload();
      await name.waitFor();
      if (await name.inputValue() !== "新的姓名") throw new Error("reload lost name");
      fail = true;
      await name.fill("失败时保留");
      await p.getByRole("button", { name: "保存", exact: true }).click();
      await p.locator(".settings-save-error").waitFor();
      if (await name.inputValue() !== "失败时保留" || profile.name !== "新的姓名") throw new Error("failed save changed identity or lost draft");
      fail = false;
      await name.fill("");
      await p.getByRole("button", { name: "保存", exact: true }).click();
      await waitFor(() => !document.querySelector(".settings-save-error"));
      if (writes.at(-1).name !== null) throw new Error("clear name was not null");
      await name.fill("最终姓名");
      await p.getByRole("button", { name: "保存", exact: true }).click();
      await waitFor(() => !document.querySelector(".settings__btn--primary").disabled);
      const overflow = await p.evaluate(() => document.documentElement.scrollWidth > innerWidth);
      if (overflow) throw new Error("horizontal overflow at " + width);
      await p.getByRole("button", { name: "返回用户首页", exact: false }).click();
      await p.waitForURL("**/user/home");
      await p.locator(".user-menu__name").filter({ hasText: "最终姓名" }).waitFor({ state: "attached" });
      if (!(await p.locator("body").innerText()).includes("HELLO, 最终姓名.")) throw new Error("home name is stale");
      if (errors.length) throw new Error(errors.join("; "));
      results.push({ width, nameSaved: true, reload: true, menuAndHome: true, clear: true, validation: true, failurePreservesDraft: true, fieldsUnchanged: true, overflow: false });
    } finally { await context.close(); }
  }
  return results;
}
