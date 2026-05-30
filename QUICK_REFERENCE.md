# AegisGuard OpenClaw 测试 - 快速参考卡

## 🚀 一键启动

```powershell
# 1. 进入项目目录
cd f:\2026信安赛\AegisGuard

# 2. 激活环境
.venv\Scripts\Activate.ps1

# 3. 启动后端（新终端中保持运行）
cd backend && go run ./cmd/server

# 4. 运行完整测试（返回项目目录后）
cd .. && .\test_aegisguard_openclaw_complete.ps1
```

---

## 📊 测试架构

```
┌─────────────────────────────────────────────────────┐
│         AegisGuard OpenClaw 防御测试框架             │
├─────────────────────────────────────────────────────┤
│                                                     │
│  无防御组 (Baseline)    vs    AegisGuard防御组     │
│  ├─ DPI 测试            vs    DPI + 防御           │
│  ├─ OPI 测试            vs    OPI + 防御           │
│  ├─ MP 测试             vs    MP + 防御            │
│  └─ Mixed 测试          vs    Mixed + 防御         │
│                                                     │
│  📊 对比指标：                                       │
│  ├─ ASR (攻击成功率)                                │
│  ├─ 任务完成率                                      │
│  ├─ 拒绝率                                          │
│  └─ 平均延迟                                        │
│                                                     │
└─────────────────────────────────────────────────────┘
```

---

## ⏱️ 预估时间表

| 阶段 | 任务 | 耗时 |
|------|------|------|
| 0 | 环境准备 | 5分钟 |
| 1 | DPI 两组 | 30-40分钟 |
| 2 | OPI 两组 | 20-30分钟 |
| 3 | MP 两组 | 20-30分钟 |
| 4 | Mixed 两组 | 20-30分钟 |
| 5 | 结果分析 | 5分钟 |
| **总计** | | **2-4小时** |

---

## 📋 生成的配置文件

自动创建在 `ASB/config/`：

```
DPI_openclaw_none.yml              # DPI 无防御
DPI_openclaw_aegisguard.yml        # DPI + 防御
OPI_openclaw_none.yml              # OPI 无防御  
OPI_openclaw_aegisguard.yml        # OPI + 防御
MP_openclaw_none.yml               # MP 无防御
MP_openclaw_aegisguard.yml         # MP + 防御
Mixed_openclaw_none.yml            # Mixed 无防御
Mixed_openclaw_aegisguard.yml      # Mixed + 防御
```

---

## 📁 结果输出位置

```
ASB/logs/aegisguard_openclaw_test/
├─ dpi_no_defense.log
├─ dpi_with_aegisguard.log
├─ opi_no_defense.log
├─ opi_with_aegisguard.log
├─ mp_no_defense.log
├─ mp_with_aegisguard.log
├─ mixed_no_defense.log
└─ mixed_with_aegisguard.log

ASB/logs/direct_prompt_injection/gpt-4o-mini/no_memory/
├─ fake_completion-all_.csv         (DPI 结果)
├─ escape_characters-all_.csv
└─ naive-all_.csv

# 其他攻击类型类似...
```

---

## 📊 核心指标速查

| 指标缩写 | 全名 | 计算方式 | 理想值 |
|---------|------|--------|--------|
| **ASR** | Attack Success Rate | 攻击成功数/总数 | **越低越好** |
| **SR** | Success Rate | 任务完成数/总数 | **越高越好** |
| **RR** | Refusal Rate | 拒绝数/总数 | **中等** |
| **ASR-R** | ASR Reduction | (无防ASR-有防ASR)/无防ASR×100% | **>80%** |

---

## 🎯 预期结果参照

基于实际 LangGraph 测试数据：

### DPI (预期最优)
```
无防御:  ASR=17.33%  SR=77.33%  RR=1.33%
防御后:  ASR=0.00%   SR=90.67%  RR=0.00%
效果:    ✓ 完全防御，实用性提升
```

### OPI (预期保守)
```
无防御:  ASR=0.00%   SR=100.00% RR=0.00%
防御后:  ASR=0.00%   SR=88.00%  RR=12.00%
效果:    ⚠ 过度拒绝，可优化
```

### MP (预期良好)
```
无防御:  ASR=4.00%   SR=96.00%  RR=0.00%
防御后:  ASR=0.00%   SR=100.00% RR=0.00%
效果:    ✓ 有效防御，无负面影响
```

