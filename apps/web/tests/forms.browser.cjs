// Standalone browser regression using Node 22+ WebSocket/CDP. No production writes.
// node apps/web/tests/forms.browser.cjs <browser-websocket-url> [origin] [screenshot-directory]
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");

async function main() {
  const [endpoint, origin = "http://127.0.0.1:5176", output] =
    process.argv.slice(2);
  if (!endpoint) throw new Error("Pass an isolated Chrome CDP websocket URL");
  const socket = new WebSocket(endpoint);
  await new Promise((resolve, reject) => {
    socket.onopen = resolve;
    socket.onerror = reject;
  });
  let nextId = 0;
  const pending = new Map();
  const errors = [];
  function command(method, params = {}, sessionId) {
    const id = ++nextId;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pending.delete(id);
        reject(new Error("CDP timeout: " + method));
      }, 15000);
      pending.set(id, { resolve, reject, timer });
      socket.send(
        JSON.stringify({
          id,
          method,
          params,
          ...(sessionId ? { sessionId } : {}),
        }),
      );
    });
  }
  let context;
  let submission = null,
    failSubmit = false,
    submitCount = 0,
    nextFieldId = 100;
  let role = "member";
  const writes = [];
  const fields = [
    {
      id: 11,
      type: "text",
      label: "作品链接",
      required: true,
      placeholder: "你的作品地址",
      sort_order: 0,
    },
    {
      id: 12,
      type: "email",
      label: "联系邮箱",
      required: false,
      placeholder: "",
      sort_order: 1,
    },
    {
      id: 13,
      type: "textarea",
      label: "项目经历",
      required: true,
      placeholder: "",
      sort_order: 2,
    },
    {
      id: 14,
      type: "select",
      label: "主要语言",
      required: true,
      placeholder: "",
      options: ["Go", "Java"],
      sort_order: 3,
    },
    {
      id: 15,
      type: "checkbox",
      label: "确认信息真实",
      required: true,
      placeholder: "",
      sort_order: 4,
    },
  ];
  let form = {
    id: 1,
    title: "2026 秋季招新 · 后端开发",
    description: "认真表达你的兴趣。\n期待看到你的作品与思考。",
    round_id: 1,
    direction_code: "back-end",
    share_token: "token",
    is_open: true,
    submission_count: 0,
    created_at: "2026-10-15T00:00:00Z",
    updated_at: "2026-10-15T00:00:00Z",
  };
  let formFields = fields.map((f) => ({ ...f }));
  function response(url, method, body) {
    if (method !== "GET") writes.push({ url, method, body });
    if (url.pathname === "/api/v1/me")
      return {
        user_id: 7,
        username: "account@example.cn",
        name: "测试同学",
        role,
        detail: {
          class: "物联2401",
          student_id: 2024001,
          email: "account@example.cn",
          qq: "123456",
        },
      };
    if (url.pathname === "/api/v1/admin/recruitment/rounds")
      return {
        items: [
          { id: 1, name: "2026 秋季招新", round_num: 1, accepting: true },
        ],
      };
    if (url.pathname === "/api/v1/recruitment/directions")
      return {
        items: [
          {
            id: 1,
            code: "back-end",
            name: "后端开发",
            open: true,
            sort_order: 1,
          },
        ],
      };
    if (url.pathname === "/api/v1/forms/public/invalid")
      throw new Error("表单不存在或分享链接无效");
    if (url.pathname.endsWith("/submit")) {
      submitCount++;
      if (failSubmit) throw new Error("模拟提交失败，请稍后重试");
      assert.deepEqual(body.values["15"], ["确认信息真实"]);
      assert.equal(Object.keys(body).join(), "values");
      submission = {
        id: 3,
        form_id: 1,
        submitter_id: 7,
        application_id: 8,
        submitted_at: "2026-10-15T10:00:00Z",
        values: body.values,
      };
      form.submission_count = 1;
      return { submission, application_id: 8, application_status: "applied" };
    }
    if (url.pathname.startsWith("/api/v1/forms/public/"))
      return {
        form: {
          ...form,
          is_open: url.pathname.endsWith("/closed") ? false : form.is_open,
        },
        fields: formFields,
        submitted: !!submission,
        my_submission_id: submission?.id ?? 0,
      };
    if (url.pathname === "/api/v1/forms" && method === "POST") {
      assert.equal(body.round_id, 1);
      assert.equal(body.direction_code, "back-end");
      form = { ...form, ...body, id: 1, is_open: false, submission_count: 0 };
      formFields = body.fields.map((f, i) => ({
        ...f,
        id: nextFieldId++,
        sort_order: i,
      }));
      return { form, fields: formFields };
    }
    if (url.pathname === "/api/v1/forms")
      return { items: [form], total: 1, page: 1, page_size: 20 };
    if (url.pathname === "/api/v1/forms/1/submissions/3") return submission;
    if (url.pathname === "/api/v1/forms/1/submissions")
      return {
        items: submission ? [submission] : [],
        total: submission ? 1 : 0,
        page: 1,
        page_size: 20,
      };
    if (url.pathname === "/api/v1/forms/1") {
      if (method === "PATCH") {
        form = { ...form, ...body };
        if (body.fields)
          formFields = body.fields.map((f, i) => ({
            ...f,
            id: f.id || nextFieldId++,
            sort_order: i,
          }));
      }
      return { form, fields: formFields };
    }
    if (url.pathname.includes("/recruitment/applications"))
      return { items: [], total: 0, page: 1, page_size: 20 };
    throw new Error("Unexpected API: " + method + " " + url.pathname);
  }
  socket.onmessage = async (event) => {
    const message = JSON.parse(event.data);
    if (message.id) {
      const request = pending.get(message.id);
      if (!request) return;
      pending.delete(message.id);
      clearTimeout(request.timer);
      if (message.error)
        request.reject(new Error(JSON.stringify(message.error)));
      else request.resolve(message.result);
    } else if (message.method === "Runtime.exceptionThrown")
      errors.push(
        message.params.exceptionDetails.text +
          " " +
          (message.params.exceptionDetails.exception?.description || ""),
      );
    else if (message.method === "Fetch.requestPaused") {
      const { requestId, request } = message.params;
      let data,
        status = 200;
      try {
        assert.match(
          request.headers.Authorization || request.headers.authorization || "",
          /^Bearer /,
        );
        data = {
          code: 0,
          data: response(
            new URL(request.url),
            request.method,
            request.postData ? JSON.parse(request.postData) : undefined,
          ),
        };
      } catch (e) {
        status = 400;
        data = { code: 4000, message: e.message };
      }
      await command(
        "Fetch.fulfillRequest",
        {
          requestId,
          responseCode: status,
          responseHeaders: [
            { name: "Content-Type", value: "application/json" },
          ],
          body: Buffer.from(JSON.stringify(data)).toString("base64"),
        },
        message.sessionId,
      );
    }
  };
  try {
    context = (await command("Target.createBrowserContext")).browserContextId;
    const target = await command("Target.createTarget", {
      url: "about:blank",
      browserContextId: context,
    });
    const { sessionId } = await command("Target.attachToTarget", {
      targetId: target.targetId,
      flatten: true,
    });
    const cdp = (name, params) => command(name, params, sessionId);
    await cdp("Page.enable");
    await cdp("Runtime.enable");
    await cdp("Fetch.enable", {
      patterns: [{ urlPattern: origin + "/api/*", requestStage: "Request" }],
    });
    await cdp("Emulation.setEmulatedMedia", {
      features: [{ name: "prefers-reduced-motion", value: "reduce" }],
    });
    const evaluate = async (expression) => {
      const result = await cdp("Runtime.evaluate", {
        expression,
        awaitPromise: true,
        returnByValue: true,
      });
      if (result.exceptionDetails)
        throw new Error(
          result.exceptionDetails.exception?.description ||
            result.exceptionDetails.text,
        );
      return result.result.value;
    };
    const wait = async (expression) => {
      const deadline = Date.now() + 10000;
      while (Date.now() < deadline) {
        try {
          if (await evaluate(expression)) return;
        } catch {}
        await new Promise((resolve) => setTimeout(resolve, 40));
      }
      throw new Error(
        "Wait failed: " +
          expression +
          "\n" +
          (await evaluate("document.body.innerText")),
      );
    };
    const navigate = async (pathname) => {
      await cdp("Page.navigate", { url: origin + pathname });
    };
    const click = async (text) => {
      const clicked = await evaluate(
        `(() => { const button = [...document.querySelectorAll('button,a')].find(b => b.textContent.trim() === ${JSON.stringify(text)} && b.getClientRects().length); if(!button) return false; button.click(); return true; })()`,
      );
      assert.equal(clicked, true, "Button not found: " + text);
    };
    const fill = (id, value) =>
      evaluate(
        `(() => { const el = document.getElementById(${JSON.stringify(id)}); if(!el) throw Error('Missing input'); el.value = ${JSON.stringify(value)}; el.dispatchEvent(new Event('input', {bubbles:true})); })()`,
      );
    const screenshot = async (name) => {
      if (!output) return;
      await fs.mkdir(output, { recursive: true });
      const { data } = await cdp("Page.captureScreenshot", {
        format: "png",
        captureBeyondViewport: false,
      });
      await fs.writeFile(
        path.join(output, name + ".png"),
        Buffer.from(data, "base64"),
      );
    };
    await navigate("/f/token");
    await wait(
      `location.pathname === '/' && new URLSearchParams(location.search).get('redirect') === '/f/token'`,
    );
    assert.equal(submitCount, 0);
    await evaluate(
      `localStorage.setItem('iot-pillot.auth', JSON.stringify({user_id:7,access_token:'fixture-access',refresh_token:'fixture-refresh'}))`,
    );
    await navigate("/f/token");
    await wait(`!!document.getElementById('answer-11')`);
    await click("确认并提交 ↗");
    await wait(`document.activeElement.id === 'answer-11'`);
    await fill("answer-11", "https://example.cn/work");
    await fill("answer-12", "answer@example.cn");
    await fill("answer-13", "我参与的真实项目");
    await fill("answer-14", "Go");
    await evaluate(`document.getElementById('answer-15').click()`);
    assert.equal(
      await evaluate(`document.querySelector('.form-identity a').target`),
      "_blank",
    );
    await evaluate(`document.querySelector('.view-nav button').click()`);
    await wait(
      `document.body.innerText.includes('离开后将丢失尚未提交的答案')`,
    );
    await click("继续填写");
    await wait(
      `!document.querySelector('.el-message-box__wrapper') || document.querySelector('.el-message-box__wrapper').getClientRects().length === 0`,
    );
    assert.equal(
      await evaluate(`document.getElementById('answer-13').value`),
      "我参与的真实项目",
    );
    const widths = [320, 375, 390, 430, 1440];
    for (const width of widths) {
      await cdp("Emulation.setDeviceMetricsOverride", {
        width,
        height: 1000,
        deviceScaleFactor: 1,
        mobile: width < 500,
      });
      await evaluate("window.scrollTo(0, 0)");
      assert.equal(
        await evaluate("document.documentElement.scrollWidth > innerWidth"),
        false,
        "User page overflow at " + width,
      );
      if (width === 390 || width === 1440)
        await screenshot("form-user-" + width);
    }
    failSubmit = true;
    await click("确认并提交 ↗");
    await wait(`document.body.innerText.includes('模拟提交失败')`);
    assert.equal(
      await evaluate(`document.getElementById('answer-13').value`),
      "我参与的真实项目",
    );
    assert.equal(submitCount, 1);
    failSubmit = false;
    await evaluate(
      `document.querySelector('button[type=submit]').click(); document.querySelector('button[type=submit]').click()`,
    );
    await wait(`document.body.innerText.includes('提交成功')`);
    assert.equal(submitCount, 2);
    await navigate("/f/token");
    await wait(`document.body.innerText.includes('你已提交过')`);
    submission = null;
    await navigate("/f/closed");
    await wait(`document.body.innerText.includes('本次报名已关闭')`);
    assert.equal(
      await evaluate(`!!document.querySelector('button[type=submit]')`),
      false,
    );
    await navigate("/f/invalid");
    await wait(`document.body.innerText.includes('链接无效')`);
    role = "admin";
    await navigate("/forms");
    await wait(`document.body.innerText.includes('编辑与预览')`);
    await screenshot("form-admin-list");
    for (const width of widths) {
      await cdp("Emulation.setDeviceMetricsOverride", {
        width,
        height: 1000,
        deviceScaleFactor: 1,
        mobile: width < 500,
      });
      assert.equal(
        await evaluate("document.documentElement.scrollWidth > innerWidth"),
        false,
        "List overflow at " + width,
      );
    }
    await navigate("/forms/1/edit");
    await wait(`!!document.getElementById('form-title')`);
    assert.equal(
      await evaluate(
        `document.getElementById('label-11').disabled || document.getElementById('label-11').closest('fieldset').disabled`,
      ),
      true,
    );
    assert.equal(
      await evaluate(`document.getElementById('form-round').disabled`),
      true,
    );
    await fill("form-title", "已更新报名表");
    await click("保存表单");
    await wait(`document.body.innerText.includes('表单已保存')`);
    assert.equal(form.title, "已更新报名表");
    const patch = writes.filter((w) => w.method === "PATCH").at(-1);
    assert.equal("fields" in patch.body, false);
    form.submission_count = 0;
    await navigate("/forms/1/edit");
    await wait(`!!document.getElementById('label-11')`);
    await fill("label-11", "作品地址");
    await click("保存表单");
    await wait(`document.body.innerText.includes('表单已保存')`);
    assert.equal(formFields[0].id, 11);
    assert.equal(formFields[0].label, "作品地址");
    await screenshot("form-admin-editor");
    for (const width of widths) {
      await cdp("Emulation.setDeviceMetricsOverride", {
        width,
        height: 1000,
        deviceScaleFactor: 1,
        mobile: width < 500,
      });
      assert.equal(
        await evaluate("document.documentElement.scrollWidth > innerWidth"),
        false,
        "Editor overflow at " + width,
      );
      if (width < 500) {
        await evaluate(
          `document.querySelectorAll('.forms-mobile-tabs button')[1].click()`,
        );
        assert.equal(
          await evaluate("document.documentElement.scrollWidth > innerWidth"),
          false,
          "Preview overflow at " + width,
        );
        if (width === 390) await screenshot("form-admin-preview-390");
        await evaluate(
          `document.querySelectorAll('.forms-mobile-tabs button')[0].click()`,
        );
      }
    }
    submission = {
      id: 3,
      form_id: 1,
      submitter_id: 7,
      application_id: 8,
      submitted_at: "2026-10-15T10:00:00Z",
      values: {
        11: "https://example.cn/work",
        12: "answer@example.cn",
        13: "项目经历\n第二行",
        14: "Go",
        15: ["确认信息真实"],
        99: "历史答案",
      },
    };
    await navigate("/forms/1/submissions");
    await wait(`!!document.querySelector('.forms-record')`);
    await evaluate(`document.querySelector('.forms-record').click()`);
    await wait(`document.body.innerText.includes('已删除字段 #99')`);
    assert.match(
      await evaluate(`document.querySelector('.forms-answer-list').innerText`),
      /answer@example.cn/,
    );
    await screenshot("form-admin-submissions");
    for (const width of widths) {
      await cdp("Emulation.setDeviceMetricsOverride", {
        width,
        height: 1000,
        deviceScaleFactor: 1,
        mobile: width < 500,
      });
      assert.equal(
        await evaluate("document.documentElement.scrollWidth > innerWidth"),
        false,
        "Records overflow at " + width,
      );
    }
    submission = null;
    await navigate("/forms/new");
    await wait(`!!document.getElementById('form-title')`);
    await fill("form-title", "新建前端测试表单");
    await fill("form-round", "1");
    await fill("form-direction", "back-end");
    await evaluate(
      `document.getElementById('form-round').dispatchEvent(new Event('change',{bubbles:true})); document.getElementById('form-direction').dispatchEvent(new Event('change',{bubbles:true}));`,
    );
    await fill("label--1", "测试问题");
    await click("保存表单");
    await wait(
      `location.pathname === '/forms/1/edit' && document.getElementById('form-title')?.value === '新建前端测试表单'`,
    );
    assert.equal(form.is_open, false);
    await click("开放报名");
    await wait(`document.body.innerText.includes('报名开放中')`);
    assert.equal(form.is_open, true);
    await click("分享链接");
    await wait(`!!document.getElementById('form-share-url')`);
    assert.equal(
      await evaluate(`document.getElementById('form-share-url').value`),
      origin + "/f/token",
    );
    await evaluate(
      `Object.defineProperty(navigator, 'clipboard', { configurable:true, value:{writeText:async()=>{throw Error('denied')}} })`,
    );
    await click("复制链接");
    await wait(`document.body.innerText.includes('手动复制')`);
    assert.equal(
      writes.some((w) => w.url.pathname === "/api/v1/me"),
      false,
    );
    assert.deepEqual(errors, []);
    console.log(
      JSON.stringify(
        {
          passed: true,
          widths,
          responsivePages: ["filling", "list", "editor", "preview", "records"],
          guestReturn: true,
          fiveFields: true,
          draftLeaveWarning: true,
          profileInNewTab: true,
          failurePreservesAnswers: true,
          duplicateGuard: true,
          closedAndInvalid: true,
          realRecordRendering: true,
          historicalValues: true,
          createAndOpen: true,
          preservesFieldIDs: true,
          locksSubmittedFields: true,
          manualCopyFallback: true,
          accountEmailUnchanged: true,
          screenshots: output || null,
        },
        null,
        2,
      ),
    );
  } finally {
    if (context)
      await command("Target.disposeBrowserContext", {
        browserContextId: context,
      });
    socket.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
