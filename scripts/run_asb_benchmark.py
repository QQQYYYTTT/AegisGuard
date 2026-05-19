#!/usr/bin/env python3
"""
run_asb_benchmark.py

用途：从仓库根目录运行 ASB 基准测试的 wrapper。会：
- 检查 ASB 目录是否存在
- 可选创建虚拟环境（推荐）
- 安装 ASB 所需依赖（使用 requirements.txt）
- 调用 ASB 的 main_attacker.py 执行指定的基准用例
- 将结果保存到 results/asb_results_*.json

用法示例：
python scripts/run_asb_benchmark.py --benchmark data/agent_task_pot.jsonl --output results/out.json --venv .venv

注意：本脚本仅作为自动化 wrapper；具体参数请参考 ASB/main_attacker.py
"""

import argparse
import os
import subprocess
import sys
import json
from datetime import datetime

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ASB_DIR = os.path.join(ROOT, 'ASB')
RESULTS_DIR = os.path.join(ROOT, 'results')


def run_cmd(cmd, cwd=None, env=None):
    print(f"$ {' '.join(cmd)} (cwd={cwd})")
    proc = subprocess.run(cmd, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    print(proc.stdout)
    return proc.returncode, proc.stdout


def ensure_results_dir():
    os.makedirs(RESULTS_DIR, exist_ok=True)


def install_requirements(python_exec, requirements_file):
    if not os.path.exists(requirements_file):
        print(f"requirements 文件不存在: {requirements_file}")
        return 1
    return run_cmd([python_exec, '-m', 'pip', 'install', '-r', requirements_file], cwd=ASB_DIR)[0]


def run_asb(python_exec, benchmark_file, extra_args, output_path):
    main_py = os.path.join(ASB_DIR, 'main_attacker.py')
    if not os.path.exists(main_py):
        print(f"找不到 ASB 主程序：{main_py}")
        return 2, "missing main_attacker.py"

    cmd = [python_exec, main_py]
    if benchmark_file:
        cmd += ['--task-file', benchmark_file]
    cmd += extra_args

    # 将输出同时写入文件
    rc, out = run_cmd(cmd, cwd=ASB_DIR)
    if output_path:
        with open(output_path, 'w', encoding='utf-8') as f:
            f.write(out)
    return rc, out


def main():
    parser = argparse.ArgumentParser(description='Run ASB benchmark wrapper')
    parser.add_argument('--venv', help='Python executable or virtualenv folder (e.g. .venv)', default=sys.executable)
    parser.add_argument('--benchmark', help='ASB benchmark task file (相对于仓库根或 ASB 文件夹)', default='')
    parser.add_argument('--requirements', help='requirements file 相对于 ASB 文件夹', default='requirements.txt')
    parser.add_argument('--output', help='输出文件路径 (相对于仓库根)', default='')
    parser.add_argument('--extra', help='传递给 main_attacker.py 的额外参数（用空格分隔）', nargs='*', default=[])
    args = parser.parse_args()

    if not os.path.isdir(ASB_DIR):
        print('错误：未检测到 ASB 目录。请先拉取 ASB 代码。参考 TESTING_ASB.md')
        return 10

    python_exec = args.venv
    # 如果传入的是 venv 目录，尝试解析实际 python 可执行文件
    if os.path.isdir(python_exec):
        candidate = os.path.join(python_exec, 'Scripts', 'python.exe') if os.name == 'nt' else os.path.join(python_exec, 'bin', 'python')
        if os.path.exists(candidate):
            python_exec = candidate

    print('使用 Python：', python_exec)

    req_path = os.path.join(ASB_DIR, args.requirements)
    print('安装依赖：', req_path)
    rc = install_requirements(python_exec, req_path)
    if rc != 0:
        print('安装依赖失败，退出')
        return rc

    ensure_results_dir()
    timestamp = datetime.utcnow().strftime('%Y%m%dT%H%M%SZ')
    default_output = os.path.join(RESULTS_DIR, f'asb_results_{timestamp}.log')
    output_path = args.output or default_output

    # 处理 benchmark 路径
    benchmark_file = args.benchmark
    if benchmark_file and not os.path.isabs(benchmark_file):
        # 优先尝试 ASB 下路径
        candidate1 = os.path.join(ASB_DIR, benchmark_file)
        candidate2 = os.path.join(ROOT, benchmark_file)
        if os.path.exists(candidate1):
            benchmark_file = candidate1
        elif os.path.exists(candidate2):
            benchmark_file = candidate2
        else:
            print('警告：找不到指定的 benchmark 文件，继续运行框架默认任务')
            benchmark_file = ''

    print('输出日志：', output_path)
    rc, out = run_asb(python_exec, benchmark_file, args.extra, output_path)

    if rc == 0:
        print('ASB 基准测试完成，输出保存在：', output_path)
    else:
        print('ASB 基准测试执行失败，返回码：', rc)
    return rc


if __name__ == '__main__':
    sys.exit(main())
