# ASB 测试命令指南

本文档汇总项目中所有 ASB（Agent Security Benchmark）测试命令，方便直接复制执行。

---

## 一、环境准备

### 1.1 创建虚拟环境

```powershell
# Windows PowerShell
python -m venv .venv
.\.venv\Scripts\Activate.ps1
pip install -r ASB\requirements.txt
```

```bash
# Linux/macOS
python3 -m venv .venv
source .venv/bin/activate
pip install -r ASB/requirements.txt
```

### 1.2 配置 LLM API 环境变量

**重要**：运行测试前必须配置 LLM API，否则脚本会报错提示。

```powershell
# Windows PowerShell - 使用 OpenAI API
$env:LANGGRAPH_OPENAI_BASE_URL='https://api.openai.com/v1'
$env:LANGGRAPH_OPENAI_API_KEY='sk-your-api-key'
$env:LANGGRAPH_OPENAI_MODEL='gpt-4o-mini'
```

```powershell
# Windows PowerShell - 使用第三方 API（如 qnaigc）
$env:LANGGRAPH_OPENAI_BASE_URL='https://api.qnaigc.com/v1'
$env:LANGGRAPH_OPENAI_API_KEY='sk-e1eeb1a96e487ab9065c89a18997f5bb4bf7149ddf93ee535c4c2dff522a1a78'
$env:LANGGRAPH_OPENAI_MODEL='deepseek/deepseek-v4-flash'
```

```bash
# Linux/macOS
export LANGGRAPH_OPENAI_BASE_URL='https://api.openai.com/v1'
export LANGGRAPH_OPENAI_API_KEY='sk-your-api-key'
export LANGGRAPH_OPENAI_MODEL='gpt-4o-mini'
```

---

## 二、主要测试脚本

| 脚本路径 | 用途 | 输出目录 |
|---------|------|---------|
| `experiments/asb/langgraph/run_batch.py` | **主测试脚本**，支持完整参数配置 | `ASB/logs/langgraph_batch/` |
| `scripts/run_asb_category.py` | 简化 wrapper，按分类运行 | `results/` |
| `scripts/run_asb_compare.py` | 对比测试 wrapper | `results/` |

---

## 三、run_batch.py 完整测试命令（推荐）

这是最完整的测试脚本，支持多种攻击类型和防护策略。

### 3.1 基本命令格式

```powershell
# Windows PowerShell
python .\experiments\asb\langgraph\run_batch.py `
  --attack-family <攻击类型> `
  --run-id <运行标识> `
  --tasks-path <任务文件> `
  --attacker-tools-path <攻击工具文件> `
  --task-num <任务数> `
  --timeout <超时秒数> `
  --defense-type <防护策略>
```

**注意**：脚本会自动检测当前 Python 环境，无需手动指定 `--python` 参数。

### 3.2 攻击类型 (--attack-family)

| 参数值 | 说明 | 默认攻击方式 |
|-------|------|-------------|
| `dpi` | Direct Prompt Injection | naive, fake_completion, escape_characters |
| `opi` | Observation Prompt Injection | context_ignoring |
| `mp` | Memory Poisoning | combined_attack |
| `mixed` | Mixed DPI + OPI | combined_attack |
| `pot` | Plan-of-Thought Backdoor | naive |
| `clean` | Clean baseline | naive |

### 3.3 防护策略 (--defense-type)

| 参数值 | 说明 |
|-------|------|
| `""` (空，默认) | 无防护 |
| `aegisguard_gate` | AegisGuard 门控防护 |
| 其他自定义策略 | 根据项目配置 |

### 3.4 数据文件路径

| 用途 | 文件路径 |
|-----|---------|
| 快速测试 | `data/agent_task_langgraph_smoke.jsonl` |
| 金融场景 5 任务 | `data/agent_task_langgraph_finance_5.jsonl` |
| 攻击工具（smoke） | `data/attack_tools_langgraph_smoke.jsonl` |
| 攻击工具（finance 5） | `data/attack_tools_langgraph_finance_5.jsonl` |
| 正常工具信息 | `data/all_normal_tools.jsonl` |

### 3.5 完整示例命令

#### DPI 测试（无防护）

```powershell
# Windows PowerShell
$env:LANGGRAPH_OPENAI_BASE_URL='https://api.qnaigc.com/v1'
$env:LANGGRAPH_OPENAI_API_KEY='sk-your-api-key'
$env:LANGGRAPH_OPENAI_MODEL='deepseek/deepseek-v4-flash'

