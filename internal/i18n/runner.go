package i18n

// Runner holds human-facing runner setup and CLI text.
type Runner struct {
	Help, Scope, WholeCollection, MissingFolder, FolderPath string
	BrowseData, ClearData, DataSelected, InvalidData        string
	IterationLimit, Iterations, Iteration, IterationRows    string
	EmptyScope                                              string
}

// RunnerFor returns runner strings without requiring data access.
// As with For, unsupported languages fall back to Chinese.
func RunnerFor(lang Lang) Runner {
	if lang == En {
		return runnerEn
	}
	return runnerZh
}

var runnerZh = Runner{
	Help: `用法：reva-run --collection ID [--folder ID] [--environment ID] [--data-dir DIR]
               [--data-file FILE] [--report-format json|junit] [--cookie-session] [--stop-on-failure]

串行运行原生 Reva 集合。默认不选择环境。
数据目录：依次使用 --data-dir、REVA_HOME、用户配置目录/Reva。
仅读取现有数据；不保存设置、历史记录、集合或环境。
会执行原生前置脚本和测试脚本；可能读取并发送已启用的上传文件。
--folder 按原生文件夹 ID 限定范围。--data-file 按 CSV 或 JSON 文件的行顺序迭代。
--cookie-session 为每次迭代启用独立的临时 Cookie 会话。
标准输出：运行后输出一份安全报告，包括取消的运行。默认 JSON（版本 1），可选 JUnit XML。
标准错误：仅输出固定诊断代码。输入错误不生成报告。
退出码：0 全部通过或帮助，1 运行失败或报告写入失败，2 用法或加载错误，130 已取消。
需要读取退出码时请使用编译后的程序；go run 可能包装非零退出状态。
使用 -h 或 --help 查看帮助。
`,
	Scope:           "运行范围",
	WholeCollection: "整个集合",
	MissingFolder:   "所选文件夹已不存在，请重新选择运行范围。",
	FolderPath:      "%s / %s",
	BrowseData:      "选择 CSV 或 JSON 数据文件",
	ClearData:       "清除数据文件",
	DataSelected:    "已选择数据文件",
	InvalidData:     "无法读取数据文件，或文件不符合 CSV/JSON 格式和大小限制。",
	IterationLimit:  "数据行数与请求数的乘积不能超过 10,000。",
	Iterations:      "数据迭代：%d",
	Iteration:       "迭代",
	IterationRows:   "请求结果（按运行顺序）",
	EmptyScope:      "所选范围内没有已保存的请求。",
}

var runnerEn = Runner{
	Help: `Usage: reva-run --collection ID [--folder ID] [--environment ID] [--data-dir DIR]
                      [--data-file FILE] [--report-format json|junit] [--cookie-session] [--stop-on-failure]

Runs a native Reva collection serially. No environment is selected by default.
Data directory: --data-dir, then REVA_HOME, then the user config directory/Reva.
Reads existing data only; never saves settings, history, collections or environments.
Native pre-request/test scripts execute; enabled upload files may be read and sent.
--folder limits the run to a native folder ID. --data-file runs each CSV or JSON row in order.
--cookie-session uses a fresh temporary cookie session for each iteration.
Stdout: one safe report after a run, including cancellation. Defaults to JSON (version 1). JUnit XML is optional.
Stderr: fixed diagnostics only. Input errors produce no report.
Exit codes: 0 passed/help, 1 run/report-write failure, 2 usage/load error, 130 cancelled.
Use a built binary when consuming exit codes; go run may wrap nonzero exit status.
Use -h or --help to show this help.
`,
	Scope:           "Run scope",
	WholeCollection: "Whole collection",
	MissingFolder:   "The selected folder no longer exists. Choose a run scope again.",
	FolderPath:      "%s / %s",
	BrowseData:      "Choose CSV or JSON data file",
	ClearData:       "Clear data file",
	DataSelected:    "Data file selected",
	InvalidData:     "The data file could not be read or exceeds the CSV/JSON format or size limits.",
	IterationLimit:  "The number of data rows times the number of requests must be 10,000 or fewer.",
	Iterations:      "Data iterations: %d",
	Iteration:       "Iteration",
	IterationRows:   "Request results in run order",
	EmptyScope:      "There are no saved requests in the selected scope.",
}
