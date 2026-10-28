<script setup lang="ts">
/**
 * 工作室项目与参与（#198）。
 *
 * 闭环：创建/编辑项目 → 列表筛选 → 项目详情 → 增删改参与行 → 删除项目（级联参与）。
 *
 * 全部接口仅管理员可用；参与行的姓名是**快照**，不随成员档案改名变化。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";

import PageHeader from "@/components/common/PageHeader.vue";
import {
  addStudioProjectMember,
  createStudioProject,
  deleteStudioProject,
  fetchStudioProject,
  listStudioProjects,
  removeStudioProjectMember,
  updateStudioProject,
  updateStudioProjectMember,
  type StudioProject,
  type StudioProjectFilters,
  type StudioProjectMember,
  type StudioProjectMemberRole,
  type StudioProjectStatus,
} from "@/api/studio-projects";
import {
  EMPTY_MEMBER_DRAFT,
  EMPTY_PROJECT_DRAFT,
  MEMBER_LIMITS,
  PROJECT_LIMITS,
  PROJECT_ROLE_OPTIONS,
  PROJECT_STATUS_OPTIONS,
  deleteProjectConfirmText,
  describeProjectError,
  formatDay,
  memberDraftErrors,
  memberEndSummary,
  memberToDraft,
  projectDraftErrors,
  projectExcerpt,
  projectRoleLabel,
  projectStatusLabel,
  removeMemberConfirmText,
  toCreateInput,
  toMemberCreateInput,
  toMemberUpdateInput,
  toUpdateInput,
  projectToDraft,
  isExternalParticipant,
  type MemberDraft,
  type ProjectDraft,
} from "../model";
import "../studio-projects.css";

/* ── 列表 ─────────────────────────────────────────────────────── */

