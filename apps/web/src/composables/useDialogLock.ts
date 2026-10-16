/**
 * 弹窗打开期间的整页滚动锁（issue #85 系列 review 修复的公共化）。
 *
 * 两件事：
 *   1. html + body 双锁——项目给 html 显式设了 overflow: auto，只锁 body
 *      不会传导到视口，高页面照样能滚
 *   2. Element 自带 lock-scroll 只锁 body，弹窗上必须 :lock-scroll="false"
 *      避免与本锁语义重叠
 *
 * 不做 body padding-right 补偿：theme.css 的 html { scrollbar-gutter: stable }
 * 已让滚动条槽位常驻，锁滚动时视口内容宽度本来就不变；再补 15px padding
 * 是双重补偿，实测会把页面内容左推 ~7.5px——正是「开关弹窗页面横跳」的来源。
 *
 * 关闭或组件卸载时全部归还，避免整页永久锁死。
 *
 * 配套居中样式：el-dialog 加 modal-class="centered-dialog"（theme.css）。
 */
import { onBeforeUnmount, watch } from "vue";

export function useDialogLock(visible: () => boolean): void {
  /** 测试环境的假 document 没有 documentElement/body，直接降级为空操作 */
  const canTouchDom = (): boolean =>
    typeof document !== "undefined" &&
    document.documentElement !== undefined &&
    document.body !== undefined;

  watch(visible, (open) => {
    if (!canTouchDom()) return;
    document.documentElement.style.overflow = open ? "hidden" : "";
    document.body.style.overflow = open ? "hidden" : "";
  }, { immediate: true });

  onBeforeUnmount(() => {
    if (!canTouchDom()) return;
    document.documentElement.style.overflow = "";
    document.body.style.overflow = "";
  });
}
