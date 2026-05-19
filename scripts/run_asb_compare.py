#!/usr/bin/env python3
"""
run_asb_compare.py

按下列维度自动执行 ASB 基准并汇总指标：
- 分类（dpi/opi/pot/mp/langgraph/...）
- Agent（langgraph/openclaw 等）
- 防护策略（no_defense, aegisguard, ...）

指标（默认计算方式）：
- ASR (Attack Success Rate) = 成功攻击数 / 总样本数
- RR (Remediation Rate) = 拦截/阻断数 / 总样本数 (decision == Block|Deny)
- Avg Latency = duration 平均值 (字段名可配置)

脚本会调用 `scripts/run_asb_benchmark.py` 来运行单次基准，并解析输出日志（JSONL 或包含 JSON 行）来统计指标。

示例：
python scripts/run_asb_compare.py --category opi --agents langgraph openclaw --defenses no_defense aegisguard --venv .venv --repeat 1

注意：请根据 ASB 输出字段调整 --success-field、--remediate-field 和 --duration-field。
"""

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCRIPTS_DIR = os.path.join(ROOT, 'scripts')
RESULTS_DIR = os.path.join(ROOT, 'results')

CATEGORY_MAP_HINT = 'See scripts/run_asb_category.py for default CATEGORY_MAP'


