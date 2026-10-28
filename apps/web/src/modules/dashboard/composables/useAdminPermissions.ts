/**
 * 「用户与权限」页的编排：分页用户列表 + 管理员名单 + 角色变更。
 *
 * 从视图里抽出来，和 modules/recruitment/composables 一个路子：依赖通过参数注入，
 * 测试里塞假实现就能跑，不必起网络、也不必起 Vue 组件。
 *
 * 两处接口（用户列表 / 管理员名单）**各自独立加载**：一个挂了不该把另一个也拖成空表。
 * 令牌完全交给 api/session-request.ts（统一带 Bearer、401 刷新、刷新去重），
 * 这里只关心「数据 / 错误 / 加载中」三态。
 */
import { computed, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import { AuthRequestError } from "@/api/auth";
import {
  deleteAdminUser,
  fetchCurrentUser,
  grantAdmin,
  listAdminUsers,
  listAdmins,
  revokeAdmin,
  unfreezeAdminUser,
  type AdminMutation,
  type AdminUser,
  type CurrentUser,
} from "@/api/admin";

/** 角色筛选：全部 / 仅管理员 / 仅普通用户 */
export type RoleFilter = "all" | "admin" | "member";

/** 单次请求失败后给用户的提示，以及是否该清掉本地会话。 */
export interface AuthErrorInfo {
  message: string;
  unauthorized: boolean;
}

/**
 * 把各色失败翻译成用户能看懂的一句话。
 *
 * 401 / 403 / 网络异常必须区分开（验收项）：401 是「登录过期」，403 是「你没有管理权限」，
 * 两者都套同一句会让人以为重新登录就能解决权限问题。网络异常是 status 0
 * （见 api/auth.ts 的 requestAuth：fetch 直接抛错时用 0 兜底）。
 *
 * fallbackMessage：非 AuthRequestError（多为页面自身代码抛的异常）时的兜底文案。
 * 本函数从「用户与权限」页抽出，个人设置等页复用时传各自的措辞，
 * 免得个人资料加载失败也提示「管理数据」。
 */
export function describeAuthError(
  error: unknown,
  fallbackMessage = "暂时无法加载管理数据，请稍后重试",
): AuthErrorInfo {
  if (error instanceof AuthRequestError) {
    if (error.status === 0) {
      return { message: "网络异常，请检查连接后重试", unauthorized: false };
    }
    if (error.status === 401) {
      return { message: "登录状态已失效，请重新登录", unauthorized: true };
    }
    if (error.status === 403) {
      return { message: "当前账号没有管理员权限", unauthorized: false };
    }
    return { message: error.message, unauthorized: false };
  }
  return { message: fallbackMessage, unauthorized: false };
}

/**
 * 一行用户在该显示什么操作。
 *
 * 抽成纯函数是因为「不能撤销自己」是 issue 的硬要求，而这个判定在**拿不到当前
 * 登录人**时会退化：currentUserId 由 /me 得来，若那次请求失败（或列表本身先失败），
 * 它会停在 null，此时 user_id 与任何值比较都为 false，「自己」就认不出来了。
 * 所以这里对 null 显式采取保守策略——
 * 不给出撤销按钮，只解释原因，绝不把用户放进一个会被服务端 400 拒绝的操作里。
 */
export type RowAction =
  | { kind: "grant" }
  | { kind: "revoke" }
  | { kind: "self" }
  | { kind: "unknown" };

export function rowActionFor(
  user: AdminUser,
  currentUserId: number | null,
): RowAction {
  if (currentUserId === null) {
    return { kind: "unknown" };
  }
  if (user.user_id === currentUserId) {
    return { kind: "self" };
  }
  return user.is_admin ? { kind: "revoke" } : { kind: "grant" };
}

/** 身份未知时给出的解释，和「不能撤销自己」的措辞保持一致以免两种文案打架。 */
export const UNKNOWN_IDENTITY_NOTE = "无法确认当前登录账号，已停用撤销操作";

/**
 * 依赖清单。
 *
 * 全部是**无令牌参数**的：受保护请求统一走 api/session-request.ts，Bearer 头、
 * 401 刷新、刷新去重都在那一层处理，这里再手动读会话传 token 反而是重复且易错
 * 的第二套机制。
 */
export interface AdminPermissionsDeps {
  fetchCurrentUser: () => Promise<CurrentUser>;
  listUsers: (
    page: number,
    pageSize: number,
    includeFrozen: boolean,
  ) => Promise<{ items: AdminUser[]; total: number }>;
  listAdmins: () => Promise<{ items: AdminUser[] }>;
  grantAdmin: (userId: number) => Promise<AdminMutation>;
  revokeAdmin: (userId: number) => Promise<AdminMutation>;
  /** 注销账号（issue #142，仅创始管理员可调；后端另有三重守卫） */
  deleteAdminUser: (userId: number) => Promise<void>;
  /** 解冻账号（0012）：只清冻结标记，资料是否补齐由下一轮扫描判定 */
  unfreezeAdminUser: (userId: number) => Promise<{ user_id: number; is_frozen: boolean }>;
}

const defaultDeps: AdminPermissionsDeps = {
  fetchCurrentUser: () => fetchCurrentUser(),
  listUsers: (page, pageSize, includeFrozen) =>
    listAdminUsers(page, pageSize, includeFrozen),
  listAdmins: () => listAdmins(),
  grantAdmin: (userId) => grantAdmin(userId),
  revokeAdmin: (userId) => revokeAdmin(userId),
  // 后端删除端点不返回 data（见 api/admin.ts 的说明），真实调用返回后端提示语。
  // 这里转成 void：调用方展示自己的确认文案，不需要后端的 message。
  deleteAdminUser: (userId) => deleteAdminUser(userId).then(() => undefined),
  unfreezeAdminUser: (userId) => unfreezeAdminUser(userId),
};

/**
 * 变更成功后的 toast 文案。
 *
 * 后端 message 已经带了「对方需要重新登录」，但 postAuth 只返回 data、不把 message
 * 透传上来，所以这里本地拼一句等价的（要真的用上后端措辞就得改 api 层，不值当）。
 *
 * session_revoked 为 false 说明后端没踢成下线，对方手里的旧令牌仍可能有效，
 * 这个边缘情况既不常见也不该被吞掉，所以额外点一句。
 */
export function describeMutation(mutation: AdminMutation): string {
  const action = mutation.is_admin ? "提升" : "撤销";
  const base = `${action}成功，对方需要重新登录`;
  if (!mutation.session_revoked) {
    return `${base}；旧登录可能仍然有效，待其自然过期后失效`;
  }
  return base;
}

export function useAdminPermissions(deps: AdminPermissionsDeps = defaultDeps) {
  const users = ref<AdminUser[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(20);
  // 初值 true：进页面立刻是「加载中」而不是「暂无数据」。留 false 会在首帧
  // 闪一下空态（实测多数情况被同一帧吃掉，但慢设备/慢网络下会露出来）。
  const usersLoading = ref(true);
  const usersError = ref("");
  /** 加载失败后是否还把「重试」按钮露出来：401 得先重新登录，重试没意义 */
  const usersUnauthorized = ref(false);

  const admins = ref<AdminUser[]>([]);
  const adminsLoading = ref(true);
  const adminsError = ref("");
  const adminsUnauthorized = ref(false);

  /** 当前登录用户的 user_id：用来挡住「撤销自己」这个自锁操作 */
  const currentUserId = ref<number | null>(null);
  /** 当前登录用户是否创始管理员（issue #142 Q1）：只有创始能看到「注销」按钮 */
  const isFounding = ref(false);
  /** 正在变更中的 user_id 集合，用于把对应行的按钮置灰 */
  const busyIds = ref<Set<number>>(new Set());

  const keyword = ref("");
  const roleFilter = ref<RoleFilter>("all");
  /**
   * 是否把已冻结账号纳入列表（0012）。
   *
   * 默认 false：冻结是系统按「资料不合规」自动做的处置，混在日常名单里
   * 会淹没真正在推进招新的人。开关交给后端过滤（include_frozen），
   * 而不是前端把已有行筛掉——否则分页 total 与可翻页数会对不上。
   */
  const showFrozen = ref(false);

  const pageCount = computed(() =>
    Math.max(1, Math.ceil(total.value / pageSize.value)),
  );

  /** 关键词 + 角色筛选**只作用于当前页**：后端这两个接口不接受任何查询参数。 */
  const filteredUsers = computed(() => {
    const kw = keyword.value.trim().toLowerCase();
    return users.value.filter((user) => {
      const matchKeyword = !kw || user.name.toLowerCase().includes(kw);
      const matchRole =
        roleFilter.value === "all" ||
        (roleFilter.value === "admin" ? user.is_admin : !user.is_admin);
      return matchKeyword && matchRole;
    });
  });

  function isSelf(userId: number): boolean {
    return currentUserId.value !== null && currentUserId.value === userId;
  }

  /** 当前行该显示哪种操作（见 rowActionFor 的保守策略说明）。 */
  function actionFor(user: AdminUser): RowAction {
    return rowActionFor(user, currentUserId.value);
  }

  function isBusy(userId: number): boolean {
    return busyIds.value.has(userId);
  }

  function markBusy(userId: number, busy: boolean): void {
    const next = new Set(busyIds.value);
    if (busy) {
      next.add(userId);
    } else {
      next.delete(userId);
    }
    busyIds.value = next;
  }

  /**
   * 确认当前登录人，只为拿到 user_id 挡「撤销自己」。
   *
   * 令牌的事一概不管：受保护请求由 api/session-request.ts 统一带 Bearer 并处理
   * 401 刷新（含并发去重）。这里失败只影响「认不认得出自己」，不影响取数——
   * currentUserId 停在 null 时，rowActionFor 会对「自锁」采取保守策略。
   *
   * 由 loadAll 调一次即可：currentUserId 在两次加载之间不会变，翻页/重试没必要重取。
   */
  async function loadCurrentUser(): Promise<void> {
    try {
      const me = await deps.fetchCurrentUser();
      currentUserId.value = me.user_id;
      // 创始标记只作入口可见性（issue #142 Q1）；后端 RequireFounding 才是强制
      isFounding.value = me.is_founding === true;
    } catch {
      currentUserId.value = null;
      isFounding.value = false;
    }
  }

  async function loadUsers(): Promise<void> {
    usersLoading.value = true;
    try {
      const result = await deps.listUsers(
        page.value,
        pageSize.value,
        showFrozen.value,
      );
      users.value = result.items;
      total.value = result.total;
      usersError.value = "";
      usersUnauthorized.value = false;
    } catch (error: unknown) {
      const info = describeAuthError(error);
      usersError.value = info.message;
      usersUnauthorized.value = info.unauthorized;
      users.value = [];
      total.value = 0;
    } finally {
      usersLoading.value = false;
    }
  }

  /**
   * 切换「显示冻结账号」并重新取数。
   *
   * 回到第 1 页：过滤变化后 total 会变，停在原页码可能落到最后一页之外，
   * 表现为「打开开关后一片空白」。
   */
  async function toggleFrozenVisibility(next?: boolean): Promise<void> {
    showFrozen.value = next ?? !showFrozen.value;
    page.value = 1;
    await loadUsers();
  }

  async function loadAdmins(): Promise<void> {
    adminsLoading.value = true;
    try {
      const result = await deps.listAdmins();
      admins.value = result.items;
      adminsError.value = "";
      adminsUnauthorized.value = false;
    } catch (error: unknown) {
      const info = describeAuthError(error);
      adminsError.value = info.message;
      adminsUnauthorized.value = info.unauthorized;
      admins.value = [];
    } finally {
      adminsLoading.value = false;
    }
  }

  /** 两个区块一起重新加载。守卫只会拦一次，保证初始视图两边都是同一时刻的数据。 */
  /** 进页面时的首次加载：取一次身份 + 两张表，三者并发。 */
  async function loadAll(): Promise<void> {
    await Promise.all([loadCurrentUser(), loadUsers(), loadAdmins()]);
  }

  /**
   * 变更成功后刷新两张表（验收：列表与管理员名单同步刷新）。
   *
   * /me 与它们并发再取一次：变更可能刚让角色变动生效，顺便刷新 currentUserId，
   * 免得「自己」那一行的禁用态停在旧身份上。失败也不会拖垮两张表。
   */
  async function refresh(): Promise<void> {
    await Promise.all([loadCurrentUser(), loadUsers(), loadAdmins()]);
  }

  async function changePage(next: number): Promise<void> {
    const target = Math.min(Math.max(1, next), pageCount.value);
    if (target === page.value) {
      return;
    }
    page.value = target;
    await loadUsers();
  }

  async function changePageSize(size: number): Promise<void> {
    pageSize.value = size;
    page.value = 1;
    await loadUsers();
  }

  /**
   * 提升为管理员。确认后执行，与撤销对称。
   */
  async function grant(user: AdminUser): Promise<void> {
    try {
      await ElMessageBox.confirm(
        `将「${user.name}」提升为管理员？对方会被强制退出，需要重新登录。`,
        "提升为管理员",
        { type: "warning", confirmButtonText: "确认提升", cancelButtonText: "取消" },
      );
    } catch {
      return; // 用户取消
    }

    markBusy(user.user_id, true);
    try {
      const mutation = await deps.grantAdmin(user.user_id);
      ElMessage.success(describeMutation(mutation));
      await refresh();
    } catch (error: unknown) {
      ElMessage.error(describeAuthError(error).message);
    } finally {
      markBusy(user.user_id, false);
    }
  }

  /**
   * 撤销管理员。撤销自己会被后端 400 挡住，这里先在前端可见地拦一道，
   * 免得点下去才被拒绝（也解释不清为什么）。
   *
   * 身份未知（currentUserId 仍为 null）时也拦下：此时认不出哪一行是自己，
   * 唯一安全的做法是不发这个请求。
   */
  async function revoke(user: AdminUser): Promise<void> {
    if (currentUserId.value === null) {
      ElMessage.warning(UNKNOWN_IDENTITY_NOTE);
      return;
    }
    if (isSelf(user.user_id)) {
      ElMessage.warning("不能撤销自己的管理员身份，请让另一位管理员操作");
      return;
    }

    try {
      await ElMessageBox.confirm(
        `撤销「${user.name}」的管理员身份？对方会被强制退出，需要重新登录。`,
        "撤销管理员",
        { type: "warning", confirmButtonText: "确认撤销", cancelButtonText: "取消" },
      );
    } catch {
      return; // 用户取消
    }

    markBusy(user.user_id, true);
    try {
      const mutation = await deps.revokeAdmin(user.user_id);
      ElMessage.success(describeMutation(mutation));
      await refresh();
    } catch (error: unknown) {
      ElMessage.error(describeAuthError(error).message);
    } finally {
      markBusy(user.user_id, false);
    }
  }

  /**
   * 注销账号（issue #142 Q4）。
   *
   * 守卫分层：这里拦「自己 / 已注销 / 身份未识别」三个可见分支；
   * 「目标是创始管理员」前端无法可靠识别（列表不暴露 is_founding），
   * 由后端业务错误兜底——弹窗提交后错误原样展示。
   */
  async function deleteUser(user: AdminUser): Promise<void> {
    if (currentUserId.value === null) {
      ElMessage.warning(UNKNOWN_IDENTITY_NOTE);
      return;
    }
    if (isSelf(user.user_id)) {
      ElMessage.warning("不能注销自己的账号");
      return;
    }
    if (user.is_deleted) {
      ElMessage.warning("该账号已注销");
      return;
    }

    try {
      await ElMessageBox.confirm(
        `此操作不可逆：注销「${user.name}」后其无法再登录，历史记录保留。确定注销？`,
        "注销账号",
        {
          type: "warning",
          confirmButtonText: "确认注销",
          cancelButtonText: "取消",
        },
      );
    } catch {
      return; // 用户取消
    }

    markBusy(user.user_id, true);
    try {
      await deps.deleteAdminUser(user.user_id);
      ElMessage.success("账号已注销，其会话已全部失效");
      await refresh();
    } catch (error: unknown) {
      ElMessage.error(describeAuthError(error).message);
    } finally {
      markBusy(user.user_id, false);
    }
  }

  function resetFilters(): void {
    keyword.value = "";
    roleFilter.value = "all";
  }

  /**
   * 解冻账号（0012）。
   *
   * 只清冻结标记、不校验资料是否已补齐：解冻的语义是「允许对方重新登录去改」。
   * 因此确认文案要提醒操作者先确认对方已把资料改对，否则下一轮扫描会再冻结。
   */
  async function unfreezeUser(user: AdminUser): Promise<void> {
    if (!user.is_frozen) {
      ElMessage.warning("该账号当前未被冻结");
      return;
    }

    try {
      await ElMessageBox.confirm(
        `解冻「${user.name}」后其可重新登录。若资料仍不合规，下一轮定时检查会再次冻结。确认解冻？`,
        "解冻账号",
        {
          type: "warning",
          confirmButtonText: "确认解冻",
          cancelButtonText: "取消",
        },
      );
    } catch {
      return; // 用户取消
    }

    markBusy(user.user_id, true);
    try {
      await deps.unfreezeAdminUser(user.user_id);
      ElMessage.success("账号已解冻，该用户可重新登录补全资料");
      await refresh();
    } catch (error: unknown) {
      ElMessage.error(describeAuthError(error).message);
    } finally {
      markBusy(user.user_id, false);
    }
  }

  return {
    // 用户列表
    users,
    total,
    page,
    pageSize,
    pageCount,
    usersLoading,
    usersError,
    usersUnauthorized,
    filteredUsers,
    // 管理员名单
    admins,
    adminsLoading,
    adminsError,
    adminsUnauthorized,
    // 当前登录身份与忙碌状态
    currentUserId,
    isFounding,
    deleteUser,
    isSelf,
    actionFor,
    isBusy,
    UNKNOWN_IDENTITY_NOTE,
    // 筛选
    keyword,
    roleFilter,
    resetFilters,
    // 冻结（0012）
    showFrozen,
    toggleFrozenVisibility,
    unfreezeUser,
    // 动作
    loadAll,
    loadUsers,
    loadAdmins,
    refresh,
    changePage,
    changePageSize,
    grant,
    revoke,
  };
}
