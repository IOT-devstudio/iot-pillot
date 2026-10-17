// Run with PLAYWRIGHT_MODULE set to an installed Playwright module. No real API calls.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const assert = require("node:assert/strict");
const base = process.env.TEST_BASE_URL || "http://127.0.0.1:5173";
(async () => {
  const browser = await chromium.launch({channel:"msedge",headless:true});
  try {
    for (const width of [1440,320,375,390,430]) {
      const context = await browser.newContext({viewport:{width,height:1000},hasTouch:width<541});
      await context.addInitScript(() => localStorage.setItem("iot-pillot.auth", JSON.stringify({user_id:7,access_token:"test-only",refresh_token:"test-only"})));
      const page = await context.newPage();
      const errors = [];
      page.on("pageerror", e => errors.push(e.message));
      await page.route("**/api/v1/**", route => {
        assert.equal(route.request().method(),"GET");
        assert.equal(new URL(route.request().url()).pathname,"/api/v1/me");
        return route.fulfill({contentType:"application/json",body:JSON.stringify({code:0,data:{user_id:7,username:"测试姓名",name:"测试姓名",role:"member",detail:{class:"测试班级",student_id:123,qq:"12345",email:"test@example.com"}}})});
      });
      await page.goto(base+"/user/about",{waitUntil:"networkidle"});
      const tabs = page.getByRole("tab");
      assert.equal(await tabs.count(),4);
      assert.equal(await page.getByRole("tablist").getAttribute("aria-orientation"),width<541?"horizontal":"vertical");
      const expectPanel = async (id,title) => {
        await page.waitForFunction(({id,title}) => {
          const panel=document.querySelector("#about-content-"+id);
          return panel?.getAttribute("aria-labelledby") === "about-tab-"+id
            && panel?.textContent.includes(title) && !panel.hidden;
        },{id,title});
        assert.equal(await page.locator('[role="tabpanel"]:visible').count(),1);
      };
      const entries=[["intro","从界面到服务"],["focus","前端开发"],["works","成果资料待补充"],["members","成员资料待补充"]];
      for(const [id,title] of entries){
        const tab=page.locator("#about-tab-"+id);
        if(width===1440) await tab.hover(); else await tab.tap();
        await expectPanel(id,title);
        assert.equal(await tab.getAttribute("aria-selected"),"true");
        assert.equal(await tab.getAttribute("aria-controls"),"about-content-"+id);
      }
      await page.locator("#about-tab-focus").focus();
      await page.keyboard.press("End");
      await expectPanel("members","成员资料待补充");
      await page.keyboard.press("Home");
      await expectPanel("intro","从界面到服务");
      await page.keyboard.press("ArrowDown");
      await expectPanel("focus","前端开发");
      await page.keyboard.press("ArrowLeft");
      await expectPanel("intro","从界面到服务");
      await page.keyboard.press("ArrowUp");
      await expectPanel("members","成员资料待补充");
      await page.locator("#about-tab-focus").click();
      await expectPanel("focus","前端开发");
      if(width===1440){
        await page.getByRole("tabpanel").hover();
        assert.equal(await page.locator("#about-tab-focus").getAttribute("aria-selected"),"true");
      }
      const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);
      assert.equal(overflow,false,"No horizontal page overflow");
      const glow=await page.locator(".glass-backdrop").evaluate(el=>({
        pointerEvents:getComputedStyle(el).pointerEvents,
        position:getComputedStyle(el).position,
        blue:getComputedStyle(el,"::before").backgroundImage,
        yellow:getComputedStyle(el,"::after").backgroundImage,
      }));
      assert.equal(glow.pointerEvents,"none");
      assert.equal(glow.position,"fixed");
      assert.match(glow.blue,/radial-gradient.*23, 57, 245/);
      assert.match(glow.yellow,/radial-gradient.*233, 255, 108/);
      if(process.env.SCREENSHOT_DIR)await page.screenshot({path:process.env.SCREENSHOT_DIR+"/about-sidebar-"+width+".png",fullPage:true});
      await page.emulateMedia({reducedMotion:"reduce"});
      await tabs.first().click();
      await expectPanel("intro","从界面到服务");
      assert.equal(await tabs.first().evaluate(el=>getComputedStyle(el).transitionDuration),"0s");
      assert.deepEqual(errors,[]);
      console.log("PASS "+width+"px: hover/tap, keyboard, content access, glow, reduced motion, no overflow");
      await context.close();
    }
  } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exit(1)});
