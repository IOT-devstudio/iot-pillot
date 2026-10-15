<script setup lang="ts">
/**
 * 顶栏右上角「姓名 + 下拉」菜单（issue #87 统一资料展示）。
 *
 * 把散在各页 PageHeader#actions 里的 SignOutButton 收进同一个入口：
 *   - 个人设置（/user/settings）
 *   - 退出登录
 *
 * 姓名从 GET /me 拿，失败时降级显示「未登录」并禁用操作——和 useAdminPermissions
 * 挡「撤销自己」的保守策略同思路：拿不到身份时不假装能操作。
 *
 * el-popover trigger="click"：hover 在 sticky header 上容易误触，且要支持触屏。
 */
import { computed, inject, onMounted, ref } from "vue";
import { useRouter } from "vue-router";

import { AuthRequestError } from "@/api/auth";
import { fetchMyProfile } from "@/api/profile";
import { beginProfileLoad, currentProfile as user, profileDisplayName } from "@/modules/user-settings/profileIdentity";
import { signOut } from "@/auth/session";
import { SIGNAL_TRANSITION } from "@/modules/swiss-signal/context";

const router = useRouter();
/**
 * 壳层的擦除过渡（issue #94）：进入个人设置与「返回用户首页」共用同一款
 * 登录转场动画。壳外使用（理论上不会发生）时降级为直接跳转。
 */
const transition = inject(
  SIGNAL_TRANSITION,
  async (change: () => unknown) => {
    await change();
  },
);
const displayName = computed(() => profileDisplayName(user.value));
/**
 * 创始管理员（#120）：无邮箱、不完善资料、不走自助改密——隐藏「个人设置」
 * 入口。后端 is_founding 字段就绪前恒为 false，菜单行为不变。
 */
const isFounding = computed(() => user.value?.is_founding === true);
const loadError = ref("");
const signingOut = ref(false);
const popoverVisible = ref(false);

async function loadUser(): Promise<void> {
  const publish = beginProfileLoad();
  try {
    publish(await fetchMyProfile());
    loadError.value = "";
  } catch (error: unknown) {
    loadError.value =
      error instanceof AuthRequestError ? error.message : "暂时无法加载用户信息";
    publish(null);
  }
}

onMounted(loadUser);

/**
 * 跳到个人设置页，并主动收起弹层。
 * 主动收起是为了避免「点击 RouterLink 之后弹层还在原位」这种半透明盖在目标
 * 页上的视觉残留；用户期望点完链接就干净了。
 */
function openSettings(): void {
  popoverVisible.value = false;
  void transition(() => router.push("/user/settings"));
}

async function handleSignOut(): Promise<void> {
  if (signingOut.value) {
    return;
  }
  signingOut.value = true;
  popoverVisible.value = false;
  try {
    await signOut();
  } catch {
    // signOut 已在 finally 中清掉本地会话；网络失败也不应让用户继续留在此页面
  } finally {
    signingOut.value = false;
    await router.replace("/");
  }
}

/** 角色徽章的中文标签。 */
const ROLE_LABELS: Record<string, string> = {
  admin: "管理员",
  member: "成员",
};

function roleLabel(): string {
  return user.value === null ? "" : (ROLE_LABELS[user.value.role] ?? user.value.role);
}
</script>

<template>
  <el-popover
    v-model:visible="popoverVisible"
    placement="bottom-end"
    :width="220"
    trigger="click"
    :hide-after="0"
  >
    <template #reference>
      <button
        type="button"
        class="user-menu"
        :aria-label="user === null ? '未登录' : `${displayName} 的菜单`"
      >
        <span class="user-menu__avatar" aria-hidden="true">
          {{ user === null ? "?" : Array.from(displayName)[0]?.toUpperCase() }}
        </span>
        <span class="user-menu__meta">
          <span class="user-menu__name">
            {{ user === null ? "未登录" : displayName }}
          </span>
          <span v-if="user !== null" class="user-menu__role">{{ roleLabel() }}</span>
          <span v-else-if="loadError" class="user-menu__role user-menu__role--err">
            {{ loadError }}
          </span>
        </span>
        <span class="user-menu__chevron" aria-hidden="true">▾</span>
      </button>
    </template>

    <div class="user-menu__panel" role="menu">
      <template v-if="!isFounding">
        <button
          type="button"
          class="user-menu__item"
          role="menuitem"
          :disabled="user === null"
          @click="openSettings"
        >
          <span class="user-menu__item-label">个人设置</span>
          <span class="user-menu__item-hint">资料 · 方向</span>
        </button>
        <hr class="user-menu__sep" />
      </template>
      <button
        type="button"
        class="user-menu__item user-menu__item--danger"
        role="menuitem"
        :disabled="user === null || signingOut"
        @click="handleSignOut"
      >
        {{ signingOut ? "退出中…" : "退出登录" }}
      </button>
    </div>
  </el-popover>
</template>

<style scoped>
.user-menu {
  display: inline-flex;
  gap: 10px;
  align-items: center;
  padding: 4px 10px 4px 4px;
  color: var(--ink);
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  cursor: pointer;
  font: inherit;
  transition: border-color 160ms ease, background 160ms ease;
}

.user-menu:hover {
  border-color: var(--line);
  background: rgba(255, 255, 255, 0.6);
}

.user-menu__avatar {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border: 1px solid var(--blue);
  border-radius: 50%;
  color: var(--blue);
  background: #fff;
  font-size: 13px;
  font-weight: 700;
}

.user-menu__meta {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0;
  line-height: 1.15;
}

.user-menu__name {
  font-size: 13px;
  font-weight: 600;
}

.user-menu__role {
  color: var(--ink-faint);
  font-size: 10px;
  letter-spacing: 0.04em;
}

.user-menu__role--err {
  color: #a33d32;
}

.user-menu__chevron {
  margin-left: 2px;
  color: var(--ink-soft);
  font-size: 11px;
}

.user-menu__panel {
  display: flex;
  flex-direction: column;
  padding: 4px;
}

.user-menu__item {
  display: flex;
  gap: 8px;
  align-items: baseline;
  justify-content: space-between;
  width: 100%;
  padding: 9px 10px;
  color: var(--ink);
  border: 0;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  text-align: left;
}

.user-menu__item:hover:not(:disabled) {
  background: rgba(22, 77, 128, 0.08);
}

.user-menu__item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.user-menu__item-label {
  font-weight: 600;
}

.user-menu__item-hint {
  color: var(--ink-faint);
  font-size: 11px;
}

.user-menu__item--danger {
  color: #a33d32;
}

.user-menu__item--danger:hover:not(:disabled) {
  background: rgba(163, 61, 50, 0.08);
}

.user-menu__sep {
  margin: 4px 6px;
  border: 0;
  border-top: 1px solid var(--line);
}

@media (max-width: 640px) {
  .user-menu__meta {
    display: none;
  }
}
</style>
