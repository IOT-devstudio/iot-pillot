# ADR 0005：正文 Markdown 在前端渲染成 VNode，不产生 HTML 字符串

## 状态

已确认（2026-10-30）。来自地图 #207「[探索地图] 招新问答与学习路线」的决策票 #214「[前端] Markdown 渲染与 XSS 防护方案定稿」。

**未落地**：本文只记录渲染层的决策，代码见后续实施票。

## 上下文

ADR 0003 已定内容模型（帖子 / 回复 / 学习路线共用一套正文），ADR 0004 已定正文里用标准 Markdown 图片语法指向附件读取端点。剩下没定的是：谁来把 Markdown 变成可看的东西，以及 XSS 怎么挡。

调研确认的事实：

- 前端零 Markdown 依赖：`apps/web/package.json` 无 `markdown-it` / `marked` / `DOMPurify`，`pnpm-lock.yaml` 里也没有。
- 后端零 Markdown 依赖：`apps/api/go.mod` 无 `goldmark` / `blackfriday`；`go.sum` 里 `goldmark v1.4.13/go.mod` 只是 `swag` 传递依赖的残留，没有进 `require`。
- 全仓只有两处 `v-html`，都在邮件模块，且 `apps/web/src/modules/mail/components/MailHtmlPreview.vue:5-8` 的注释明确写了「用带 sandbox 的 iframe srcdoc 渲染而不是 div v-html」。这个仓库的既有价值观是**能不用 `v-html` 就不用**。
- 全仓无 CSP 响应头（`deploy/nginx.conf` 与后端均未设置），所以不能指望浏览器兜底。
- 正文里嵌的图片路径是 `/api/v1/attachments/123`（ADR 0004），**不是**公开 URL。

本票的取舍全部围绕一个判断：**渲染的安全性应该由「不存在危险的东西」保证，还是由「过滤掉危险的东西」保证**。

## 决策

### 1. 前端渲染，数据库只存 Markdown 原文

正文在浏览器里渲染，服务端不渲染、不存 HTML。

- 存 HTML 会把展示逻辑变成需要迁移、需要版本化的数据：附件 URL 规则一变就得回填全部历史正文。
- 存 Markdown 则渲染层改一次全站生效，历史内容自动跟着变。
- 编辑体验要求原文可再编辑；存 HTML 就得写 HTML→Markdown 的反向转换（重灾区），或者存两份互相同步。

**代价**：服务端无法保证各客户端渲染一致。本期只有 Vue 一个客户端，这个代价暂时不存在；将来若出现第二个客户端，一致性靠这份 ADR 与共享的渲染规则，而不是靠服务端渲染。

### 2. `markdown-it` 只做解析，token 流自己映射成 Vue VNode

用 `markdown-it` 的 `md.parse()` 拿 token 数组，**不调** `md.render()`。遍历 token 直接产出 VNode 树。

**全程不产生 HTML 字符串**，因此：

- 不用 `v-html`，`innerHTML` 在这个模块里根本不出现；
- **不引入 DOMPurify** —— 没有字符串可注入，sanitizer 没有存在必要。

这不是「过滤得更彻底」，是**让攻击面不存在**。`html: false` 的 `markdown-it` 已经能拦住原始 HTML 块，但那条防线依赖配置不被改动；VNode 路径下即使有人打开 `html` 选项，`html_block` / `html_inline` token 也只会走到「未知 token 降级」分支变成纯文本。

**代价**：约 150–200 行 token→VNode 映射代码，且 `markdown-it` 的 token 类型演进要跟。收在一个模块里，配单测。

### 3. 图片：本站路径与外链都渲染，统一加两个属性

`![图1](/api/v1/attachments/123)` 渲染成 `<img src="/api/v1/attachments/123" alt="图1">`；外链图片同样渲染成 `<img>`，渲染层不做额外拦截（ADR 0004 已允许外链，且拦截会让「用户写的」与「用户看到的」不一致）。

所有 `<img>` 统一加：

- `loading="lazy"` —— 一屏十几张大图不会同时拉；
- `referrerpolicy="no-referrer"` —— 外链图片拿不到当前页面地址，而内部论坛的帖子 URL 本身就是敏感信息。

### 4. 支持的语法是明确子集，未知 token 降级成纯文本

支持：段落、H1–H3、粗体 / 斜体 / 行内代码 / 删除线、有序与无序列表（单层）、引用块、围栏代码块、链接、图片、分割线、硬换行。

不支持：**表格、脚注、HTML 块、嵌套列表、任务列表**。表格用贴图代替（ADR 0004 已支持），HTML 块正是要结构性排除的东西。

