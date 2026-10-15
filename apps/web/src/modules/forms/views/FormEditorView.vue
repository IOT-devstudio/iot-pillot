<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import {
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
  useRoute,
  useRouter,
  RouterLink,
} from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import PageHeader from "@/components/common/PageHeader.vue";
import {
  createForm,
  fetchForm,
  updateForm,
  type FormCreateInput,
  type FormDefinition,
  type FormFieldInput,
} from "@/api/forms";
import {
  fetchAdminRounds,
  fetchDirections,
  type RecruitmentRound,
  type RecruitmentDirection,
} from "@/api/recruitment";
import FormFields from "../components/FormFields.vue";
import ShareLinkDialog from "../components/ShareLinkDialog.vue";
import {
  FIELD_LABELS,
  describeFormError,
  editorError,
  initialAnswers,
} from "../model";
import type { FormAnswers } from "@/api/forms";
import "../forms.css";

const route = useRoute(),
  router = useRouter();
const saved = ref<FormDefinition | null>(null);
const draft = ref<FormCreateInput>({
  title: "",
  description: "",
  round_id: 0,
  direction_code: "",
  fields: [],
});
const rounds = ref<RecruitmentRound[]>([]),
  directions = ref<RecruitmentDirection[]>([]);
const loading = ref(false),
  saving = ref(false),
  toggling = ref(false),
  error = ref(""),
  saveError = ref("");
const sharing = ref(false),
  mobileTab = ref("edit"),
  previewValues = ref<FormAnswers>({});
const snapshot = ref(""),
  fieldSnapshot = ref("");
const dirty = computed(
  () => snapshot.value !== "" && snapshot.value !== JSON.stringify(draft.value),
);
const locked = computed(() => (saved.value?.submission_count ?? 0) > 0);
let temporaryId = -1,
  revision = 0;

function newField(type: FormFieldInput["type"]) {
  if (locked.value || draft.value.fields.length >= 50) return;
  draft.value.fields.push({
    id: temporaryId--,
    type,
    label: "",
    required: false,
    placeholder: "",
    ...(type === "select" ? { options: ["选项一", "选项二"] } : {}),
  });
}
function payloadFields() {
  return draft.value.fields.map((field) => ({
    ...(field.id && field.id > 0 ? { id: field.id } : {}),
    type: field.type,
    label: field.label.trim(),
    required: field.required,
    placeholder: field.placeholder.trim(),
    ...(field.type === "select"
      ? { options: field.options?.map((option) => option.trim()) }
      : {}),
  }));
}
async function load() {
  const current = ++revision;
  loading.value = true;
  error.value = "";
  saveError.value = "";
  snapshot.value = "";
  saved.value = null;
  try {
    const id = route.params.id ? Number(route.params.id) : null;
    const [detail, roundList, directionList] = await Promise.all([
      id ? fetchForm(id) : Promise.resolve(null),
      fetchAdminRounds(),
      fetchDirections(),
    ]);
    if (current !== revision) return;
    rounds.value = roundList.items;
    directions.value = directionList.items;
    saved.value = detail?.form ?? null;
    draft.value = detail
      ? {
          title: detail.form.title,
          description: detail.form.description,
          round_id: detail.form.round_id,
          direction_code: detail.form.direction_code,
          fields: detail.fields.map((field) => ({
            ...field,
            options: field.options ? [...field.options] : undefined,
          })),
        }
      : {
          title: "",
          description: "",
          round_id: 0,
          direction_code: "",
          fields: [],
        };
    if (!detail) newField("text");
    snapshot.value = JSON.stringify(draft.value);
    fieldSnapshot.value = JSON.stringify(payloadFields());
    previewValues.value = initialAnswers(draft.value.fields);
  } catch (e) {
    if (current === revision) error.value = describeFormError(e);
  } finally {
    if (current === revision) loading.value = false;
  }
}
watch(
  () => route.params.id,
  () => void load(),
  { immediate: true },
);
watch(
  () => draft.value.fields.map((f) => `${f.id}:${f.type}`).join("|"),
  () => {
    previewValues.value = initialAnswers(draft.value.fields);
  },
);
onBeforeUnmount(() => {
  revision++;
});

