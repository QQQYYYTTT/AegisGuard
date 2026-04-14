# AegisGuard

AegisGuard 当前按“前端 + 后端 + 实验资料”来组织仓库，目录更直观：

- `frontend/`：前端静态界面
- `backend/`：后端代码与后端数据
- `experiments/`：实验记录与报告素材

## 目录说明

```text
AegisGuard/
|-- frontend/                    # 前端静态界面，保持不动
|-- backend/
|   |-- cmd/
|   |   `-- server/              # Go 后端启动入口
|   |-- internal/
|   |   |-- audit/               # 审计存储
|   |   |-- catalog/             # 实验元数据
|   |   |-- config/              # 配置与路径
|   |   |-- http/                # 路由与静态资源服务
|   |   |-- runtime/             # 运行时编排
|   |   `-- security/            # 授权、闸门、沙箱
|   `-- data/                    # 后端持久化数据，目前主要是 audit-store.json
|-- experiments/                 # 实验记录与报告素材
|-- go.mod
`-- README.md
```

## `backend/internal` 和 `backend/data` 的关系

现在可以这样理解：

- `backend/internal/`：放 Go 后端的源码
- `backend/data/`：放 Go 后端运行时产生的数据

也就是说：

- 代码在 `backend/cmd` 和 `backend/internal`
- 数据在 `backend/data`

这样就和 `frontend/` 对应得比较自然了。

## 当前后端职责

后端现在按作品模块组织，而不是按具体 Agent 名称组织：

- `security/`：令牌签发、令牌校验、策略闸门、记忆沙箱
- `runtime/`：把授权、拦截、审计串成完整链路
- `audit/`：审计数据读写
- `catalog/`：给前端提供实验对象、攻击类型、实验层级等展示数据
- `http/`：提供 API，并托管前端静态页面

## 实验资料怎么放

`experiments/` 目录是专门给后续写报告和答辩准备的，不属于作品主逻辑。

建议按三层实验分别记录：

- `experiments/native/`：纯 Agent 原生实验
- `experiments/guardrail/`：第三方或传统防护对照实验
- `experiments/aegisguard/`：接入你们作品后的实验

每层下面可以继续放：

- `plans/`：测试计划
- `cases/`：攻击样例和正常任务
- `results/`：表格、日志、导出结果
- `notes/`：测试备注
- `screenshots/`：截图证据

## 启动方式

先确保本机已经安装 Go，并且 `go` 命令能在 PowerShell 里直接执行。

在项目根目录运行：

```powershell
npm start
```

它实际会调用：

```powershell
go run ./backend/cmd/server
```

启动后打开：

[http://localhost:8080](http://localhost:8080)

## 说明

后面新增后端功能时，统一往这些地方放：

- `backend/cmd/`
- `backend/internal/`
- `backend/data/`
- `experiments/`
