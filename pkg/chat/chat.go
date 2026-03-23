package chat

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/toringzhang/nano-claw-go/pkg/agent"
	"github.com/toringzhang/nano-claw-go/pkg/memory"
)

const (
	systemPrompt = `You are a helpful assistant. You are able to answer questions and perform tasks.`
)

var (
	openaiBaseUrl = os.Getenv("OPENAI_BASE_URL")
	openaiToken   = os.Getenv("OPENAI_AUTH_TOKEN")
	module        = os.Getenv("OPENAI_MODEL")
)

func usage() {
	fmt.Println(`Usage:
  /exit: exit the program.
  /help: show this help message.
  /clear: clear the chat history.`)
}

func Main() {

	channelId := uuid.NewString()
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), "channelId", channelId))
	defer cancel()

	// create main memory
	mem := memory.NewMemory(channelId)
	go mem.Run(ctx)

	scanner := bufio.NewScanner(os.Stdin)
	openaiCli := initOpenaiClient()
	tools := agent.StandardTools()
	mainAgent := agent.NewAgent(openaiCli, module, systemPrompt, tools, mem)

	fmt.Printf("channel %s\n", channelId)
	fmt.Println("Please enter the content and press Enter (enter '/exit' to exit):")
	usage()
	for {
		fmt.Println("-------------------------------")
		fmt.Print("\033[32mYou: \033[0m")

		if scanner.Scan() {
			input := scanner.Text()
			if strings.HasPrefix(input, "/") {
				switch input {
				case "/exit":
					fmt.Println("Bye!")
					return
				case "/help":
					usage()
					continue
				case "/clear":
					fmt.Println("Chat history cleared.")
					continue
				default:
					fmt.Printf("\033[31mSystem: Unknow command: %s\n\033[0m", input)
					continue
				}
			}

			mem.Append(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: input})
			err := mainAgent.Loop(ctx, 10)
			if err != nil {
				fmt.Printf("\033[31mSystem: %v\n\033[0m", err)
				continue
			}
			last := mem.LastHistory()
			fmt.Printf("\033[34mAI: %s\n\033[0m", last.Content)
		} else {
			break
		}
	}
}

func initOpenaiClient() *openai.Client {
	if openaiToken == "" {
		panic("openai token is empty")
	}
	config := openai.DefaultConfig(openaiToken)
	config.BaseURL = openaiBaseUrl
	return openai.NewClientWithConfig(config)
}
