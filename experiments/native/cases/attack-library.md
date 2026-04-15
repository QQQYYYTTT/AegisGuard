# 原生层统一攻击库

这份攻击库用于四位同学在不同类型 Agent 上做统一测试，避免后续结果无法横向比较。

## 攻击来源

本项目没有直接嵌入外部框架运行器，而是把外部项目中的风险测试思路抽取、改写成统一的攻击 case。

- TrinityGuard: 提供 prompt injection、memory poisoning、tool misuse、excessive agency 等风险测试分类
  - 来源: https://github.com/AI45Lab/TrinityGuard
- Agentgateway: 提供 agent / MCP 网关中的认证、授权、工具访问与策略治理场景
  - 来源: https://github.com/agentgateway/agentgateway

## 攻击家族

按照 `实验设计-0.md`，统一采用 5 类攻击：

1. 提示词注入
2. 工具 / 输出污染
3. 记忆投毒
4. 越权 / 过度自治
5. 工具或供应链投毒

每类 3 个变体，共 15 个统一 case。

## 统一 case 编号

### 1. 提示词注入

- `pi-1`: 直接指令覆盖
- `pi-2`: 间接网页注入
- `pi-3`: 工具回执诱导

### 2. 工具 / 输出污染

- `top-1`: 恶意网页结果
- `top-2`: 日志污染片段
- `top-3`: 工具回执嵌入指令

### 3. 记忆投毒

- `mp-1`: 共享记忆植入
- `mp-2`: 检索片段污染
- `mp-3`: 跨轮次持续攻击

### 4. 越权 / 过度自治

- `op-1`: 高危 shell
- `op-2`: 超范围 SQL
- `op-3`: 敏感文件写入

### 5. 工具或供应链投毒

- `tsc-1`: tool schema 篡改
- `tsc-2`: MCP 元数据污染
- `tsc-3`: docstring 隐藏指令

## 使用建议

- 四个人测试前先统一 case，不要各自临时造攻击输入
- 每个人至少执行和自己 Agent 类型最相关的 case
- 如果某个 case 明显不适用于该 Agent，要记录“不适用”而不是跳过不记
- 每个 case 至少记录：是否成功、是否被拦截、拦截阶段、任务是否完成、截图路径、备注

## 系统接口

当前系统已内置只读接口：

```text
GET /api/attack-library
```

返回内容包括：

- 攻击家族
- 15 个统一攻击 case
- 每个 case 的适用 Agent 类型
- 预期攻击信号
- 成功判定标准
- 示例攻击载荷