python .\experiments\asb\langgraph\run_batch.py `
  --attack-family dpi `
  --run-id test-dpi-no-defense `
  --tasks-path data/agent_task_langgraph_finance_5.jsonl `
  --attacker-tools-path data/attack_tools_langgraph_finance_5.jsonl `
  --task-num 5 `
  --timeout 600 `
  --tool-chunk-size 1
```

#### DPI 测试（AegisGuard 防护）

```powershell
# Windows PowerShell
$env:LANGGRAPH_OPENAI_BASE_URL='https://api.qnaigc.com/v1'
$env:LANGGRAPH_OPENAI_API_KEY='sk-your-api-key'
$env:LANGGRAPH_OPENAI_MODEL='deepseek/deepseek-v4-flash'

python .\experiments\asb\langgraph\run_batch.py `
  --attack-family dpi `
  --run-id test-dpi-aegisguard `
  --tasks-path data/agent_task_langgraph_finance_5.jsonl `
  --attacker-tools-path data/attack_tools_langgraph_finance_5.jsonl `
  --task-num 5 `
  --timeout 600 `
  --tool-chunk-size 1 `
  --defense-type aegisguard_gate
```

#### OPI 测试

```powershell
python .\experiments\asb\langgraph\run_batch.py `
  --attack-family opi `
  --run-id test-opi `
  --tasks-path data/agent_task_langgraph_finance_5.jsonl `
  --attacker-tools-path data/attack_tools_langgraph_finance_5.jsonl `
  --task-num 5 `
  --timeout 600 `
  --tool-chunk-size 1
```

#### MP（Memory Poisoning）测试

```powershell
python .\experiments\asb\langgraph\run_batch.py `
  --attack-family mp `
  --run-id test-mp `
  --tasks-path data/agent_task_langgraph_finance_5.jsonl `
  --attacker-tools-path data/attack_tools_langgraph_finance_5.jsonl `
  --task-num 5 `
  --timeout 600 `
  --tool-chunk-size 1
```

#### POT（Plan-of-Thought Backdoor）测试

```powershell
python .\experiments\asb\langgraph\run_batch.py `
  --attack-family pot `
  --run-id test-pot `
  --tasks-path data/agent_task_langgraph_finance_5.jsonl `
  --attacker-tools-path data/attack_tools_langgraph_finance_5.jsonl `
  --task-num 5 `
  --timeout 600 `
  --tool-chunk-size 1
```

#### Clean Baseline 测试

```powershell
python .\experiments\asb\langgraph\run_batch.py `
  --attack-family clean `
  --run-id test-clean-baseline `
  --tasks-path data/agent_task_langgraph_finance_5.jsonl `
  --attacker-tools-path data/attack_tools_langgraph_finance_5.jsonl `
  --task-num 5 `
  --timeout 600 `
  --tool-chunk-size 1
```

### 3.6 快速 Smoke 测试

```powershell
python .\experiments\asb\langgraph\run_batch.py `
  --attack-family dpi `
  --run-id smoke-test `
  --tasks-path data/agent_task_langgraph_smoke.jsonl `
  --attacker-tools-path data/attack_tools_langgraph_smoke.jsonl `
  --task-num 1 `
  --timeout 300
```

---

## 四、run_asb_category.py 简化命令

适用于快速按分类运行测试。

### 4.1 分类映射

| 分类 | 数据文件 |
|-----|---------|
| `smoke` | `ASB/data/agent_task_test.jsonl` |
| `dpi` | `ASB/data/agent_task_langgraph_smoke.jsonl`, `ASB/data/agent_task_langgraph_finance_5.jsonl` |
| `opi` | `ASB/data/agent_task_pot.jsonl`, `ASB/data/agent_task_pot_msg.jsonl` |
| `pot` | `ASB/data/agent_task_pot_all.jsonl` |
| `mp` | `ASB/data/agent_task.jsonl` |
| `langgraph` | `ASB/data/agent_task_langgraph_smoke.jsonl` |

