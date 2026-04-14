# AegisGuard——开发中

面向 Agent 原生安全机制测试与后续运行时安全接入的基础框架。

## 当前目标

当前优先服务于第一层实验：

- 对主流 Agent 的原生安全机制做结构化测试
- 统一整理实验对象、攻击家族、实验层级与记录方式
- 为后续接入 AegisGuard 授权链路、策略闸门、记忆沙箱和审计闭环预留清晰接口

## 目录结构

```text
AegisGuard/
├─ frontend/                 # 独立前端静态应用
│  ├─ index.html
│  ├─ app.js
│  └─ styles.css
├─ backend/
│  ├─ data/                  # 审计持久化数据
│  └─ src/
│     ├─ adapters/agents/    # 各类 Agent 适配入口预留
│     ├─ app/                # 服务启动装配
│     ├─ config/             # 路径、端口等配置
│     ├─ data/               # 实验对象、攻击家族、场景模板
│     ├─ lib/                # http / crypto 通用能力
│     ├─ routes/             # API 路由
│     └─ services/           # 授权、闸门、沙箱、审计、实验服务
├─ package.json
└─ server.js                 # 根入口，转发到 backend/src/server.js
```

## 当前已经搭好的能力

### 1. 实验框架元数据

后端已经内置：

- 主流 Agent 测试对象列表
- 五类攻击家族
- 三层实验对照结构
- 第一层实验的总体规划摘要

### 2. 基础运行时模拟链路

已经实现：

- `RequireToken` 签发
- 请求校验
- 闸门决策
- 记忆沙箱过滤
- 审计日志持久化

### 3. 前后端分离

- 前端位于 `frontend/`
- 后端位于 `backend/`
- 前端只通过 `/api/*` 访问后端

## 后续建议扩展

建议后续优先补这几块：

1. 在 `backend/src/adapters/agents/` 下为 `OpenHands`、`DB-GPT`、`OpenClaw`、`LangChain` 分别建立适配目录。
2. 为每个 Agent 增加统一接口：
   - `getNativeSecurityProfile()`
   - `runBaselineTask()`
   - `runAttackCase()`
   - `collectAuditEvidence()`
3. 增加实验任务编排器，把“攻击家族 × 变体 × 重复次数”真正调度起来。
4. 增加实验结果导出能力，便于后续论文和答辩制图。

## 启动

```powershell
npm start
```

启动后访问：

[http://localhost:8080](http://localhost:8080)

## 检查

```powershell
npm run check
```
