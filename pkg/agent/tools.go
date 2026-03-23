package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/toringzhang/nano-claw-go/pkg/skill"
)

type Tool interface {
	Name() string
	Tool() openai.Tool
	Prompt() string
	Call(ctx context.Context, parameters map[string]any) string
}

type Tools []Tool

var _ Tools

func StandardTools() Tools {

	sl := &skillLoader{loader: skill.NewSkillLoader("./skills")}
	err := sl.loader.Load()
	if err != nil {
		panic(err)
	}

	// default tools
	return Tools{
		&calculator{},
		&reader{},
		&writer{},
		&executor{},
		sl,
	}
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
				var parameters map[string]any
				err := json.Unmarshal([]byte(toolCall.Function.Arguments), &parameters)
				if err != nil {
					resultChan <- fmt.Sprintf("tool [%s] parse arguments failed: %v\n", targetTool.Name(), err)
					return
				}
				resultChan <- targetTool.Call(ctx, parameters)
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

func (c *calculator) Prompt() string {
	return "Use calculator to calculate the result of the expression. Your output must be the exact raw data or answer expected, nothing more."
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

func (c *calculator) Call(ctx context.Context, parameters map[string]any) string {
	expression, ok := parameters["expression"].(string)
	if !ok {
		return fmt.Sprintf("tool [%s] parse parameters failed: %v\n", c.Name(), fmt.Errorf("expression type incorrect, must be a string"))
	}
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

type reader struct {
}

func (r *reader) Prompt() string {
	return ""
}

func (r *reader) Name() string {
	return "reader"
}

func (r *reader) Tool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        r.Name(),
			Description: "Read file contents.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"path": {
						Type: jsonschema.String,
					},
					"lines": {
						Type:        jsonschema.Integer,
						Description: "Maximum allowed file lines.",
					},
				},
				Required: []string{"path"},
			},
		},
	}
}

func (r *reader) Call(ctx context.Context, parameters map[string]any) string {

	path, ok := parameters["path"].(string)
	if !ok {
		return fmt.Sprintf("tool [%s] parse parameters failed: %v\n", r.Name(), fmt.Errorf("path type incorrect"))
	}

	lineLimit := 1000
	if lines, ok := parameters["lines"]; ok {
		num, ok := lines.(int)
		if !ok {
			return fmt.Sprintf("tool [%s] parse parameters failed: %v\n", r.Name(), fmt.Errorf("lines type incorrect, must be a integer"))
		}
		lineLimit = num
	}
	fmt.Printf("\033[30m>> tool [%s] call path: %s, lines: %d\n\033[0m", r.Name(), path, lineLimit)

	file, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("tool [%s] open file failed: %v", r.Name(), err)
	}
	defer file.Close()
	lines := make([]string, 0, lineLimit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= lineLimit {
			break
		}
	}
	result := strings.Join(lines, "\n")
	fmt.Printf("\033[30m>> tool [%s] call result: %v\n\033[0m", r.Name(), result)
	return fmt.Sprintf("%v", result)
}

type writer struct {
}

func (w *writer) Prompt() string {
	return ""
}

func (w *writer) Name() string {
	return "writer"
}

func (w *writer) Tool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        w.Name(),
			Description: "Write content to file",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"path": {
						Type: jsonschema.String,
					},
					"content": {
						Type: jsonschema.String,
					},
				},
				Required: []string{"path", "content"},
			},
		},
	}
}

func (w *writer) Call(ctx context.Context, parameters map[string]any) string {
	path, ok := parameters["path"].(string)
	if !ok {
		return fmt.Sprintf("tool [%s] parse parameters failed: %v\n", w.Name(), fmt.Errorf("path type incorrect"))
	}
	content, ok := parameters["content"].(string)
	if !ok {
		return fmt.Sprintf("tool [%s] parse parameters failed: %v\n", w.Name(), fmt.Errorf("content type incorrect"))
	}
	fmt.Printf("\033[30m>> tool [%s] call path: %s, content: %s\n\033[0m", w.Name(), path, content)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Sprintf("tool [%s] open file failed: %v", w.Name(), err)
	}
	defer file.Close()
	n, err := file.WriteString(content)
	if err != nil {
		return fmt.Sprintf("tool [%s] write file failed: %v", w.Name(), err)
	}
	result := fmt.Sprintf("write to %s success. size: %d", path, n)
	fmt.Printf("\033[30m>> tool [%s] call result: %v\n\033[0m", w.Name(), result)
	return fmt.Sprintf("%v", result)
}