### Mixed (预期良好)
```
无防御:  ASR=0.00%   SR=92.00%  RR=0.00%
防御后:  ASR=0.00%   SR=96.00%  RR=0.00%
效果:    ✓ 保持防御，提升可用性
```

---

## 🔍 实时监控命令

### 查看当前进度
```powershell
# 检查已完成的日志大小
Get-ChildItem "ASB\logs\aegisguard_openclaw_test\" -Filter "*.log" | 
  ForEach-Object { Write-Host "$($_.Name): $([math]::Round($_.Length/1KB))KB" }
```

### 实时追踪日志
```powershell
# 跟踪某个正在运行的测试
Get-Content "ASB\logs\aegisguard_openclaw_test\dpi_no_defense.log" -Wait -Tail 20
```

### 检查结果文件
```powershell
# 统计某类攻击的样本数
$csv = "ASB\logs\direct_prompt_injection\gpt-4o-mini\no_memory\fake_completion-all_.csv"
(Import-Csv $csv).Count  # 显示总行数
```

---

## 🛠️ 快速调试命令

### 问题：后端未连接
```powershell
# 检查健康状态
curl http://localhost:8090/health

# 启动后端
cd backend && go run ./cmd/server
```

### 问题：配置文件缺失
```powershell
# 检查配置
dir "ASB\config\*openclaw*.yml"

# 查看某个配置内容
cat "ASB\config\DPI_openclaw_none.yml"
```

### 问题：CSV文件为空
```powershell
# 查看原始日志错误
Get-Content "ASB\logs\direct_prompt_injection\gpt-4o-mini\no_memory\*.log" | 
  Select-String "error|Error|ERROR|Exception"
```

---

## 📈 生成对比报告

```powershell
# 自动生成防御效果对比表
python analyze_aegisguard_results.py `
  --logs-root "ASB\logs\aegisguard_openclaw_test" `
  --output "test_report.json"

# 输出示例：
# ════════════════════════════════════════════════════
# AegisGuard OpenClaw 测试结果对比表
# ════════════════════════════════════════════════════
# 攻击类型  模式       样本数  攻击成功  ASR    拒绝率
# ────────────────────────────────────────────────────
# DPI      无防御      75     13       17.33% 1.33%
#          AegisGuard  75     0        0.00%  0.00%
# 📊 差异                     ↓ 13     ↓ 100%
# ────────────────────────────────────────────────────
# ...
```

---

## ✅ 验证清单

- [ ] 后端服务运行正常 (`curl http://localhost:8090/health`)
- [ ] ASB 环境完整 (scripts/agent_attack.py 存在)
- [ ] 配置文件已创建 (8个 yml 文件)
- [ ] 日志目录已创建 (`ASB/logs/aegisguard_openclaw_test/`)
- [ ] 所有测试已完成 (8个 .log 文件)
- [ ] CSV 结果已生成 (多个 .csv 文件)
- [ ] 防御配置文件包含 `gateway_url` 字段
- [ ] 结果分析脚本成功运行

---

## 💡 常见坑点

1. **后端端口被占用**
   - 检查：`netstat -ano | findstr :8090`
   - 解决：杀死进程后重新启动

2. **OpenClaw 超时**
   - 症状：测试卡住很久
   - 解决：检查 openclaw_state/ 目录权限

3. **Python 编码错误**
   - 症状：日志显示中文乱码
   - 解决：脚本已设置 `PYTHONIOENCODING=utf-8`

4. **网络延迟**
   - 症状：ASR 和任务成功率异常低
   - 解决：增加超时时间，或检查 LLM API 连接

5. **防御未激活**
   - 症状：防御组 ASR 与无防御相同
   - 解决：检查配置文件中 `defense_enabled: true` 是否存在

---

## 📞 获取帮助

### 查看完整指南
```powershell
Get-Content "TEST_GUIDE_OPENCLAW.md" | more
```

### 检查日志错误
```powershell
Get-Content "ASB\logs\aegisguard_openclaw_test\*.log" | 
  Where-Object { $_ -match "error|Error|ERROR|Exception" } | 
  Select-Object -First 20
```

### 提交问题
- 包含：测试命令、错误日志、环境信息
- 位置：项目 issue tracker

---

**祝测试顺利！** 🎉

最后更新：2026-05-29
