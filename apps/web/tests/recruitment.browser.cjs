// Run with PLAYWRIGHT_MODULE pointing to an installed playwright module.
// All API calls are intercepted: this test never sends real mail.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const assert = require("node:assert/strict");
const base = process.env.TEST_BASE_URL || "http://127.0.0.1:5184";
async function run() {
  const browser = await chromium.launch({ channel: "msedge", headless: true });
  const reports = [];
  try {
    for (const width of [1440, 390]) {
      const context = await browser.newContext({
        viewport: { width, height: 1000 },
        reducedMotion: "reduce",
      });
      const page = await context.newPage();
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      let role = "admin",
        fail = false,
        failRead = false,
        writes = 0;
      const timestamp = "2026-10-15T08:00:00Z";
      const application = {
        id: 42,
        user_id: 7,
        name: "测试申请人",
        email: "test@example.com",
        direction_code: "front-end",
        status: "accepted",
        created_at: timestamp,
      };
      let invitation = {
        id: 9,
        application_id: 42,
        status: "pending",
        expires_at: "2099-10-22T08:00:00Z",
        email_sent_at: timestamp,
        accepted_at: "",
        created_at: timestamp,
      };
      let candidate = {
        id: 8,
        application_id: 42,
        user_id: 7,
        name: "测试申请人",
        email: "test@example.com",
        direction: "front-end",
        status: "interviewing",
        created_at: timestamp,
        updated_at: timestamp,
      };
      let timeline = [
        {
          from_status: "",
          to_status: "interviewing",
          operator_id: 7,
          note: "接受邀请",
          created_at: timestamp,
        },
      ];
      const token = "0123456789abcdef0123456789abcdef";
      await page.addInitScript(() => {
        if (window.top !== window || sessionStorage.getItem("test-seeded"))
          return;
        sessionStorage.setItem("test-seeded", "yes");
        localStorage.setItem(
          "iot-pillot.auth",
          JSON.stringify({
            user_id: 7,
            access_token: "test-only",
            refresh_token: "test-only-refresh",
          }),
        );
      });
      await page.route("**/api/v1/**", async (route) => {
        const req = route.request(),
          path = new URL(req.url()).pathname;
        const ok = (data) =>
          route.fulfill({ json: { code: 0, message: "ok", data } });
        const denied = (message) =>
          route.fulfill({ status: 400, json: { code: 4000, message } });
        if (req.method() === "POST") {
          writes++;
          await new Promise((resolve) => setTimeout(resolve, 200));
          if (fail) return denied("测试邮件投递失败，请重试");
          if (path.endsWith("/invitations/send")) return ok(invitation);
          if (path.endsWith("/revoke")) {
            invitation.status = "revoked";
            return ok(invitation);
          }
          if (path.endsWith("/accept")) {
            invitation.status = "accepted";
            invitation.accepted_at = timestamp;
            return ok({ candidate });
          }
          if (path.endsWith("/advance")) {
            const body = req.postDataJSON();
            candidate.status = {
              send_offer: "offered",
              hire: "hired",
              reject: "rejected",
            }[body.action];
            timeline.push({
              from_status: "interviewing",
              to_status: candidate.status,
              note: body.note,
              operator_id: 7,
              created_at: timestamp,
            });
            return ok(candidate);
          }
          throw new Error("Unexpected write: " + path);
        }
        if (path.endsWith("/me"))
          return ok({
            user_id: 7,
            username: "tester",
            name: "测试申请人",
            role,
            detail: {
              class: "测试一班",
              student_id: 123,
              qq: "123456",
              email: "test@example.com",
              direction: "front-end",
            },
          });
        if (path.endsWith("/mail-templates"))
          return ok({
            items: ["invitation", "offer", "rejection"].map((type, i) => ({
              id: i + 1,
              type,
              name: type + " 测试模板",
              title: "招新通知",
              mail_model: "<p>您好，欢迎参加招新。</p>",
              mail_example: "",
              variables: [],
            })),
            total: 3,
          });
        if (path.endsWith("/recruitment/applications/42"))
          return ok({ application, timeline: [] });
        if (path.endsWith("/admin/invitations"))
          return ok({ items: [invitation], total: 1, page: 1, page_size: 20 });
        if (path.endsWith("/admin/candidates/8"))
          return ok({ candidate, timeline });
        if (path.endsWith("/admin/candidates"))
          return ok({ items: [candidate], total: 1, page: 1, page_size: 20 });
        if (path.endsWith("/me/invitations")) {
          if (failRead) return denied("刷新失败，请重试");
          return ok({
            items: [
              { ...invitation, token, direction: "front-end", expired: false },
              {
                ...invitation,
                application_id: 43,
                token: "expired-token-12345",
                status: "pending",
                expired: true,
              },
              {
                ...invitation,
                application_id: 44,
                token: "revoked-token-12345",
                status: "revoked",
                expired: false,
              },
            ],
            total: 3,
          });
        }
        if (path.endsWith("/me/candidates"))
          return ok({
            items: invitation.status === "accepted" ? [candidate] : [],
            total: 1,
          });
        return route.fulfill({
          status: 404,
          json: { code: 4040, message: "Test route not configured: " + path },
        });
      });
      await page.route("**/health", (route) =>
        route.fulfill({ json: { status: "ok" } }),
      );
      async function text(value) {
        await page.getByText(value, { exact: false }).first().waitFor();
      }
      async function open(path) {
        await page.goto(base + path);
        await page.waitForLoadState("networkidle");
        if (errors.length) throw new Error(errors.join("\n"));
      }
      async function chooseTemplate() {
        await page.locator(".el-dialog .el-select").click();
        await page.locator(".el-select-dropdown__item:visible").first().click();
      }
      await open("/recruitment/invitations?application_id=42");
      await page.getByRole("button", { name: "发送 / 重发邀请" }).click();
      await chooseTemplate();
      fail = true;
      await page.getByRole("button", { name: "确认发送邮件" }).click();
      await text("测试邮件投递失败");
      assert.equal(invitation.status, "pending");
      fail = false;
      await page.getByRole("button", { name: "确认发送邮件" }).click();
      await text("邀请邮件已发送");
      await page.getByRole("button", { name: "撤销", exact: true }).click();
      await page.getByRole("button", { name: "确认撤销", exact: true }).click();
      await text("邀请已撤销");
      assert.equal(invitation.status, "revoked");
      invitation.status = "pending";
      await open("/recruitment/candidates/8");
      await page
        .getByRole("button", { name: "通过面试并发送 Offer", exact: true })
        .click();
      await chooseTemplate();
      fail = true;
      await page.getByRole("button", { name: "确认发送", exact: true }).click();
      await text("测试邮件投递失败");
      assert.equal(candidate.status, "interviewing");
      fail = false;
      await page.getByRole("button", { name: "确认发送", exact: true }).click();
      await text("邮件已发送，候选人状态已更新");
      assert.equal(candidate.status, "offered");
      await page.getByRole("button", { name: "确认录用", exact: true }).click();
      await page
        .locator(".el-dialog")
        .getByRole("button", { name: "确认录用", exact: true })
        .click();
      await text("已确认录用");
      assert.equal(candidate.status, "hired");
      assert.equal(
        await page
          .getByRole("button", { name: "不通过并发送感谢信", exact: true })
          .count(),
        0,
      );
      candidate.status = "interviewing";
      await open("/recruitment/candidates/8");
      await page
        .getByRole("button", { name: "不通过并发送感谢信", exact: true })
        .click();
      await chooseTemplate();
      await page.getByRole("button", { name: "确认发送", exact: true }).click();
      await text("邮件已发送，候选人状态已更新");
      assert.equal(candidate.status, "rejected");
      role = "member";
      candidate.status = "interviewing";
      await open("/user/invitations?token=" + token);
      await text("已过期");
      await text("已撤销");
      assert.equal(
        await page
          .getByRole("button", { name: "确认接受邀请", exact: true })
          .count(),
        1,
      );
      fail = true;
      await page
        .getByRole("button", { name: "确认接受邀请", exact: true })
        .click();
      await text("测试邮件投递失败");
      assert.equal(invitation.status, "pending");
      fail = false;
      failRead = true;
      const before = writes;
      await page
        .getByRole("button", { name: "确认接受邀请", exact: true })
        .evaluate((button) => {
          button.click();
          button.click();
        });
      await text("邀请已接受");
      await text("刷新失败");
      assert.equal(writes, before + 1);
      failRead = false;
      await page.getByRole("button", { name: "刷新记录" }).click();
      await text("已接受");
      assert.equal(
        await page
          .getByRole("button", { name: "确认接受邀请", exact: true })
          .count(),
        0,
      );
      await page.reload();
      await text("已接受");
      assert.equal(
        await page.evaluate(
          () => document.documentElement.scrollWidth > innerWidth,
        ),
        false,
      );
      if (process.env.SCREENSHOT_DIR)
        await page.screenshot({
          path: require("node:path").join(
            process.env.SCREENSHOT_DIR,
            `recruitment-${width}.png`,
          ),
          fullPage: true,
        });
      await open("/user/invitations?token=wrong-account-token");
      await text("此邀请不属于当前账号");
      await open("/recruitment/candidates");
      await page.waitForURL("**/user/home");
      await page.evaluate(() => localStorage.removeItem("iot-pillot.auth"));
      await open("/user/invitations?token=" + token);
      assert.equal(
        new URL(page.url()).searchParams.get("redirect"),
        "/user/invitations?token=" + token,
      );
      assert.deepEqual(errors, []);
      reports.push({
        width,
        inviteSendRevoke: true,
        offerHireRejection: true,
        acceptFailureRetry: true,
        duplicateSuppressed: true,
        refreshFailureHonest: true,
        wrongAccount: true,
        loginReturn: true,
        noOverflow: true,
      });
      await context.close();
    }
    console.log(JSON.stringify(reports, null, 2));
  } finally {
    await browser.close();
  }
}
run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
