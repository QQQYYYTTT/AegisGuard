# ASB 分类化测试说明（按攻击种类组织）

本说明面向团队测试人员，要求按攻击类别逐项执行 ASB 基准测试（不要一次性跑完所有类别）。文档包含：分类定义、数据文件映射、逐类执行脚本、示例命令与结果解析方法。

目录

- 目标
- 前提与环境准备
- 拉取 ASB 代码
- 分类与数据文件映射（示例）
- 脚本与运行示例（按分类逐项运行）
- 结果保存、解析与提交流程
- 常见问题与排查要点
- 示例记录模板

---

## 目标

按攻击类型（DPI、OPI、POT、MP、LangGraph 等）分批次、可重复地执行测试，便于对比各类攻击的拦截率、误报、处置时延与策略命中。

## 前提与环境准备

- 仓库根目录应包含 `ASB/`（若无，请拉取，见下节）。
- Python 3.8+，建议使用虚拟环境（`.venv`）。
- 安装依赖：`ASB/requirements.txt` 或 `ASB/requirements-cuda.txt`（GPU 情况）。

快速命令（Windows PowerShell）：

```powershell
python -m venv .venv
.\.venv\Scripts\Activate.ps1
pip install -r ASB\requirements.txt
```

Linux/macOS：

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r ASB/requirements.txt
```

## 拉取 ASB 代码

- 直接 clone（ASB 为独立仓库）：

```bash
git clone https://github.com/<org>/ASB.git ASB
```

- 若 ASB 在主仓库但本地缺失：

```bash
git pull origin main
git submodule update --init --recursive
```

> 小心：在删除或覆盖前请确保无未提交改动。

## 分类与数据文件映射（示例）

下面给出常用分类及推荐的 benchmark 文件（请以仓库中实际文件为准）：

- `smoke`（快速自检）: `ASB/data/agent_task_test.jsonl`
- `dpi`（数据泄露）: `ASB/data/agent_task_langgraph_smoke.jsonl`, `ASB/data/agent_task_langgraph_finance_5.jsonl`
- `langgraph`（语义链路）: `ASB/data/agent_task_langgraph_smoke.jsonl`
- `opi`（Prompt Injection）: `ASB/data/agent_task_pot.jsonl`, `ASB/data/agent_task_pot_msg.jsonl`
- `pot`（复杂 Prompt 场景）: `ASB/data/agent_task_pot_all.jsonl`
- `mp`（模型滥用）: `ASB/data/agent_task.jsonl` 或团队自定义数据

> 如需新增/调整映射，请编辑 `scripts/run_asb_category.py` 中的 `CATEGORY_MAP`。

## 脚本与运行示例（按分类逐项运行）

仓库已包含两个辅助脚本：

- `scripts/run_asb_benchmark.py`：跨平台 wrapper，安装依赖并调用 ASB 主程序，输出日志到 `results/`。
- `scripts/run_asb_category.py`：按分类运行脚本，会读取 `CATEGORY_MAP` 中的文件列表，逐个运行并保存日志。

示例：运行 `dpi` 分类的一次测试（Linux/macOS）

```bash
source .venv/bin/activate
python scripts/run_asb_category.py --category dpi --venv .venv --repeat 1
```

Windows PowerShell：

```powershell
.\.venv\Scripts\Activate.ps1
python .\scripts\run_asb_category.py --category dpi --venv .\.venv --repeat 1
```

说明：

- 每次运行只选择一个分类（例如 `dpi` 或 `opi`），避免一次性跑完所有分类。
- `--repeat` 控制每个文件的重复运行次数。
- `--extra` 可用于传递额外参数给 `run_asb_benchmark.py`（如 `--verbose`）。

运行后日志文件保存在 `results/`，文件名格式示例：

```
results/asb_<category>_<index>_run<r>_<timestamp>.log
```

例如：`results/asb_opi_1_run1_20260519T142000Z.log`

## 结果保存、解析与提交流程

- 建议将每次测试的原始日志上传至 `results/` 并在 PR/Issue 中附上摘要。
- 如果日志为 JSONL，可使用 `jq`/`awk` 等工具做聚合统计：

```bash
# 统计 Block 决策次数
cat results/asb_opi_*.log | jq -r '.decision' | grep -c Block

