package i18n

// Scripts holds the strings of the pre-request / test scripts feature.
// Owned by that feature's PR.
type Scripts struct {
	Title, PreRequest, Tests, Help, Snippets                       string
	StatusSnippetLabel, TimeSnippetLabel, VariableSnippetLabel     string
	StatusSnippet, TimeSnippet, VariableSnippet                    string
	PreScriptError, TestScriptError, PassedSummary, Empty, Console string
	PassMarker, FailMarker                                         string
	ResizeEditor                                                   string
}

var scriptsZh = Scripts{
	Title: "脚本", PreRequest: "预请求", Tests: "测试",
	Help:     "脚本使用 ES5 语法。使用 pm.request、pm.environment、pm.variables 和 console.log；通过 pm.test 与 pm.expect 检查响应。脚本限时 2 秒，变量仅用于本次发送。",
	Snippets: "插入片段", StatusSnippetLabel: "状态码为 200", TimeSnippetLabel: "响应时间低于 500 毫秒", VariableSnippetLabel: "从 JSON 字段设置变量",
	StatusSnippet:   "pm.test(\"状态码为 200\", function() { pm.response.to.have.status(200); });",
	TimeSnippet:     "pm.test(\"响应时间低于 500 毫秒\", function() { pm.expect(pm.response.responseTime).to.be.below(500); });",
	VariableSnippet: "pm.variables.set(\"token\", pm.response.json().token);",
	PreScriptError:  "预请求脚本错误", TestScriptError: "测试脚本错误", PassedSummary: "%d/%d 通过",
	Empty: "尚无测试结果。添加测试脚本并发送请求。", Console: "控制台", PassMarker: "✓", FailMarker: "✗",
	ResizeEditor: "拖动调整脚本编辑器高度",
}

var scriptsEn = Scripts{
	Title: "Scripts", PreRequest: "Pre-request", Tests: "Tests",
	Help:     "Scripts use ES5 syntax. Use pm.request, pm.environment, pm.variables and console.log; check responses with pm.test and pm.expect. Scripts have a 2-second limit; variables apply to this send only.",
	Snippets: "Insert snippet", StatusSnippetLabel: "Status is 200", TimeSnippetLabel: "Response time below 500 ms", VariableSnippetLabel: "Set a variable from a JSON field",
	StatusSnippet:   "pm.test(\"Status is 200\", function() { pm.response.to.have.status(200); });",
	TimeSnippet:     "pm.test(\"Response time below 500 ms\", function() { pm.expect(pm.response.responseTime).to.be.below(500); });",
	VariableSnippet: "pm.variables.set(\"token\", pm.response.json().token);",
	PreScriptError:  "Pre-request script error", TestScriptError: "Test script error", PassedSummary: "%d/%d passed",
	Empty: "No test results yet. Add a test script and send the request.", Console: "Console", PassMarker: "✓", FailMarker: "✗",
	ResizeEditor: "Drag to resize script editor",
}