const projects = ref<StudioProject[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;

const loading = ref(false);
const loadError = ref("");
/** 列表区动作（删除项目）的错误。 */
const actionError = ref("");

const keyword = ref("");
const statusFilter = ref<StudioProjectStatus | "">("");
const yearFilter = ref("");

const hasActiveFilters = computed(
  () => keyword.value.trim() !== "" || statusFilter.value !== "" || yearFilter.value.trim() !== "",
);

const filters = computed<StudioProjectFilters>(() => ({
  q: keyword.value,
  status: statusFilter.value,
  year: yearFilter.value,
}));

async function load(nextPage = page.value): Promise<void> {
  if (loading.value) return;
  loading.value = true;
  loadError.value = "";
  try {
    const data = await listStudioProjects(filters.value, nextPage, pageSize);
    projects.value = data.items;
    total.value = data.total;
    page.value = data.page;
  } catch (error) {
    loadError.value = describeProjectError(error);
  } finally {
    loading.value = false;
  }
}

onMounted(() => void load());

function applyFilters(): void {
  void load(1);
}

function resetFilters(): void {
  keyword.value = "";
  statusFilter.value = "";
  yearFilter.value = "";
  void load(1);
}

/* ── 项目弹窗（创建 / 编辑共用） ───────────────────────────────── */

const projectDialogOpen = ref(false);
const editingProject = ref<StudioProject | null>(null);
const projectDraft = ref<ProjectDraft>({ ...EMPTY_PROJECT_DRAFT });
const projectFieldErrors = ref<Record<string, string>>({});
const projectSaving = ref(false);
/** 弹窗里保存的错误——与列表错误分开，否则保存失败会同时弹在两处。 */
const projectSaveError = ref("");

function openProjectCreate(): void {
  editingProject.value = null;
  projectDraft.value = { ...EMPTY_PROJECT_DRAFT };
  projectFieldErrors.value = {};
  projectSaveError.value = "";
  projectDialogOpen.value = true;
}

function openProjectEdit(project: StudioProject): void {
  editingProject.value = project;
  projectDraft.value = projectToDraft(project);
  projectFieldErrors.value = {};
  projectSaveError.value = "";
  projectDialogOpen.value = true;
}

async function saveProject(): Promise<void> {
  if (projectSaving.value) return;
  projectFieldErrors.value = projectDraftErrors(projectDraft.value);
  if (Object.keys(projectFieldErrors.value).length > 0) return;

  projectSaving.value = true;
  projectSaveError.value = "";
  try {
    if (editingProject.value) {
      const patch = toUpdateInput(projectDraft.value, editingProject.value);
      // 除了 status 没有任何改动：不必白跑一次写库。
      if (Object.keys(patch).length === 0) {
        ElMessage.info("没有需要保存的改动");
        projectDialogOpen.value = false;
        return;
      }
      await updateStudioProject(editingProject.value.id, patch);
      ElMessage.success("项目已更新");
    } else {
      await createStudioProject(toCreateInput(projectDraft.value));
      ElMessage.success("项目已创建");
    }
    projectDialogOpen.value = false;
    await load();
  } catch (error) {
    projectSaveError.value = describeProjectError(error);
  } finally {
    projectSaving.value = false;
  }
}

/* ── 项目详情与参与名单 ───────────────────────────────────────── */

const detailOpen = ref(false);
const detailProject = ref<StudioProject | null>(null);
const detailMembers = ref<StudioProjectMember[]>([]);
const detailLoading = ref(false);
const detailError = ref("");
const detailEditing = ref(false);
const detailProjectDraft = ref<ProjectDraft>({ ...EMPTY_PROJECT_DRAFT });
const detailProjectFieldErrors = ref<Record<string, string>>({});
const detailProjectSaving = ref(false);
const detailProjectSaveError = ref("");

async function openDetail(project: StudioProject): Promise<void> {
  detailProject.value = project;
  detailMembers.value = [];
  detailError.value = "";
  detailEditing.value = false;
  detailProjectSaveError.value = "";
  detailOpen.value = true;
  await loadDetail(project.id);
}

async function loadDetail(projectId: number): Promise<void> {
  detailLoading.value = true;
  detailError.value = "";
  try {
    const detail = await fetchStudioProject(projectId);
    detailProject.value = detail.project;
    detailMembers.value = detail.members;
  } catch (error) {
    detailError.value = describeProjectError(error);
  } finally {
    detailLoading.value = false;
  }
}

async function saveDetailProject(): Promise<void> {
  if (detailProjectSaving.value || !detailProject.value) return;
  detailProjectFieldErrors.value = projectDraftErrors(detailProjectDraft.value);
  if (Object.keys(detailProjectFieldErrors.value).length > 0) return;

  detailProjectSaving.value = true;
  detailProjectSaveError.value = "";
  try {
    const patch = toUpdateInput(detailProjectDraft.value, detailProject.value);
    if (Object.keys(patch).length === 0) {
      ElMessage.info("没有需要保存的改动");
      detailEditing.value = false;
      return;
    }
    const updated = await updateStudioProject(detailProject.value.id, patch);
    detailProject.value = updated;
    detailProjectDraft.value = projectToDraft(updated);
    ElMessage.success("项目已更新");
    detailEditing.value = false;
    await load();
  } catch (error) {
    detailProjectSaveError.value = describeProjectError(error);
  } finally {
    detailProjectSaving.value = false;
  }
}

function startDetailEdit(): void {
  if (!detailProject.value) return;
  detailProjectDraft.value = projectToDraft(detailProject.value);
  detailProjectFieldErrors.value = {};
  detailProjectSaveError.value = "";
  detailEditing.value = true;
}

/* ── 参与行 ───────────────────────────────────────────────────── */

const memberDialogOpen = ref(false);
const editingMember = ref<StudioProjectMember | null>(null);
const memberDraft = ref<MemberDraft>({ ...EMPTY_MEMBER_DRAFT });
const memberFieldErrors = ref<Record<string, string>>({});
const memberSaving = ref(false);
const memberSaveError = ref("");

function openMemberCreate(): void {
  editingMember.value = null;
  memberDraft.value = { ...EMPTY_MEMBER_DRAFT };
  memberFieldErrors.value = {};
  memberSaveError.value = "";
  memberDialogOpen.value = true;
}

function openMemberEdit(member: StudioProjectMember): void {
  editingMember.value = member;
  memberDraft.value = memberToDraft(member);
  memberFieldErrors.value = {};
  memberSaveError.value = "";
  memberDialogOpen.value = true;
}

async function saveMember(): Promise<void> {
  if (memberSaving.value || !detailProject.value) return;
  memberFieldErrors.value = memberDraftErrors(memberDraft.value);
  if (Object.keys(memberFieldErrors.value).length > 0) return;

  memberSaving.value = true;
  memberSaveError.value = "";
  try {
    if (editingMember.value) {
      const patch = toMemberUpdateInput(memberDraft.value, editingMember.value);
      // 没有任何改动就不发请求。
      if (Object.keys(patch).length === 0) {
        ElMessage.info("没有需要保存的改动");
        memberDialogOpen.value = false;
        return;
      }
      await updateStudioProjectMember(
        detailProject.value.id,
        editingMember.value.id,
        patch,
      );
      ElMessage.success("参与已更新");
    } else {
      await addStudioProjectMember(
        detailProject.value.id,
        toMemberCreateInput(memberDraft.value),
      );
      ElMessage.success("参与已添加");
    }
    memberDialogOpen.value = false;
    await loadDetail(detailProject.value.id);
  } catch (error) {
    memberSaveError.value = describeProjectError(error);
  } finally {
    memberSaving.value = false;
  }
}

const removingMemberId = ref<number | null>(null);

async function removeMember(member: StudioProjectMember): Promise<void> {
  if (removingMemberId.value !== null || !detailProject.value) return;
  try {
    await ElMessageBox.confirm(removeMemberConfirmText(member), "移除参与", {
      confirmButtonText: "移除",
      cancelButtonText: "取消",
      type: "warning",
    });
  } catch {
    // 用户取消：不是错误。
    return;
  }

  removingMemberId.value = member.id;
  detailError.value = "";
  try {
    await removeStudioProjectMember(detailProject.value.id, member.id);
    ElMessage.success("参与已移除");
    await loadDetail(detailProject.value.id);
  } catch (error) {
    detailError.value = describeProjectError(error);
  } finally {
    removingMemberId.value = null;
  }
}

/* ── 删除项目 ─────────────────────────────────────────────────── */

const removingProjectId = ref<number | null>(null);

async function removeProject(project: StudioProject): Promise<void> {
  if (removingProjectId.value !== null) return;
  try {
    await ElMessageBox.confirm(
      deleteProjectConfirmText(project, detailMembers.value.length),
      "删除项目",
      { confirmButtonText: "删除", cancelButtonText: "取消", type: "warning" },
    );
  } catch {
    return;
  }

  removingProjectId.value = project.id;
  actionError.value = "";
  try {
    await deleteStudioProject(project.id);
    ElMessage.success("项目已删除");
    await load();
  } catch (error) {
    actionError.value = describeProjectError(error);
  } finally {
    removingProjectId.value = null;
  }
}
</script>
<template>
  <div class="page studio-projects-page">
    <main class="page__body">
      <PageHeader
        eyebrow="工作室运营 · 项目管理"
        title="项目管理"
        description="工作室承接或交付的项目，以及每个项目的参与名单。"
        :meta="'共 ' + total + ' 个项目'"
      >
        <template #actions>
          <button
            class="studio-project-button studio-project-button--primary"
            type="button"
            @click="openProjectCreate"
          >
            新建项目 ↗
          </button>
        </template>
      </PageHeader>

      <!-- 筛选条 -->
      <section class="panel panel--pad studio-project-filters" aria-label="项目筛选">
        <label class="studio-project-field">
          <span class="studio-project-field__label">关键词</span>
          <input
            v-model="keyword"
            class="studio-project-control"
            type="search"
            placeholder="项目名 / 简介"
            @keyup.enter="applyFilters"
          />
        </label>
        <label class="studio-project-field">
          <span class="studio-project-field__label">状态</span>
          <select v-model="statusFilter" class="studio-project-control">
            <option value="">全部状态</option>
            <option
              v-for="option in PROJECT_STATUS_OPTIONS"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
        </label>
        <label class="studio-project-field">
          <span class="studio-project-field__label">年份</span>
          <input
            v-model="yearFilter"
            class="studio-project-control"
            placeholder="如 2024"
            @keyup.enter="applyFilters"
          />
        </label>
        <div class="studio-project-filters__actions">
          <button class="studio-project-button" type="button" @click="applyFilters">筛选</button>
          <button class="studio-project-button" type="button" @click="resetFilters">重置</button>
        </div>
      </section>

      <p v-if="actionError" class="studio-project-error" role="alert">{{ actionError }}</p>

      <!-- 加载 / 错误 / 空态 -->
      <section v-if="loadError" class="panel panel--pad" role="alert">
        <p class="studio-project-error">{{ loadError }}</p>
        <button class="studio-project-button" type="button" @click="load()">重试</button>
      </section>
      <p v-else-if="loading" class="panel panel--pad" role="status">正在加载项目…</p>
      <!--
        空态分两种：「库里本来没有项目」和「筛选没命中」。
        混成一句会让管理员在筛选后误以为数据丢了，然后重复创建同一个项目。
      -->
      <section v-else-if="!projects.length" class="panel panel--pad studio-project-empty">
        <template v-if="hasActiveFilters">
          <span class="studio-project-kicker">NO MATCH</span>
          <h2>没有符合筛选条件的项目。</h2>
          <p>换个关键词、状态或年份试试。</p>
          <button class="studio-project-button" type="button" @click="resetFilters">清空筛选</button>
        </template>
        <template v-else>
          <span class="studio-project-kicker">START WITH A PROJECT</span>
          <h2>项目列表还是空的。</h2>
          <p>写下第一个项目；之后可以往里添加参与名单。</p>
          <button class="studio-project-button" type="button" @click="openProjectCreate">新建项目</button>
        </template>
      </section>

      <!-- 项目列表 -->
      <div v-else class="studio-project-list">
        <article
          v-for="project in projects"
          :key="project.id"
          class="panel studio-project-list__item"
        >
          <div class="studio-project-list__heading">
            <span class="studio-project-kicker">PROJECT / {{ String(project.id).padStart(3, "0") }}</span>
            <span class="studio-project-badge">{{ projectStatusLabel(project.status) }}</span>
          </div>
          <h2>{{ project.name }}</h2>
          <p class="studio-project-list__meta">{{ project.year }}</p>
          <p class="studio-project-list__excerpt">
            {{ projectExcerpt(project.summary) || "（还没有简介）" }}
          </p>
          <div class="studio-project-actions">
            <button class="studio-project-button" type="button" @click="openDetail(project)">
              详情与参与
            </button>
            <button class="studio-project-button" type="button" @click="openProjectEdit(project)">
              编辑
            </button>
            <button
              class="studio-project-button studio-project-button--danger"
              type="button"
              :disabled="removingProjectId !== null"
              :aria-busy="removingProjectId === project.id"
              @click="removeProject(project)"
            >
              删除
            </button>
          </div>
        </article>
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

      <!-- 项目弹窗（创建 / 编辑共用） -->
      <el-dialog
        v-model="projectDialogOpen"
        :title="editingProject ? '编辑项目' : '新建项目'"
        width="min(620px, 94vw)"
      >
        <form class="studio-project-form" novalidate @submit.prevent="saveProject">
          <label class="studio-project-field">
            <span class="studio-project-field__label">项目名 <em>必填</em></span>
            <input
              v-model="projectDraft.name"
              class="studio-project-control"
              :aria-invalid="Boolean(projectFieldErrors.name)"
              placeholder="全局唯一，如「校园导览小程序」"
            />
            <span v-if="projectFieldErrors.name" class="studio-project-field__error">
              {{ projectFieldErrors.name }}
            </span>
          </label>

          <label class="studio-project-field">
            <span class="studio-project-field__label">年份 <em>必填</em></span>
            <input
              v-model="projectDraft.year"
              class="studio-project-control"
              placeholder="如 2024（展示用字符串）"
              :aria-invalid="Boolean(projectFieldErrors.year)"
            />
            <span v-if="projectFieldErrors.year" class="studio-project-field__error">
              {{ projectFieldErrors.year }}
            </span>
          </label>

          <label class="studio-project-field">
            <span class="studio-project-field__label">状态</span>
            <select v-model="projectDraft.status" class="studio-project-control">
              <option
                v-for="option in PROJECT_STATUS_OPTIONS"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </option>
            </select>
          </label>

          <label class="studio-project-field">
            <span class="studio-project-field__label">简介</span>
            <textarea
              v-model="projectDraft.summary"
              class="studio-project-control studio-project-control--body"
              rows="5"
              placeholder="选填；列表里只显示摘要。"
              :aria-invalid="Boolean(projectFieldErrors.summary)"
            />
            <span v-if="projectFieldErrors.summary" class="studio-project-field__error">
              {{ projectFieldErrors.summary }}
            </span>
          </label>

          <p v-if="projectSaveError" class="studio-project-error" role="alert">
            {{ projectSaveError }}
          </p>

          <div class="studio-project-form__actions">
            <button class="studio-project-button" type="button" @click="projectDialogOpen = false">
              取消
            </button>
            <button
              class="studio-project-button studio-project-button--primary"
              type="submit"
              :disabled="projectSaving"
            >
              {{ projectSaving ? "保存中…" : "保存" }}
            </button>
          </div>
        </form>
      </el-dialog>

      <!-- 项目详情弹窗 -->
      <el-dialog
        v-model="detailOpen"
        :title="detailProject ? detailProject.name : '项目详情'"
        width="min(900px, 96vw)"
      >
        <p v-if="detailError" class="studio-project-error" role="alert">{{ detailError }}</p>
        <p v-else-if="detailLoading" class="studio-project-hint" role="status">
          正在加载项目详情…
        </p>

        <template v-else-if="detailProject">
          <!-- 非编辑态：只读摘要 + 操作按钮 -->
          <template v-if="!detailEditing">
            <dl class="studio-project-summary">
              <dt>年份</dt>
              <dd>{{ detailProject.year }}</dd>
              <dt>状态</dt>
              <dd>{{ projectStatusLabel(detailProject.status) }}</dd>
              <dt>简介</dt>
              <dd class="studio-project-summary__text">
                {{ detailProject.summary || "（还没有简介）" }}
              </dd>
            </dl>

            <div class="studio-project-toolbar">
              <button class="studio-project-button" type="button" @click="startDetailEdit">
                编辑项目
              </button>
              <button
                class="studio-project-button studio-project-button--danger"
                type="button"
                :disabled="removingProjectId !== null"
                @click="removeProject(detailProject)"
              >
                删除项目
              </button>
              <button
                class="studio-project-button studio-project-button--primary"
                type="button"
                @click="openMemberCreate"
              >
                添加参与
              </button>
            </div>
          </template>

          <!-- 编辑态：与项目弹窗同一套字段 -->
          <form v-else class="studio-project-form" novalidate @submit.prevent="saveDetailProject">
            <label class="studio-project-field">
              <span class="studio-project-field__label">项目名 <em>必填</em></span>
              <input v-model="detailProjectDraft.name" class="studio-project-control" />
              <span v-if="detailProjectFieldErrors.name" class="studio-project-field__error">
                {{ detailProjectFieldErrors.name }}
              </span>
            </label>
            <label class="studio-project-field">
              <span class="studio-project-field__label">年份 <em>必填</em></span>
              <input v-model="detailProjectDraft.year" class="studio-project-control" />
              <span v-if="detailProjectFieldErrors.year" class="studio-project-field__error">
                {{ detailProjectFieldErrors.year }}
              </span>
            </label>
            <label class="studio-project-field">
              <span class="studio-project-field__label">状态</span>
              <select v-model="detailProjectDraft.status" class="studio-project-control">
                <option
                  v-for="option in PROJECT_STATUS_OPTIONS"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </select>
            </label>
            <label class="studio-project-field">
              <span class="studio-project-field__label">简介</span>
              <textarea
                v-model="detailProjectDraft.summary"
                class="studio-project-control studio-project-control--body"
                rows="5"
              />
              <span v-if="detailProjectFieldErrors.summary" class="studio-project-field__error">
                {{ detailProjectFieldErrors.summary }}
              </span>
            </label>
            <p v-if="detailProjectSaveError" class="studio-project-error" role="alert">
              {{ detailProjectSaveError }}
            </p>
            <div class="studio-project-form__actions">
              <button class="studio-project-button" type="button" @click="detailEditing = false">
                取消
              </button>
              <button
                class="studio-project-button studio-project-button--primary"
                type="submit"
                :disabled="detailProjectSaving"
              >
                {{ detailProjectSaving ? "保存中…" : "保存" }}
              </button>
            </div>
          </form>

          <!-- 参与名单 -->
          <section class="studio-project-members" aria-label="参与名单">
            <h3>参与名单</h3>
            <p v-if="!detailMembers.length" class="studio-project-hint">
              还没有参与记录。点「添加参与」加入第一位。
            </p>
            <table v-else class="studio-project-table">
              <caption class="studio-project-sr-only">项目参与名单</caption>
              <thead>
                <tr>
                  <th scope="col">姓名</th>
                  <th scope="col">角色</th>
                  <th scope="col">关联成员</th>
                  <th scope="col">参与时间</th>
                  <th scope="col"><span class="studio-project-sr-only">操作</span></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="member in detailMembers" :key="member.id">
                  <td><strong>{{ member.name }}</strong></td>
                  <td>{{ projectRoleLabel(member.role) }}</td>
                  <td>
                    <span v-if="isExternalParticipant(member)" class="studio-project-muted">
                      外部参与者
                    </span>
                    <span v-else>成员 #{{ member.member_id }}</span>
                  </td>
                  <td>
                    <span>{{ formatDay(member.started_at) }}</span>
                    <span v-if="member.ended_at"> · </span>
                    <span>{{ memberEndSummary(member) }}</span>
                  </td>
                  <td class="studio-project-table__actions">
                    <div class="studio-project-table__actions-inner">
                      <button
                        class="studio-project-button studio-project-button--small"
                        type="button"
                        @click="openMemberEdit(member)"
                      >
                        编辑
                      </button>
                      <button
                        class="studio-project-button studio-project-button--small studio-project-button--danger"
                        type="button"
                        :disabled="removingMemberId !== null"
                        :aria-busy="removingMemberId === member.id"
                        @click="removeMember(member)"
                      >
                        移除
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </section>
        </template>
      </el-dialog>

      <!-- 参与行弹窗（添加 / 编辑共用） -->
      <el-dialog
        v-model="memberDialogOpen"
        :title="editingMember ? '编辑参与' : '添加参与'"
        width="min(560px, 94vw)"
      >
        <form class="studio-project-form" novalidate @submit.prevent="saveMember">
          <label class="studio-project-field">
            <span class="studio-project-field__label">姓名 <em>必填</em></span>
            <input
              v-model="memberDraft.name"
              class="studio-project-control"
              :aria-invalid="Boolean(memberFieldErrors.name)"
              placeholder="姓名快照：成员以后改名，这里仍显示当时的名字"
            />
            <span v-if="memberFieldErrors.name" class="studio-project-field__error">
              {{ memberFieldErrors.name }}
            </span>
          </label>

          <label class="studio-project-field">
            <span class="studio-project-field__label">角色</span>
            <select v-model="memberDraft.role" class="studio-project-control">
              <option
                v-for="option in PROJECT_ROLE_OPTIONS"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </option>
            </select>
          </label>

          <label class="studio-project-field">
            <span class="studio-project-field__label">关联成员档案</span>
            <input
              v-model="memberDraft.member_id"
              class="studio-project-control"
              inputmode="numeric"
              placeholder="选填；留空表示外部参与者"
              :aria-invalid="Boolean(memberFieldErrors.member_id)"
            />
            <span v-if="memberFieldErrors.member_id" class="studio-project-field__error">
              {{ memberFieldErrors.member_id }}
            </span>
          </label>

          <label class="studio-project-field">
            <span class="studio-project-field__label">开始参与</span>
            <input
              v-model="memberDraft.started_at"
              class="studio-project-control"
              type="datetime-local"
              placeholder="留空则按当前时间"
            />
          </label>

          <label class="studio-project-field">
            <span class="studio-project-field__label">结束参与</span>
            <input
              v-model="memberDraft.ended_at"
              class="studio-project-control"
              type="datetime-local"
              placeholder="留空表示仍在参与"
            />
          </label>

          <p v-if="memberSaveError" class="studio-project-error" role="alert">
            {{ memberSaveError }}
          </p>

          <div class="studio-project-form__actions">
            <button class="studio-project-button" type="button" @click="memberDialogOpen = false">
              取消
            </button>
            <button
              class="studio-project-button studio-project-button--primary"
              type="submit"
              :disabled="memberSaving"
            >
              {{ memberSaving ? "保存中…" : "保存" }}
            </button>
          </div>
        </form>
      </el-dialog>
    </main>
  </div>
</template>