**未知 token 一律降级成纯文本**（取 token 的 `content` 原样插进去），**不丢弃**。丢内容比多显示几个标记符号糟得多——用户会以为自己的正文丢了。

### 5. 链接协议显式白名单，不合规的降级成纯文本

`markdown-it` 默认的 `validateLink` 是黑名单式放行（拦 `javascript:` / `vbscript:` / `file:`，`data:` 只放行几种图片格式）。本票再显式白名单一次：**只允许 `http:` / `https:` / 以 `/` 开头的站内相对路径**。

不认识的协议**保留文字、去掉可点击性**，不整段丢弃。

理由与决策 4 同源：不依赖库的默认值，依赖我们自己的判断；库升级或换插件时行为不会静默变化。

### 6. 编辑 / 预览双 Tab，预览与详情共用同一个组件

`MarkdownContent.vue` 是唯一的渲染组件：详情页直接用它，编辑器的「预览」Tab 也用它。

两套渲染代码的漂移是「预览看起来对、发出去不对」的唯一来源，而这恰恰是最难被测试发现的 bug。共用组件还让 ADR 0004 要求的「图片加载失败占位块」自动在预览里也生效。

不用左右分栏：手机窄屏下不可用，而用户侧问答页必须手机可用。

### 7. 不做代码语法高亮

围栏代码块只给等宽字体 + 浅底色，不引 highlight.js / Shiki。

本期正文里的代码量极少（问答里贴几行配置、学习路线里贴一段示例），而高亮库的包体增量远大于收益。`apps/web/src/modules/mail/utils/highlight.ts` 是关键词搜索高亮，与代码高亮无关，不可复用。

真需要时再开票，那时也有真实样本判断值不值。

### 8. 列表页摘要先剥 Markdown 标记，再走既有 `bodyExcerpt`

`apps/web/src/modules/studio-knowledge/model.ts:139` 的 `bodyExcerpt` 是给纯文本知识库用的（压空白 → 截断 → 加省略号）。直接套到 Markdown 上会把 `**加粗**`、`## 标题`、`![图1](/api/...)` 原样显示。

新增 `markdownExcerpt`：图片语法整体替换成 `[图]`、链接只留文字、去掉行首的 `#` / `>` / `-` / 数字序号、去掉强调与行内代码的标记符号，然后交给 `bodyExcerpt`。

**不引入渲染器**——列表页渲染 20 条正文的 VNode 树纯属浪费，摘要本来也不该有排版。

### 9. 正文长度上限：正文与回复各 20000 字符，标题 120 字符

本票只定数值，**校验与建表 CHECK 由迁移票落地**（ADR 0003 已定迁移从 `0016` 起）。

20000 是「一篇长技术说明够用、又不会让列表页摘要与全文检索失控」的量级。数值不影响渲染实现（渲染器对 2 万字符与 20 万字符行为一致）。

### 10. 渲染器放在 `apps/web/src/components/markdown/`，不留任何配置项

目录内容：`MarkdownContent.vue` + `renderMarkdown.ts` + `markdownExcerpt.ts` + 就近单测。

放共享位置而非 `modules/forum/`：渲染器至少被招新问答、内部论坛、学习路线三处使用，放论坛模块会让学习路线反过来依赖论坛模块。仓内现有 14 个模块**没有任何跨模块 import**（`grep "from '@/modules"` 零命中），跨模块复用的东西都在 `components/common/`。

**不留配置项**（不开 `options` 参数）：渲染规则的唯一真源是这个模块本身。留了口子，第一个调用方就会为了「我这里正好需要表格」打开它，然后三处渲染行为不一致——而那正是决策 6 要避免的事。

### 11. 正文标题降一级渲染

正文里的 `#` / `##` / ``@ 统一渲染成 `<h2>` / `<h3>` / `<h4>`。

详情页本身已有一个 `<h1>`（帖子标题），正文里再出现 `<h1>` 会产生多个一级标题，屏幕阅读器与 SEO 都会乱。这个降级是渲染器的固定行为，**不暴露给调用方配置**——能配就一定会有人配错。

### 12. 空 alt 视为装饰性图片

`![](...)` 渲染成 `<img alt="">`；占位块文案退化成「图片不可用」，不带补充说明。alt 非空时占位块显示「图片不可用：{alt}」。

空 alt 在无障碍语义里是「这张图是装饰性的」，是正确的默认；硬编一个「图片」当 alt 反而制造噪音。占位块是可见文本，本来就该带上用户写的信息（如果有）。

### 13. 代码块横向滚动，不折行

