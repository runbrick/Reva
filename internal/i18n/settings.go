package i18n

// Settings holds every label and description on the settings page.
type Settings struct {
	Title, Appearance, Requests, History, Data                          string
	Theme, ThemeDescription, Light, Dark, System                        string
	Language, LanguageDescription, Chinese, English                     string
	Timeout, TimeoutDescription                                         string
	FollowRedirects, FollowRedirectsDescription                         string
	VerifyTLS, VerifyTLSDescription, TLSWarning                         string
	HistoryLimit, HistoryDescription                                    string
	DataDirectory, OpenFolder, About, Close                             string
	CheckUpdates, CheckingUpdates, UpdateAvailable, OpenRelease         string
	UpToDate, NoRelease, UpdateFailed                                   string
	BackupSync, BackupFile, BackupDescription, Export, Restore          string
	Sync, Server, ServerDescription, WebDAV, AutoSync                   string
	AutoSyncDescription, SyncNow, RestoreRemote, Configure              string
	Address, Username, RemoteFile, ServerPassword, BackupPassword       string
	Save, Cancel, Working, SyncDone, BackupDone, RestoreDone            string
	NotConfigured, PasswordHint, ConfirmRestore, RestoreWarning         string
	SyncError, FileType, FileError, ShortPassword, LocalChanges         string
	S3, GitHub, Gitee, Endpoint, Region, Bucket, ObjectKey              string
	AccessKey, SecretKey, SnippetID, SnippetFile, APIAddress, PathStyle string
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
	BackupSync: "备份与同步", BackupFile: "备份文件", BackupDescription: "用密码加密导出收藏夹、环境、设置和历史记录。",
	Export: "导出", Restore: "恢复", Sync: "同步", Server: "服务器", ServerDescription: "将加密备份存放在远端服务器。",
	WebDAV: "WebDAV", AutoSync: "自动同步", AutoSyncDescription: "本次会话输入密码后，自动同步本机已保存的变更。",
	SyncNow: "立即同步", RestoreRemote: "从远端恢复", Configure: "配置服务器", Address: "服务器地址", Username: "用户名",
	RemoteFile: "远端文件", ServerPassword: "服务器密码", BackupPassword: "备份密码", Save: "保存", Cancel: "取消", Working: "正在处理…",
	SyncDone: "同步完成", BackupDone: "备份已导出", RestoreDone: "数据已恢复", NotConfigured: "尚未配置服务器",
	PasswordHint: "密码至少 8 个字符；每次启动应用需重新输入密码。", ConfirmRestore: "确认从远端恢复",
	RestoreWarning: "将用远端数据替换本机数据。会先在数据目录保存一份加密安全副本。",
	SyncError:      "同步失败", FileType: "Reqo 加密备份", FileError: "文件操作失败",
	ShortPassword: "备份密码至少需要 8 个字符", LocalChanges: "操作期间本机数据发生变化，请再次同步",
	S3: "S3", GitHub: "GitHub Gist", Gitee: "Gitee", Endpoint: "端点", Region: "区域", Bucket: "存储桶", ObjectKey: "对象路径",
	AccessKey: "访问密钥 ID", SecretKey: "访问密钥", SnippetID: "代码片段 ID", SnippetFile: "代码片段文件", APIAddress: "API 地址", PathStyle: "路径式存储桶",
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
	BackupSync: "Backup & sync", BackupFile: "Backup file", BackupDescription: "Export collections, environments, settings and history in an encrypted file.",
	Export: "Export", Restore: "Restore", Sync: "Sync", Server: "Server", ServerDescription: "Store the encrypted backup on a remote server.",
	WebDAV: "WebDAV", AutoSync: "Automatic sync", AutoSyncDescription: "Sync saved changes automatically after entering passwords for this session.",
	SyncNow: "Sync now", RestoreRemote: "Restore from remote", Configure: "Configure server", Address: "Server address", Username: "Username",
	RemoteFile: "Remote file", ServerPassword: "Server password", BackupPassword: "Backup password", Save: "Save", Cancel: "Cancel", Working: "Working…",
	SyncDone: "Sync complete", BackupDone: "Backup exported", RestoreDone: "Data restored", NotConfigured: "Server not configured",
	PasswordHint: "Use at least 8 characters. Enter passwords again after restarting the app.", ConfirmRestore: "Confirm remote restore",
	RestoreWarning: "Remote data will replace local data. An encrypted safety copy will be saved in the data directory first.",
	SyncError:      "Sync failed", FileType: "Reqo encrypted backup", FileError: "File operation failed",
	ShortPassword: "Backup password needs at least 8 characters", LocalChanges: "Local data changed during the operation; sync again",
	S3: "S3", GitHub: "GitHub Gist", Gitee: "Gitee", Endpoint: "Endpoint", Region: "Region", Bucket: "Bucket", ObjectKey: "Object key",
	AccessKey: "Access key ID", SecretKey: "Secret key", SnippetID: "Snippet ID", SnippetFile: "Snippet file", APIAddress: "API address", PathStyle: "Path-style bucket",
}
