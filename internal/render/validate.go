package render

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aymerick/raymond/ast"
	"github.com/aymerick/raymond/parser"
)

// allowedHelpers is the frozen call surface: raymond built-ins plus the
// helpers registered in render.go. Anything else in call position fails at
// Compile, not at render time — a typo must never silently blank output.
var allowedHelpers = []string{
	// raymond built-ins
	"if", "unless", "with", "each", "log", "lookup", "equal",
	// ISPPr helpers (render.go init)
	"eq", "ne", "lt", "le", "gt", "ge",
	"and", "or", "not",
	"upper", "lower", "title",
	"join", "date", "currency",
}

// validateHelpers walks the parsed AST and rejects any call expression
// whose callee isn't in allowedHelpers. Pure path lookups ({{foo}},
// {{a.b.c}}) carry no params and are always allowed — they render empty
// when missing.
func validateHelpers(src string) error {
	prog, err := parser.Parse(src)
	if err != nil {
		return err
	}
	for _, node := range prog.Body {
		if err := validateNode(node); err != nil {
			return err
		}
	}
	return nil
}

func validateNode(node ast.Node) error {
	switch n := node.(type) {
	case *ast.MustacheStatement:
		return validateExpr(n.Expression)
	case *ast.BlockStatement:
		if err := validateExpr(n.Expression); err != nil {
			return err
		}
		if err := validateProgram(n.Program); err != nil {
			return err
		}
		return validateProgram(n.Inverse)
	case *ast.ContentStatement, *ast.CommentStatement, *ast.PartialStatement:
		return nil
	}
	return nil
}

func validateProgram(p *ast.Program) error {
	if p == nil {
		return nil
	}
	for _, node := range p.Body {
		if err := validateNode(node); err != nil {
			return err
		}
	}
	return nil
}

func validateExpr(e *ast.Expression) error {
	if e == nil {
		return nil
	}
	if len(e.Params) > 0 || e.Hash != nil {
		name := e.HelperName()
		if name == "" {
			return fmt.Errorf("line %d: unsupported expression (calls require a registered helper)", e.Line)
		}
		if !slices.Contains(allowedHelpers, name) {
			return fmt.Errorf("line %d: unsupported helper %q (supported: %s)",
				e.Line, name, strings.Join(allowedHelpers, ", "))
		}
	}
	for _, p := range e.Params {
		if sub, ok := p.(*ast.SubExpression); ok {
			if err := validateExpr(sub.Expression); err != nil {
				return err
			}
		}
	}
	if e.Hash != nil {
		for _, pair := range e.Hash.Pairs {
			if sub, ok := pair.Val.(*ast.SubExpression); ok {
				if err := validateExpr(sub.Expression); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
