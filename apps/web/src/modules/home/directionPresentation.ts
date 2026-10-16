/**
 * 方向卡片的纯展示元数据（issue #89）。
 *
 * 真实方向来自后端（GET /recruitment/directions，字段只有 code/name/open/
 * sort_order）；首页卡片上的标语、技术栈标签与符号是**前端视觉层**的
 * 东西，不属于接口契约，因此按 code 键控放在这里——后端新增方向时
 * 页面照常渲染（走通用降级样式），只是没有定制标语。
 *
 * ⚠️ 这里不放任何「事实性」内容（人数/年份/成果），与 issue #94 的
 * 「未确认事实不展示」约束一致；summary 只描述技术栈与工作内容。
 */

export interface DirectionPresentation {
  /** 英文行（mono 小字） */
  headline: string;
  /** 技术栈标签行 */
  tag: string;
  /** 卡片右上角符号 */
  symbol: string;
  /** 一句话简介 */
  summary: string;
}

export const DIRECTION_PRESENTATION: Record<string, DirectionPresentation> = {
  "front-end": {
    headline: "INTERFACE / EXPERIENCE",
    tag: "VUE / TYPESCRIPT / EXPERIENCE",
    symbol: "↗",
    summary: "负责工作室的管理后台与用户侧界面，从排版到交互都由前端落地。",
  },
  "back-end": {
    headline: "DATA / SYSTEM",
    tag: "GO / API / POSTGRESQL",
    symbol: "{ }",
    summary: "负责 API、数据存储、邮件流程与部署，让服务稳定而且好维护。",
  },
  agent: {
    headline: "MODEL / WORKFLOW",
    tag: "LLM / TOOLS / WORKFLOW",
    symbol: "⌁",
    summary: "负责大模型接入、工具调用与工作流编排，把智能体能力接进真实项目。",
  },
};

const FALLBACK: DirectionPresentation = {
  headline: "OPEN / RECRUITING",
  tag: "JOIN / BUILD / SHIP",
  symbol: "✳",
  summary: "开放报名中，加入我们一起把想法做成作品。",
};

/** 按 code 取展示元数据；后端新增方向时降级为通用样式 */
export function presentationOf(code: string): DirectionPresentation {
  return DIRECTION_PRESENTATION[code] ?? FALLBACK;
}

/* ------------------------------------------------------------------ *
 * 面试路径置灰判定（issue #136）
 * ------------------------------------------------------------------ */

/** 方向绑定的面试表单；与 api/recruitment.ts 的 DirectionForm 同构 */
export interface DirectionFormLike {
  is_open: boolean;
}

/** 判定所需的方向切片：只要 form 字段 */
export interface DirectionWithForm {
  form?: DirectionFormLike | null;
}

/** 判定所需的申请切片：只要 status 字段 */
export interface ApplicationLike {
  status: string;
}

/**
 * 面试按钮的置灰原因；null = 可点。
 *
 * 顺序即优先级（issue #136 §5 边界表）：
 *   1. 方向无表单（form 缺省 / null，含 #135 交付前的字段缺省）
 *   2. 表单未开放
 *   3. 本人申请已到终态（accepted / rejected）
 * applied / reviewing 反而是「可补交面试」的状态，不拦。
 */
export function interviewBlockReason(
  direction: DirectionWithForm | null,
  application: ApplicationLike | null,
): string | null {
  const form = direction?.form ?? null;
  if (form === null) return "该方向暂未开放面试";
  if (!form.is_open) return "面试表单未开放";
  const status = application?.status;
  if (status === "accepted" || status === "rejected") return "申请已结束";
  return null;
}
