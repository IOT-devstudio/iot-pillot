<script setup lang="ts">
/**
 * 工作室成员名录（#195）。
 *
 * 闭环：录入 → 列表筛选 → 编辑 → 归档（可恢复为在读）。
 * 全部接口仅管理员可用（ADR 0001 §权限拍板）；非 admin 由路由守卫与
 * 后端 403 双重拦截，页面不额外做角色判断。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import {
  archiveStudioMember,
  createStudioMember,
  listStudioMembers,
  updateStudioMember,
  type StudioMember,
  type StudioMemberFilters,
} from "@/api/studio-members";
import {
  EMPTY_STUDIO_MEMBER_DRAFT,
  STUDIO_DIRECTION_OPTIONS,
  archiveConfirmText,
  describeStudioMemberError,
  draftErrors,
  studioDirectionLabel,
  toCreateInput,
  toDraft,
  toUpdateInput,
  type StudioMemberDraft,
} from "../model";
import "../studio-members.css";

/* ── 列表 ─────────────────────────────────────────────────────── */

const members = ref<StudioMember[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;

const loading = ref(false);
const loadError = ref("");
/** 列表区动作（归档 / 恢复）的错误。 */
const actionError = ref("");
/** 抽屉里保存的错误——与列表错误分开，否则保存失败会同时弹在两处。 */
const saveError = ref("");

/** 筛选条件：关键词 + 方向 + 三态在读状态。 */
const keyword = ref("");
const directionFilter = ref("");
/** "" = 全部；"active" = 仅在读；"archived" = 仅已归档 */
const activeFilter = ref<"" | "active" | "archived">("");

const filters = computed<StudioMemberFilters>(() => ({
  q: keyword.value,
  direction: directionFilter.value,
  ...(activeFilter.value === ""
    ? {}
    : { is_active: activeFilter.value === "active" }),
}));

/** 是否有生效中的筛选（决定空态文案是「没数据」还是「没命中」）。 */
const hasActiveFilters = computed(
  () =>
    keyword.value.trim() !== "" ||
    directionFilter.value !== "" ||
    activeFilter.value !== "",
);

/** 正在提交归档的成员 id，用于禁用按钮防重复提交。 */
const archivingID = ref<number | null>(null);

async function load(nextPage = page.value): Promise<void> {
  if (loading.value) return;
  loading.value = true;
  loadError.value = "";
  try {
    const data = await listStudioMembers(filters.value, nextPage, pageSize);
    members.value = data.items;
    total.value = data.total;
    page.value = data.page;
  } catch (error) {
    loadError.value = describeStudioMemberError(error);
  } finally {
    loading.value = false;
  }
}

onMounted(() => void load());

/** 筛选条件变化：回到第 1 页重新查（否则会停在越界页拿到空列表）。 */
function applyFilters(): void {
  void load(1);
}

function resetFilters(): void {
  keyword.value = "";
  directionFilter.value = "";
  activeFilter.value = "";
  void load(1);
}

/* ── 表单（录入 / 编辑共用同一个抽屉） ─────────────────────────── */

const dialogOpen = ref(false);
const editing = ref<StudioMember | null>(null);
const draft = ref<StudioMemberDraft>({ ...EMPTY_STUDIO_MEMBER_DRAFT });
const fieldErrors = ref<Record<string, string>>({});
const saving = ref(false);

const isEditing = computed(() => editing.value !== null);
const dialogTitle = computed(() =>
  isEditing.value ? `编辑「${editing.value?.name}」` : "录入工作室成员",
);

function openCreate(): void {
  editing.value = null;
  draft.value = { ...EMPTY_STUDIO_MEMBER_DRAFT };
  fieldErrors.value = {};
  saveError.value = "";
  dialogOpen.value = true;
}

function openEdit(member: StudioMember): void {
  editing.value = member;
  draft.value = toDraft(member);
  fieldErrors.value = {};
  saveError.value = "";
  dialogOpen.value = true;
}

async function save(): Promise<void> {
  if (saving.value) return;
  fieldErrors.value = draftErrors(draft.value);
  if (Object.keys(fieldErrors.value).length > 0) return;

  saving.value = true;
  saveError.value = "";
  try {
    if (editing.value) {
      const patch = toUpdateInput(draft.value, editing.value);
      // 没有任何改动就不发请求：后端会照常返回 200，
      // 但白跑一次写库并刷新 updated_at 会误导「谁改过这条」。
      if (Object.keys(patch).length === 0) {
        ElMessage.info("没有需要保存的改动");
        dialogOpen.value = false;
        return;
      }
      await updateStudioMember(editing.value.id, patch);
      ElMessage.success("成员已更新");
    } else {
      await createStudioMember(toCreateInput(draft.value));
      ElMessage.success("成员已录入");
    }
    dialogOpen.value = false;
    await load();
  } catch (error) {
    saveError.value = describeStudioMemberError(error);
  } finally {
    saving.value = false;
  }
}

/* ── 归档 / 恢复 ──────────────────────────────────────────────── */

async function archive(member: StudioMember): Promise<void> {
  if (archivingID.value !== null) return;
  try {
    await ElMessageBox.confirm(archiveConfirmText(member), "归档成员", {
      confirmButtonText: "归档",
      cancelButtonText: "取消",
      type: "warning",
    });
  } catch {
    // 用户取消：不是错误，什么都不做。
    return;
  }

  archivingID.value = member.id;
  actionError.value = "";
  try {
    await archiveStudioMember(member.id);
    ElMessage.success(`已归档「${member.name}」`);
    await load();
  } catch (error) {
    actionError.value = describeStudioMemberError(error);
  } finally {
    archivingID.value = null;
  }
}

/** 恢复在读：归档的反向动作，走同一个 PUT（is_active=true 会清归档时刻）。 */
async function restore(member: StudioMember): Promise<void> {
  if (archivingID.value !== null) return;
  archivingID.value = member.id;
  actionError.value = "";
  try {
    await updateStudioMember(member.id, { is_active: true });
    ElMessage.success(`已恢复「${member.name}」为在读`);
    await load();
  } catch (error) {
    actionError.value = describeStudioMemberError(error);
  } finally {
    archivingID.value = null;
  }
}

/* ── 展示辅助 ─────────────────────────────────────────────────── */

/** 归档时刻只展示到日：名录里精确到秒没有意义。 */
function archivedOn(member: StudioMember): string {
  const value = member.archived_at;
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value.slice(0, 10);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}
</script>

<template>
  <div class="page studio-members-page">
    <main class="page__body">
      <PageHeader
        eyebrow="工作室运营 · 成员名录"
        title="成员名录"
        description="收录在读与已毕业的工作室成员；无系统账号的校友也可以建档。"
        :meta="`共 ${total} 人`"
      >
        <template #actions>
          <button class="studio-button studio-button--primary" type="button" @click="openCreate">
            录入成员 ↗
          </button>
        </template>
      </PageHeader>

      <!-- 筛选条：关键词 / 方向 / 在读状态 -->
      <section class="panel panel--pad studio-filters" aria-label="成员筛选">
        <label class="studio-field">
          <span class="studio-field__label">关键词</span>
          <input
            v-model="keyword"
            class="studio-control"
            type="search"
            placeholder="姓名 / 届别 / 方向 / 职能"
            @keyup.enter="applyFilters"
          />
        </label>
        <label class="studio-field">
          <span class="studio-field__label">方向</span>
          <select v-model="directionFilter" class="studio-control">
            <option value="">全部方向</option>
            <option v-for="option in STUDIO_DIRECTION_OPTIONS" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
        <label class="studio-field">
          <span class="studio-field__label">在读状态</span>
          <select v-model="activeFilter" class="studio-control">
            <option value="">全部（含已归档）</option>
            <option value="active">仅在读</option>
            <option value="archived">仅已归档</option>
          </select>
        </label>
        <div class="studio-filters__actions">
          <button class="studio-button" type="button" @click="applyFilters">筛选</button>
          <button class="studio-button" type="button" @click="resetFilters">重置</button>
        </div>
      </section>

      <p v-if="actionError" class="studio-error" role="alert">{{ actionError }}</p>

      <!-- 加载 / 错误 / 空态 -->
      <section v-if="loadError" class="panel panel--pad" role="alert">
        <p class="studio-error">{{ loadError }}</p>
        <button class="studio-button" type="button" @click="load()">重试</button>
      </section>
      <p v-else-if="loading" class="panel panel--pad" role="status">正在加载成员名录…</p>
      <!--
        空态要分两种：「名录本来就是空的」和「筛选没命中」。
        混成一句「名录还是空的」会让管理员在筛选之后误以为数据丢了，
        然后去重复录入同一个人（撞唯一键又看不懂为什么失败）。
      -->
      <section v-else-if="!members.length" class="panel panel--pad studio-empty">
        <template v-if="hasActiveFilters">
          <span class="studio-kicker">NO MATCH</span>
          <h2>没有符合筛选条件的成员。</h2>
          <p>换个关键词或方向试试；已归档的成员默认也在结果里。</p>
          <button class="studio-button" type="button" @click="resetFilters">清空筛选</button>
        </template>
        <template v-else>
          <span class="studio-kicker">START WITH A NAME</span>
          <h2>名录还是空的。</h2>
          <p>录入第一位成员；没有系统账号的校友也可以直接建档。</p>
          <button class="studio-button" type="button" @click="openCreate">录入成员</button>
        </template>
      </section>

      <!-- 名录表格 -->
      <div v-else class="panel studio-table-wrap">
        <table class="studio-table">
          <caption class="studio-sr-only">工作室成员名录</caption>
          <thead>
            <tr>
              <th scope="col">姓名</th>
              <th scope="col">届别</th>
              <th scope="col">方向</th>
              <th scope="col">工作室职能</th>
              <th scope="col">实习 / 就业</th>
              <th scope="col">状态</th>
              <th scope="col"><span class="studio-sr-only">操作</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="member in members" :key="member.id">
              <td>
                <strong class="studio-table__name">{{ member.name }}</strong>
                <span class="studio-table__sub">{{ member.birth_year }} 年生</span>
              </td>
              <td>{{ member.cohort }}</td>
              <td>{{ studioDirectionLabel(member.direction) }}</td>
              <td>{{ member.role_in_studio }}</td>
              <td>
                <span v-if="member.internship">{{ member.internship }}</span>
                <span v-if="member.internship && member.employment"> · </span>
                <span v-if="member.employment">{{ member.employment }}</span>
                <span v-if="!member.internship && !member.employment" class="studio-muted">—</span>
              </td>
              <td>
                <span class="studio-status" :class="{ 'studio-status--archived': !member.is_active }">
                  {{ member.is_active ? "在读" : "已归档" }}
                </span>
                <span v-if="!member.is_active && member.archived_at" class="studio-table__sub">
                  {{ archivedOn(member) }}
                </span>
              </td>
              <td class="studio-table__actions">
                <div class="studio-table__actions-inner">
                  <button class="studio-button studio-button--small" type="button" @click="openEdit(member)">
                    编辑
                  </button>
                  <button
                    v-if="member.is_active"
                    class="studio-button studio-button--small"
                    type="button"
                    :disabled="archivingID !== null"
                    :aria-busy="archivingID === member.id"
                    @click="archive(member)"
                  >
                    归档
                  </button>
                  <button
                    v-else
                    class="studio-button studio-button--small"
                    type="button"
                    :disabled="archivingID !== null"
                    :aria-busy="archivingID === member.id"
                    @click="restore(member)"
                  >
                    恢复在读
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <el-pagination
        v-if="total > pageSize"
        layout="prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        :disabled="loading"
        @current-change="load"
      />

      <!-- 录入 / 编辑抽屉 -->
      <el-dialog v-model="dialogOpen" :title="dialogTitle" width="min(560px, 92vw)">
        <form class="studio-form" novalidate @submit.prevent="save">
          <label class="studio-field">
            <span class="studio-field__label">姓名 <em>必填</em></span>
            <input
              v-model="draft.name"
              class="studio-control"
              :aria-invalid="Boolean(fieldErrors.name)"
              placeholder="中文名或英文名均可"
            />
            <span v-if="fieldErrors.name" class="studio-field__error">{{ fieldErrors.name }}</span>
          </label>

          <label class="studio-field">
            <span class="studio-field__label">出生年 <em>必填</em></span>
            <input
              v-model="draft.birth_year"
              class="studio-control"
              inputmode="numeric"
              placeholder="如 2003（与姓名共同构成唯一标识）"
              :aria-invalid="Boolean(fieldErrors.birth_year)"
            />
            <span v-if="fieldErrors.birth_year" class="studio-field__error">{{ fieldErrors.birth_year }}</span>
          </label>

          <label class="studio-field">
            <span class="studio-field__label">届别 <em>必填</em></span>
            <input
              v-model="draft.cohort"
              class="studio-control"
              placeholder="如 2024 级"
              :aria-invalid="Boolean(fieldErrors.cohort)"
            />
            <span v-if="fieldErrors.cohort" class="studio-field__error">{{ fieldErrors.cohort }}</span>
          </label>

          <label class="studio-field">
            <span class="studio-field__label">方向 <em>必填</em></span>
            <input
              v-model="draft.direction"
              class="studio-control"
              list="studio-direction-options"
              placeholder="如 front-end"
              :aria-invalid="Boolean(fieldErrors.direction)"
            />
            <datalist id="studio-direction-options">
              <option v-for="option in STUDIO_DIRECTION_OPTIONS" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </datalist>
            <span v-if="fieldErrors.direction" class="studio-field__error">{{ fieldErrors.direction }}</span>
          </label>

          <label class="studio-field">
            <span class="studio-field__label">工作室职能 <em>必填</em></span>
            <input
              v-model="draft.role_in_studio"
              class="studio-control"
              placeholder="如 前端负责人 / 成员"
              :aria-invalid="Boolean(fieldErrors.role_in_studio)"
            />
            <span v-if="fieldErrors.role_in_studio" class="studio-field__error">{{ fieldErrors.role_in_studio }}</span>
          </label>

          <label class="studio-field">
            <span class="studio-field__label">实习去向</span>
            <input v-model="draft.internship" class="studio-control" placeholder="选填" />
            <span v-if="fieldErrors.internship" class="studio-field__error">{{ fieldErrors.internship }}</span>
          </label>

          <label class="studio-field">
            <span class="studio-field__label">就业去向</span>
            <input v-model="draft.employment" class="studio-control" placeholder="选填" />
            <span v-if="fieldErrors.employment" class="studio-field__error">{{ fieldErrors.employment }}</span>
          </label>

          <label class="studio-field">
            <span class="studio-field__label">系统账号 ID</span>
            <input
              v-model="draft.user_id"
              class="studio-control"
              inputmode="numeric"
              placeholder="选填；留空表示无系统账号的校友 / 外部成员"
              :aria-invalid="Boolean(fieldErrors.user_id)"
            />
            <span v-if="fieldErrors.user_id" class="studio-field__error">{{ fieldErrors.user_id }}</span>
          </label>

          <p v-if="saveError" class="studio-error" role="alert">{{ saveError }}</p>

          <div class="studio-form__actions">
            <button class="studio-button" type="button" @click="dialogOpen = false">取消</button>
            <button class="studio-button studio-button--primary" type="submit" :disabled="saving">
              {{ saving ? "保存中…" : "保存" }}
            </button>
          </div>
        </form>
      </el-dialog>
    </main>
  </div>
</template>
