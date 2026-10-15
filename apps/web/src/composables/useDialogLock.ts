/**
 * 弹窗打开期间的整页滚动锁（issue #85 系列 review 修复的公共化）。
 *
 * 三件事，缺一不可：
 *   1. html + body 双锁——项目给 html 显式设了 overflow: auto，只锁 body
 *      不会传导到视口，高页面照样能滚
 *   2. body padding-right 补偿滚动条宽度——滚动条消失会让视口变宽 15px，
 *      开关弹窗时页面「抽动一下再恢复」
 *   3. Element 自带 lock-scroll 只锁 body 且也做 padding 补偿，会与本锁
 *      叠加成双重 padding——弹窗上必须 :lock-scroll="false"
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
    if (open) {
      const scrollbarGap =
        window.innerWidth - document.documentElement.clientWidth;
      document.documentElement.style.overflow = "hidden";
      document.body.style.overflow = "hidden";
      if (scrollbarGap > 0) {
        document.body.style.paddingRight = `${scrollbarGap}px`;
      }
    } else {
      document.documentElement.style.overflow = "";
      document.body.style.overflow = "";
      document.body.style.paddingRight = "";
    }
  }, { immediate: true });

  onBeforeUnmount(() => {
    if (!canTouchDom()) return;
    document.documentElement.style.overflow = "";
    document.body.style.overflow = "";
    document.body.style.paddingRight = "";
  });
}
