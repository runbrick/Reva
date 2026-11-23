package i18n

import (
	"fmt"

	"github.com/runbrick/reva/internal/responsejson"
)

type ResponseFilter struct {
	Open, Title, Query, Placeholder, Run, Result, Copy   string
	Syntax, Limits, NoMatches, InvalidQuery, InvalidJSON string
	QueryLimit, InputLimit, ResultLimit, Truncated       string
}

func ResponseFilterFor(lang Lang) *ResponseFilter {
	if lang == En {
		return &responseFilterEn
	}
	return &responseFilterZh
}

func responseFilterLimits(pattern string) string {
	return fmt.Sprintf(pattern, responsejson.FilterMaxBodyBytes/1024, responsejson.FilterMaxQueryBytes,
		responsejson.FilterMaxSegments, responsejson.FilterMaxMatches, responsejson.FilterMaxResultBytes/1024)
}

var responseFilterZh = ResponseFilter{
	Open: "筛选 JSON", Title: "JSON 响应筛选", Query: "JSONPath 查询", Placeholder: "$.orders[*].id",
	Run: "执行查询", Result: "JSON 筛选结果", Copy: "复制筛选结果",
	Syntax:    `支持根节点 $、字段 .name 或 ["字段名"]、从零开始的非负索引 [0]，以及数组或对象通配符 [*] 和 .*。点号字段名由英文字母、数字和下划线组成，且不能以数字开头。中括号内的字段名使用 JSON 字符串语法。路径段之间不加空格。结果为 JSON 数组。`,
	Limits:    responseFilterLimits("上限：正文 %d KiB，查询 %d 字节、%d 个路径段，结果 %d 项、%d KiB。"),
	NoMatches: "没有匹配的 JSON 值。", InvalidQuery: "JSONPath 语法无效或不受支持。",
	InvalidJSON: "正文不是有效、无歧义的 JSON。", QueryLimit: "查询超过字节数或路径段数上限。",
	InputLimit: "JSON 正文超过大小或结构复杂度上限。", ResultLimit: "筛选结果超过项数或大小上限，请缩小查询范围。",
	Truncated: "响应正文已截断，筛选需要完整正文。",
}

var responseFilterEn = ResponseFilter{
	Open: "Filter JSON", Title: "JSON response filter", Query: "JSONPath query", Placeholder: "$.orders[*].id",
	Run: "Run query", Result: "JSON filter results", Copy: "Copy filter results",
	Syntax:    `Use root $, fields .name or ["field name"], zero-based nonnegative indexes [0], and array or object wildcards [*] or .*. Dot names use ASCII letters, digits and underscores and cannot start with a digit. Bracketed names use JSON string syntax. Omit spaces between segments. Results form a JSON array.`,
	Limits:    responseFilterLimits("Limits: %d KiB body, %d query bytes, %d path segments, %d matches and %d KiB of results."),
	NoMatches: "No matching JSON values.", InvalidQuery: "Invalid or unsupported JSONPath syntax.",
	InvalidJSON: "The body is not valid, unambiguous JSON.", QueryLimit: "The query exceeds the byte or path segment limit.",
	InputLimit: "The JSON body exceeds the size or complexity limit.", ResultLimit: "The results exceed the match or size limit. Narrow the query.",
	Truncated: "The response body was truncated. Filtering requires the complete body.",
}