def run_cmd(cmd, cwd=None):
    print(f"$ {' '.join(cmd)}")
    proc = subprocess.run(cmd, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    print(proc.stdout)
    return proc.returncode, proc.stdout


def ensure_results_dir():
    os.makedirs(RESULTS_DIR, exist_ok=True)


def parse_jsonl_like(path, success_field_candidates, success_values, remediate_field_candidates, remediate_values, duration_field_candidates):
    total = 0
    succ = 0
    remediate = 0
    durations = []

    if not os.path.exists(path):
        print(f"日志文件不存在: {path}")
        return None

    with open(path, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            obj = None
            try:
                obj = json.loads(line)
            except Exception:
                # try to find JSON object inside the line
                idx = line.find('{')
                if idx >= 0:
                    try:
                        obj = json.loads(line[idx:])
                    except Exception:
                        obj = None
            if not obj or not isinstance(obj, dict):
                continue
            total += 1

            # success detection
            for k in success_field_candidates:
                if k in obj:
                    v = obj.get(k)
                    if isinstance(v, bool):
                        if v:
                            succ += 1
                        break
                    if isinstance(v, str):
                        if v.lower() in [sv.lower() for sv in success_values]:
                            succ += 1
                        break
                    if isinstance(v, (int, float)):
                        if v == 1:
                            succ += 1
                        break

            # remediation detection
            for k in remediate_field_candidates:
                if k in obj:
                    v = obj.get(k)
                    if isinstance(v, str):
                        if v.lower() in [rv.lower() for rv in remediate_values]:
                            remediate += 1
                        break
                    if isinstance(v, bool):
                        if v:
                            remediate += 1
                        break

            # duration
            for k in duration_field_candidates:
                if k in obj:
                    try:
                        d = float(obj.get(k) or 0)
                        durations.append(d)
                    except Exception:
                        pass
                    break

    avg_latency = (sum(durations) / len(durations)) if durations else None
    asr = (succ / total) if total else None
    rr = (remediate / total) if total else None

    return {
        'total': total,
        'success': succ,
        'remediate': remediate,
        'avg_latency': avg_latency,
        'ASR': asr,
        'RR': rr
    }


def run_and_parse(python_exec, benchmark_file, extra_args, output_path, parse_args):
    cmd = [python_exec, os.path.join(SCRIPTS_DIR, 'run_asb_benchmark.py'), '--venv', python_exec, '--benchmark', benchmark_file, '--output', output_path]
    if extra_args:
        cmd += ['--extra'] + extra_args
    rc, out = run_cmd(cmd, cwd=ROOT)
    if rc != 0:
        print(f"运行失败，返回码 {rc}")
        return None
    stats = parse_jsonl_like(output_path, parse_args['success_field_candidates'], parse_args['success_values'], parse_args['remediate_field_candidates'], parse_args['remediate_values'], parse_args['duration_field_candidates'])
    return stats


def main():
    parser = argparse.ArgumentParser(description='Run ASB compare across agents and defenses')
    parser.add_argument('--category', required=True, help='分类（dpi/op i/pot/mp/langgraph/smoke）')
    parser.add_argument('--agents', nargs='+', required=True, help='agents 列表，如 langgraph openclaw')
    parser.add_argument('--defenses', nargs='+', required=True, help='防护策略列表，如 no_defense aegisguard')
    parser.add_argument('--venv', default=sys.executable, help='Python 可执行或 venv 目录')
    parser.add_argument('--repeat', type=int, default=1, help='每个配置重复运行次数')
    parser.add_argument('--extra', nargs='*', help='额外参数传递给 ASB 主程序（例如 --verbose）')
    parser.add_argument('--success-field', nargs='*', default=['attack_success','success','result'], help='用于识别攻击是否成功的字段名候选（优先级从左到右）')
    parser.add_argument('--success-value', nargs='*', default=['true','success','succeeded'], help='识别为成功的字符串值')
    parser.add_argument('--remediate-field', nargs='*', default=['decision','remediation','blocked'], help='用于识别被拦截/阻断的字段名候选')
    parser.add_argument('--remediate-value', nargs='*', default=['block','deny','blocked','true'], help='识别为拦截/阻断的字符串值')
    parser.add_argument('--duration-field', nargs='*', default=['duration_ms','duration','latency_ms'], help='时延字段名候选')

    args = parser.parse_args()

    python_exec = args.venv
    if os.path.isdir(python_exec):
        candidate = os.path.join(python_exec, 'Scripts', 'python.exe') if os.name == 'nt' else os.path.join(python_exec, 'bin', 'python')
        if os.path.exists(candidate):
            python_exec = candidate

    ensure_results_dir()

    report = {}
    timestamp = datetime.utcnow().strftime('%Y%m%dT%H%M%SZ')

    parse_args = {
        'success_field_candidates': args.success_field,
        'success_values': args.success_value,
        'remediate_field_candidates': args.remediate_field,
        'remediate_values': args.remediate_value,
        'duration_field_candidates': args.duration_field
    }

    # find benchmark file(s) for category using run_asb_category mapping
    from scripts.run_asb_category import CATEGORY_MAP
    if args.category not in CATEGORY_MAP:
        print(f"未在 scripts/run_asb_category.py 中找到分类 {args.category}. {CATEGORY_MAP_HINT}")
        sys.exit(2)

    benchmarks = CATEGORY_MAP[args.category]

    for agent in args.agents:
        report[agent] = {}
        for defense in args.defenses:
            report[agent][defense] = {}
            for idx, bench in enumerate(benchmarks, start=1):
                for r in range(1, max(1, args.repeat)+1):
                    out_name = f'asb_{args.category}_{agent}_{defense}_{idx}_run{r}_{timestamp}.log'
                    out_path = os.path.join(RESULTS_DIR, out_name)

                    # build extra args to pass agent/defense to ASB main
                    extra = (args.extra or [])[:]
                    extra += [f'--agent', agent, f'--defense', defense]

                    stats = run_and_parse(python_exec, bench, extra, out_path, parse_args)
                    report[agent][defense][f'{idx}_run{r}'] = {
                        'benchmark': bench,
                        'log': out_path,
                        'stats': stats
                    }

    # 保存汇总报告
    rpt_path = os.path.join(RESULTS_DIR, f'asb_compare_{args.category}_{timestamp}.json')
    with open(rpt_path, 'w', encoding='utf-8') as f:
        json.dump(report, f, ensure_ascii=False, indent=2)

    print(f"比较报告已保存：{rpt_path}")
    return 0


if __name__ == '__main__':
    sys.exit(main())
