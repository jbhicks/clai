package llm

import (
	"clai/internal/logger"
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

type JSExecutor struct {
	vm *goja.Runtime
}

func NewJSExecutor() *JSExecutor {
	vm := goja.New()

	var logBuffer strings.Builder
	vm.Set("log", func(call goja.FunctionCall) goja.Value {
		msg := fmt.Sprintf("%v", call.Argument(0).Export())
		logger.Debug("[JS-LOG] %s", msg)
		logBuffer.WriteString(msg)
		logBuffer.WriteString("\n")
		return goja.Undefined()
	})

	vm.Set("console", map[string]interface{}{
		"log": func(call goja.FunctionCall) goja.Value {
			msg := fmt.Sprintf("%v", call.Argument(0).Export())
			logger.Debug("[JS-CONSOLE] %s", msg)
			logBuffer.WriteString(msg)
			logBuffer.WriteString("\n")
			return goja.Undefined()
		},
	})

	executor := &JSExecutor{vm: vm}
	vm.Set("__logBuffer", &logBuffer)

	return executor
}

func (e *JSExecutor) Execute(code string) (string, error) {
	logger.Debug("[JS-EXEC] Running code:\n%s", code)

	logBuffer := e.vm.Get("__logBuffer").Export().(*strings.Builder)
	logBuffer.Reset()

	val, err := e.vm.RunString(code)
	if err != nil {
		logger.Debug("[JS-EXEC-ERROR] %v", err)
		return "", fmt.Errorf("JavaScript execution error: %w", err)
	}

	output := logBuffer.String()

	if val != nil && !goja.IsUndefined(val) && !goja.IsNull(val) {
		result := fmt.Sprintf("%v", val.Export())
		if output != "" {
			output += result
		} else {
			output = result
		}
	}

	if output == "" {
		output = "Executed successfully (no output)"
	}

	logger.Debug("[JS-EXEC-OUTPUT] %s", output)
	return strings.TrimSpace(output), nil
}
