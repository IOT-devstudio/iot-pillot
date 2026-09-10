"""从 deploy.yml 抽出 ssh-action 的远程部署脚本，替换 Actions 表达式后落盘。

供 deploy/test/run.sh 做打桩测试使用。单独成文件是因为从 YAML 里安全地取出
嵌套的多行字符串，用 sed/awk 很脆，而 YAML 解析是确定性的。
"""

import re
import sys

import yaml

WORKFLOW = ".github/workflows/deploy.yml"


def main() -> int:
    if len(sys.argv) != 2:
        print("用法: extract-server-script.py <输出路径>", file=sys.stderr)
        return 2
    out_path = sys.argv[1]

    with open(WORKFLOW, encoding="utf-8") as f:
        workflow = yaml.safe_load(f)

    script = None
    for step in workflow["jobs"]["build-and-ship"]["steps"]:
        with_block = step.get("with") or {}
        candidate = with_block.get("script")
        if candidate and "docker load" in candidate:
            script = candidate
            break

    if script is None:
        print(f"未在 {WORKFLOW} 的 build-and-ship 中找到服务器部署脚本", file=sys.stderr)
        return 1

    # Actions 表达式在真实运行前已由 runner 替换掉，这里代入一个假的 short sha。
    script = re.sub(r"\$\{\{[^}]*\}\}", "newsha123456", script)

    with open(out_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(script)
    return 0


if __name__ == "__main__":
    sys.exit(main())
