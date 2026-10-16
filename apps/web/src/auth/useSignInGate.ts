import { computed, inject, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { currentSession, syncAuthSession } from "./session";
import { SIGNAL_TRANSITION } from "@/modules/swiss-signal/context";

/** 游客主动操作的统一提示；confirm 时才构造回跳地址。 */
export function useSignInGate() {
  syncAuthSession();
  const route = useRoute();
  const router = useRouter();
  const transition = inject(SIGNAL_TRANSITION, async (change: () => unknown) => {
    await change();
  });
  const action = ref<string | null>(null);
  const signedIn = computed(() => currentSession.value !== null);
  function requireSignIn(label: string): boolean {
    if (signedIn.value) return true;
    action.value = label;
    return false;
  }
  function dismiss(): void {
    action.value = null;
  }
  async function signIn(): Promise<void> {
    dismiss();
    const target = { path: "/", query: { redirect: route.fullPath } };
    await transition(() => router.push(target));
  }
  return { signedIn, action, requireSignIn, dismiss, signIn };
}
