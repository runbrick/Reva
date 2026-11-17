package i18n

// Runner holds the headless collection command's human-facing text.
type Runner struct {
	Help string
}

// RunnerFor returns CLI strings without requiring GUI settings or data access.
// As with For, unsupported languages fall back to Chinese.
func RunnerFor(lang Lang) Runner {
	if lang == En {
		return runnerEn
	}
	return runnerZh
}

var runnerZh = Runner{
	Help: `用法：reva-run --collection ID [--data-dir DIR] [--environment ID] [--stop-on-failure]

串行运行原生 Reva 集合。默认不选择环境。
数据目录：依次使用 --data-dir、REVA_HOME、用户配置目录/Reva。
仅读取现有数据；不保存设置、历史记录、集合或环境。
会执行原生前置脚本和测试脚本；可能读取并发送已启用的上传文件。
标准输出：运行后输出一份精简的安全 JSON 报告（版本 1），包括取消的运行。
标准错误：仅输出固定诊断代码。输入错误不生成报告。
退出码：0 全部通过或帮助，1 运行失败或报告写入失败，2 用法或加载错误，130 已取消。
需要读取退出码时请使用编译后的程序；go run 可能包装非零退出状态。
参数：--collection ID、--environment ID、--data-dir DIR、--stop-on-failure、-h、--help。
`,
}

var runnerEn = Runner{
	Help: `Usage: reva-run --collection ID [--data-dir DIR] [--environment ID] [--stop-on-failure]

Runs a native Reva collection serially. No environment is selected by default.
Data directory: --data-dir, then REVA_HOME, then the user config directory/Reva.
Reads existing data only; never saves settings, history, collections or environments.
Native pre-request/test scripts execute; enabled upload files may be read and sent.
Stdout: one compact safe JSON report (version 1) after a run, including cancellation.
Stderr: fixed diagnostics only. Input errors produce no report.
Exit codes: 0 passed/help, 1 run/report-write failure, 2 usage/load error, 130 cancelled.
Use a built binary when consuming exit codes; go run may wrap nonzero exit status.
Flags: --collection ID, --environment ID, --data-dir DIR, --stop-on-failure, -h, --help.
`,
}
