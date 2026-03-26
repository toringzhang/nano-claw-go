package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
	mem "github.com/toringzhang/nano-claw-go/pkg/memory"
)

// Subagent runs a task with fresh context (no conversation history with main agent)
type Subagent interface {
	Run(ctx context.Context) string
}

type subAgent struct {
	Name        string
	Description string
	Module      string
	Prompt      string
	openaiCli   *openai.Client
	tools       []Tool
}

func NewSubagent(name string, description string, openaiCli *openai.Client, module string, prompt string, tools []Tool) Subagent {
	return &subAgent{
		Name:        name,
		Description: description,
		Module:      module,
		Prompt:      prompt,
		openaiCli:   openaiCli,
		tools:       tools,
	}
}

// Run executes subagent with its own isolated memory
func (s *subAgent) Run(ctx context.Context) string {
	channelId := ctx.Value("channelId")
	memory := mem.NewMemory(fmt.Sprintf("%s-%s-%s", channelId, s.Name, time.Now().Format(time.RFC3339)))
	go memory.Run(ctx)

	sub := NewAgent(s.openaiCli, s.Module, s.Prompt, s.tools, memory)
	err := sub.Loop(ctx, defaultMaxRounds)
	if err != nil {
		return fmt.Sprintf("subagent %s run error: %v", s.Name, err)
	}

	last := memory.LastHistory()
	return last.Content
}