type executor struct {
}

func (e *executor) Name() string {
	return "executor"
}

func (e *executor) Tool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        e.Name(),
			Description: "Run a shell command.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"command": {
						Type:        jsonschema.String,
						Description: "The command to run, e.g. 'ls -l'",
					},
				},
				Required: []string{"command"},
			},
		},
	}
}

func (e *executor) Prompt() string {
	return ""
}

func (e *executor) Call(ctx context.Context, parameters map[string]any) string {
	dangerous := func(command string) bool {
		command = strings.TrimSpace(command)
		if command == "rm -rf /" || command == "shutdown" || command == "reboot" || strings.Contains(command, "sudo") {
			return true
		}
		return false
	}

	command, ok := parameters["command"].(string)
	if !ok {
		return fmt.Sprintf("tool [%s] parse parameters failed: %v\n", e.Name(), fmt.Errorf("command type incorrect"))
	}
	fmt.Printf("\033[30m>> tool [%s] call command: %s, \n\033[0m", e.Name(), command)
	result := ""
	if dangerous(command) {
		result = fmt.Sprintf("tool [%s] execute command failed: %v", e.Name(), fmt.Errorf("dangerous command: %s", command))
	} else {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err := cmd.CombinedOutput()
		if err != nil {
			result = fmt.Sprintf("tool [%s] execute command failed: %v, output: %s", e.Name(), err, string(output))
		} else {
			result = string(output)
		}
	}
	fmt.Printf("\033[30m>> tool [%s] call result: %v\n\033[0m", e.Name(), result)
	return fmt.Sprintf("%v", result)
}

type skillLoader struct {
	loader skill.SkillLoader
}

func (s *skillLoader) Prompt() string {
	return s.loader.Prompt()
}

func (s *skillLoader) Name() string {
	return "load_skill"
}

func (s *skillLoader) Tool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        s.Name(),
			Description: "Load specialized knowledge by name.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"name": {
						Type:        jsonschema.String,
						Description: "Skill name to load",
					},
				},
				Required: []string{"name"},
			},
		},
	}
}

func (s *skillLoader) Call(ctx context.Context, parameters map[string]any) string {
	skill, ok := parameters["name"].(string)
	if !ok {
		return fmt.Sprintf("tool [%s] parse parameters failed: %v\n", s.Name(), fmt.Errorf("name type incorrect"))
	}
	fmt.Printf("\033[30m>> tool [%s] call skill: %s\n\033[0m", s.Name(), skill)
	result := s.loader.Skill(skill)
	fmt.Printf("\033[30m>> tool [%s] call result: %s\n\033[0m", s.Name(), result)
	return fmt.Sprintf("%s", result)
}

type task struct {
	Module    string
	openaiCli *openai.Client
	tools     []Tool
}

func (t *task) Name() string {
	return "task"
}

func (t *task) Tool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        t.Name(),
			Description: "Spawn a subagent with fresh context. It not conversation history. For complex tasks, please use it to complete one or some of the steps.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"prompt": {
						Type:        jsonschema.String,
						Description: "Short description of the task",
					},
				},
				Required: []string{"prompt"},
			},
		},
	}
}

func (t *task) Prompt() string {
	return ""
}

func (t *task) Call(ctx context.Context, parameters map[string]any) string {
	prompt, ok := parameters["prompt"].(string)
	if !ok {
		return fmt.Sprintf("tool [%s] parse parameters failed: %v\n", t.Name(), fmt.Errorf("prompt type incorrect"))
	}
	fmt.Printf("\033[30m>> tool [%s] call prompt: %s\n\033[0m", t.Name(), prompt)

	sub := NewSubagent("task", "For complex tasks, use it to complete one or some of the steps.", t.openaiCli, t.Module, prompt, t.tools)
	result := sub.Run(ctx)
	fmt.Printf("\033[30m>> tool [%s] call result: %s\n\033[0m", t.Name(), result)
	return fmt.Sprintf("%s", result)
}
