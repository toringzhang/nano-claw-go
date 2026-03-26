# nano-claw-go

使用 Golang 实现的 Claude Code 核心功能教学 demo。

## 这是什么？

这是一个**教学演示项目**，展示如何用 Go 构建类似 Claude Code 的 AI Agent。主要演示了 Claude Code 的核心机制：

- 工具调用（LLM 自行决定调用工具并使用结果）
- 技能系统（领域知识）
- 持久化记忆（对话历史）
- 子代理（为复杂任务创建隔离的 Agent）
- 多轮对话循环

## 实现原理

### 核心流程

```
用户输入 → Agent.Loop() → LLM API → [有工具调用?]
                                         ↓
                                   执行工具
                                         ↓
                                   继续循环
                                         ↓
                                   返回最终答案
```

Claude Code 的核心是**工具调用循环**：
1. 发送消息给 LLM
2. 如果 LLM 返回工具调用，执行并把结果加到消息中
3. 回到步骤 1（直到没有工具调用为止）

### 项目架构

```
main.go
    └── cmd/root.go            # CLI 入口 (Cobra)
            └── chat.Main()    # 用户输入循环
                    ├── agent.Loop()      # 核心循环
                    │       ├── 调用 LLM
                    │       ├── 执行工具（并行）
                    │       └── 有调用则继续
                    ├── memory/           # 聊天历史持久化
                    └── skills/           # 知识库
```

### 核心组件

| 组件 | 文件 | 作用 |
|------|------|------|
| 聊天循环 | `pkg/chat/chat.go` | 读取用户输入，打印 AI 回复 |
| Agent | `pkg/agent/agent.go` | 核心循环：调用LLM → 执行工具 → 循环 |
| 工具集 | `pkg/agent/tools.go` | 计算器、读写文件、执行命令、任务、技能 |
| 记忆 | `pkg/memory/memory.go` | 聊天历史持久化（原子写入） |
| 技能 | `pkg/skill/skill.go` | 从 `./skills/` 加载知识 |
| 子代理 | `pkg/agent/subagent.go` | 创建独立上下文的 Agent |

### 实现细节

#### 1. 工具调用（核心机制）

agent.go 中的循环是整个系统的核心：

```go
func (a *agent) Loop(ctx context.Context, maxRounds int) error {
    for range maxRounds {
        // 1. 调用 LLM
        resp, err := a.openaiClient.CreateChatCompletion(req)

        // 2. 没有工具调用，直接返回
        if resp.Choices[0].FinishReason != openai.FinishReasonToolCalls {
            return nil
        }

        // 3. 并行执行工具调用
        for _, toolCall := range msg.ToolCalls {
            result := a.tools.Dispatch(toolCall)
            // 把结果作为新消息加入
            a.memory.Append(toolResult)
        }

        // 4. 继续循环
    }
}
```

#### 2. 持久化记忆

每10秒自动保存，使用原子写入（临时文件+重命名）：

```go
func (m *memory) Run(ctx context.Context) {
    ticker := time.NewTicker(time.Second * 10)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.saveHistory()  // 原子写入
        }
    }
}
```

#### 3. 技能系统

技能从 `./skills/` 目录加载，每个技能是一个包含 `SKILL.md` 的目录：

```markdown
---
name: weather
description: 获取天气信息。用户问天气时使用。
---

# 天气技能

## 何时使用
用户问："今天天气怎么样？"

## 如何使用
curl wttr.in/{城市}
```

`load_skill` 工具让 LLM 可以在需要时加载技能内容。

#### 4. 子代理

对于复杂任务，`task` 工具会创建一个完全隔离的 Agent：

```go
func (s *subAgent) Run(ctx context.Context) string {
    // 全新内存（没有对话历史）
    memory := mem.NewMemory(...)
    // 使用独立上下文的新 Agent
    sub := NewAgent(..., memory)
    sub.Loop(ctx, defaultMaxRounds)
    return memory.LastHistory().Content
}
```

## 使用方法

```bash
# 设置环境变量
export OPENAI_AUTH_TOKEN="your-api-key"
export OPENAI_MODEL="gpt-4"

# 运行
go run main.go

# 示例交互：
# You: 123 * 456 等于多少？
# AI: (调用计算器工具)
# >> tool [calculator] call expression: 123 * 456
# >> tool [calculator] call result: 56088
# AI: 结果是 56,088。
```

## 可用工具

- **calculator**: 数学表达式计算（使用 govaluate）
- **reader**: 读取文件内容
- **writer**: 写入文件
- **executor**: 执行 Shell 命令（会拦截危险命令）
- **load_skill**: 加载技能知识
- **task**: 创建子代理

## 扩展

### 添加新工具

在 `pkg/agent/tools.go` 中实现 `Tool` 接口：

```go
type Tool interface {
    Name() string
    Tool() openai.Tool          // JSON Schema 定义
    Prompt() string            // 何时使用
    Call(ctx context.Context, parameters map[string]any) string
}
```

### 添加新技能

1. 创建 `skills/my-skill/SKILL.md`
2. 添加 YAML 头信息（name/description）
3. 用 Markdown 写使用说明

## 环境变量

| 变量 | 描述 |
|------|------|
| `OPENAI_AUTH_TOKEN` | API 密钥 |
| `OPENAI_MODEL` | 模型名称（如 gpt-4） |
| `OPENAI_BASE_URL` | API 基础 URL（可选） |

## 许可证

MIT