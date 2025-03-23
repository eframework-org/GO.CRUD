// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/beego/beego/v2/client/orm"
)

func TestNewCondition(t *testing.T) {
	tests := []struct {
		name        string
		args        []any
		expectPanic bool
		checkFunc   func(*testing.T, *condition)
	}{
		{
			name: "EmptyCondition",
			args: []any{},
			checkFunc: func(t *testing.T, c *condition) {
				if c == nil {
					t.Error("Expected non-nil condition")
				}
			},
		},
		{
			name: "FromExistingCondition",
			args: []any{orm.NewCondition()},
			checkFunc: func(t *testing.T, c *condition) {
				if c.Base == nil {
					t.Error("Expected non-nil base condition")
				}
			},
		},
		{
			name: "SimpleExpression",
			args: []any{"name == {0}", "test"},
			checkFunc: func(t *testing.T, c *condition) {
				if c.Base == nil {
					t.Error("Expected non-nil base condition")
				}
				params := getCondParams(c.Base)
				if len(params) == 0 {
					t.Error("Expected non-empty params")
				}
			},
		},
		{
			name: "ComplexExpressionWithAnd",
			args: []any{"age > {0} && name contains {1}", 18, "test"},
			checkFunc: func(t *testing.T, c *condition) {
				if c.Base == nil {
					t.Error("Expected non-nil base condition")
				}
				params := getCondParams(c.Base)
				if len(params) == 0 {
					t.Error("Expected non-empty params")
				}
			},
		},
		{
			name: "ComplexExpressionWithOR",
			args: []any{"age < {0} || name startswith {1}", 20, "test"},
			checkFunc: func(t *testing.T, c *condition) {
				if c.Base == nil {
					t.Error("Expected non-nil base condition")
				}
				params := getCondParams(c.Base)
				if len(params) == 0 {
					t.Error("Expected non-empty params")
				}
			},
		},
		{
			name: "ExpressionWithNOT",
			args: []any{"!(age >= {0})", 30},
			checkFunc: func(t *testing.T, c *condition) {
				if c.Base == nil {
					t.Error("Expected non-nil base condition")
				}
			},
		},
		{
			name: "ExpressionWithPagination",
			args: []any{"name == {0} limit {1} offset {2}", "test", 10, 20},
			checkFunc: func(t *testing.T, c *condition) {
				if c.Limit != 10 {
					t.Errorf("Expected limit 10, got %d", c.Limit)
				}
				if c.Offset != 20 {
					t.Errorf("Expected offset 20, got %d", c.Offset)
				}
			},
		},
		{
			name:        "InvalidParamCount",
			args:        []any{"name == {0} && age > {1}", "test"},
			expectPanic: true,
		},
		{
			name:        "InvalidArgumentType",
			args:        []any{123},
			expectPanic: true,
		},
		{
			name: "ComplexNestedExpressionWithCache",
			args: []any{"((age > {0} && name contains {1}) || status == {2}) && active == {3}", 18, "test", "active", true},
			checkFunc: func(t *testing.T, c *condition) {
				parsed := expressionCache.getParsedExpression("((age > {0} && name contains {1}) || status == {2}) && active == {3}")
				if parsed == nil {
					t.Error("Expression should be cached")
				}
			},
		},
		{
			name: "ExpressionWithAllOperatorTypes",
			args: []any{
				"age > {0} && score >= {1} && price < {2} && quantity <= {3} && " +
					"name == {4} && status != {5} && desc contains {6} && " +
					"title startswith {7} && path endswith {8} && deleted isnull {9}",
				20, 90, 100, 50, "test", "inactive", "keyword", "prefix", "suffix", true,
			},
			checkFunc: func(t *testing.T, c *condition) {
				params := getCondParams(c.Base)
				if len(params) == 0 {
					t.Error("Expected non-empty params for all operators")
				}
			},
		},
		{
			name: "ExpressionWithMultiplePagination",
			args: []any{"name == {0} limit {1} offset {2} && age > {3} limit {4}", "test", 10, 20, 30, 40},
			checkFunc: func(t *testing.T, c *condition) {
				if c.Limit != 40 {
					t.Errorf("Expected limit 40, got %d", c.Limit)
				}
				if c.Offset != 20 {
					t.Errorf("Expected offset 20, got %d", c.Offset)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic")
					}
				}()
			}
			c := Condition(tt.args...)
			if !tt.expectPanic && tt.checkFunc != nil {
				tt.checkFunc(t, c)
			}
		})
	}
}

