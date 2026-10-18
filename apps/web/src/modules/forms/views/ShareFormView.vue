<script setup lang="ts">
/**
 * 用户填写页 `/f/:shareToken`（issue #183）。
 *
 * 只做呈现与交互编排：表单业务（读取、校验、提交、防重、账号切换保护）都在
 * useFormFilling 里，这里不重复实现，也不改接口契约。
 *
 * 状态判定优先级固定为：加载 → 加载失败 → 成功/已提交 → 关闭 → 可填写。
 * 成功与已提交必须排在关闭之前：已提交的用户不该看到「暂不接受提交」。
 */
import { computed, nextTick, onBeforeUnmount, watch } from "vue";
import {
  useRoute,
  RouterLink,
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
} from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import FormFields from "../components/FormFields.vue";
import { firstErrorFieldId } from "../model";
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

const hasErrors = computed(() => Object.keys(errors.value).length > 0);
/** 空说明不占内容区：接口返回空串或纯空白时整块不渲染。 */
const description = computed(() => data.value?.form.description?.trim() ?? "");
const submittedAt = computed(() =>
  result.value
    ? new Date(result.value.submission.submitted_at).toLocaleString("zh-CN")
    : "",
);

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

function reducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-reduced-motion: reduce)").matches === true
  );
}

/**
 * 提交后按**题目顺序**定位首个错误字段（issue #183 §4）：
 * 顺序由 firstErrorFieldId 保证，不依赖错误对象的键枚举顺序；
 * 先滚动到可见再聚焦，避免浏览器自己的滚动把位置又顶走。
 */
async function submit() {
  const sent = await send();
  await nextTick();
  if (sent || data.value === null) return;
  const id = firstErrorFieldId(data.value.fields, errors.value);
  if (id === null) return;
  const control = document.getElementById(`answer-${id}`);
  if (control === null) return;
  control.scrollIntoView({
    block: "center",
    behavior: reducedMotion() ? "auto" : "smooth",
  });
  control.focus({ preventScroll: true });
}
</script>
<template>
  <main class="share-form-page">
    <!-- 与用户首页/个人设置同一层品牌柔光晕（.glass-backdrop）：蓝色在左上、
         黄绿色在右下，纯装饰、不接事件。内容层由 CSS 抬到它上面。 -->
    <div class="glass-backdrop" aria-hidden="true"></div>
    <header class="share-form-header">
      <p class="share-form-kicker">RECRUITMENT / YOUR NEXT CHAPTER</p>
      <template v-if="data">
        <p class="share-form-meta">
          轮次 #{{ data.form.round_id }} · {{ data.form.direction_code }}
        </p>
        <h1 class="share-form-title">{{ data.form.title }}</h1>
        <p v-if="description" class="share-form-description">{{ description }}</p>
      </template>
    </header>
    <section v-if="loading" class="share-form-state" role="status">
      正在打开表单…
    </section>
    <section v-else-if="loadError" class="share-form-state" role="alert">
      <h2>暂时无法打开表单</h2>
      <p class="share-form-state__body">{{ loadError }}</p>
      <button
        class="form-button"
        type="button"
        @click="load(String(route.params.shareToken))"
      >
        重新加载
      </button>
    </section>
    <template v-else-if="data">
      <section v-if="done" class="share-form-result" role="status">
        <p class="share-form-kicker">
          {{ result ? "RECEIVED / THANK YOU" : "ALREADY RECEIVED" }}
        </p>
        <h2>
          {{ result ? "提交成功，谢谢你的认真填写。" : "你已提交过这张表单" }}
        </h2>
        <p class="share-form-state__body">每位用户仅可提交一次。</p>
        <dl v-if="result" class="share-form-facts">
          <div>
            <dt>申请编号</dt>
            <dd>#{{ result.application_id }}</dd>
          </div>
          <div>
            <dt>提交时间</dt>
            <dd>{{ submittedAt }}</dd>
          </div>
        </dl>
        <RouterLink class="form-button form-button--primary" to="/user/home"
          >{{ result ? "查看我的申请" : "返回用户首页" }} ↗</RouterLink
        >
      </section>
      <section
        v-else-if="!data.form.is_open"
        class="share-form-result"
        role="status"
      >
        <h2>当前表单暂不接受提交</h2>
        <p class="share-form-state__body">
          可返回首页了解其他招新方向。
        </p>
        <RouterLink class="form-button" to="/user/home"
          >返回用户首页 ↗</RouterLink
        >
      </section>
      <form v-else novalidate @submit.prevent="submit">
        <section
          class="share-form-identity"
          aria-labelledby="share-form-identity-title"
        >
          <h2 id="share-form-identity-title" class="share-form-section-title">
            当前申请身份
          </h2>
          <dl class="share-form-facts">
            <div>
              <dt>姓名</dt>
              <dd>{{ profile?.name || profile?.username }}</dd>
            </div>
            <div>
              <dt>班级</dt>
              <dd>{{ profile?.detail?.class || "未填写" }}</dd>
            </div>
            <div>
              <dt>学号</dt>
              <dd>{{ profile?.detail?.student_id || "未填写" }}</dd>
            </div>
            <div>
              <dt>账号邮箱</dt>
              <dd>
                {{ profile?.detail?.email || profile?.username
                }}<span class="share-form-locked">（不可修改）</span>
              </dd>
            </div>
          </dl>
          <RouterLink
            class="share-form-link"
            to="/user/settings"
            target="_blank"
            rel="noopener"
            >更新个人资料（新标签页）↗</RouterLink
          >
        </section>
        <section
          class="share-form-questions"
          aria-labelledby="share-form-questions-title"
        >
          <h2 id="share-form-questions-title" class="share-form-section-title">
            表单问题
          </h2>
          <fieldset class="form-fieldset" :disabled="submitting">
            <legend class="form-sr-only">表单问题</legend>
            <FormFields
              v-model="values"
              :fields="data.fields"
              :errors="errors"
              :disabled="submitting"
            />
          </fieldset>
        </section>
        <section
          class="share-form-submit"
          aria-labelledby="share-form-submit-title"
        >
          <h2 id="share-form-submit-title" class="share-form-section-title">
            提交
          </h2>
          <p v-if="hasErrors" class="form-error" role="alert">
            请检查上方标注的题目后再次提交。
          </p>
          <p v-if="submitError" class="form-error" role="alert">
            {{ submitError }}
          </p>
          <p class="form-hint">
            提交后将生成本轮次、此方向的申请；答案提交后不可修改。
          </p>
          <button
            class="form-button form-button--primary share-form-submit__button"
            type="submit"
            :disabled="submitting"
          >
            {{ submitting ? "正在提交…" : "确认并提交 ↗" }}
          </button>
          <span v-if="submitting" role="status" class="form-sr-only"
            >正在提交，请稍候</span
          >
        </section>
      </form>
    </template>
  </main>
</template>
