# 代码审计报告（含 FP Check 验证）

## 总览

审计范围：`internal/vkey/`、`internal/config/`、`internal/gateway/`、`internal/http/`、`internal/gates/`、`internal/auth/`、`internal/audit/`、`pkg/smcrypto/`

FP Check 验证结果：**10 个 TRUE POSITIVE，1 个 FALSE POSITIVE**

---

## Finding #1: `CreateStore` DSN 解析算法存在严重逻辑错误

- **Verdict: TRUE POSITIVE**
- **Primitive:** C (Logic / Assertion)
- **Confidence:** **Practical**
- **Path Evidence:**
  - [store.go:219-228](file:///d:/Projects/AegisGuard/backend/internal/vkey/store.go#L219-L228) `CreateStore` 函数
- **问题描述：**

DSN 解析逻辑试图从后向前扫描 `:` 分隔符来拆分 `scheme:path`，但算法有致命缺陷：

```go
func CreateStore(dsn string) (Store, error) {
    scheme := "sqlite"
    path := dsn
    for schemeSuffix := ""; len(dsn) > 0; dsn = dsn[:len(dsn)-1] {
        if dsn[len(dsn)-1] == ':' {
            scheme = dsn[:len(dsn)-1]
            path = schemeSuffix
            break
        }
        schemeSuffix = string(dsn[len(dsn)-1]) + schemeSuffix
    }
    ...
}
```

**当输入 `sqlite:data/vkeys.db` 时：**
- 循环从末尾逐个剥离字符，`schemeSuffix` 累积为 `data/vkeys.db`
- 遇到 `:` 时，`scheme = "sqlite"` ✅，`path = "data/vkeys.db"` ✅
- 看起来正确，但 **如果输入不含 `:`**（如 `data/vkeys.db`），循环会把整个字符串拆碎，`scheme` 保持默认 `"sqlite"`，`path` 变成空字符串 `""`。

**当输入 Windows 路径 `sqlite:D:\data\vkeys.db` 时：**
- 从后往前扫描，第一个 `:` 出现在盘符 `D:` 处
- `scheme = "sqlite:D"` ❌，查不到该 factory → 报错

- **实际影响评估：** [manager.go:68](file:///d:/Projects/AegisGuard/backend/internal/vkey/manager.go#L68) 硬编码调用 `CreateStore("sqlite:data/vkeys.db")`，使用相对路径，**当前不会触发**。但如果用户通过其他方式传入带盘符的 DSN，就会失败。
- **Fix:** 使用 `strings.SplitN(dsn, ":", 2)` 替代手写循环：
```go
func CreateStore(dsn string) (Store, error) {
    scheme := "sqlite"
    path := dsn
    if parts := strings.SplitN(dsn, ":", 2); len(parts) == 2 && parts[0] != "" {
        scheme, path = parts[0], parts[1]
    }
    if f, ok := storeFactories[scheme]; ok {
        return f(path)
    }
    return nil, fmt.Errorf("unsupported store scheme: %s", scheme)
}
```

---

## Finding #2: `storeWithCtx` 设计导致 context 立即失效

- **Verdict: TRUE POSITIVE (严重)**
- **Primitive:** B (State / Lifecycle)
- **Confidence:** **Practical**
- **Path Evidence:**
  - [store.go:206-214](file:///d:/Projects/AegisGuard/backend/internal/vkey/store.go#L206-L214) sqlite 工厂函数
  - [store.go:232-245](file:///d:/Projects/AegisGuard/backend/internal/vkey/store.go#L232-L245) `storeWithCtx` 结构体
- **问题描述：**

```go
"sqlite": func(path string) (Store, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()  // ← 函数返回时 cancel 被调用，ctx 立即失效
    store, err := NewSQLiteStore(path)
    if err != nil {
        return nil, err
    }
    return &storeWithCtx{store, ctx}, nil  // ← 返回时 ctx 已被取消
}
```

`defer cancel()` 在匿名工厂函数返回时执行，context 立即被取消。之后所有通过 `storeWithCtx` 的数据库操作都会使用已取消的 context。

```go
func (c *storeWithCtx) Save(ctx context.Context, key *VirtualKey) error {
    return c.s.Save(c.ctx, key)  // ← 使用已取消的 c.ctx
}
```

**实际影响：所有 SQLite 持久化操作（Save/Get/Delete/List/Update）100% 返回 `context canceled` 错误，持久化完全失效。**

- **Fix:** 删除 `storeWithCtx` 包装层，让 `SQLiteStore` 直接实现 `Store` 接口（它已经实现了）。工厂函数直接返回 `*SQLiteStore`：
```go
"sqlite": func(path string) (Store, error) {
    return NewSQLiteStore(path)
},
```

---

## Finding #3: `usedNonces` 全局 map 存在竞态条件

- **Verdict: TRUE POSITIVE**
- **Primitive:** B (State / Lifecycle) — Race Condition
- **Confidence:** **Practical**
- **Path Evidence:**
  - [verifier.go:18](file:///d:/Projects/AegisGuard/backend/internal/auth/verifier.go#L18) `var usedNonces = make(map[string]bool)`
  - [verifier.go:128-135](file:///d:/Projects/AegisGuard/backend/internal/auth/verifier.go#L128-L135) `verifyNonce` 读写无锁
- **问题描述：**

```go
var usedNonces = make(map[string]bool)  // 全局变量，无锁保护

func (v *Verifier) verifyNonce(token *RequireToken) error {
    if usedNonces[token.Nonce] {    // 读
        return fmt.Errorf("nonce already used")
    }
    usedNonces[token.Nonce] = true  // 写
    return nil
}
```

Gin 默认并发处理 HTTP 请求。两个携带相同 nonce 的请求同时到达时：
1. Go race detector 会直接 panic（`fatal error: concurrent map writes`）
2. 即使不 panic，TOCTOU 窗口允许两个请求都通过验证

**重放攻击路径：** 攻击者截获一个合法请求后，在极短时间内并发重放两次，两个都可能通过 nonce 检查。

- **Fix:** 使用 `sync.Mutex` 保护：
```go
var (
    usedNonces   = make(map[string]bool)
    nonceMu      sync.Mutex
)

func (v *Verifier) verifyNonce(token *RequireToken) error {
    nonceMu.Lock()
    defer nonceMu.Unlock()
    if usedNonces[token.Nonce] {
        return fmt.Errorf("nonce already used: %s", token.Nonce)
    }
    usedNonces[token.Nonce] = true
    return nil
}
```

---

## Finding #4: `loadFromConfig` 中 Viper 全局状态污染

- **Verdict: TRUE POSITIVE (低影响)**
- **Primitive:** D (Environment / Boundary)
- **Confidence:** **Probable**
- **Path Evidence:**
  - [manager.go:120-122](file:///d:/Projects/AegisGuard/backend/internal/vkey/manager.go#L120-L122)
- **问题描述：**

```go
func (m *Manager) loadFromConfig() error {
    viper.SetConfigFile(m.configPath)  // ❌ 修改全局 Viper 状态
    viper.SetConfigType("yaml")
    if err := viper.ReadInConfig(); err != nil {
        ...
    }
```

`viper` 是全局单例。如果系统中存在多个 Manager 实例或多次调用 `loadFromConfig`，`SetConfigFile` 会互相覆盖。

**实际影响：** 当前只有一个 Manager 实例，只在启动时调用一次，**实际不会触发**。但如果未来有多个 Manager 实例或热重载配置，就会有问题。

- **Fix:** 使用 `viper.New()` 创建独立实例：
```go
func (m *Manager) loadFromConfig() error {
    v := viper.New()
    v.SetConfigFile(m.configPath)
    v.SetConfigType("yaml")
    ...
}
```

---

## Finding #5: `proxy.go` 中 `Content-Length` 设置错误

- **Verdict: TRUE POSITIVE (严重)**
- **Primitive:** C (Logic / Assertion)
- **Confidence:** **Practical**
- **Path Evidence:**
  - [proxy.go:143](file:///d:/Projects/AegisGuard/backend/internal/gateway/proxy.go#L143)
- **问题描述：**

```go
resp.Header.Set("Content-Length", string(rune(len(cleaned))))
```

`string(rune(len(cleaned)))` 将整数长度值转换为 Unicode 码点对应的字符，而非数字字符串。例如 `len=100` 会变成 `"d"`（Unicode U+0064），而不是 `"100"`。

**实际影响：** HTTP 客户端收到错误的 Content-Length 后，会错误地截断或等待读取响应体，**导致响应解析失败**。

- **Fix:** 使用 `strconv.Itoa`：
```go
resp.Header.Set("Content-Length", strconv.Itoa(len(cleaned)))
```

---

## Finding #6: `director` 中 `io.ReadAll` 后未检查错误

- **Verdict: TRUE POSITIVE**
- **Primitive:** A (Taint / Flow)
- **Confidence:** **Theoretical**
- **Path Evidence:**
  - [proxy.go:112](file:///d:/Projects/AegisGuard/backend/internal/gateway/proxy.go#L112)
- **问题描述：**

```go
body, _ := io.ReadAll(req.Body)  // ❌ 忽略 error
req.Body = io.NopCloser(bytes.NewBuffer(body))
```

如果读取请求体时发生错误（如连接中断），`body` 可能不完整或为空，但代码继续执行后续 Gate 判断。Gate 拿到的数据不完整，可能导致安全判定失误。

- **Fix:** 检查 error 并记录日志：
```go
body, err := io.ReadAll(req.Body)
if err != nil {
    ap.logger.Error("读取请求体失败", zap.Error(err))
    return
}
req.Body = io.NopCloser(bytes.NewBuffer(body))
```

---

## Finding #7: `handleProxy` 中 `GetRawData()` 后 body 恢复不完整

- **Verdict: TRUE POSITIVE**
- **Primitive:** C (Logic / Assertion)
- **Confidence:** **Probable**
- **Path Evidence:**
  - [router.go:113-116](file:///d:/Projects/AegisGuard/backend/internal/http/router.go#L113-L116)
- **问题描述：**

```go
bodyBytes, _ := c.GetRawData()
if len(bodyBytes) > 0 {
    c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}
```

`GetRawData()` 会消费 `c.Request.Body`。如果 `bodyBytes` 长度为 0（如 GET 请求），`c.Request.Body` 不会被恢复，后续 `proxy.ServeHTTP` 读取 body 时得到空内容。

**实际影响：** LLM API 请求基本都是 POST with body，所以实际触发概率低，但逻辑上确实有 bug。

- **Fix:** 无论 body 是否为空都恢复：
```go
bodyBytes, _ := c.GetRawData()
c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
```

---

## Finding #8: `InMemoryStore` 无并发保护

- **Verdict: TRUE POSITIVE (实际不可达)**
- **Primitive:** B (State / Lifecycle) — Race Condition
- **Confidence:** **Practical**
- **Path Evidence:**
  - [store.go:149-185](file:///d:/Projects/AegisGuard/backend/internal/vkey/store.go#L149-L185)
- **问题描述：**

```go
type InMemoryStore struct {
    keys map[string]*VirtualKey  // 无锁保护
}

func (s *InMemoryStore) Save(ctx context.Context, key *VirtualKey) error {
    s.keys[key.KeyID] = key  // ❌ 并发写
    return nil
}
```

map 读写无锁，并发场景下会 `fatal error: concurrent map writes`。

**实际影响评估：** [manager.go:68-72](file:///d:/Projects/AegisGuard/backend/internal/vkey/manager.go#L68-L72) 中，InMemoryStore 只在 SQLite 初始化失败时作为降级使用。而 SQLite 初始化失败后，Manager 的 `store` 字段为 `nil`（看 `NewManager` 的逻辑），**InMemoryStore 实际上从未被使用**。

- **Fix:** 添加 `sync.RWMutex`：
```go
type InMemoryStore struct {
    mu   sync.RWMutex
    keys map[string]*VirtualKey
}
```

---

## Finding #9: `selectRepresentativeEvents` 可能产生重复事件

- **Verdict: TRUE POSITIVE**
- **Primitive:** C (Logic / Assertion)
- **Confidence:** **Theoretical**
- **Path Evidence:**
  - [batch_window.go:175-195](file:///d:/Projects/AegisGuard/backend/internal/gates/batch_window.go#L175-L195)
- **问题描述：**

```go
func selectRepresentativeEvents(events []WindowEvent, maxCount int) []WindowEvent {
    selected := make([]WindowEvent, 0, maxCount)
    
    // 先选高风险事件
    for _, event := range events {
        if event.RiskLevel >= 70 && len(selected) < maxCount {
            selected = append(selected, event)
        }
    }
    
    // 再选最近事件
    if len(selected) < maxCount {
        for i := len(events) - 1; i >= 0 && len(selected) < maxCount; i-- {
            selected = append(selected, events[i])  // ❌ 可能重复添加已选的高风险事件
        }
    }
    return selected
}
```

第二轮"选最近事件"时，没有跳过第一轮已选中的高风险事件，导致同一事件可能被添加两次。

**实际影响：** 日志噪音，不影响安全判定。

- **Fix:** 使用 map 去重或添加索引检查：
```go
selected := make([]WindowEvent, 0, maxCount)
selectedIdx := make(map[int]bool)

for i, event := range events {
    if event.RiskLevel >= 70 && len(selected) < maxCount {
        selected = append(selected, event)
        selectedIdx[i] = true
    }
}

if len(selected) < maxCount {
    for i := len(events) - 1; i >= 0 && len(selected) < maxCount; i-- {
        if !selectedIdx[i] {
            selected = append(selected, events[i])
        }
    }
}
```

---

## Finding #10: `auth/token.go` 全局签名密钥非线程安全

- **Verdict: TRUE POSITIVE (理论)**
- **Primitive:** B (State / Lifecycle)
- **Confidence:** **Probable**
- **Path Evidence:**
  - [token.go:17-19](file:///d:/Projects/AegisGuard/backend/internal/auth/token.go#L17-L19)
- **问题描述：**

```go
var (
    signingPrivateKey *sm2.PrivateKey  // 全局变量，无锁保护
    signingUID        = []byte("AegisGuard-Agent-Auth")
)
```

`InitSigningKey` 和 `GetSigningPublicKey` 对 `signingPrivateKey` 的读写没有同步保护。

**实际影响评估：** `InitSigningKey` 只在启动时调用一次，之后只读不写，**当前架构下不会触发**。如果未来实现密钥轮换功能，就会有问题。

- **Fix:** 使用 `sync/atomic` 或 `sync.RWMutex` 保护：
```go
var (
    signingPrivateKey   atomic.Pointer[sm2.PrivateKey]
    signingUID          = []byte("AegisGuard-Agent-Auth")
)
```

---

## Finding #11: `handleProxy` 使用 `context.WithValue` 传 string key

- **Verdict: FALSE POSITIVE**
- **Primitive:** D (Environment / Boundary)
- **Confidence:** **Theoretical**
- **Path Evidence:**
  - [router.go:163](file:///d:/Projects/AegisGuard/backend/internal/http/router.go#L163)
- **问题描述：**

```go
ctx := context.WithValue(c.Request.Context(), "vkey_info", vk)
```

使用字符串作为 context key 是代码风格问题，不是安全漏洞。Go 推荐自定义类型 key 是为了避免命名冲突，但在这个项目中不存在冲突风险。

**结论：属于最佳实践建议，不是 bug。**

---

## 代码质量评估

| 维度 | 评分 | 说明 |
|:---|:---|:---|
| **可读性** | ⭐⭐⭐⭐ | 命名清晰，注释充分，模块职责分明 |
| **时间复杂度** | ⭐⭐⭐⭐ | 大部分操作 O(1) 或 O(n)，滑动窗口设计合理 |
| **空间复杂度** | ⭐⭐⭐ | `usedNonces` 和 `InMemoryStore` 无限增长，无清理机制 |
| **架构设计** | ⭐⭐⭐⭐ | Store 接口抽象良好，Manager 双存储回退设计合理 |

---

## FP Check 验证汇总

| # | 发现 | Verdict | 实际影响 |
|:---|:---|:---|:---|
| 1 | DSN 解析错误 | **TRUE POSITIVE** | 算法有缺陷，但硬编码相对路径使当前不受影响 |
| 2 | storeWithCtx context 已取消 | **TRUE POSITIVE** | 所有 SQLite 操作 100% 失败 |
| 3 | Nonce 竞态条件 | **TRUE POSITIVE** | 并发 map 读写会 panic，TOCTOU 可被利用 |
| 4 | Viper 全局状态 | **TRUE POSITIVE** | 架构隐患，当前单实例不受影响 |
| 5 | Content-Length 错误 | **TRUE POSITIVE** | `string(rune())` 转换错误，响应损坏 |
| 6 | io.ReadAll 忽略 error | **TRUE POSITIVE** | 异常情况下 Gate 拿到不完整数据 |
| 7 | GetRawData body 恢复 | **TRUE POSITIVE** | 空 body 时不恢复，但 LLM 请求都有 body |
| 8 | InMemoryStore 并发 | **TRUE POSITIVE** | 代码确实不安全，但实际路径中未被使用 |
| 9 | selectRepresentativeEvents 重复 | **TRUE POSITIVE** | 事件可能重复添加，仅日志噪音 |
| 10 | 全局密钥线程安全 | **TRUE POSITIVE** | 启动后只读不写，当前不受影响 |
| 11 | context string key | **FALSE POSITIVE** | 代码风格问题，不是安全漏洞 |

**统计：10 个 TRUE POSITIVE，1 个 FALSE POSITIVE**

---

## 修复优先级

| 优先级 | Finding | 影响 |
|:---|:---|:---|
| 🔴 P0 | #2 storeWithCtx context 已取消 | 所有持久化操作立即失败 |
| � P0 | #5 Content-Length 错误 | 响应头损坏，客户端解析失败 |
| 🟠 P1 | #3 Nonce 竞态条件 | 重放攻击可成功，并发直接 panic |
| 🟡 P2 | #1 DSN 解析错误 | Windows 绝对路径下 SQLite 无法工作 |
| 🟡 P2 | #6 io.ReadAll 忽略 error | 异常情况下行为不可预测 |
| � P2 | #7 GetRawData body 恢复 | 空 body 时下游拿到空内容 |
| �🟢 P3 | #4 Viper 全局状态 | 架构隐患，当前无影响 |
| 🟢 P3 | #8 InMemoryStore 并发 | 代码不安全但实际不可达 |
| 🟢 P3 | #9 selectRepresentativeEvents 重复 | 日志噪音，不影响安全 |
| 🟢 P3 | #10 全局密钥非线程安全 | 当前只在启动时设置一次 |
| ❌ 排除 | #11 context string key | 代码风格问题，非安全漏洞 |
