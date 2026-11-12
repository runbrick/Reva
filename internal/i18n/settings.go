package i18n

// Settings holds every label and description on the settings page.
type Settings struct {
	Title, Appearance, Requests, History, Data                  string
	Theme, ThemeDescription, Light, Dark, System                string
	Language, LanguageDescription, Chinese, English             string
	Timeout, TimeoutDescription                                 string
	FollowRedirects, FollowRedirectsDescription                 string
	VerifyTLS, VerifyTLSDescription, TLSWarning                 string
	HistoryLimit, HistoryDescription                            string
	DataDirectory, OpenFolder, About, Close                     string
	CheckUpdates, CheckingUpdates, UpdateAvailable, OpenRelease string
	UpToDate, NoRelease, UpdateFailed                           string
}

var settingsZh = Settings{
	Title: "设置", Appearance: "外观", Requests: "请求", History: "历史记录", Data: "数据",
	Theme: "主题", ThemeDescription: "选择应用外观，或跟随系统设置。", Light: "浅色", Dark: "深色", System: "跟随系统",
	Language: "语言", LanguageDescription: "选择界面显示语言。", Chinese: "中文", English: "English",
	Timeout: "默认超时（秒）", TimeoutDescription: "未单独设置时，请求等待响应的最长时间。",
	FollowRedirects: "跟随重定向", FollowRedirectsDescription: "自动跟随服务器返回的重定向。",
	VerifyTLS: "验证 TLS 证书", VerifyTLSDescription: "验证 HTTPS 服务器的证书。", TLSWarning: "证书验证已关闭，HTTPS 连接可能不安全。",
	HistoryLimit: "历史记录上限", HistoryDescription: "保留的最近请求数量。",
	DataDirectory: "数据目录", OpenFolder: "打开文件夹", About: "关于", Close: "关闭",
	CheckUpdates: "检查更新", CheckingUpdates: "正在检查更新…", UpdateAvailable: "发现新版本", OpenRelease: "打开下载页",
	UpToDate: "已是最新版本", NoRelease: "尚无已发布版本", UpdateFailed: "检查更新失败",
}

var settingsEn = Settings{
	Title: "Settings", Appearance: "Appearance", Requests: "Requests", History: "History", Data: "Data",
	Theme: "Theme", ThemeDescription: "Choose an appearance or follow your system.", Light: "Light", Dark: "Dark", System: "Follow system",
	Language: "Language", LanguageDescription: "Choose the interface language.", Chinese: "中文", English: "English",
	Timeout: "Default timeout (seconds)", TimeoutDescription: "Maximum wait for requests without their own timeout.",
	FollowRedirects: "Follow redirects", FollowRedirectsDescription: "Automatically follow server redirects.",
	VerifyTLS: "Verify TLS certificates", VerifyTLSDescription: "Validate the HTTPS server certificate.", TLSWarning: "Certificate verification is off. HTTPS connections may be unsafe.",
	HistoryLimit: "History limit", HistoryDescription: "Number of recent requests to keep.",
	DataDirectory: "Data directory", OpenFolder: "Open folder", About: "About", Close: "Close",
	CheckUpdates: "Check for updates", CheckingUpdates: "Checking for updates…", UpdateAvailable: "New version available", OpenRelease: "Open download page",
	UpToDate: "You're up to date", NoRelease: "No published release yet", UpdateFailed: "Update check failed",
}
