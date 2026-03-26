# nano-claw-go

A Golang demo that implements core Claude Code features.

## What is this?

This is an **educational demo project** showing how to build a Claude Code-like AI agent in Go. It demonstrates the fundamental mechanisms behind Claude Code:

- Tool calling (LLM decides to call tools and uses the results)
- Skills system (domain-specific knowledge)
- Persistent memory (conversation history)
- Sub-agents (spawn isolated agents for complex tasks)
- Multi-round conversation loop

## How It Works

### Core Flow

```
User Input → Agent.Loop() → LLM API → [Tool Calls?]
                                              ↓
                                        Execute Tools
                                              ↓
                                        Continue Loop
                                              ↓
                                    Return Final Answer
```

The key insight of Claude Code is the **tool calling loop**:
1. Send messages to LLM
2. If LLM returns tool calls, execute them and add results to messages
3. Loop back to step 1 (until no more tool calls)

### Architecture

```
main.go
    └── cmd/root.go            # CLI entry (Cobra)
            └── chat.Main()    # User input loop
                    ├── agent.Loop()      # Core agent loop
                    │       ├── LLM API call
                    │       ├── Tool execution (parallel)
                    │       └── Continue if tool calls
                    ├── memory/           # Chat history persistence
                    └── skills/            # Knowledge base
```

### Key Components

| Component | File | Purpose |
|-----------|------|---------|
| Chat Loop | `pkg/chat/chat.go` | Reads user input, prints AI responses |
| Agent | `pkg/agent/agent.go` | Core Loop: call LLM → execute tools → repeat |
| Tools | `pkg/agent/tools.go` | calculator, reader, writer, executor, task, load_skill |
| Memory | `pkg/memory/memory.go` | Persists chat history to disk (atomic write) |
| Skills | `pkg/skill/skill.go` | Loads knowledge from `./skills/` directory |
| Subagent | `pkg/agent/subagent.go` | Spawns isolated agent with fresh context |

### Implementation Details

#### 1. Tool Calling (The Core Mechanism)

The agent loop in `agent.go` is the heart of the system:

```go
func (a *agent) Loop(ctx context.Context, maxRounds int) error {
    for range maxRounds {
        // 1. Call LLM with current messages
        resp, err := a.openaiClient.CreateChatCompletion(req)

        // 2. If no tool calls, we're done
        if resp.Choices[0].FinishReason != openai.FinishReasonToolCalls {
            return nil
        }

        // 3. Execute tool calls in parallel
        for _, toolCall := range msg.ToolCalls {
            result := a.tools.Dispatch(toolCall)
            // Add result as a new message
            a.memory.Append(toolResult)
        }

        // 4. Continue loop with updated messages
    }
}
```

#### 2. Persistent Memory

Memory auto-saves every 10 seconds using atomic writes (temp file + rename):

```go
func (m *memory) Run(ctx context.Context) {
    ticker := time.NewTicker(time.Second * 10)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.saveHistory()  // Atomic write
        }
    }
}
```

#### 3. Skills System

Skills are loaded from `./skills/` directory. Each skill is a directory with `SKILL.md`:

```markdown
---
name: weather
description: Get weather info. Use when user asks about weather.
---

# Weather Skill

## When to use
User asks: "What's the weather?"

## How to use
curl wttr.in/{city}
```

The `load_skill` tool lets the LLM fetch skill content when needed.

#### 4. Sub-agents

For complex tasks, the `task` tool spawns a completely isolated agent:

```go
func (s *subAgent) Run(ctx context.Context) string {
    // Fresh memory (no conversation history)
    memory := mem.NewMemory(...)
    // New agent with its own context
    sub := NewAgent(..., memory)
    sub.Loop(ctx, defaultMaxRounds)
    return memory.LastHistory().Content
}
```

## Usage

```bash
# Set environment variables
export OPENAI_AUTH_TOKEN="your-api-key"
export OPENAI_MODEL="gpt-4"

# Run
go run main.go

# Example interaction:
# You: What is 123 * 456?
# AI: (calls calculator tool)
# >> tool [calculator] call expression: 123 * 456
# >> tool [calculator] call result: 56088
# AI: The result is 56,088.
```

## Available Tools

- **calculator**: Math expressions (uses govaluate)
- **reader**: Read file contents
- **writer**: Write to files
- **executor**: Run shell commands (blocks dangerous commands)
- **load_skill**: Load skill knowledge
- **task**: Spawn sub-agent

## Extending

### Add a New Tool

Implement the `Tool` interface in `pkg/agent/tools.go`:

```go
type Tool interface {
    Name() string
    Tool() openai.Tool          // JSON Schema for LLM
    Prompt() string             // When to use this tool
    Call(ctx context.Context, parameters map[string]any) string
}
```

### Add a New Skill

1. Create `skills/my-skill/SKILL.md`
2. Add YAML frontmatter with name/description
3. Write usage instructions in Markdown

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_AUTH_TOKEN` | API key |
| `OPENAI_MODEL` | Model name (e.g., gpt-4) |
| `OPENAI_BASE_URL` | API base URL (optional) |

## License

MIT