### 4.2 示例命令

```powershell
# Windows PowerShell
.\.venv\Scripts\Activate.ps1
python .\scripts\run_asb_category.py --category dpi --venv .\.venv --repeat 1
```

```bash
# Linux/macOS
source .venv/bin/activate
python scripts/run_asb_category.py --category dpi --venv .venv --repeat 1
```

---

## 五、run_asb_compare.py 对比测试

用于比较不同 agent 和防护策略的表现。

### 5.1 示例命令

```powershell
# Windows PowerShell
python .\scripts\run_asb_compare.py `
  --category dpi `
  --agents langgraph openclaw `
  --defenses no_defense aegisguard `
  --venv .\.venv `
  --repeat 1
```

---

## 六、输出文件

### 6.1 run_batch.py 输出

输出目录：`ASB/logs/langgraph_batch/`

| 文件 | 说明 |
|-----|------|
| `<run-id>-summary.json` | 汇总 JSON |
| `<run-id>-report.md` | Markdown 报告 |
| `<run-id>-main-table.csv` | 主结果表 |
| `<run-id>-cases.csv` | 详细用例表 |
| `<run-id>-<attack_type>.csv` | 单攻击类型结果 |
| `<run-id>-<attack_type>.log` | 单攻击类型日志 |

### 6.2 wrapper 脚本输出

输出目录：`results/`

| 文件模式 | 说明 |
|---------|------|
| `asb_<category>_<index>_run<r>_<timestamp>.log` | 单次运行日志 |
| `asb_compare_<category>_<timestamp>.json` | 对比汇总报告 |

---

## 七、常见问题

### 7.1 环境变量缺失

**错误信息**：
```
[ERROR] Missing required environment variables!
...
[ERROR] Missing: LANGGRAPH_OPENAI_BASE_URL, LANGGRAPH_OPENAI_API_KEY, LANGGRAPH_OPENAI_MODEL
```

**解决**：运行前设置环境变量：

```powershell
$env:LANGGRAPH_OPENAI_BASE_URL='https://api.example.com/v1'
$env:LANGGRAPH_OPENAI_API_KEY='your-api-key'
$env:LANGGRAPH_OPENAI_MODEL='model-name'
```

### 7.2 Python 路径找不到

**错误信息**：
```
[ERROR] Python executable not found: ...
```

**解决**：使用 `--python` 参数指定路径：

```powershell
python .\experiments\asb\langgraph\run_batch.py --python ".\.venv\Scripts\python.exe" ...
```

### 7.3 依赖缺失

**解决**：

```powershell
.\.venv\Scripts\Activate.ps1
pip install -r ASB\requirements.txt
```

---

## 八、参数速查表

### run_batch.py 参数

| 参数 | 默认值 | 说明 |
|-----|-------|------|
| `--python` | 自动检测 | Python 路径（通常无需指定） |
| `--run-id` | `langgraph-batch-<timestamp>` | 运行标识 |
| `--llm-name` | `gpt-4o-mini` | LLM 名称（用于报告） |
| `--attack-family` | `dpi` | 攻击类型 |
| `--attack-types` | 根据攻击类型 | 具体攻击方式列表 |
| `--tasks-path` | `data/agent_task_langgraph_smoke.jsonl` | 任务文件 |
| `--attacker-tools-path` | `data/attack_tools_langgraph_smoke.jsonl` | 攻击工具文件 |
| `--tools-info-path` | `data/all_normal_tools.jsonl` | 正常工具信息 |
| `--task-num` | `1` | 每个任务数 |
| `--timeout` | `240` | 超时秒数 |
| `--tool-chunk-size` | `0` | 工具分块大小，`1` 表示逐个运行 |
| `--defense-type` | `""` | 防护策略 |
| `--output-dir` | `ASB/logs/langgraph_batch` | 输出目录 |

---

## 九、自动化改进说明

脚本已进行以下改进，提升易用性：

1. **自动检测 Python 路径**：优先使用当前运行的 Python，无需手动指定 `--python`
2. **环境变量检查**：启动时自动检查必要的 LLM API 环境变量，缺失时给出友好提示
3. **运行信息显示**：开始运行时显示 Python 路径、攻击类型、LLM 名称、防护策略等信息
