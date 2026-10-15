<script setup lang="ts">
import { nextTick, onBeforeUnmount, watch } from "vue";
import {
  useRoute,
  RouterLink,
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
} from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import FormFields from "../components/FormFields.vue";
import { useFormFilling } from "../useFormFilling";
import "../forms.css";
const route = useRoute();
const {
  data,
  profile,
  values,
  errors,
  loading,
  loadError,
  submitting,
  submitError,
  result,
  done,
  hasDraft,
  load,
  send,
  dispose,
} = useFormFilling();
watch(
  () => String(route.params.shareToken ?? ""),
  (token) => void load(token),
  { immediate: true },
);
onBeforeUnmount(dispose);
async function canLeave() {
  if (submitting.value) {
    ElMessage.warning("正在提交，请稍候");
    return false;
  }
  if (!hasDraft.value) return true;
  try {
    await ElMessageBox.confirm("离开后将丢失尚未提交的答案。", "离开填写页？", {
      confirmButtonText: "离开",
      cancelButtonText: "继续填写",
      type: "warning",
    });
    return true;
  } catch {
    return false;
  }
}
onBeforeRouteLeave(canLeave);
onBeforeRouteUpdate(
  (to, from) => to.params.shareToken === from.params.shareToken || canLeave(),
);
function beforeUnload(event: BeforeUnloadEvent) {
  if (hasDraft.value || submitting.value) {
    event.preventDefault();
    event.returnValue = "";
  }
}
window.addEventListener("beforeunload", beforeUnload);
onBeforeUnmount(() => window.removeEventListener("beforeunload", beforeUnload));
async function submit() {
  await send();
  await nextTick();
  const id = Object.keys(errors.value)[0];
  if (id) document.getElementById(`answer-${id}`)?.focus();
}
</script>
<template>
  <main class="share-form-page">
    <div class="share-form-intro">
      <p class="forms-kicker">RECRUITMENT / YOUR NEXT CHAPTER</p>
      <span class="share-form-mark" aria-hidden="true">↗</span>
    </div>
    <section v-if="loading" class="share-form-state" role="status">
      正在打开报名表…
    </section>
    <section v-else-if="loadError" class="share-form-state" role="alert">
      <h1>暂时无法打开表单</h1>
      <p>{{ loadError }}</p>
      <button
        class="form-button"
        @click="load(String(route.params.shareToken))"
      >
        重新加载
      </button>
    </section>
    <template v-else-if="data">
      <header class="share-form-header">
        <p class="forms-kicker">
          轮次 #{{ data.form.round_id }} / {{ data.form.direction_code }}
        </p>
        <h1>{{ data.form.title }}</h1>
        <p class="form-description">{{ data.form.description }}</p>
      </header>
      <section v-if="done" class="share-form-state" role="status">
        <p class="forms-kicker">RECEIVED / THANK YOU</p>
        <h2>
          {{ result ? "提交成功，谢谢你的认真填写。" : "你已提交过这张表单。" }}
        </h2>
        <p>每位用户仅可提交一次。</p>
        <p v-if="result">
          申请 #{{ result.application_id }} · 待审核<br />提交时间：{{
            new Date(result.submission.submitted_at).toLocaleString("zh-CN")
          }}
        </p>
        <RouterLink class="form-button form-button--primary" to="/user/home"
          >查看我的申请 ↗</RouterLink
        >
      </section>
      <section
        v-else-if="!data.form.is_open"
        class="share-form-state"
        role="status"
      >
        <h2>本次报名已关闭</h2>
        <p>当前表单不接受提交，可返回首页了解其他招新方向。</p>
        <RouterLink class="form-button" to="/user/home"
          >返回用户首页 ↗</RouterLink
        >
      </section>
      <form v-else novalidate @submit.prevent="submit">
        <section class="form-identity" aria-label="当前报名身份">
          <span class="forms-kicker">当前报名身份</span
          ><strong>{{ profile?.name || profile?.username }}</strong>
          <p>
            {{ profile?.detail?.class || "班级未填写" }} ·
            {{ profile?.detail?.student_id || "学号未填写" }}
          </p>
          <p>
            账号邮箱：{{
              profile?.detail?.email || profile?.username
            }}（不可修改）
          </p>
          <RouterLink to="/user/settings" target="_blank" rel="noopener"
            >更新个人资料（新标签页）↗</RouterLink
          >
        </section>
        <fieldset class="form-fieldset" :disabled="submitting">
          <legend class="form-sr-only">报名问题</legend>
          <FormFields
            v-model="values"
            :fields="data.fields"
            :errors="errors"
            :disabled="submitting"
          />
        </fieldset>
        <p v-if="Object.keys(errors).length" class="form-error" role="alert">
          请检查上方标注的题目。
        </p>
        <p v-if="submitError" class="form-error" role="alert">
          {{ submitError }}
        </p>
        <div class="share-form-submit">
          <p class="form-hint">
            提交后将生成本轮次、此方向的报名申请。答案提交后不可修改。
          </p>
          <button
            class="form-button form-button--primary"
            type="submit"
            :disabled="submitting"
          >
            {{ submitting ? "正在提交…" : "确认并提交 ↗" }}</button
          ><span v-if="submitting" role="status" class="form-sr-only"
            >正在提交，请稍候</span
          >
        </div>
      </form>
    </template>
  </main>
</template>
