package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Knetic/govaluate"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type FunctionCall func(ctx context.Context, arguments string) string

type Tool interface {
	Name() string
	Tool() openai.Tool
	Call() FunctionCall
}

type Tools []Tool

var _ Tools

func NewTools(customTools []Tool) Tools {
	// default tools
	tools := Tools{
		&calculator{},
	}
	return append(tools, customTools...)
}

func (t Tools) Tools() []openai.Tool {
	tools := make([]openai.Tool, len(t))
	for i, tool := range t {
		tools[i] = tool.Tool()
	}
	return tools
}

func (t Tools) Dispatch(ctx context.Context, toolCall *openai.ToolCall) string {

	for _, tool := range t {
		if toolCall.Function.Name == tool.Name() {
			resultChan := make(chan string, 1)
			go func(targetTool Tool) {
				resultChan <- targetTool.Call()(ctx, toolCall.Function.Arguments)
			}(tool)
			select {
			case <-ctx.Done():
				return fmt.Sprintf("tool [%s] execution timeout or cancelled: %v", tool.Name(), ctx.Err())
			case result := <-resultChan:
				return result
			}
		}
	}
	return fmt.Sprintf("tool [%s] not found", toolCall.Function.Name)
}

type calculator struct {
}

func (c *calculator) Name() string {
	return "calculator"
}

func (c *calculator) Tool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        c.Name(),
			Description: "Used to perform basic mathematical calculations. This tool is called when it is necessary to calculate numbers, expressions.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"expression": {
						Type:        jsonschema.String,
						Description: "Mathematical expressions to be evaluated, eg '2 + 3 * 4', '100 / 5', '2 ** 3'",
					},
				},
				Required: []string{"expression"},
			},
		},
	}
}

func (c *calculator) Call() FunctionCall {
	return func(ctx context.Context, arguments string) string {
		var args map[string]interface{}
		err := json.Unmarshal([]byte(arguments), &args)
		if err != nil {
			return fmt.Sprintf("tool [%s] parse arguments failed: %v\n", c.Name(), err)
		}

		expression := args["expression"].(string)
		fmt.Printf("\033[30m>> tool [%s] call expression: %s\n\033[0m", c.Name(), expression)

		expr, err := govaluate.NewEvaluableExpression(expression)
		if err != nil {
			return fmt.Sprintf("tool [%s] parse expressions failed: %v", c.Name(), err)
		}

		result, err := expr.Evaluate(nil)
		if err != nil {
			return fmt.Sprintf("tool [%s] calculation failed: %v", c.Name(), err)
		}
		fmt.Printf("\033[30m>> tool [%s] call result: %v\n\033[0m", c.Name(), result)
		return fmt.Sprintf("%v", result)
	}
}
