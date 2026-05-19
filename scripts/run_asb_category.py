#!/usr/bin/env python3
"""
run_asb_category.py

按攻击分类运行 ASB 基准测试的脚本。
- 定义分类到 benchmark 文件的映射
- 支持只运行单个分类，或按类别逐个运行对应文件（不会一次跑完所有分类，默认只跑指定分类）
- 使用现有的 scripts/run_asb_benchmark.py wrapper 调用 ASB

用法：
python scripts/run_asb_category.py --category dpi --venv .venv --repeat 1

"""

import argparse
import os
import subprocess
import sys
from datetime import datetime

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCRIPTS_DIR = os.path.join(ROOT, 'scripts')
RESULTS_DIR = os.path.join(ROOT, 'results')

# 请根据仓库中实际文件调整映射
CATEGORY_MAP = {
    'smoke': ['ASB/data/agent_task_test.jsonl'],
    'dpi': ['ASB/data/agent_task_langgraph_smoke.jsonl', 'ASB/data/agent_task_langgraph_finance_5.jsonl'],
    'opi': ['ASB/data/agent_task_pot.jsonl', 'ASB/data/agent_task_pot_msg.jsonl'],
    'pot': ['ASB/data/agent_task_pot_all.jsonl'],
    'mp': ['ASB/data/agent_task.jsonl'],
    'langgraph': ['ASB/data/agent_task_langgraph_smoke.jsonl']
}


def run_cmd(cmd, cwd=None):
    print(f"$ {' '.join(cmd)}")
    proc = subprocess.run(cmd, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    print(proc.stdout)
    return proc.returncode, proc.stdout


def ensure_results_dir():
    os.makedirs(RESULTS_DIR, exist_ok=True)


def find_benchmark(path):
    if os.path.isabs(path) and os.path.exists(path):
        return path
    p1 = os.path.join(ROOT, path)
    p2 = os.path.join(ROOT, 'ASB', os.path.basename(path))
    if os.path.exists(p1):
        return p1
    if os.path.exists(p2):
        return p2
    return None


def run_category(category, python_exec, repeat=1, extra_args=None):
    extra_args = extra_args or []
    if category not in CATEGORY_MAP:
        print(f"未识别的分类: {category}")
        return 2

    files = CATEGORY_MAP[category]
    ensure_results_dir()

    index = 0
    for f in files:
        index += 1
        bench = find_benchmark(f)
        if not bench:
            print(f"跳过：找不到基准文件 {f}")
            continue

        for r in range(max(1, repeat)):
            timestamp = datetime.utcnow().strftime('%Y%m%dT%H%M%SZ')
            out_name = f'asb_{category}_{index}_run{r+1}_{timestamp}.log'
            out_path = os.path.join(RESULTS_DIR, out_name)

            cmd = [python_exec, os.path.join(SCRIPTS_DIR, 'run_asb_benchmark.py'), '--venv', python_exec, '--benchmark', bench, '--output', out_path]
            if extra_args:
                cmd += ['--extra'] + extra_args

            print(f"运行分类 {category} -> {bench} (重复 {r+1})，输出: {out_path}")
            rc, out = run_cmd(cmd, cwd=ROOT)
            if rc != 0:
                print(f"警告：命令返回码 {rc}，查看 {out_path} 以获取更多信息。")

    print('分类运行完成')
    return 0


def main():
    parser = argparse.ArgumentParser(description='Run ASB benchmarks by category')
    parser.add_argument('--category', required=True, help='分类名称，例如 dpi, opi, pot, mp, langgraph, smoke')
    parser.add_argument('--venv', default=sys.executable, help='Python 可执行文件或虚拟环境目录')
    parser.add_argument('--repeat', type=int, default=1, help='每个文件重复运行次数')
    parser.add_argument('--extra', nargs='*', help='传递给 wrapper 的额外参数')

    args = parser.parse_args()

    python_exec = args.venv
    if os.path.isdir(python_exec):
        candidate = os.path.join(python_exec, 'Scripts', 'python.exe') if os.name == 'nt' else os.path.join(python_exec, 'bin', 'python')
        if os.path.exists(candidate):
            python_exec = candidate

    return run_category(args.category, python_exec, repeat=args.repeat, extra_args=args.extra)


if __name__ == '__main__':
    sys.exit(main())
