// Copyright (c) 2025 EFramework Organization. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package XOrm

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/beego/beego/v2/client/orm"
	"github.com/eframework-org/GO.UTIL/XLog"
	"github.com/eframework-org/GO.UTIL/XObject"
)

// condition 表示一个查询条件，包含基础条件和分页信息
type condition struct {
	Base   *orm.Condition // 基础条件
	Limit  int            // 分页限制
	Offset int            // 分页偏移
}

// Ctor 初始化条件
func (c *condition) Ctor(obj any) {
	c.Base = orm.NewCondition()
}

// Condition 创建新的条件
//
// 用法:
// 1. Condition() - 创建空条件
// 2. Condition(existingCond *orm.Condition) - 从现有条件创建
// 3. Condition("a > {0} && b == {1}", 1, 2) - 从表达式和参数创建
func Condition(condOrExprAndArgs ...any) *condition {
	c := XObject.New[condition]()
	if len(condOrExprAndArgs) == 0 {
		return c
	}

	if cond, ok := condOrExprAndArgs[0].(*orm.Condition); ok {
		c.Base = cond
		return c
	}

	if expr, ok := condOrExprAndArgs[0].(string); ok {
		if expr == "" {
			return c
		}

		args := condOrExprAndArgs[1:]
		count := strings.Count(expr, "{")
		if count != len(args) {
			XLog.Panic("XOrm.Condition('%v'): args count doesn't comply with format count.", expr)
		}

		parsed := expressionCache.getParsedExpression(expr)
		if len(expr) > 0 && parsed.LRIndex[0] != len(expr)-1 {
			expr = "(" + expr + ")"
			parsed = expressionCache.getParsedExpression(expr)
		}

		ctx := &parseContext{
			rcond:  c,
			expr:   expr,
			params: args,
			cond:   c.Base,
		}

		c.Base = doParse(ctx, parsed.LRIndex, 0, len(expr)-1, "", "")
		return c
	}

	XLog.Panic("XOrm.Condition: invalid arguments type: %T", condOrExprAndArgs[0])
	return nil
}

var operatorMap = map[string]string{
	">":          "__gt",
	">=":         "__gte",
	"<":          "__lt",
	"<=":         "__lte",
	"==":         "__exact",
	"!=":         "__ne",
	"contains":   "__contains",
	"startswith": "__startswith",
	"endswith":   "__endswith",
	"isnull":     "__isnull",
}

var expressionCache = &expressionMap{}

// expressionInfo 缓存已解析的表达式结构
type expressionInfo struct {
	LRIndex map[int]int // 左右括号索引映射
	Tokens  []string    // 分词后的表达式
}

// expressionMap 表达式解析缓存
type expressionMap struct {
	sync.Map
}

// parseContext 解析上下文
type parseContext struct {
	rcond  *condition     // 根条件
	expr   string         // 原始表达式
	params []any          // 参数列表
	key    string         // 当前键
	value  any            // 当前值
	andStr string         // AND/OR 操作符
	notStr string         // NOT 操作符
	cond   *orm.Condition // 当前条件
}

// getParsedExpression 获取或解析表达式
func (ec *expressionMap) getParsedExpression(expr string) *expressionInfo {
	if cached, ok := ec.Load(expr); ok {
		return cached.(*expressionInfo)
	}

	parsed := &expressionInfo{
		LRIndex: getLRIndex(expr),
		Tokens:  strings.Fields(expr),
	}
	ec.Store(expr, parsed)
	return parsed
}

// getLRIndex 获取括号的左右索引映射
func getLRIndex(expression string) map[int]int {
	lArr, rArr := make([]int, 0, len(expression)/2), make([]int, 0, len(expression)/2)
	for i, ch := range expression {
		switch ch {
		case '(':
			lArr = append(lArr, i)
		case ')':
			rArr = append(rArr, i)
		}
	}

	if len(lArr) != len(rArr) {
		panic(fmt.Sprintf("getLRIndex('%v'): left bracket count %v doesn't equals right bracket count %v",
			expression, len(lArr), len(rArr)))
	}

	lrMap := make(map[int]int, len(rArr))
	for _, rIndex := range rArr {
		for j := len(lArr) - 1; j >= 0; j-- {
			lIndex := lArr[j]
			if rIndex > lIndex {
				lrMap[lIndex] = rIndex
				lArr = append(lArr[:j], lArr[j+1:]...)
				break
			}
		}
	}
	return lrMap
}

// doParse 执行解析
func doParse(ctx *parseContext, lrMap map[int]int, left, right int, andStr, notStr string) *orm.Condition {
	if andStr != "" {
		ncond := orm.NewCondition()
		ncond = parseSubExpr(ctx, lrMap, left, right, andStr, notStr, ncond)
		ctx.cond = applyLogicalOp(ctx.cond, ncond, andStr, notStr == "!")
	} else {
		ctx.cond = parseSubExpr(ctx, lrMap, left, right, andStr, notStr, ctx.cond)
	}
	return ctx.cond
}

