// Run against Vite; API fixtures never reach the real server.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const assert = require("node:assert/strict");
const base = process.env.TEST_BASE_URL || "http://127.0.0.1:5173";
(async () => {
 const browser = await chromium.launch({ channel: "msedge", headless: true });
 try {
  for (const width of [1440, 390]) {
   for (const reducedMotion of ["no-preference", "reduce"]) {
    const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion });
    await context.addInitScript(() => {
      localStorage.setItem("iot-pillot.auth", JSON.stringify({ user_id: 7, access_token: "test-only", refresh_token: "test-only" }));
      const Observer = window.IntersectionObserver;
      window.IntersectionObserver = class extends Observer {
       constructor(callback, options) { super((entries, observer) => {
        if (entries.some(e => e.isIntersecting && e.target.matches(".space-stats"))) window.__statsEntered = true;
        callback(entries, observer);
       }, options); }
      };
    });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", e => errors.push(e.message));
    let release;
    let gate;
    const resetGate = () => { gate = new Promise(resolve => { release = resolve; }); };
    resetGate();
    await page.route("**/api/v1/**", async route => {
     const path = new URL(route.request().url()).pathname;
     assert.equal(route.request().method(), "GET", "Statistics must not write data");
     const ok = data => route.fulfill({ contentType: "application/json", body: JSON.stringify({code:0,message:"ok",data}) });
     if (path === "/api/v1/me") return ok({user_id:7, username:"测试账号", name:"测试姓名", role:"member", detail:{class:"测试班级",student_id:123,qq:"12345",email:"test@example.com",direction:"front-end"}});
     if (path === "/api/v1/recruitment/directions") {
      await gate;
      return ok({items:["front-end","back-end","agent"].map((code,i)=>({id:i+1,code,name:code === "front-end" ? "前端开发" : code,open:true,sort_order:i,form:null}))});
     }
     if (path === "/api/v1/recruitment/applications") {
      await gate;
      return ok({items:[{id:42,user_id:7,round_id:1,direction_code:"front-end",status:"accepted",name:"测试姓名",created_at:"2026-10-16T00:00:00Z",updated_at:"2026-10-16T00:00:00Z"}],total:1,page:1,page_size:20});
     }
     throw Error("Unexpected API: " + path);
    });
    for (const phase of ["initial login", "reload"]) {
     if (phase === "reload") { resetGate(); await page.reload({waitUntil:"domcontentloaded"}); }
     else await page.goto(base + "/user/home", {waitUntil:"domcontentloaded"});
     const stats = page.locator(".space-stats");
     await stats.waitFor();
     await stats.evaluate(el => el.scrollIntoView({behavior:"instant"}));
     // Let the entry animation capture the initial zero before API data arrives.
     await page.waitForFunction(() => window.__statsEntered === true);
     await page.waitForTimeout(180);
     release();
     await page.locator(".journey-summary h3").filter({hasText:"前端开发"}).waitFor();
     await page.waitForTimeout(1000);
     const counts = await stats.locator(":scope > div:not(.profile-readout) strong > span").allTextContents();
     assert.deepEqual(counts, ["03", "01"], `${width}px ${reducedMotion} ${phase}: stats must match server records`);
     assert.deepEqual(errors, []);
     console.log(`PASS ${width}px ${reducedMotion} ${phase}: 03 directions / 01 application`);
    }
    await context.close();
   }
  }
 } finally { await browser.close(); }
})().catch(e=>{console.error(e);process.exit(1)});
