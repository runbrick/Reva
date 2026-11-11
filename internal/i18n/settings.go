package i18n

// Settings holds the strings of the settings dialog. Owned by that feature's PR.
type Settings struct {
	Title         string
	Theme         string
	Light         string
	Dark          string
	Language      string
	Chinese       string
	English       string
	HistoryLimit  string
	DataDirectory string
	OpenFolder    string
	Close         string
}

var settingsZh = Settings{
	Title:         "设置",
	Theme:         "主题",
	Light:         "浅色",
	Dark:          "深色",
	Language:      "语言",
	Chinese:       "中文",
	English:       "English",
	HistoryLimit:  "历史记录上限",
	DataDirectory: "数据目录",
	OpenFolder:    "打开文件夹",
	Close:         "关闭",
}

var settingsEn = Settings{
	Title:         "Settings",
	Theme:         "Theme",
	Light:         "Light",
	Dark:          "Dark",
	Language:      "Language",
	Chinese:       "中文",
	English:       "English",
	HistoryLimit:  "History limit",
	DataDirectory: "Data directory",
	OpenFolder:    "Open folder",
	Close:         "Close",
}