// parseSubExpr 解析子表达式并构建条件
// 处理两种情况：
// 1. 包含括号的复杂表达式：递归解析括号内的内容
// 2. 简单表达式：直接解析tokens
func parseSubExpr(ctx *parseContext, lrMap map[int]int, left, right int, andStr, notStr string, cond *orm.Condition) *orm.Condition {
	ctx.cond = cond
	subExpression := subStr(ctx.expr, left+1, right)

	if strings.Contains(subExpression, "(") {
		leftIndex, rightIndex := findBrackets(lrMap, left, right)
		andStr, notStr, ctx.cond = parseTokens(ctx, subStr(ctx.expr, left+1, leftIndex), andStr, notStr)
		ctx.cond = doParse(ctx, lrMap, leftIndex, rightIndex, andStr, notStr)
		ctx.cond = parseRemaining(ctx, lrMap, rightIndex+1, right)
	} else {
		_, _, ctx.cond = parseTokens(ctx, subExpression, andStr, notStr)
	}
	return ctx.cond
}

// parseTokens 解析表达式中的标记
func parseTokens(ctx *parseContext, subExpression, andStr, notStr string) (string, string, *orm.Condition) {
	tokens := strings.Fields(subExpression)
	if len(tokens) == 0 {
		return andStr, notStr, ctx.cond
	}

	for _, token := range tokens {
		if err := parseToken(ctx, token); err != nil {
			XLog.Panic("XOrm.parseTokens: %v", err)
		}
	}

	return ctx.andStr, ctx.notStr, ctx.cond
}

// parseToken 解析单个标记
func parseToken(ctx *parseContext, token string) error {
	switch token {
	case "!":
		ctx.notStr = token
	case "||", "&&":
		ctx.andStr = token
	default:
		return parseOperatorOrValue(ctx, token)
	}
	return nil
}

// parseOperatorOrValue 解析操作符或值
func parseOperatorOrValue(ctx *parseContext, token string) error {
	if op, exist := operatorMap[token]; exist {
		if ctx.key == "" {
			return fmt.Errorf("unidentified operator: %v", token)
		}
		ctx.key += op
		return nil
	}

	if strings.HasPrefix(token, "{") && strings.HasSuffix(token, "}") {
		return handleParamValue(ctx, token)
	}

	if ctx.key != "" {
		return fmt.Errorf("unexpected token: %v", token)
	}
	ctx.key = token
	return nil
}

// handleParamValue 处理参数值
func handleParamValue(ctx *parseContext, token string) error {
	idxStr := token[1 : len(token)-1]
	idx := toInt(idxStr)
	if idx < 0 {
		panic(fmt.Sprintf("negative index: parameter index cannot be negative (%d)", idx))
	}
	if idx >= len(ctx.params) {
		panic(fmt.Sprintf("index out of range: parameter index %d exceeds argument count %d", idx, len(ctx.params)))
	}
	ctx.value = ctx.params[idx]
	ctx.cond = applyCondition(ctx.rcond, ctx.cond, ctx.key, ctx.value, ctx.andStr)
	ctx.key = ""
	return nil
}

// findBrackets 查找括号对
func findBrackets(lrMap map[int]int, left, right int) (int, int) {
	for i := left + 1; i <= right; i++ {
		if rIndex, exist := lrMap[i]; exist {
			return i, rIndex
		}
	}
	return left, right
}

// parseRemaining 解析剩余部分
func parseRemaining(ctx *parseContext, lrMap map[int]int, left, right int) *orm.Condition {
	subExpression := subStr(ctx.expr, left, right)
	if strings.Contains(subExpression, "(") {
		return doParse(ctx, lrMap, left, right, "", "")
	}
	_, _, ctx.cond = parseTokens(ctx, subExpression, "", "")
	return ctx.cond
}

// applyCondition 应用条件
func applyCondition(rcond *condition, cond *orm.Condition, key string, value any, andStr string) *orm.Condition {
	if key == "limit" {
		rcond.Limit = value.(int)
	} else if key == "offset" {
		rcond.Offset = value.(int)
	} else {
		switch andStr {
		case "||":
			cond = cond.Or(key, value)
		default:
			cond = cond.And(key, value)
		}
	}
	return cond
}

// applyLogicalOp 应用逻辑操作符
func applyLogicalOp(cond, ncond *orm.Condition, andStr string, isNot bool) *orm.Condition {
	if andStr == "||" {
		if isNot {
			return cond.OrNotCond(ncond)
		}
		return cond.OrCond(ncond)
	}
	if isNot {
		return cond.AndNotCond(ncond)
	}
	return cond.AndCond(ncond)
}

// subStr 提取子字符串
func subStr(str string, from, to int) string {
	if from < 0 || to < 0 || from > to || from >= len(str) || to > len(str) {
		return ""
	}
	return str[from:to]
}

// toInt 将字符串转换为整数
func toInt(str string) int {
	val, err := strconv.Atoi(str)
	if err != nil {
		panic(fmt.Sprintf("invalid syntax: %s is not a valid parameter index", str))
	}
	return val
}

type beegoCondition struct {
	params []beegoCondValue
}

// beegoCondValue 条件值结构
type beegoCondValue struct {
	Exprs  []string
	Args   []any
	Cond   *orm.Condition
	IsOr   bool
	IsNot  bool
	IsCond bool
	IsRaw  bool
	Sql    string
}

// getCondParams 获取条件参数（内部使用）
func getCondParams(cond *orm.Condition) []beegoCondValue {
	ncond := (*beegoCondition)(unsafe.Pointer(cond))
	return ncond.params
}
