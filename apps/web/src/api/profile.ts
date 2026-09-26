/**
 * 个人资料接口（GET/PUT /api/v1/me 的扩展字段）。
 *
 * 后端契约详见 issue #57：
 *   - GET /me   → data 含 name + detail { class, student_id, qq, direction, email }
 *   - PUT /me   → body 只接受资料字段；不接受 user_id；改的永远是令牌本人
 *
 * 前端接入遵循与 admin.ts 同一原则：受保护请求一律走
 * apps/web/src/api/session-request.ts（Bearer + 401 刷新 + 刷新去重），
 * 这里不重复读 token。
 */
import type { Direction } from "@iot-pillot/shared-types";

import { requestWithSession } from "./session-request";
import type { UserRole } from "./admin";

/**
 * 用户的资料字段。
 *
 * 三态合一：未设置 / 已清空 / 已填。`undefined` = 后端没返这个字段；
 * `null` = 后端认为已清空（PUT body 用 `null` 表示清除）；`string` /
 * `number` 等才是真实值。前端 hydrate 时只区分「有值 vs 没值」，
 * 不会因为 null 单独走一条分支。
 */
export interface UserDetail {
  class?: string | null;
  student_id?: number | null;
  qq?: string | null;
  direction?: Direction | null;
  email?: string | null;
}

/**
 * GET /me 扩展后的完整形状：name + detail 都在 data 内。
 * detail 可选：#57 后端落地前 /me 不返回它，前端按空资料渲染而不是崩（见 detailToForm）。
 */
export interface MyProfile {
  user_id: number;
  username: string;
  role: UserRole | string;
  name?: string;
  detail?: UserDetail;
}

/** PUT /me 入参：只接受资料字段；空值表示「清空该字段」，未传表示「不动该字段」。 */
export interface MyProfileUpdate {
  class?: string | null;
  student_id?: number | null;
  qq?: string | null;
  direction?: Direction | null;
}

/** 读取自己的完整资料。 */
export function fetchMyProfile(): Promise<MyProfile> {
  return requestWithSession<MyProfile>("/api/v1/me");
}

/** 更新自己的资料。body 不允许带 user_id，由后端从令牌取本人。 */
export function updateMyProfile(input: MyProfileUpdate): Promise<MyProfile> {
  return requestWithSession<MyProfile>("/api/v1/me", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

/**
 * 把 detail 摊平成表单字段。空字符串视为「未填」而非「空值」——
 * 表单的 input 控件天然就是字符串，所以这里统一规范化，提交时
 * 空串变 null，让后端知道「用户清掉了这个字段」而不是「存了一个空串」。
 */
export interface FormShape {
  class: string;
  studentId: string;
  qq: string;
  direction: Direction | "";
}

export function detailToForm(detail: UserDetail = {}): FormShape {
  // student_id === 0 按「未填」处理（与 #57 后端契约一致）；编辑表单里空串比「0」更诚实
  const hasStudentId =
    detail.student_id !== undefined && detail.student_id !== null && detail.student_id !== 0;
  return {
    class: detail.class ?? "",
    studentId: hasStudentId ? String(detail.student_id) : "",
    qq: detail.qq ?? "",
    direction: detail.direction ?? "",
  };
}

/**
 * 把表单反向摊平回 update payload。空串 → null（清空），非空 → 原值。
 *
 * 学号做一次「必须是正整数」的兜底：input type=number 已经能挡住大部分非法字符，
 * 但粘贴仍可能塞进来 0 / 负数 / 小数 —— 后端契约里 student_id 是非负整数，
 * 0 在 #57 里被定义为「未填」，所以这里把 0 也归一到 null。
 *
 * 不静默截断小数（3.7 → 3）：用户键入非整数就该在客户端就拒掉，不然保存成功
 * 后看自己资料发现值被悄悄改了，反而更难排查。
 */
export function formToUpdate(form: FormShape): MyProfileUpdate {
  const studentIdRaw = form.studentId.trim();
  let studentId: number | null = null;
  if (studentIdRaw !== "") {
    const parsed = Number(studentIdRaw);
    if (Number.isInteger(parsed) && parsed > 0) {
      studentId = parsed;
    }
  }

  return {
    class: form.class.trim() === "" ? null : form.class.trim(),
    student_id: studentId,
    qq: form.qq.trim() === "" ? null : form.qq.trim(),
    direction: form.direction === "" ? null : form.direction,
  };
}

/**
 * 表单字段是否全部为空（用于决定是否显示「建议补全」提示）。
 * 只检查可编辑字段，不看用户名 / 邮箱（那是登录凭据，不在补全范围）。
 */
export function isProfileEmpty(form: FormShape): boolean {
  return (
    form.class.trim() === "" &&
    form.studentId.trim() === "" &&
    form.qq.trim() === "" &&
    form.direction === ""
  );
}