# 计算平均时延（假设字段为 duration_ms）
cat results/asb_opi_*.log | jq -r '.duration_ms' | awk '{sum+=$1; n++} END{print sum/n}'
```

## 常见问题与排查要点

- 依赖缺失：请激活虚拟环境并执行 `pip install -r ASB/requirements.txt`。
- 后端不可达（如 127.0.0.1:8090 被拒绝）：确认后端服务已启动并开放端口。
- 文件路径找不到：`scripts/run_asb_category.py` 会在仓库根和 `ASB/` 下尝试查找文件，必要时使用绝对路径。

## 示例记录模板（复制到 issue/PR）

- 测试人：@yourname
- 日期：YYYY-MM-DD
- 分类：POT
- 数据集文件：`ASB/data/agent_task_pot.jsonl`
- 虚拟环境：`.venv` Python 3.x
- 运行命令：`python scripts/run_asb_category.py --category pot --venv .venv --repeat 1`
- 结果摘要：拦截 X / 允许 Y / 成功率 Z% / 平均时延 N ms
- 日志文件：`results/asb_pot_1_run1_<timestamp>.log`
- 备注：如果遇到模块缺失，请记录模块名并尝试 `pip install <模块>` 后重试。

---

## 关键说明摘要（可直接复制执行的命令）

下面汇总所有常用测试场景和完整命令，包含：单分类执行、按 agent 与防护策略对比、PowerShell 示例、以及解析与可配置项。请按需复制并替换占位符。

一、只跑某一分类（逐类执行，推荐）

- 目的：对单一攻击类型（例如 `dpi`）进行稳定可重复的测试，便于观察该类攻击下的指标。
- 命令（Linux/macOS）：

```bash
source .venv/bin/activate
python scripts/run_asb_category.py --category dpi --venv .venv --repeat 1
```

- 命令（Windows PowerShell）：

```powershell
.\.venv\Scripts\Activate.ps1
python .\scripts\run_asb_category.py --category dpi --venv .\.venv --repeat 1
```

说明：
- `--category`：分类名（例如 `dpi`, `opi`, `pot`, `mp`, `langgraph`, `smoke`）。
- `--repeat`：对该分类下每个 benchmark 文件重复执行次数。

二、按 agent 与防护策略跑对比并生成汇总（多维对比）

- 目的：比较不同 agent（如 `langgraph`、`openclaw`）在不同防护配置（`no_defense`、`aegisguard`）下的表现，自动统计 ASR、RR、平均时延等。
- 命令示例（Linux/macOS）：

```bash
source .venv/bin/activate
python scripts/run_asb_compare.py \
  --category opi \
  --agents langgraph openclaw \
  --defenses no_defense aegisguard \
  --venv .venv \
  --repeat 1
```

- 命令示例（Windows PowerShell）：

```powershell
.\.venv\Scripts\Activate.ps1
python .\scripts\run_asb_compare.py --category pot --agents langgraph openclaw --defenses no_defense aegisguard --venv .\.venv --repeat 1
```

说明：
- 脚本会为每个组合（agent × defense × benchmark × run）生成日志文件并最终输出汇总 JSON，保存到 `results/`，示例名：`asb_compare_<category>_<timestamp>.json`。

三、按攻击方式（分类）枚举所有测试情况（完整列表）

对于每一个攻击分类，推荐的完整测试流程如下（要点：逐类、按 agent、按 defense、重复运行）：

- smoke（自检）：
  - 单分类运行：
    ```bash
    python scripts/run_asb_category.py --category smoke --venv .venv --repeat 1
    ```
  - 对比运行（示例）：
    ```bash
    python scripts/run_asb_compare.py --category smoke --agents langgraph openclaw --defenses no_defense aegisguard --venv .venv
    ```

- dpi（数据泄露）：
  - 单分类运行：
    ```bash
    python scripts/run_asb_category.py --category dpi --venv .venv --repeat 1
    ```
  - 对比运行（agents × defenses）：
    ```bash
    python scripts/run_asb_compare.py --category dpi --agents langgraph openclaw --defenses no_defense aegisguard --venv .venv
    ```

- langgraph（语义链路）：同上，替换 `--category langgraph`。

- opi（Prompt Injection）：
  - 单分类运行：
    ```bash
    python scripts/run_asb_category.py --category opi --venv .venv --repeat 1
    ```
  - 对比运行（agents × defenses）：
    ```bash
    python scripts/run_asb_compare.py --category opi --agents langgraph openclaw --defenses no_defense aegisguard --venv .venv
    ```

- pot（复杂 Prompt 场景）：同上，替换 `--category pot`。

- mp（模型滥用）：同上，替换 `--category mp`。

四、PowerShell 全量示例（Windows 团队可复制执行）

```powershell
# 激活虚拟环境
.\.venv\Scripts\Activate.ps1

