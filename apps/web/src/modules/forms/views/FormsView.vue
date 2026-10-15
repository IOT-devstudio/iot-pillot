<script setup lang="ts">
import { onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import PageHeader from "@/components/common/PageHeader.vue";
import { listForms, updateForm, type FormDefinition } from "@/api/forms";
import { describeFormError } from "../model";
import ShareLinkDialog from "../components/ShareLinkDialog.vue";
import "../forms.css";

const items = ref<FormDefinition[]>([]);
const loading = ref(false),
  error = ref(""),
  actionError = ref("");
const total = ref(0),
  page = ref(1),
  busy = ref<number | null>(null);
const sharing = ref(false),
  token = ref("");
async function load(nextPage = page.value) {
  if (loading.value) return;
  loading.value = true;
  error.value = "";
  try {
    const data = await listForms(nextPage);
    items.value = data.items;
    total.value = data.total;
    page.value = nextPage;
  } catch (e) {
    error.value = describeFormError(e);
  } finally {
    loading.value = false;
  }
}
async function toggle(form: FormDefinition) {
  if (busy.value !== null) return;
  busy.value = form.id;
  actionError.value = "";
  try {
    const data = await updateForm(form.id, { is_open: !form.is_open });
    items.value = items.value.map((item) =>
      item.id === form.id ? data.form : item,
    );
  } catch (e) {
    actionError.value = describeFormError(e);
  } finally {
    busy.value = null;
  }
}
onMounted(() => load());
</script>
<template>
  <div class="page forms-page">
    <main class="page__body">
      <PageHeader
        eyebrow="招新管理 · 报名表"
        title="表单管理"
        description="把想了解的问题，交给每一份认真填写的答案。"
        :meta="`共 ${total} 张表单`"
      >
        <template #actions
          ><RouterLink class="form-button form-button--primary" to="/forms/new"
            >新建表单 ↗</RouterLink
          ></template
        >
      </PageHeader>
      <p v-if="actionError" class="form-error" role="alert">
        {{ actionError }}
      </p>
      <section v-if="error" class="panel panel--pad" role="alert">
        <p>{{ error }}</p>
        <el-button @click="load()">重试</el-button>
      </section>
      <p v-else-if="loading" class="panel panel--pad" role="status">
        正在加载表单…
      </p>
      <section v-else-if="!items.length" class="panel panel--pad forms-empty">
        <span class="forms-kicker">START WITH A QUESTION</span>
        <h2>第一份报名表，从这里开始。</h2>
        <p>新建时绑定招新轮次和方向，配置题目后开放分享。</p>
        <RouterLink class="form-button" to="/forms/new">新建表单</RouterLink>
      </section>
      <div v-else class="forms-list">
        <article
          v-for="form in items"
          :key="form.id"
          class="panel forms-list__item"
        >
          <div class="forms-list__heading">
            <span class="forms-kicker"
              >FORM / {{ String(form.id).padStart(3, "0") }}</span
            ><span
              class="form-status"
              :class="{ 'form-status--open': form.is_open }"
              >{{ form.is_open ? "报名开放中" : "已关闭" }}</span
            >
          </div>
          <h2>
            <RouterLink :to="`/forms/${form.id}/edit`">{{
              form.title
            }}</RouterLink>
          </h2>
          <p class="form-hint">
            轮次 #{{ form.round_id }} · {{ form.direction_code }} ·
            {{ form.submission_count }} 份提交
          </p>
          <p class="forms-list__description">
            {{ form.description || "暂无表单说明" }}
          </p>
          <div class="forms-actions">
            <RouterLink class="form-button" :to="`/forms/${form.id}/edit`"
              >编辑与预览</RouterLink
            >
            <RouterLink
              class="form-button"
              :to="`/forms/${form.id}/submissions`"
              >提交记录 · {{ form.submission_count }}</RouterLink
            >
            <el-button
              @click="
                token = form.share_token;
                sharing = true;
              "
              >分享链接</el-button
            >
            <el-button
              :disabled="busy !== null"
              :loading="busy === form.id"
              @click="toggle(form)"
              >{{ form.is_open ? "关闭报名" : "开放报名" }}</el-button
            >
          </div>
        </article>
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
      <ShareLinkDialog v-model="sharing" :token="token" />
    </main>
  </div>
</template>
