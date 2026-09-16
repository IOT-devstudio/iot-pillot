# Auth Page Stability and Full-Stack Recruitment Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the left brand panel visually stable while switching auth modes and refresh the login page for an IoT full-stack studio recruitment flow.

**Architecture:** Keep `AuthView.vue` as the single auth-page component. Stabilize the desktop shell with a fixed `100dvh` grid and an independently scrollable right panel; leave mobile in natural document flow. Update only the left-panel copy/diagram and right-panel copy while preserving existing form state, API submission, validation, ARIA, and responsive behavior.

**Tech Stack:** Vue 3 `<script setup>`, scoped CSS, Vitest with the existing custom renderer, TypeScript, Vite.

## Global Constraints

- Keep login/register API contracts, form validation, auth state, routing, and disabled verification-code behavior unchanged.
- Desktop auth shell must use a fixed `100dvh` viewport and isolate vertical scrolling to `.form-panel`.
- Mobile (`max-width: 819px`) must restore natural page height and document scrolling.
- Use the existing navy / paper-white / red annotation palette and no new dependencies or external assets.
- All user-facing copy must be Chinese and describe frontend/backend-centered full-stack practice.

---

### Task 1: Lock down mode-switch stability and recruitment copy with failing tests

**Files:**
- Modify: `apps/web/src/views/AuthView.test.ts`

**Interfaces:**
- Consumes: `AuthView` rendered through the existing custom Vue renderer and `click` helper.
- Produces: regression coverage proving the `.brand-panel` node survives mode changes and the updated full-stack copy is rendered.

- [ ] **Step 1: Write the failing tests**

Add these cases inside `describe("AuthView", ...)`:

```ts
it("switches modes without remounting or replacing the brand panel", async () => {
  const mounted = mount();
  unmount = mounted.unmount;
  const brandPanel = findElement(
    mounted.root,
    (element) => element.props.class === "brand-panel",
  );

  await click(findButton(mounted.root, "注册"));

  expect(
    findElement(mounted.root, (element) => element.props.class === "brand-panel"),
  ).toBe(brandPanel);
});

it("describes the studio as a frontend and backend focused full-stack team", () => {
  const mounted = mount();
  unmount = mounted.unmount;
  const text = visibleText(mounted.root);

  expect(text).toContain("全栈工作室");
  expect(text).toContain("前端");
  expect(text).toContain("后端");
  expect(text).toContain("招新");
});
```

Keep the existing Chinese-copy assertion aligned with the new terms and remove expectations for the abandoned hardware-oriented copy.

- [ ] **Step 2: Run the focused tests and verify the new expectations fail**

Run from the repository root:

```bash
pnpm --filter @iot-pillot/web test -- src/views/AuthView.test.ts
```

Expected: the existing DOM identity test may already pass because the brand panel is outside the conditional subtree; the copy test must fail against the current baseline because it does not contain the new full-stack wording. If the identity test passes, retain it as regression coverage and use the copy failure as the RED signal.

---

### Task 2: Stabilize the desktop shell and refresh AuthView content

**Files:**
- Modify: `apps/web/src/views/AuthView.vue:66-380` (copy/diagram/template)
- Modify: `apps/web/src/views/AuthView.vue:407-1050` (layout and responsive CSS)

**Interfaces:**
- Consumes: Existing `mode`, `setMode`, `handleSubmit`, form objects, errors, and renderer contract.
- Produces: A fixed-height desktop shell where `.form-panel` owns overflow, plus full-stack studio recruitment copy and diagram.

- [ ] **Step 1: Apply the minimal layout fix**

Update the desktop shell declarations as follows:

```css
.auth-page {
  height: 100dvh;
  min-height: 100vh;
  overflow: hidden;
}

.brand-panel,
.form-panel {
  min-height: 0;
  height: 100%;
}

.form-panel {
  overflow-y: auto;
  align-items: start;
}

.form-shell {
  margin: auto 0;
}
```

Keep the existing grid columns and padding. At the mobile breakpoint, override the fixed-height rules:

```css
.auth-page {
  height: auto;
  min-height: 100dvh;
}

.form-panel {
  height: auto;
  min-height: 100dvh;
  overflow: visible;
  align-items: center;
}
```

This makes the right panel scroll when registration is taller than the viewport while preventing the outer grid from changing width and moving the left panel.

- [ ] **Step 2: Replace only the agreed visual copy and diagram labels**

Use these exact strings while retaining the existing SVG structure and palette:

```text
aside aria-label: iot 全栈工作室招新
edition: 2026 春季招新
eyebrow: 全栈招新 · 01
headline: 从界面到服务，把想法做成作品。
description: iot 全栈工作室以前端与后端开发为核心，围绕真实问题做项目：拆需求、写代码、联调服务，再把结果交付给真正的用户。
diagram labels: 01 / 界面, 02 / 服务, 03 / 数据
diagram footer: 全栈练习 · 比例 1:2
diagram annotation: 一起完成
section index: 招新入口 / 01 / 02
login eyebrow: 欢迎回来
register eyebrow: 建立成员档案
login title: 回到工作台
register title: 加入 iot 全栈工作室
login description: 使用工作室账号继续协作。
register description: 填写资料，加入一群认真做作品的人。
register note: 验证码服务暂未开放，注册资料会在后续招新流程中使用。
```

Do not change IDs, form names, submit labels, API calls, or verification-code behavior.

- [ ] **Step 3: Run the focused tests and verify they pass**

Run:

```bash
pnpm --filter @iot-pillot/web test -- src/views/AuthView.test.ts
```

Expected: all AuthView tests pass, including mode switching, validation, API failure, loading, and full-stack copy assertions.

- [ ] **Step 4: Commit the focused implementation**

```bash
git add apps/web/src/views/AuthView.vue apps/web/src/views/AuthView.test.ts
git commit -m "feat(auth): stabilize mode switching and refresh studio copy"
```

---

### Task 3: Run the full frontend verification suite

**Files:**
- No additional source files; verify the files changed in Tasks 1–2.

**Interfaces:**
- Consumes: Updated `AuthView.vue` and its tests.
- Produces: Verified type-safe, production-buildable frontend output.

- [ ] **Step 1: Run frontend tests**

```bash
pnpm --filter @iot-pillot/web test
```

Expected: Vitest exits 0 with no failed tests.

- [ ] **Step 2: Run type checking**

```bash
pnpm --filter @iot-pillot/web typecheck
```

Expected: `vue-tsc` exits 0.

- [ ] **Step 3: Run the production build**

```bash
pnpm --filter @iot-pillot/web build
```

Expected: `vue-tsc -b` and Vite build both exit 0 and produce the normal frontend `dist` output.

- [ ] **Step 4: Report the verification evidence**

Record the three command results and summarize the behavior change, including that mobile height rules remain natural and that no API or auth-flow code changed.
