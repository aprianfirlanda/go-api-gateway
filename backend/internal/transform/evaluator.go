package transform

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"backend/internal/protocol"
)

type Evaluator struct {
	now  func() time.Time
	stan func() string
}

func NewEvaluator(now func() time.Time, stan func() string) *Evaluator {
	return &Evaluator{now: now, stan: stan}
}

func (e *Evaluator) Validate(expression string) error {
	_, err := e.parse(expression)
	return err
}

func (e *Evaluator) Evaluate(expression string, msg protocol.CanonicalMessage) (any, error) {
	parsed, err := e.parse(expression)
	if err != nil {
		return nil, err
	}
	return e.evaluate(parsed, msg)
}

type expressionNode interface {
	expressionNode()
}

type pathNode struct {
	path string
}

func (pathNode) expressionNode() {}

type literalNode struct {
	value string
}

func (literalNode) expressionNode() {}

type functionNode struct {
	name string
	args []expressionNode
}

func (functionNode) expressionNode() {}

func (e *Evaluator) parse(expression string) (expressionNode, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("expression is required")
	}
	if strings.HasPrefix(expression, "$.") {
		return pathNode{path: expression}, validatePath(expression)
	}
	if strings.HasPrefix(expression, "'") || strings.HasSuffix(expression, "'") {
		if len(expression) < 2 || !strings.HasPrefix(expression, "'") || !strings.HasSuffix(expression, "'") {
			return nil, fmt.Errorf("invalid string literal")
		}
		value := strings.TrimSuffix(strings.TrimPrefix(expression, "'"), "'")
		if strings.Contains(value, "'") {
			return nil, fmt.Errorf("single quotes are not allowed inside string literals")
		}
		return literalNode{value: value}, nil
	}

	open := strings.Index(expression, "(")
	if open <= 0 || !strings.HasSuffix(expression, ")") {
		return nil, fmt.Errorf("unsupported expression")
	}

	name := strings.TrimSpace(expression[:open])
	if !isAllowedFunction(name) {
		return nil, fmt.Errorf("function %q is not allowed", name)
	}

	rawArgs := strings.TrimSpace(expression[open+1 : len(expression)-1])
	args, err := e.parseArgs(rawArgs)
	if err != nil {
		return nil, err
	}

	if err := validateArgCount(name, len(args)); err != nil {
		return nil, err
	}

	return functionNode{name: name, args: args}, nil
}

func (e *Evaluator) parseArgs(raw string) ([]expressionNode, error) {
	if raw == "" {
		return nil, nil
	}

	parts, err := splitArgs(raw)
	if err != nil {
		return nil, err
	}

	args := make([]expressionNode, 0, len(parts))
	for _, part := range parts {
		arg, err := e.parse(part)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}

	return args, nil
}

func (e *Evaluator) evaluate(node expressionNode, msg protocol.CanonicalMessage) (any, error) {
	switch node := node.(type) {
	case pathNode:
		return evaluatePath(node.path, msg)
	case literalNode:
		return node.value, nil
	case functionNode:
		args := make([]any, 0, len(node.args))
		for _, arg := range node.args {
			value, err := e.evaluate(arg, msg)
			if err != nil {
				return nil, err
			}
			args = append(args, value)
		}
		return e.call(node.name, args)
	default:
		return nil, fmt.Errorf("unknown expression")
	}
}

func (e *Evaluator) call(name string, args []any) (any, error) {
	switch name {
	case "formatAmount":
		return formatAmount(args[0])
	case "currencyNumeric":
		return currencyNumeric(args[0])
	case "nowMMddHHmmss":
		return e.now().UTC().Format("0102150405"), nil
	case "generateStan":
		return e.stan(), nil
	case "maskPan":
		return maskPan(toString(args[0])), nil
	default:
		return nil, fmt.Errorf("function %q is not allowed", name)
	}
}

func validatePath(path string) error {
	if !strings.HasPrefix(path, "$.") {
		return fmt.Errorf("path must start with $.")
	}

	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	if len(parts) != 2 {
		return fmt.Errorf("path must reference exactly one field")
	}

	switch parts[0] {
	case "fields", "headers", "metadata":
	default:
		return fmt.Errorf("path root %q is not allowed", parts[0])
	}
	if parts[1] == "" {
		return fmt.Errorf("path field is required")
	}
	for _, char := range parts[1] {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '-' {
			continue
		}
		return fmt.Errorf("path field contains invalid character %q", char)
	}

	return nil
}

func evaluatePath(path string, msg protocol.CanonicalMessage) (any, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	switch parts[0] {
	case "fields":
		return msg.Fields[parts[1]], nil
	case "headers":
		return msg.Headers.Get(parts[1]), nil
	case "metadata":
		return msg.Metadata[parts[1]], nil
	default:
		return nil, fmt.Errorf("path root %q is not allowed", parts[0])
	}
}

func splitArgs(raw string) ([]string, error) {
	var args []string
	start := 0
	depth := 0
	inLiteral := false

	for idx, char := range raw {
		switch char {
		case '\'':
			inLiteral = !inLiteral
		case '(':
			if !inLiteral {
				depth++
			}
		case ')':
			if !inLiteral {
				depth--
				if depth < 0 {
					return nil, fmt.Errorf("invalid function arguments")
				}
			}
		case ',':
			if !inLiteral && depth == 0 {
				args = append(args, strings.TrimSpace(raw[start:idx]))
				start = idx + 1
			}
		}
	}

	if inLiteral || depth != 0 {
		return nil, fmt.Errorf("invalid function arguments")
	}

	args = append(args, strings.TrimSpace(raw[start:]))
	for _, arg := range args {
		if arg == "" {
			return nil, fmt.Errorf("empty function argument")
		}
	}

	return args, nil
}

func isAllowedFunction(name string) bool {
	switch name {
	case "formatAmount", "currencyNumeric", "nowMMddHHmmss", "generateStan", "maskPan":
		return true
	default:
		return false
	}
}

func validateArgCount(name string, count int) error {
	expected := map[string]int{
		"formatAmount":    1,
		"currencyNumeric": 1,
		"nowMMddHHmmss":   0,
		"generateStan":    0,
		"maskPan":         1,
	}

	if expected[name] != count {
		return fmt.Errorf("function %s expects %d arguments", name, expected[name])
	}

	return nil
}

func toString(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", value)
	}
}

func toInt64(value any) (int64, error) {
	switch value := value.(type) {
	case int:
		return int64(value), nil
	case int64:
		return value, nil
	case int32:
		return int64(value), nil
	case float64:
		return int64(value), nil
	case float32:
		return int64(value), nil
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("value %q is not an integer", value)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("value %T is not an integer", value)
	}
}
