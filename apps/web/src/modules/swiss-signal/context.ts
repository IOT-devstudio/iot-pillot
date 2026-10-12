import type { InjectionKey, Ref } from "vue";
import type { AuthMode } from "@/auth/submit";

export const SIGNAL_MODE: InjectionKey<Ref<AuthMode>> = Symbol("signal-mode");
export const SIGNAL_BUSY: InjectionKey<Ref<boolean>> = Symbol("signal-busy");
export const SIGNAL_TRANSITION: InjectionKey<
  (change: () => unknown) => Promise<void>
> = Symbol("signal-transition");