func TestOperators(t *testing.T) {
	operatorTests := []struct {
		expr     string
		args     []any
		operator string
	}{
		{"age > {0}", []any{18}, "__gt"},
		{"age >= {0}", []any{18}, "__gte"},
		{"age < {0}", []any{30}, "__lt"},
		{"age <= {0}", []any{30}, "__lte"},
		{"name == {0}", []any{"test"}, "__exact"},
		{"name != {0}", []any{"test"}, "__ne"},
		{"name contains {0}", []any{"test"}, "__contains"},
		{"name startswith {0}", []any{"test"}, "__startswith"},
		{"name endswith {0}", []any{"test"}, "__endswith"},
		{"active isnull {0}", []any{true}, "__isnull"},
	}

	for _, tt := range operatorTests {
		t.Run(tt.expr, func(t *testing.T) {
			c := Condition(append([]any{tt.expr}, tt.args...)...)
			if c.Base == nil {
				t.Error("Expected non-nil base condition")
			}
			params := getCondParams(c.Base)
			if len(params) == 0 {
				t.Error("Expected non-empty params")
			}
		})
	}
}

func TestComplexConditions(t *testing.T) {
	tests := []struct {
		name string
		expr string
		args []any
	}{
		{
			name: "nested AND/OR",
			expr: "(age > {0} && name contains {1}) || (status == {2})",
			args: []any{18, "test", "active"},
		},
		{
			name: "multiple NOT",
			expr: "!(age < {0}) && !(name == {1})",
			args: []any{20, "test"},
		},
		{
			name: "complex nesting",
			expr: "((age >= {0} && age <= {1}) || (score > {2})) && active == {3}",
			args: []any{18, 30, 90, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Condition(append([]any{tt.expr}, tt.args...)...)
			if c.Base == nil {
				t.Error("Expected non-nil base condition")
			}
			params := getCondParams(c.Base)
			if len(params) == 0 {
				t.Error("Expected non-empty params")
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("GetBracketIndex", func(t *testing.T) {
		expr := "(a > {0}) && (b < {1})"
		lrMap := getLRIndex(expr)
		if len(lrMap) != 2 {
			t.Errorf("Expected 2 bracket pairs, got %d", len(lrMap))
		}
	})

	t.Run("subStr", func(t *testing.T) {
		tests := []struct {
			str      string
			from     int
			to       int
			expected string
		}{
			{"hello", 0, 5, "hello"},
			{"hello", 1, 4, "ell"},
			{"hello", -1, 4, ""},
			{"hello", 0, 10, ""},
			{"hello", 2, 1, ""},
		}

		for _, tt := range tests {
			result := subStr(tt.str, tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("subStr(%q, %d, %d) = %q; want %q",
					tt.str, tt.from, tt.to, result, tt.expected)
			}
		}
	})

	t.Run("toInt", func(t *testing.T) {
		tests := []struct {
			str         string
			expected    int
			expectPanic bool
			panicMsg    string
		}{
			{"123", 123, false, ""},
			{"-123", -123, false, ""},
			{"0", 0, false, ""},
			{"abc", 0, true, "invalid syntax: abc is not a valid parameter index"},
			{"", 0, true, "invalid syntax:  is not a valid parameter index"},
		}

		for _, tt := range tests {
			t.Run(tt.str, func(t *testing.T) {
				defer func() {
					r := recover()
					if (r != nil) != tt.expectPanic {
						t.Errorf("toInt(%q) panic = %v, expectPanic %v", tt.str, r, tt.expectPanic)
						return
					}
					if tt.expectPanic && r != nil {
						panicStr := fmt.Sprintf("%v", r)
						if !strings.Contains(panicStr, tt.panicMsg) {
							t.Errorf("Expected panic message containing '%s', got '%v'", tt.panicMsg, panicStr)
						}
					}
				}()
				result := toInt(tt.str)
				if !tt.expectPanic {
					if result != tt.expected {
						t.Errorf("toInt(%q) = %d; want %d", tt.str, result, tt.expected)
					}
				}
			})
		}
	})
}

// TestExpressionCache 测试表达式缓存机制
func TestExpressionCache(t *testing.T) {
	expr := "name == {0} && age > {1}"

	// 首次获取应该解析并缓存
	t.Run("InitialParse", func(t *testing.T) {
		parsed1 := expressionCache.getParsedExpression(expr)
		if parsed1 == nil {
			t.Error("Expected non-nil parsed expression")
		}
	})

	t.Run("CacheHit", func(t *testing.T) {
		parsed2 := expressionCache.getParsedExpression(expr)
		if parsed2 == nil {
			t.Error("Expected non-nil parsed expression from cache")
		}
	})

	// 测试并发安全性
	t.Run("ConcurrentAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				parsed := expressionCache.getParsedExpression(expr)
				if parsed == nil {
					t.Error("Concurrent access: got nil parsed expression")
				}
			}()
		}
		wg.Wait()
	})
}

// TestParseContext 测试解析上下文
func TestParseContext(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		args    []any
		wantErr bool
		check   func(*testing.T, *parseContext)
	}{
		{
			name: "NormalContext",
			expr: "name == {0}",
			args: []any{"test"},
			check: func(t *testing.T, ctx *parseContext) {
				if ctx.key != "" {
					t.Error("Key should be empty after parsing")
				}
			},
		},
		{
			name: "ContextWithMultipleOperators",
			expr: "age > {0} && score <= {1}",
			args: []any{18, 100},
			check: func(t *testing.T, ctx *parseContext) {
				tokens := strings.Fields(ctx.expr)
				for _, token := range tokens {
					if err := parseToken(ctx, token); err != nil {
						t.Errorf("Failed to parse token %q: %v", token, err)
					}
				}
				if ctx.andStr != "&&" {
					t.Error("Expected AND operator in context")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &parseContext{
				expr:   tt.expr,
				params: tt.args,
				cond:   orm.NewCondition(),
			}
			if tt.check != nil {
				tt.check(t, ctx)
			}
		})
	}
}

// TestTokenParsing 测试标记解析
func TestTokenParsing(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		ctx     *parseContext
		wantErr bool
	}{
		{
			name:  "logical operator",
			token: "&&",
			ctx:   &parseContext{},
		},
		{
			name:  "comparison operator",
			token: ">",
			ctx: &parseContext{
				key: "age",
			},
		},
		{
			name:    "invalid operator sequence",
			token:   ">",
			ctx:     &parseContext{},
			wantErr: true,
		},
		{
			name:  "field name",
			token: "name",
			ctx:   &parseContext{},
		},
		{
			name:    "duplicate field name",
			token:   "age",
			ctx:     &parseContext{key: "name"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseToken(tt.ctx, tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestEdgeCases 测试边界情况
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		args        []any
		expectPanic bool
		panicMsg    string
	}{
		{
			name:        "UnmatchedBrackets",
			expr:        "((a > {0})",
			args:        []any{1},
			expectPanic: true,
			panicMsg:    "left bracket count",
		},
		{
			name:        "InvalidParamIndex",
			expr:        "a > {abc}",
			args:        []any{1},
			expectPanic: true,
			panicMsg:    "invalid syntax: abc is not a valid parameter index",
		},
		{
			name:        "EmptyOperator",
			expr:        "a  b",
			args:        []any{},
			expectPanic: true,
			panicMsg:    "unexpected token",
		},
		{
			name:        "InvalidLimitValue",
			expr:        "a > {0} limit {1}",
			args:        []any{1, "invalid"},
			expectPanic: true,
			panicMsg:    "interface conversion",
		},
		{
			name:        "ParamIndexOutOfRange",
			expr:        "a > {0} && b > {2}",
			args:        []any{1},
			expectPanic: true,
			panicMsg:    "XOrm.Condition('a > {0} && b > {2}'): args count doesn't comply with format count",
		},
		{
			name:        "NegativeParamIndex",
			expr:        "a > {-1}",
			args:        []any{1},
			expectPanic: true,
			panicMsg:    "negative index: parameter index cannot be negative (-1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.expectPanic {
					t.Errorf("NewCondition() panic = %v, expectPanic %v", r, tt.expectPanic)
					return
				}
				if tt.expectPanic && r != nil {
					panicStr := ""
					switch v := r.(type) {
					case string:
						panicStr = v
					case error:
						panicStr = v.Error()
					case interface{ Error() string }:
						panicStr = v.Error()
					default:
						panicStr = fmt.Sprintf("%v", r)
					}
					if tt.panicMsg != "" && !strings.Contains(panicStr, tt.panicMsg) {
						t.Errorf("Expected panic message containing '%s', got '%v'", tt.panicMsg, panicStr)
					}
				}
			}()
			Condition(append([]any{tt.expr}, tt.args...)...)
		})
	}
}