async function save() {
  if (saving.value || toggling.value) return;
  saveError.value = editorError(draft.value);
  if (saveError.value) return;
  saving.value = true;
  try {
    const fields = payloadFields();
    const fieldsChanged = JSON.stringify(fields) !== fieldSnapshot.value;
    if (saved.value && fieldsChanged) {
      const latest = await fetchForm(saved.value.id);
      if (latest.form.submission_count > 0) {
        saved.value = latest.form;
        saveError.value =
          "这张表单已有新提交，字段已锁定。请重新加载后编辑标题与说明。";
        return;
      }
    }
    const data = saved.value
      ? await updateForm(saved.value.id, {
          title: draft.value.title.trim(),
          description: draft.value.description.trim(),
          ...(fieldsChanged && !locked.value ? { fields } : {}),
        })
      : await createForm({
          ...draft.value,
          title: draft.value.title.trim(),
          description: draft.value.description.trim(),
          fields,
        });
    snapshot.value = JSON.stringify(draft.value);
    ElMessage.success(
      saved.value ? "表单已保存" : "表单已创建，开放后即可报名",
    );
    if (!saved.value) await router.replace(`/forms/${data.form.id}/edit`);
    else {
      saved.value = data.form;
      await load();
    }
  } catch (e) {
    saveError.value = describeFormError(e);
  } finally {
    saving.value = false;
  }
}
async function toggle() {
  if (!saved.value || saving.value || toggling.value || dirty.value) return;
  toggling.value = true;
  saveError.value = "";
  try {
    saved.value = (
      await updateForm(saved.value.id, { is_open: !saved.value.is_open })
    ).form;
  } catch (e) {
    saveError.value = describeFormError(e);
  } finally {
    toggling.value = false;
  }
}
function move(index: number, delta: number) {
  if (locked.value) return;
  const fields = draft.value.fields;
  const item = fields.splice(index, 1)[0];
  if (item) fields.splice(index + delta, 0, item);
}
async function canLeave() {
  if (!dirty.value) return true;
  try {
    await ElMessageBox.confirm(
      "离开后将丢失尚未保存的表单修改。",
      "离开编辑页？",
      {
        confirmButtonText: "离开",
        cancelButtonText: "继续编辑",
        type: "warning",
      },
    );
    return true;
  } catch {
    return false;
  }
}
onBeforeRouteLeave(() => (saving.value && dirty.value ? false : canLeave()));
onBeforeRouteUpdate(() => (saving.value && dirty.value ? false : canLeave()));
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault();
    event.returnValue = "";
  }
}
window.addEventListener("beforeunload", beforeUnload);
onBeforeUnmount(() => window.removeEventListener("beforeunload", beforeUnload));
</script>
<template>
  <div class="page forms-page">
    <main class="page__body">
      <RouterLink class="form-back" to="/forms">← 返回表单管理</RouterLink>
      <PageHeader
        eyebrow="招新管理 · 表单工作台"
        :title="saved ? '编辑报名表' : '新建报名表'"
        description="让每个问题都有目的，让每份答案被认真看见。"
      >
        <template #actions
          ><span
            v-if="saved"
            class="form-status"
            :class="{ 'form-status--open': saved.is_open }"
            >{{ saved.is_open ? "报名开放中" : "已关闭" }}</span
          ><el-button
            type="primary"
            :disabled="loading || !!error || toggling"
            :loading="saving"
            @click="save"
            >保存表单</el-button
          ></template
        >
      </PageHeader>
      <section v-if="error" class="panel panel--pad" role="alert">
        <p>{{ error }}</p>
        <el-button @click="load">重新加载</el-button>
      </section>
      <p v-else-if="loading" role="status">正在加载编辑器…</p>
      <template v-else>
        <div class="forms-toolbar">
          <p class="form-hint">
            {{
              locked
                ? "已有提交，字段结构已锁定，可修改标题、说明和开放状态。"
                : "新建表单默认关闭；保存后才能开放。轮次和方向创建后不可修改。"
            }}
          </p>
          <div v-if="saved" class="forms-actions">
            <el-button
              :disabled="dirty || saving || toggling"
              :loading="toggling"
              @click="toggle"
              >{{ saved.is_open ? "关闭报名" : "开放报名" }}</el-button
            ><el-button @click="sharing = true">分享链接</el-button
            ><RouterLink
              class="form-button"
              :to="`/forms/${saved.id}/submissions`"
              >提交记录 · {{ saved.submission_count }}</RouterLink
            >
          </div>
        </div>
        <p v-if="dirty" class="form-hint" role="status">
          有未保存的修改。预览展示当前草稿，请保存后再开放表单。
        </p>
        <p v-if="saveError" class="form-error" role="alert">{{ saveError }}</p>
        <div class="forms-mobile-tabs" aria-label="编辑器视图">
          <button
            class="form-button"
            :aria-pressed="mobileTab === 'edit'"
            @click="mobileTab = 'edit'"
          >
            编辑</button
          ><button
            class="form-button"
            :aria-pressed="mobileTab === 'preview'"
            @click="mobileTab = 'preview'"
          >
            预览
          </button>
        </div>
        <div class="forms-editor" :data-tab="mobileTab">
          <section class="forms-editor__edit panel panel--pad">
            <fieldset class="form-fieldset" :disabled="saving || toggling">
              <legend class="forms-kicker">01 / 表单信息</legend>
              <label class="form-editor-label" for="form-title"
                >表单标题 · 必填</label
              ><input
                id="form-title"
                v-model="draft.title"
                class="form-control"
                maxlength="200"
                placeholder="例如：2026 秋季招新 · 后端开发"
              />
              <label class="form-editor-label" for="form-description"
                >表单说明</label
              ><textarea
                id="form-description"
                v-model="draft.description"
                class="form-control"
                rows="3"
                maxlength="2000"
                placeholder="介绍报名要求、截止时间与填写建议"
              />
              <div class="forms-binding">
                <div>
                  <label class="form-editor-label" for="form-round"
                    >招新轮次 · 必填</label
                  ><select
                    id="form-round"
                    v-model="draft.round_id"
                    class="form-control"
                    :disabled="!!saved"
                  >
                    <option :value="0">选择轮次</option>
                    <option
                      v-for="round in rounds"
                      :key="round.id"
                      :value="round.id"
                    >
                      {{ round.name
                      }}{{ round.accepting ? "" : "（暂停报名）" }}
                    </option>
                    <option
                      v-if="
                        saved && !rounds.some((r) => r.id === draft.round_id)
                      "
                      :value="draft.round_id"
                    >
                      轮次 #{{ draft.round_id }}
                    </option>
                  </select>
                </div>
                <div>
                  <label class="form-editor-label" for="form-direction"
                    >招新方向 · 必填</label
                  ><select
                    id="form-direction"
                    v-model="draft.direction_code"
                    class="form-control"
                    :disabled="!!saved"
                  >
                    <option value="">选择方向</option>
                    <option
                      v-for="direction in directions"
                      :key="direction.code"
                      :value="direction.code"
                    >
                      {{ direction.name }}
                    </option>
                    <option
                      v-if="
                        saved &&
                        !directions.some((d) => d.code === draft.direction_code)
                      "
                      :value="draft.direction_code"
                    >
                      {{ draft.direction_code }}（已下线）
                    </option>
                  </select>
                </div>
              </div>
              <p
                v-if="!saved && (!rounds.length || !directions.length)"
                class="form-hint"
              >
                暂无可用轮次或方向，请先在申请审核页配置。
              </p>
              <div class="forms-section-heading">
                <span class="forms-kicker">02 / 题目配置</span
                ><span class="form-hint">{{ draft.fields.length }} / 50</span>
              </div>
              <article
                v-for="(field, index) in draft.fields"
                :key="field.id"
                class="forms-field-editor"
              >
                <div class="forms-field-editor__head">
                  <strong
                    >{{ String(index + 1).padStart(2, "0") }} /
                    {{ FIELD_LABELS[field.type] }}</strong
                  >
                  <div v-if="!locked" class="forms-actions">
                    <button
                      type="button"
                      class="form-icon-button"
                      :disabled="index === 0"
                      :aria-label="`上移第 ${index + 1} 题`"
                      @click="move(index, -1)"
                    >
                      ↑</button
                    ><button
                      type="button"
                      class="form-icon-button"
                      :disabled="index === draft.fields.length - 1"
                      :aria-label="`下移第 ${index + 1} 题`"
                      @click="move(index, 1)"
                    >
                      ↓</button
                    ><button
                      type="button"
                      class="form-icon-button"
                      :aria-label="`删除第 ${index + 1} 题`"
                      @click="draft.fields.splice(index, 1)"
                    >
                      删除
                    </button>
                  </div>
                </div>
                <fieldset class="form-fieldset" :disabled="locked">
                  <legend class="form-sr-only">
                    第 {{ index + 1 }} 题配置
                  </legend>
                  <label class="form-editor-label" :for="`label-${field.id}`"
                    >题目标题</label
                  ><input
                    :id="`label-${field.id}`"
                    v-model="field.label"
                    class="form-control"
                    maxlength="200"
                    placeholder="你想了解什么？"
                  /><label
                    class="form-editor-label"
                    :for="`hint-${field.id}`"
                    >{{
                      field.type === "checkbox" ? "确认说明" : "填写提示"
                    }}</label
                  ><input
                    :id="`hint-${field.id}`"
                    v-model="field.placeholder"
                    class="form-control"
                    maxlength="200"
                  /><template v-if="field.type === 'select'"
                    ><label
                      class="form-editor-label"
                      :for="`options-${field.id}`"
                      >候选选项（每行一个）</label
                    ><textarea
                      :id="`options-${field.id}`"
                      class="form-control"
                      rows="4"
                      :value="field.options?.join('\n')"
                      @input="
                        field.options = (
                          $event.target as HTMLTextAreaElement
                        ).value.split('\n')
                      "
                    /></template
                  ><label class="form-check"
                    ><input
                      v-model="field.required"
                      type="checkbox"
                    />必填</label
                  >
                </fieldset>
              </article>
              <div
                v-if="!locked"
                class="forms-add-fields"
                aria-label="添加字段"
              >
                <span class="form-hint">添加题目</span
                ><button
                  v-for="(label, type) in FIELD_LABELS"
                  :key="type"
                  type="button"
                  class="form-button"
                  :disabled="draft.fields.length >= 50"
                  @click="newField(type)"
                >
                  ＋ {{ label }}
                </button>
              </div>
            </fieldset>
          </section>
          <aside
            class="forms-editor__preview panel panel--pad"
            aria-label="表单预览"
          >
            <span class="forms-kicker">LIVE PREVIEW / 草稿预览</span>
            <h2>{{ draft.title || "你的报名表标题" }}</h2>
            <p class="form-description">
              {{ draft.description || "表单说明将显示在这里。" }}
            </p>
            <FormFields
              v-model="previewValues"
              :fields="draft.fields"
              prefix="preview"
            />
            <p class="form-hint">预览仅用于试填，不会生成报名申请。</p>
          </aside>
        </div>
        <ShareLinkDialog
          v-if="saved"
          v-model="sharing"
          :token="saved.share_token"
        />
      </template>
    </main>
  </div>
</template>
