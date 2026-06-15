package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// genericCLIAssistant runs a non-Claude AI CLI one-shot per turn: pipe the
// prompt to stdin, stream stdout lines back as assistant:token events. No
// session, no MCP, no warm cache — these CLIs are not the cost concern.
type genericCLIAssistant struct {
	emit    EventEmitter
	cliID   string
	binPath string

	mu     sync.Mutex
	cancel context.CancelFunc

	newCmd cmdFactory
}

func newGenericCLIAssistant(emit EventEmitter, cliID, binPath string) *genericCLIAssistant {
	return &genericCLIAssistant{emit: emit, cliID: cliID, binPath: binPath, newCmd: newExecCmd}
}

func (g *genericCLIAssistant) Send(turn Turn, cfg SessionConfig) error {
	g.mu.Lock()
	if g.cancel != nil {
		g.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	g.mu.Unlock()

	prompt := buildUserText(turn.UserMessage, turn.EditorCode, turn.ErrorsText)

	go func() {
		defer cancel()
		err := g.run(ctx, prompt, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			g.emit.Emit("assistant:error", err.Error())
			return
		}
		g.emit.Emit("assistant:done", "")
	}()
	return nil
}

func (g *genericCLIAssistant) run(ctx context.Context, prompt string, cfg SessionConfig) error {
	args, err := genericCLIArgs(g.cliID, cfg.Model, cfg.SystemPrompt)
	if err != nil {
		return err
	}
	cmd := g.newCmd(ctx, g.binPath, args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Dir = os.TempDir()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %v", g.cliID, err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		g.emit.Emit("assistant:token", scanner.Text()+"\n")
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("%s: output read error: %v", g.cliID, err)
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s was cancelled", g.cliID)
		}
		if msg := strings.TrimSpace(stderrBuf.String()); msg != "" {
			return fmt.Errorf("%s error: %s", g.cliID, msg)
		}
		return fmt.Errorf("%s error: %v", g.cliID, err)
	}
	return nil
}

func (g *genericCLIAssistant) Interrupt() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
}

// Reset and Close cancel any in-flight one-shot turn; a generic CLI holds no
// persistent session beyond that.
func (g *genericCLIAssistant) Reset() { g.Interrupt() }
func (g *genericCLIAssistant) Close() { g.Interrupt() }

// genericCLIArgs builds the argv for a one-shot generic CLI from the model and
// system prompt.
func genericCLIArgs(cliID, model, sysPrompt string) ([]string, error) {
	var args []string
	switch cliID {
	case "ollama":
		m := model
		if m == "" {
			m = "llama3"
		}
		args = []string{"run", m, "--nowordwrap"}
		if sysPrompt != "" {
			args = append(args, "--system", sysPrompt)
		}
	case "aichat":
		if model != "" {
			args = append(args, "-m", model)
		}
		if sysPrompt != "" {
			args = append(args, "-S", sysPrompt)
		}
	case "llm":
		if model != "" {
			args = append(args, "-m", model)
		}
		if sysPrompt != "" {
			args = append(args, "-s", sysPrompt)
		}
	case "chatgpt":
		if model != "" {
			args = append(args, "-m", model)
		}
		if sysPrompt != "" {
			args = append(args, "-p", sysPrompt)
		}
	case "qwen":
		args = []string{"--output-format", "text"}
		if model != "" {
			args = append(args, "-m", model)
		}
		if sysPrompt != "" {
			args = append(args, "--system-prompt", sysPrompt)
		}
	default:
		return nil, fmt.Errorf("unsupported CLI: %s", cliID)
	}
	return args, nil
}
