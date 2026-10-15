import { computed, ref, watch } from "vue";
import {
  fetchShareForm,
  submitForm,
  type FormAnswers,
  type ShareForm,
  type SubmitResult,
} from "@/api/forms";
import { fetchMyProfile, type MyProfile } from "@/api/profile";
import { answerErrors, describeFormError, initialAnswers } from "./model";

export interface FillingDeps {
  read: (token: string) => Promise<ShareForm>;
  profile: () => Promise<MyProfile>;
  submit: (
    token: string,
    values: FormAnswers,
    expectedUserId: number,
  ) => Promise<SubmitResult>;
}
export function useFormFilling(
  deps: FillingDeps = {
    read: fetchShareForm,
    profile: fetchMyProfile,
    submit: submitForm,
  },
) {
  const data = ref<ShareForm | null>(null);
  const profile = ref<MyProfile | null>(null);
  const values = ref<FormAnswers>({});
  const errors = ref<Record<string, string>>({});
  const loading = ref(false);
  const loadError = ref("");
  const submitting = ref(false);
  const submitError = ref("");
  const result = ref<SubmitResult | null>(null);
  const done = computed(
    () => data.value?.submitted === true || result.value !== null,
  );
  const hasDraft = computed(
    () =>
      !done.value &&
      Object.values(values.value).some((value) =>
        Array.isArray(value) ? value.length > 0 : value.length > 0,
      ),
  );
  let token = "";
  let revision = 0;
  const stopValidation = watch(
    values,
    () => {
      if (data.value && Object.keys(errors.value).length)
        errors.value = answerErrors(data.value.fields, values.value);
    },
    { deep: true, flush: "sync" },
  );
  async function load(nextToken: string) {
    const current = ++revision;
    token = nextToken;
    data.value = null;
    profile.value = null;
    values.value = {};
    result.value = null;
    errors.value = {};
    loadError.value = "";
    submitError.value = "";
    loading.value = true;
    try {
      const [definition, user] = await Promise.all([
        deps.read(nextToken),
        deps.profile(),
      ]);
      if (current !== revision) return;
      data.value = definition;
      profile.value = user;
      values.value = initialAnswers(definition.fields);
    } catch (error) {
      if (current === revision) loadError.value = describeFormError(error);
    } finally {
      if (current === revision) loading.value = false;
    }
  }
  async function send(): Promise<boolean> {
    if (
      loading.value ||
      submitting.value ||
      done.value ||
      !data.value?.form.is_open ||
      !profile.value
    )
      return false;
    errors.value = answerErrors(data.value.fields, values.value);
    submitError.value = "";
    if (Object.keys(errors.value).length) return false;
    const current = revision;
    submitting.value = true;
    try {
      // Capture answers and identity together; the requester rejects cross-tab account changes.
      const answers = Object.fromEntries(
        Object.entries(values.value).map(([id, value]) => [
          id,
          Array.isArray(value) ? [...value] : value,
        ]),
      );
      const submitted = await deps.submit(
        token,
        answers,
        profile.value.user_id,
      );
      if (current !== revision) return false;
      result.value = submitted;
      return true;
    } catch (error) {
      if (current === revision) submitError.value = describeFormError(error);
      return false;
    } finally {
      submitting.value = false;
    }
  }
  function dispose() {
    revision++;
    stopValidation();
  }
  return {
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
  };
}
