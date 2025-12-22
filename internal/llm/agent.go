package llm

import (
	"clai/internal/logger"
	"clai/internal/tools"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const AgentSystemPrompt = `You are a powerful autonomous agent with full code execution capabilities and an embedded JavaScript runtime powered by Goja (pure-Go ECMAScript 5.1+ implementation).

Your goal is to solve complex tasks through step-by-step reasoning, dynamic action-taking, and hierarchical orchestration when needed.

**Capabilities:**
- Full code execution: You can execute bash, python, and javascript/node code directly on the system.
- Internet access: Use curl, wget, or python requests in bash/python code to fetch web content.
- File system access: Read/write files, run commands, install packages if needed.
- Parallel sub-agent delegation: You can spawn multiple independent sub-agents that run concurrently.
- Iterative feedback loop: Observations from code execution or sub-agents are fed back to you for refinement.

**CRITICAL: You are an EXECUTOR, not a tutor. DO NOT explain how to do things — ACTUALLY DO THEM using code execution.**

**Strict response format — always use exactly ONE of these structures per response:**

OPTION A (Execution):
Thought: [Detailed reasoning about the current state, what you know, what needs to be done next, and your plan.]
Code:
'''bash
# Actual code to execute - NO explanatory echo statements, NO tutorials
# Use bash for: file operations, curl/wget for HTTP requests, command-line tools
'''

OPTION B (Final response after task completion):
Thought: [Brief summary of what was accomplished]
Final Answer: [The complete, final response to the user query based on observations from executed code. NEVER include code blocks here.]

**STRICT RULES:**
- ALWAYS start with Thought.
- NEVER output "Final Answer" AND "Code" in the same response — they are mutually exclusive.
- NEVER put code blocks inside "Final Answer" — code must be in the Code section to be executed.
- DO NOT write tutorial code with echo statements explaining concepts — write ACTUAL code that accomplishes the task.
- DO NOT apologize or explain limitations — just execute code and use observations to continue.
- If you need data from the internet, JUST FETCH IT with curl/wget/requests — don't explain how.
- Choose the best language: bash for HTTP/files, Python for data processing, Node for JS-specific tasks.
- After code execution, you will receive an Observation with the output — use it to continue or finalize.
- Only output "Final Answer" when you have actual results from executed code to present.
- Keep code minimal, practical, and error-resilient.
- You HAVE internet access via curl/wget/requests - use it confidently!

**Examples of CORRECT behavior:**
User: "Get today's news"
Thought: I need to fetch current news headlines from a public API.
Code:
'''bash
curl -s "https://newsapi.org/v2/top-headlines?country=us&apiKey=573e69b5b03c430d87d8f74b38399113" | jq -r '.articles[0:5] | .[] | "- " + .title'
'''

User: "What's in TODO.md?"
Thought: I need to read the TODO.md file.
Code:
'''bash
cat TODO.md
'''

**Examples of INCORRECT behavior (NEVER do this):**
❌ Final Answer: Here's how you can fetch news: [code example]
❌ Code: echo "Here's how to fetch news..."
❌ Thought: I can't do this because... Final Answer: [explanation]

**Default behavior when uncertain:**
- If asked about project status, plans, or "what's next", use Code to read TODO.md, README.md, or AGENTS.md
- If asked about code structure or how something works, use Code to read the relevant source files
- If asked about configuration or setup, use Code to read config files (.yaml, .json, .toml, Makefile, package.json, etc.)
- When you don't know something about the project, explore the filesystem first (ls, find, grep, cat via Code) rather than guessing
- Be proactive: if a question implies file knowledge, read those files before answering`

type AgentResponse struct {
	Thought    string
	Delegation []Subtask
	Code       string
	Language   string
	Final      string
}

type Subtask struct {
	Description string `json:"subtask"`
	Role        string `json:"role"`
}

type Agent struct {
	client     LLMClientInterface
	messages   []Message
	jsExecutor *JSExecutor
	maxIters   int
}

func NewAgent(client LLMClientInterface) *Agent {
	return &Agent{
		client:     client,
		messages:   []Message{},
		jsExecutor: NewJSExecutor(),
		maxIters:   20,
	}
}

func (a *Agent) AddMessage(role, content string) {
	a.messages = append(a.messages, Message{
		Role:    role,
		Content: content,
	})
}

func (a *Agent) parseResponse(response string) (*AgentResponse, error) {
	result := &AgentResponse{}

	thoughtRe := regexp.MustCompile(`(?s)Thought:\s*(.+?)(?:\n\n|$)`)
	if matches := thoughtRe.FindStringSubmatch(response); len(matches) > 1 {
		result.Thought = strings.TrimSpace(matches[1])
	}

	delegationRe := regexp.MustCompile(`(?s)Delegation:\s*(\[.+?\])`)
	if matches := delegationRe.FindStringSubmatch(response); len(matches) > 1 {
		var subtasks []Subtask
		if err := json.Unmarshal([]byte(matches[1]), &subtasks); err == nil {
			result.Delegation = subtasks
		} else {
			logger.Debug("[AGENT-PARSE] Failed to parse delegation JSON: %v", err)
		}
	}

	codeRe := regexp.MustCompile("(?s)Code:\\s*```(\\w+)?\\s*(.+?)```")
	if matches := codeRe.FindStringSubmatch(response); len(matches) > 2 {
		result.Language = strings.TrimSpace(matches[1])
		result.Code = strings.TrimSpace(matches[2])
		if result.Language == "" {
			result.Language = "bash" // default to bash if no language specified
		}
	}

	finalRe := regexp.MustCompile(`(?s)Final Answer:\s*(.+)$`)
	if matches := finalRe.FindStringSubmatch(response); len(matches) > 1 {
		result.Final = strings.TrimSpace(matches[1])
	}

	logger.Debug("[AGENT-PARSE] Thought: %q, Delegation: %d subtasks, Code: %d chars (%s), Final: %q",
		result.Thought, len(result.Delegation), len(result.Code), result.Language, result.Final)

	return result, nil
}

func (a *Agent) delegateInParallel(subtasks []Subtask) []string {
	results := make([]string, len(subtasks))
	var wg sync.WaitGroup

	for i, task := range subtasks {
		wg.Add(1)
		go func(idx int, t Subtask) {
			defer wg.Done()

			logger.Debug("[AGENT-DELEGATE] Starting subtask %d: %s (role: %s)", idx, t.Description, t.Role)

			subAgent := NewAgent(a.client)
			subAgent.maxIters = 5
			result, err := subAgent.Run(t.Description)
			if err != nil {
				results[idx] = fmt.Sprintf("Error: %v", err)
			} else {
				results[idx] = result
			}

			logger.Debug("[AGENT-DELEGATE] Completed subtask %d: %s", idx, results[idx])
		}(i, task)
	}

	wg.Wait()
	return results
}

func (a *Agent) Think() (string, error) {
	streamChan := make(chan string, 100)

	_, err := a.client.SendMessageStreamNoTools(a.messages, streamChan, false)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	var fullResponse strings.Builder
	for chunk := range streamChan {
		fullResponse.WriteString(chunk)
	}

	response := fullResponse.String()
	logger.Debug("[AGENT-THINK] Full response:\n%s", response)

	return response, nil
}

func (a *Agent) Run(query string) (string, error) {
	a.AddMessage("system", AgentSystemPrompt)
	a.AddMessage("user", query)

	logger.Debug("[AGENT-RUN] Starting agent loop with query: %s", query)

	for i := 0; i < a.maxIters; i++ {
		logger.Debug("[AGENT-ITER] Iteration %d/%d", i+1, a.maxIters)

		response, err := a.Think()
		if err != nil {
			return "", err
		}

		parsed, err := a.parseResponse(response)
		if err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if parsed.Final != "" {
			logger.Debug("[AGENT-COMPLETE] Final answer reached: %s", parsed.Final)
			return parsed.Final, nil
		}

		var observation strings.Builder

		if len(parsed.Delegation) > 0 {
			logger.Debug("[AGENT-DELEGATION] Delegating %d subtasks", len(parsed.Delegation))
			results := a.delegateInParallel(parsed.Delegation)
			observation.WriteString("Observation (Delegation results):\n")
			for idx, result := range results {
				observation.WriteString(fmt.Sprintf("Subtask %d: %s\n", idx+1, result))
			}
		}

		if parsed.Code != "" {
			logger.Debug("[AGENT-CODE] Executing %s code", parsed.Language)
			output, err := tools.ExecuteCode(parsed.Language, parsed.Code)
			if err != nil {
				observation.WriteString(fmt.Sprintf("Observation (Code execution error): %v\n", err))
			} else {
				observation.WriteString(fmt.Sprintf("Observation (Code output): %s\n", output))
			}
		}

		if observation.Len() > 0 {
			a.AddMessage("assistant", response)
			a.AddMessage("user", observation.String())
			logger.Debug("[AGENT-OBSERVATION] Added observation: %s", observation.String())
		} else {
			logger.Debug("[AGENT-WARNING] No delegation or code found, but no final answer either - returning full response")
			return response, nil
		}
	}

	return "Task incomplete: maximum iterations reached", nil
}
