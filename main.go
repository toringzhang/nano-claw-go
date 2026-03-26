/*
nano-claw-go - A learning implementation of Claude Code written in Golang.

This is an educational demo project that implements core Claude Code features:
1. Interactive chat with LLM
2. Tool calling (calculator, file operations, shell execution)
3. Skills system for specialized knowledge
4. Persistent memory (auto-save with atomic writes)
5. Sub-agents for complex tasks
6. Multi-round conversation loop

Flow:

	main() -> cmd.Execute() -> chat.Main() -> agent.Loop()
	- chat.Main(): reads user input, manages chat loop
	- agent.Loop(): calls LLM, executes tools, loops until no more tool calls
	- memory: persists chat history to disk every 10 seconds
	- skills: loaded from ./skills/ directory at startup
*/
package main

import "github.com/toringzhang/nano-claw-go/cmd"

func main() {
	cmd.Execute()
}
