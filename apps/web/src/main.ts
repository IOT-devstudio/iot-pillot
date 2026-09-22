import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
// Element Plus 的样式引入。按需方案（unplugin-vue-components 的
// ElementPlusResolver）只会为**模板里出现的组件**注入样式，所以这里要手动补两部分：
//
// 1. base.css —— 所有组件共用的基础层，含 .el-overlay 的定位规则（~8KB）。
// 2. 命令式调用的组件样式 —— ElMessageBox / ElMessage 是用函数调起来的
//    （ElMessageBox.confirm(...)），模板里没有对应标签，resolver 看不到它们，
//    样式也就永远不会被注入。缺了 el-message-box.css，弹窗会以
//    position: static 挤在页面左上角；缺了 el-message.css，toast 同样错位。
//
// 只补齐真正用到的，不引全量 index.css（~360KB）——其余组件仍走按需注入。
import "element-plus/theme-chalk/base.css";
import "element-plus/theme-chalk/el-message-box.css";
import "element-plus/theme-chalk/el-message.css";
// 主题覆盖必须排在最后，否则会被 Element 的默认值盖回去。
import "./styles/theme.css";

const app = createApp(App);
app.use(router);
app.mount("#app");
