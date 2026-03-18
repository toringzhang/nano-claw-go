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
	systemContent    = `You are a strict data-processing engine, not a conversational assistant. Your sole purpose is to provide the direct, final output requested by the user.

STRICT RULES:
1. Output ONLY the final result or answer.
2. Absolutely NO explanations, NO step-by-step reasoning, and NO chain of thought.
3. ZERO conversational filler (e.g., do not say "Here is...", "The answer is...", "Sure!", or "Based on...").
4. Do not include introductory or concluding remarks. 
5. If the request cannot be fulfilled, output only a concise error string without apologies.

Your output must be the exact raw data or answer expected, nothing more.`
)

type Agent interface {
	Loop(memory mem.Memory, module string, maxRounds int) error
}

type agent struct {
	openaiClient *openai.Client
	tools        Tools
}

func NewAgent(openaiCli *openai.Client) Agent {
	return &agent{
		openaiClient: openaiCli,
		tools:        NewTools(nil),
	}
}

func (a *agent) Loop(memory mem.Memory, module string, maxRounds int) error {
	if maxRounds <= 0 {
		maxRounds = defaultMaxRounds
	}
	for range maxRounds {
		time.Sleep(1 * time.Second)
		if memory.Length() <= 0 {
			return fmt.Errorf("memory is empty")
		}

		toolsPrompt := ""
		for _, tool := range a.tools {
			toolsPrompt += tool.Prompt()
		}
		req := openai.ChatCompletionRequest{
			Model:               module,
			Messages:            append([]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: systemContent + toolsPrompt}}, memory.HistoryMessages()...),
			Stream:              false,
			Tools:               a.tools.Tools(),
			MaxCompletionTokens: 12000,
		}
		resp, err := a.openaiClient.CreateChatCompletion(context.Background(), req)
		if err != nil {
			return err
		}
		msg := resp.Choices[0].Message
		memory.Append(msg)
		if resp.Choices[0].FinishReason != openai.FinishReasonToolCalls {
			return nil
		}
		var wg sync.WaitGroup
		if len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				wg.Add(1)
				go func(toolCall *openai.ToolCall) {
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
					defer func() {
						wg.Done()
						cancel()
					}()
					result := a.tools.Dispatch(ctx, toolCall)
					memory.Append(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: result, Name: toolCall.Function.Name, ToolCallID: toolCall.ID})
				}(&toolCall)
			}
		}
		wg.Wait()
		req.Messages = memory.HistoryMessages()
	}
	return fmt.Errorf("loop over max rounds, [%d]", maxRounds)
}
