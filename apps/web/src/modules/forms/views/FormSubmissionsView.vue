<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";
import { useRoute, RouterLink } from "vue-router";
import PageHeader from "@/components/common/PageHeader.vue";
import {
  fetchForm,
  listFormSubmissions,
  fetchFormSubmission,
  type FormDetail,
  type FormSubmission,
} from "@/api/forms";
import { answerRows, describeFormError } from "../model";
import ShareLinkDialog from "../components/ShareLinkDialog.vue";
import "../forms.css";
const route = useRoute();
const definition = ref<FormDetail | null>(null),
  records = ref<FormSubmission[]>([]),
  selected = ref<FormSubmission | null>(null);
const loading = ref(false),
  detailLoading = ref(false),
  error = ref(""),
  detailError = ref("");
const page = ref(1),
  total = ref(0),
  selectedId = ref<number | null>(null),
  sharing = ref(false);
let revision = 0,
  detailRevision = 0;
async function load(nextPage = 1) {
  const current = ++revision;
  detailRevision++;
  selected.value = null;
  selectedId.value = null;
  detailError.value = "";
  detailLoading.value = false;
  loading.value = true;
  error.value = "";
  try {
    const id = Number(route.params.id);
    const [detail, list] = await Promise.all([
      fetchForm(id),
      listFormSubmissions(id, nextPage),
    ]);
    if (current !== revision) return;
    definition.value = detail;
    records.value = list.items;
    total.value = list.total;
    page.value = nextPage;
  } catch (e) {
    if (current === revision) error.value = describeFormError(e);
  } finally {
    if (current === revision) loading.value = false;
  }
}
async function open(record: Pick<FormSubmission, "id">) {
  const current = ++detailRevision;
  selectedId.value = record.id;
  selected.value = null;
  detailLoading.value = true;
  detailError.value = "";
  try {
    const data = await fetchFormSubmission(Number(route.params.id), record.id);
    if (current === detailRevision) selected.value = data;
  } catch (e) {
    if (current === detailRevision) detailError.value = describeFormError(e);
  } finally {
    if (current === detailRevision) detailLoading.value = false;
  }
}
watch(
  () => route.params.id,
  () => void load(),
  { immediate: true },
);
onBeforeUnmount(() => {
  revision++;
  detailRevision++;
});
</script>
<template>
  <div class="page forms-page">
    <main class="page__body">
      <RouterLink class="form-back" to="/forms">← 返回表单管理</RouterLink>
      <PageHeader
        eyebrow="招新管理 · 真实提交"
        title="每一份答案"
        :description="
          definition?.form.title || '查看报名表的提交记录与完整答案'
        "
        :meta="`共 ${total} 份提交`"
      >
        <template #actions
          ><RouterLink
            v-if="definition"
            class="form-button"
            :to="`/forms/${definition.form.id}/edit`"
            >编辑表单 ↗</RouterLink
          ></template
        >
      </PageHeader>
      <section v-if="error" class="panel panel--pad" role="alert">
        <p>{{ error }}</p>
        <el-button @click="load(page)">重试</el-button>
      </section>
      <p v-else-if="loading" role="status">正在加载提交记录…</p>
      <section v-else-if="!records.length" class="panel panel--pad forms-empty">
        <h2>还没有收到答案</h2>
        <p>开放表单并分享给报名者，真实提交会出现在这里。</p>
        <el-button v-if="definition" @click="sharing = true"
          >分享链接</el-button
        >
      </section>
      <div v-else class="forms-records">
        <section class="forms-record-list" aria-label="提交记录">
          <button
            v-for="record in records"
            :key="record.id"
            class="panel forms-record"
            :class="{ 'forms-record--active': selectedId === record.id }"
            :aria-pressed="selectedId === record.id"
            @click="open(record)"
          >
            <span class="forms-kicker">SUBMISSION / {{ record.id }}</span
            ><strong>提交人 #{{ record.submitter_id }}</strong
            ><span>申请 #{{ record.application_id }} ↗</span
            ><time :datetime="record.submitted_at">{{
              new Date(record.submitted_at).toLocaleString("zh-CN")
            }}</time>
          </button>
        </section>
        <section
          class="panel panel--pad forms-answer-detail"
          aria-label="答案详情"
          aria-live="polite"
        >
          <p v-if="detailLoading" role="status">正在加载答案…</p>
          <div v-else-if="detailError" role="alert">
            <p>{{ detailError }}</p>
            <el-button v-if="selectedId" @click="open({ id: selectedId })"
              >重新加载答案</el-button
            >
          </div>
          <template v-else-if="selected"
            ><span class="forms-kicker"
              >ANSWER ARCHIVE / {{ selected.id }}</span
            >
            <h2>提交人 #{{ selected.submitter_id }}</h2>
            <p class="form-hint">
              关联申请 #{{ selected.application_id }} ·
              <RouterLink to="/recruitment/applications"
                >前往申请审核 ↗</RouterLink
              >
            </p>
            <dl class="forms-answer-list">
              <div
                v-for="row in answerRows(
                  definition?.fields ?? [],
                  selected.values,
                )"
                :key="row.id"
              >
                <dt>{{ row.label }}</dt>
                <dd>{{ row.value }}</dd>
              </div>
            </dl></template
          >
          <p v-else class="form-hint">选择一份提交，查看完整答案。</p>
        </section>
      </div>
      <el-pagination
        v-if="total > 20"
        layout="prev, pager, next"
        :total="total"
        :page-size="20"
        :current-page="page"
        :disabled="loading"
        @current-change="load"
      />
      <ShareLinkDialog
        v-if="definition"
        v-model="sharing"
        :token="definition.form.share_token"
      />
    </main>
  </div>
</template>
