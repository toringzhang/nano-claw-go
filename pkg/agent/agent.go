package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	mem "github.com/toringzhang/nano-claw-go/pkg/memory"
)

const (
	defaultMaxRounds = 10
)

type Agent interface {
	Loop(ctx context.Context, maxRounds int) error
}

type agent struct {
	openaiClient *openai.Client
	module       string // llm module
	system       string // system prompt
	tools        Tools
	memory       mem.Memory
}

func NewAgent(openaiCli *openai.Client, module string, system string, tools []Tool, memory mem.Memory) Agent {
	return &agent{
		openaiClient: openaiCli,
		module:       module,
		system:       system,
		tools:        tools,
		memory:       memory,
	}
}

func (a *agent) Loop(ctx context.Context, maxRounds int) error {
	if maxRounds <= 0 {
		maxRounds = defaultMaxRounds
	}
	toolsPrompt := ""
	for _, tool := range a.tools {
		toolsPrompt += tool.Prompt()
	}
	prompt := a.system + toolsPrompt
	for range maxRounds {
		if a.memory.Length() <= 0 {
			return fmt.Errorf("memory is empty")
		}

		req := openai.ChatCompletionRequest{
			Model:               a.module,
			Messages:            append([]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: prompt}}, a.memory.HistoryMessages()...),
			Stream:              false,
			Tools:               a.tools.Tools(),
			MaxCompletionTokens: 12000,
		}
		resp, err := a.openaiClient.CreateChatCompletion(context.Background(), req)
		if err != nil {
			return err
		}
		msg := resp.Choices[0].Message
		a.memory.Append(msg)
		if resp.Choices[0].FinishReason != openai.FinishReasonToolCalls {
			return nil
		}
		var wg sync.WaitGroup
		if len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				wg.Add(1)
				go func(toolCall *openai.ToolCall) {
					subCtx, cancel := context.WithTimeout(ctx, time.Minute*5)
					defer func() {
						wg.Done()
						cancel()
					}()
					result := a.tools.Dispatch(subCtx, toolCall)
					a.memory.Append(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: result, Name: toolCall.Function.Name, ToolCallID: toolCall.ID})
				}(&toolCall)
			}
		}
		wg.Wait()
		req.Messages = a.memory.HistoryMessages()
	}
	return fmt.Errorf("loop over max rounds, [%d]", maxRounds)
}
