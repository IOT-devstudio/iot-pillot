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
      assert.equal(await page.locator(".about__inner").evaluate(el=>getComputedStyle(el).zoom),"1.2");
      assert.equal(await page.getByRole("tablist").getAttribute("aria-orientation"),width<541?"horizontal":"vertical");
      const expectPanel = async (id,title) => {
        await page.waitForFunction(({id,title}) => {
          const panel=document.querySelector("#about-content-"+id);
          return panel?.getAttribute("aria-labelledby") === "about-tab-"+id
            && panel?.textContent.includes(title) && !panel.hidden
            && getComputedStyle(panel).opacity === "1";
        },{id,title});
        assert.equal(await page.locator('[role="tabpanel"]:visible').count(),1);
      };
      const entries=[["intro","从界面到服务"],["focus","前端开发"],["works","成果统计"],["roles","总负责人"]];
      for(const [id,title] of entries){
        const tab=page.locator("#about-tab-"+id);
        if(width===1440) await tab.hover(); else await tab.tap();
        await expectPanel(id,title);
        assert.equal(await tab.getAttribute("aria-selected"),"true");
        assert.equal(await tab.getAttribute("aria-controls"),"about-content-"+id);
        assert.equal(await page.locator(".about__content .liquid-glass").count(),0,"Cards have no edge-flow treatment");
      }
      await page.locator("#about-tab-intro").click();
      await page.locator(".intro__stats").getByText("成立").waitFor();
      const stats=await page.locator(".intro__stats > div").evaluateAll(items=>items.map(el=>[el.querySelector("dt").textContent,el.querySelector("dd").textContent]));
      assert.deepEqual(stats,[["成立","2018"],["地点","第十教学楼 B413"],["累计实战项目","50 余个"]]);
      assert.match(await page.locator(".intro__stats-note").innerText(),/截至 2022 级/);
      // #181：占位口径与「在做项目」这类未经确认的数字不应再出现在页面上
      assert.equal(await page.locator(".intro__stats").getByText("待补充").count(),0);
      const photos = page.getByRole("region", {name:"工作室日常"});
      const projects = page.getByRole("region", {name:"历年项目"});
      assert.match(await photos.locator(".gallery__caption").innerText(),/工作室一角/);
      await photos.getByRole("button", {name:"工作室日常：下一张"}).click();
      assert.match(await photos.locator(".gallery__caption").innerText(),/工作室日常与团建/);
      await photos.locator(".gallery__frame img").evaluate(img => img.decode());
      assert.equal(await photos.locator(".gallery__frame img").evaluate(img => img.naturalWidth > 0), true);
      await photos.getByRole("button", {name:"工作室日常：下一张"}).click();
      assert.match(await photos.locator(".gallery__caption").innerText(),/工作室一角/);
      assert.equal(await page.locator("#about-content-intro").getByRole("region", {name:"历年项目"}).count(), 0);
      const destinations = page.getByRole("region", {name:"人员去向"});
      assert.match(await destinations.locator(".gallery__caption").innerText(), /美团 \/ 用友/);
      await destinations.locator(".gallery__frame img").evaluate(img => img.decode());
      await destinations.getByRole("button", {name:"人员去向：下一张"}).click();
      assert.match(await destinations.locator(".gallery__caption").innerText(), /滴滴 \/ 小米/);
      await destinations.getByRole("button", {name:"人员去向：下一张"}).click();
      assert.match(await destinations.locator(".gallery__caption").innerText(), /21 级成员去向/);
      await page.locator("#about-tab-works").click();
      assert.equal(await projects.locator("figure").count(), 2);
      assert.equal(await projects.getByRole("button").count(), 0);
      for (const img of await projects.locator("img").all()) await img.evaluate(el => el.decode());
      assert.match(await projects.innerText(), /农屿 APP 开发/);
      assert.match(await projects.innerText(), /雅安气象项目/);
      assert.equal(await page.getByText("资料仅列出项目名称，实施年份与范围待核实。").count(), 0);
      await page.locator("#about-tab-intro").click();
      assert.match(await photos.locator(".gallery__caption").innerText(),/工作室一角/);
      assert.equal(await page.locator("#about-content-intro").getByText("在做项目").count(),0);
      if(width===1440){
        await page.locator("#about-tab-intro").focus();
        await page.locator("#about-tab-works").hover();
        assert.equal(await page.locator("#about-tab-intro").getAttribute("aria-selected"),"true");
      }
      await page.locator("#about-tab-focus").focus();
      await page.keyboard.press("End");
      await expectPanel("roles","总负责人");
      await page.keyboard.press("Home");
      await expectPanel("intro","从界面到服务");
      await page.keyboard.press("ArrowDown");
      await expectPanel("focus","前端开发");
      await page.keyboard.press("ArrowLeft");
      await expectPanel("intro","从界面到服务");
      await page.keyboard.press("ArrowUp");
      await expectPanel("roles","总负责人");
      await page.locator("#about-tab-focus").click();
      await expectPanel("focus","前端开发");
      assert.match(await page.locator("#about-content-focus").evaluate(el=>getComputedStyle(el).animationName),/^about-panel-reveal/);
      // #181：Agent 必须带可见的宣讲来源；技术标签不能冒充正式技术栈
      const agent=page.locator("#about-content-focus .focus__row").filter({hasText:"Agent 开发"});
      assert.match(await agent.innerText(),/2025 年宣讲主题/);
      const tags=await page.locator("#about-content-focus .focus__tags li").allInnerTexts();
      for(const forbidden of ["Vue 3","Vite","Go","Gin","PostgreSQL","LLM API"]) assert.equal(tags.includes(forbidden),false,forbidden+" 不是学习内容标签");
      await page.locator("#about-tab-works").click();
      await expectPanel("works","成果统计");
      assert.match(await page.locator("#about-content-works .works__scope").innerText(),/截至 2022 级/);
      const achievementValues=await page.locator("#about-content-works .works__stats dd").allInnerTexts();
      assert.deepEqual(achievementValues,["8 项","25 项","5 项","50 余个"]);
      assert.equal(await page.locator("#about-content-works .works__list > li").count(),2);
      const workSources=await page.locator("#about-content-works .works__source").allInnerTexts();
      for(const source of workSources) assert.match(source,/资料来源：.*2025 年宣讲资料/);
      await page.locator("#about-tab-roles").click();
      await expectPanel("roles","总负责人");
      assert.equal(await page.locator("#about-content-roles .roles__list > li").count(),3);
      // 没有可公开名册时不生成姓名头像
      assert.equal(await page.locator("#about-content-roles img").count(),0);
      if(width===1440){
        await page.getByRole("tabpanel").hover();
        assert.equal(await page.locator("#about-tab-roles").getAttribute("aria-selected"),"true");
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
      assert.equal(await page.locator("#about-content-intro").evaluate(el=>getComputedStyle(el).animationName),"none");
      assert.deepEqual(errors,[]);
      console.log("PASS "+width+"px: hover/tap, keyboard, content access, glow, reduced motion, no overflow");
      await context.close();
    }
    const guest=await browser.newPage();
    await guest.goto(base+"/user/about",{waitUntil:"networkidle"});
    assert.match(guest.url(),/\/user\/about$/);
    assert.equal(await guest.getByRole("tab").count(),4);
    assert.equal(await guest.locator('[role="tabpanel"]:visible').count(),1);
    await guest.getByText("从界面到服务").waitFor();
    await guest.close();
  } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exit(1)});
