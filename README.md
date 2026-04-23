# AIOS Relationship Agent Runtime

一个最小可用的人机亲密关系 Agent Runtime。它不是简单聊天机器人，而是一个 Go 后端 Runtime：能进行多轮互动、抽取用户信息、维护结构化关系记忆、处理记忆冲突，并基于记忆调整回复策略。

## 核心能力

- 多轮关系型对话：同一个 `user_id` 的记忆会持续保留，后续回复会使用姓名、城市、职业、偏好、情绪、事件等上下文。
- 结构化记忆：显式定义 `UserProfile`、`MemoryItem`、`SessionState`、`Tool`、`AgentRuntime`。
- 记忆冲突处理：采用最新用户陈述覆盖当前值，同时把旧值保留到 `MemoryHistory` 和 `Conflicts`。
- Agent Runtime 编排：输入校验、记忆读取、信息抽取、状态更新、持久化、回复生成、错误 fallback。
- 工具机制：支持 `ExtractionTool`、`MemoryTool`、`ReplyTool`。
- LLM 接入：支持 OpenAI-compatible Chat Completions API。
- 可解释执行轨迹：每次 `/chat` 返回 Step0 到 Step5 的 trace。
- 持久化：默认用 JSON 文件保存到 `data/memory/<user_id>.json`。

## LLM 接入

没有设置 API key 时，系统使用本地规则抽取和模板回复；设置 `OPENAI_API_KEY` 后自动启用大模型：

- `LLMExtractionTool`：用大模型抽取结构化关系记忆。
- `LLMReplyTool`：用大模型生成更自然的关系型回复。
- `FallbackExtractionTool`：LLM 抽取失败时自动退回 `RuleBasedExtractor`。
- `TemplateReplyTool`：LLM 回复失败时自动退回模板回复。

### 安全设置 API key

不要把真实 API key 写入代码、README、提交记录或 GitHub。推荐使用脚本在本机隐藏输入：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\setup-openai-key.ps1
```

也可以只在当前 PowerShell 窗口临时设置：

```powershell
$env:OPENAI_API_KEY="<OPENAI_API_KEY>"
$env:OPENAI_MODEL="gpt-4o-mini"
$env:OPENAI_BASE_URL="https://api.openai.com/v1"
```

长期保存到 Windows 用户环境变量：

```powershell
[Environment]::SetEnvironmentVariable("OPENAI_API_KEY", "<OPENAI_API_KEY>", "User")
[Environment]::SetEnvironmentVariable("OPENAI_MODEL", "gpt-4o-mini", "User")
[Environment]::SetEnvironmentVariable("OPENAI_BASE_URL", "https://api.openai.com/v1", "User")
```

保存后需要重新打开 PowerShell。

### 验证 LLM 是否生效

启动服务时看到：

```text
LLM mode: on; model=gpt-4o-mini base_url=https://api.openai.com/v1
```

对话 trace 中看到：

```text
llm_information_extractor...
llm_relationship_reply_tool...
```

说明已经连上大模型。如果 key 错误、网络失败或余额不足，Runtime 不会崩溃，会自动 fallback。

## 运行

运行测试：

```powershell
go test ./...
```

启动 HTTP 服务：

```powershell
go run ./cmd/server
```

默认页面：

```text
http://localhost:8080/
```

CLI：

```powershell
go run ./cmd/cli --user u1
```

生成部署包：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\deploy.ps1
```

部署包会生成在：

```text
dist\relationship-agent-runtime
```

## HTTP API

对话：

```powershell
$body = @{ user_id="u1"; message="我叫小王，我在上海，是后端工程师。我喜欢咖啡。" } | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/chat" -Method POST -ContentType "application/json; charset=utf-8" -Body $body
```

读取用户画像：

```powershell
Invoke-RestMethod -Uri "http://localhost:8080/profile/u1"
```

## 项目结构

```text
cmd/
  cli/                 # CLI 入口
  server/              # HTTP API + Web UI 入口
internal/
  agent/
    runtime.go         # AgentRuntime 编排逻辑
    tools.go           # 规则抽取工具、记忆工具
    llm.go             # OpenAI-compatible LLM 客户端和 LLM 工具
    types.go           # ChatRequest/Response、Tool、SessionState
    runtime_test.go    # 核心测试
  memory/
    types.go           # UserProfile、MemoryItem、关系状态等结构
    store.go           # JSONStore、记忆更新与冲突处理
scripts/
  deploy.ps1
  setup-openai-key.ps1
web/
  index.html
```

## 测试案例

- `TestBuildRelationshipAcrossThreeTurns`：正常建立关系，至少三轮对话使用前文记忆。
- `TestMemoryConflictUpdatesLatestCityAndKeepsHistory`：城市冲突更新，保留旧值历史。
- `TestExtractionFailureFallsBackAndContinues`：抽取失败 fallback，Runtime 不崩溃。
- `TestFullRelationshipMemoryAndWarmRecall`：覆盖姓名、年龄、职业、城市、偏好、情绪、事件、关系偏好和召回。

## 安全说明

本仓库不应包含真实 API key。`.gitignore` 已忽略 `.env` 和 `.env.*`，仅允许提交 `.env.example`。如果真实 key 曾经被公开，请到 OpenAI 控制台撤销旧 key 并生成新 key。
