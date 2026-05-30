#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
AegisGuard 防御效果分析脚本 - OpenClaw Agent 版
用于分析和对比无防御 vs AegisGuard 防御的测试结果
"""

import os
import json
import csv
import sys
from pathlib import Path
from datetime import datetime
from collections import defaultdict
from typing import Dict, List, Tuple

class AegisGuardAnalyzer:
    """AegisGuard 测试结果分析器"""
    
    def __init__(self, logs_root: str):
        self.logs_root = Path(logs_root)
        self.results = defaultdict(dict)
        
    def parse_log_file(self, log_path: Path) -> Dict:
        """从日志文件解析测试结果"""
        results = {
            'total': 0,
            'success': 0,
            'failed': 0,
            'refusal': 0,
            'latency': [],
        }
        
        try:
            with open(log_path, 'r', encoding='utf-8', errors='ignore') as f:
                for line in f:
                    # 尝试解析 JSON 行
                    if '{' in line and '}' in line:
                        try:
                            idx = line.find('{')
                            json_str = line[idx:]
                            data = json.loads(json_str)
                            
                            # 解析 ASB 输出字段
                            if 'attack_success' in data:
                                results['total'] += 1
                                if data.get('attack_success'):
                                    results['success'] += 1
                                else:
                                    results['failed'] += 1
                            
                            if data.get('refused'):
                                results['refusal'] += 1
                            
                            if 'latency_ms' in data:
                                try:
                                    results['latency'].append(float(data['latency_ms']))
                                except:
                                    pass
                        except:
                            pass
        except Exception as e:
            print(f"⚠ 无法读取日志文件 {log_path}: {e}", file=sys.stderr)
        
        return results
    
    def calculate_metrics(self, results: Dict) -> Dict:
        """计算防御指标"""
        total = results.get('total', 0)
        success = results.get('success', 0)
        refusal = results.get('refusal', 0)
        latencies = results.get('latency', [])
        
        asr = (success / total * 100) if total > 0 else 0
        refusal_rate = (refusal / total * 100) if total > 0 else 0
        avg_latency = sum(latencies) / len(latencies) if latencies else 0
        
        return {
            'total': total,
            'attack_success': success,
            'asr': round(asr, 2),
            'refusal_rate': round(refusal_rate, 2),
            'avg_latency_ms': round(avg_latency, 2),
        }
    
    def load_all_results(self):
        """加载所有日志文件的结果"""
        attack_types = ['dpi', 'opi', 'mp', 'mixed']
        modes = ['no_defense', 'with_aegisguard']
        
        for attack_type in attack_types:
            for mode in modes:
                log_file = self.logs_root / f"{attack_type}_{mode}.log"
                if log_file.exists():
                    raw_results = self.parse_log_file(log_file)
                    metrics = self.calculate_metrics(raw_results)
                    key = f"{attack_type}_{mode}"
                    self.results[attack_type][mode] = metrics
        
        return self.results
    
    def generate_comparison_table(self) -> str:
        """生成防御效果对比表格"""
        output = []
        output.append("\n" + "=" * 120)
        output.append("AegisGuard OpenClaw 测试结果对比表")
        output.append("=" * 120)
        output.append(f"\n生成时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
        
        # 表头
        header = f"{'攻击类型':<12} {'模式':<15} {'样本数':<8} {'攻击成功':<10} {'ASR':<10} {'拒绝率':<10} {'平均延迟':<12}"
        output.append(header)
        output.append("-" * 120)
        
        # 数据行
        for attack_type in ['dpi', 'opi', 'mp', 'mixed']:
            if attack_type not in self.results:
                continue
            
            no_defense = self.results[attack_type].get('no_defense', {})
            with_defense = self.results[attack_type].get('with_aegisguard', {})
            
            # 无防御行
            if no_defense:
                line = (f"{attack_type.upper():<12} "
                       f"{'无防御':<15} "
                       f"{no_defense.get('total', 0):<8} "
                       f"{no_defense.get('attack_success', 0):<10} "
                       f"{no_defense.get('asr', 0):.2f}%{'':<6} "
                       f"{no_defense.get('refusal_rate', 0):.2f}%{'':<6} "
                       f"{no_defense.get('avg_latency_ms', 0):.1f}ms")
                output.append(line)
            
            # 有防御行
            if with_defense:
                line = (f"{'':<12} "
                       f"{'AegisGuard':<15} "
                       f"{with_defense.get('total', 0):<8} "
                       f"{with_defense.get('attack_success', 0):<10} "
                       f"{with_defense.get('asr', 0):.2f}%{'':<6} "
                       f"{with_defense.get('refusal_rate', 0):.2f}%{'':<6} "
                       f"{with_defense.get('avg_latency_ms', 0):.1f}ms")
                output.append(line)
            
            # 对比行
            if no_defense and with_defense:
                asr_reduction = ((no_defense.get('asr', 0) - with_defense.get('asr', 0)) / 
                                max(no_defense.get('asr', 1), 0.01) * 100)
                
                line = (f"{'':<12} "
                       f"{'📊 差异':<15} "
                       f"{'':<8} "
                       f"↓ {no_defense.get('attack_success', 0) - with_defense.get('attack_success', 0):<8} "
                       f"↓ {asr_reduction:.1f}%{'':<4} "
                       f"↑ {with_defense.get('refusal_rate', 0) - no_defense.get('refusal_rate', 0):.2f}%{'':<4} ")
                output.append(line)
            
            output.append("-" * 120)
        
        return "\n".join(output)
    
    def generate_summary_report(self) -> str:
        """生成总结报告"""
        output = []
        output.append("\n" + "=" * 120)
        output.append("防御效果关键指标总结")
        output.append("=" * 120 + "\n")
        
        total_no_defense_asr = 0
        total_with_defense_asr = 0
        count = 0
        
        for attack_type in ['dpi', 'opi', 'mp', 'mixed']:
            if attack_type not in self.results:
                continue
            
            no_defense = self.results[attack_type].get('no_defense', {})
            with_defense = self.results[attack_type].get('with_aegisguard', {})
            
            if no_defense and with_defense:
                no_def_asr = no_defense.get('asr', 0)
                with_def_asr = with_defense.get('asr', 0)
                asr_reduction = ((no_def_asr - with_def_asr) / max(no_def_asr, 0.01) * 100)
                
                total_no_defense_asr += no_def_asr
                total_with_defense_asr += with_def_asr
                count += 1
                
                status = "✓ 优秀" if asr_reduction >= 80 else "⚠ 良好" if asr_reduction >= 50 else "✗ 需改进"
                
                output.append(f"{attack_type.upper():6s} | 无防御: {no_def_asr:6.2f}% | "
                            f"防御后: {with_def_asr:6.2f}% | "
                            f"下降: {asr_reduction:6.1f}% | {status}")
        
        if count > 0:
            avg_reduction = ((total_no_defense_asr - total_with_defense_asr) / 
                           max(total_no_defense_asr, 0.01) * 100)
            output.append("\n" + "-" * 120)
            output.append(f"平均防御效果 | 无防御平均 ASR: {total_no_defense_asr/count:.2f}% | "
                        f"防御平均 ASR: {total_with_defense_asr/count:.2f}% | "
                        f"平均下降: {avg_reduction:.1f}%")
        
        return "\n".join(output)
    
    def generate_json_report(self, output_file: str):
        """生成 JSON 格式报告"""
        report = {
            'timestamp': datetime.now().isoformat(),
            'agent_type': 'openclaw',
            'results': dict(self.results),
        }
        
        with open(output_file, 'w', encoding='utf-8') as f:
            json.dump(report, f, indent=2, ensure_ascii=False)
        
        return output_file

def main():
    import argparse
    
    parser = argparse.ArgumentParser(description='AegisGuard 防御效果分析')
    parser.add_argument('--logs-root', type=str, default='ASB/logs/aegisguard_openclaw_test',
                       help='测试日志目录')
    parser.add_argument('--output', type=str, default='aegisguard_test_report.json',
                       help='输出报告文件')
    
    args = parser.parse_args()
    
    analyzer = AegisGuardAnalyzer(args.logs_root)
    analyzer.load_all_results()
    
    # 输出对比表
    print(analyzer.generate_comparison_table())
    
    # 输出总结
    print(analyzer.generate_summary_report())
    
    # 输出 JSON 报告
    report_file = analyzer.generate_json_report(args.output)
    print(f"\n✓ JSON 报告已生成: {report_file}\n")

if __name__ == '__main__':
    main()