# 单分类（DPI）
python .\scripts\run_asb_category.py --category dpi --venv .\.venv --repeat 1

# 对比（OPI，langgraph vs openclaw，no_defense vs aegisguard）
python .\scripts\run_asb_compare.py --category opi --agents langgraph openclaw --defenses no_defense aegisguard --venv .\.venv --repeat 1
```

五、输出位置与命名规范

- 所有运行日志与汇总均保存在仓库根目录下的 `results/`。
- 单运行日志命名示例（由脚本生成）：
  - `asb_<category>_<index>_run<r>_<timestamp>.log`
- 对比汇总文件：
  - `asb_compare_<category>_<timestamp>.json`

六、可配置项与解析字段（重要）

- 当日志格式或字段名与脚本默认不一致时，请使用 `run_asb_compare.py` 的参数覆盖默认字段：
  - `--success-field`：用于识别攻击成功的字段名候选（按优先级），例如：`--success-field attack_success success result`
  - `--success-value`：识别为成功的字符串值（例如 `true, success`）
  - `--remediate-field`：用于识别拦截/阻断的字段（例如：`decision remediate blocked`）
  - `--remediate-value`：识别为拦截/阻断的值（例如 `block deny blocked true`）
  - `--duration-field`：用于抽取时延的字段（例如：`duration_ms duration latency_ms`）

示例：如果 ASB 输出中标识成功的字段为 `attack_result`，拦截字段为 `decision`，时延字段为 `lat_ms`，请这样运行：

```bash
python scripts/run_asb_compare.py --category opi --agents langgraph openclaw --defenses no_defense aegisguard --venv .venv --success-field attack_result --remediate-field decision --duration-field lat_ms
```

说明：
- 脚本会按传入候选字段顺序尝试匹配并解析日志；如果没有匹配到任何字段，相关统计会为空（null）。

七、如何传参给 ASB 主程序（agent / defense）

- 默认 `run_asb_compare.py` 会把 `--agent <agent>` 和 `--defense <defense>` 作为额外参数传给 `run_asb_benchmark.py`（进而转发给 ASB 主程序）。
- 如果 ASB 主程序使用不同的参数名，请在运行时通过 `--extra` 传入正确的参数，或修改 `scripts/run_asb_compare.py` 中构建 `extra` 的逻辑。

示例（ASB 需要参数名 `--model` 与 `--policy`）：

```bash
python scripts.run_asb_compare.py --category opi --agents langgraph openclaw --defenses no_defense aegisguard --venv .venv --extra --model langgraph --extra --policy aegisguard
```

八、数据采样与统计建议

- 每个分类对每个 agent×defense 组合建议至少跑 3 次（`--repeat 3`），避免偶发性波动。
- 指标建议：
  - ASR（Attack Success Rate）= 成功攻击数 / 总样本数
  - RR（Remediation Rate）= 拦截/阻断数 / 总样本数
  - 平均时延 = duration 字段的平均值（ms)
  - 若输出为 JSONL，使用 `jq`/`python/pandas` 做更精细的聚合分析并绘图对比。

九、故障与调试要点

- 日志解析为空或字段为 null：确认 `--success-field`、`--remediate-field` 与 `--duration-field` 与实际输出字段一致。
- 运行失败（脚本返回非 0）：检查 `results/` 中对应日志文件以确定错误堆栈，并确认依赖已安装。
- 后端服务不可达：检查后端是否启动并监听正确端口（例如 127.0.0.1:8090）。

十、示例完整工作流（示范：OPI 分类对比）

1. 拉取 ASB 并准备环境：

```bash
git clone https://github.com/<org>/ASB.git ASB
python -m venv .venv
source .venv/bin/activate
pip install -r ASB/requirements.txt
```

2. 运行对比并生成汇总：

```bash
python scripts/run_asb_compare.py --category opi --agents langgraph openclaw --defenses no_defense aegisguard --venv .venv --repeat 2
```

3. 查看 `results/` 下的日志与 `asb_compare_*.json` 汇总，提取关键统计并提交 PR/Issue。