`<pre>` 加 `overflow-x: auto` 与 `tab-size: 2`，不设 `white-space: pre-wrap`。

代码折行会破坏缩进语义，读起来比滚动更糟。

## 拒绝的方案

| 方案 | 原因 |
| --- | --- |
| 服务端 `goldmark` 渲染，存 HTML | 展示逻辑变成需迁移的数据；URL 规则变化要回填历史正文；Go 侧新增依赖 |
| 服务端渲染但只存 Markdown，每次请求现算 | 每个列表页都要为 20 条正文做一次渲染，CPU 成本换不到任何客户端简化 |
| `md.render()` + `v-html` + DOMPurify | 走了 `v-html` 就有 `innerHTML` 攻击面；将来加一个允许自定义属性的插件，sanitizer 防线会**静默**失效 |
| `html: false` 就够，不需要别的 | 依赖配置不被改动；且 `html: false` 管不住链接协议（`javascript:` 走的是 `validateLink`） |
| 自己写 Markdown 解析器（零新依赖） | 边界情况长期消耗维护成本，且每次边界 bug 都是渲染错误而非安全错误，更难发现 |
| 引入 DOMPurify 作为纵深防御 | VNode 路径下没有 HTML 字符串，sanitizer 是给不存在的攻击面写代码 |
| 外链图片降级成纯链接 | 与 ADR 0004 冲突；贴官方文档截图链接是正常用法 |
| 表格 / 脚注 / 任务列表也支持 | 本期正文用不上；表格有贴图这条更直接的路径；每多一种语法就多一批 XSS 边界要验 |
| 未知 token 直接丢弃 | 用户会以为正文丢了，比多显示几个标记符号糟得多 |
| 左右分栏实时预览 | 手机窄屏不可用，而用户侧问答页必须手机可用 |
| 代码语法高亮（highlight.js / Shiki） | 本期代码量极少，包体增量远大于收益 |
| 列表页直接截 Markdown 原文 | 会把 `**`、`##`、图片语法原样显示出来 |
| 列表页也走渲染器再截 | 20 条正文的 VNode 树纯属浪费，摘要不该有排版 |
| 渲染器放 `modules/forum/` | 学习路线会反向依赖论坛模块，破坏模块边界 |
| 渲染器留 `options` 参数 | 三处渲染行为会漂移，而那正是共用组件要避免的 |
| 正文标题按原文层级渲染 | 页面出现多个 `<h1>`，破坏文档大纲与无障碍语义 |
| 代码块折行（`pre-wrap`） | 破坏缩进语义 |
| 顺手收口邮件模块那处 `v-html` | 威胁模型不同：那里渲染的是管理员自己写的模板，管理员本来就有全站权限；属另一个目的地 |

## 后果

**前端新增 1 个依赖**：`markdown-it`（约 100KB min / 35KB gzip）。不加 DOMPurify，不加代码高亮库。

**新增目录** `apps/web/src/components/markdown/`：渲染组件、渲染函数、摘要函数、就近单测。

**不新增后端依赖**，后端不参与渲染。

**新增术语**：`GLOSSARY.md` 内容域新增「Markdown 渲染器」与「正文摘要」两个词。

**下游约束**：

- 迁移票要给 `forum_posts.title` 加 `CHECK (char_length(title) <= 120)`，给 `forum_posts.body` 与 `forum_replies.body` 加 `CHECK (char_length(body) <= 20000)`。
- 前端表单校验同步这两个数值，但**后端 CHECK 是最终判定**。
- ADR 0004 要求的「图片加载失败占位块」由 `MarkdownContent.vue` 实现一次，预览与详情自动共享。

**可逆性边界**：渲染层换成 `v-html` + sanitizer 只需改一个模块，但「不产生 HTML 字符串」这条一旦被放弃，后续每加一个 Markdown 插件都要重新评估 XSS，且评估失效是静默的。反过来，从 `v-html` 迁到 VNode 是纯收益的重构，没有沉没成本——这也是选当前方案的一个理由：**它在两个方向上都可逆，而对手方案只有一个方向可逆**。

## 参考

- 地图：#207「[探索地图] 招新问答与学习路线」；本票：#214
- 内容模型：`docs/adr/0003-content-scope-shared-model.md`
- 附件与正文图片引用：`docs/adr/0004-attachment-upload-chain.md`
- 既有 `v-html` 规避先例：`apps/web/src/modules/mail/components/MailHtmlPreview.vue:5-8`、`apps/web/src/modules/mail/utils/highlight.ts:5`
- 既有摘要函数：`apps/web/src/modules/studio-knowledge/model.ts:139`
