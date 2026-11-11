package i18n

// Tabs holds the strings of the request tabs bar. Owned by that feature's PR.
type Tabs struct {
	Title          string
	New            string
	NewSymbol      string
	Close          string
	CloseSymbol    string
	CloseOthers    string
	Duplicate      string
	DirtySymbol    string
	ConfirmTitle   string
	ConfirmDiscard string
	Discard        string
	Cancel         string
}

var tabsZh = Tabs{
	Title: "新标签页", New: "新建标签页", NewSymbol: "+",
	Close: "关闭", CloseSymbol: "×", CloseOthers: "关闭其他标签页", Duplicate: "复制标签页",
	DirtySymbol: "●", ConfirmTitle: "放弃未保存的更改？",
	ConfirmDiscard: "此标签页有未保存的更改。关闭后将丢失这些更改。",
	Discard:        "放弃更改", Cancel: "取消",
}

var tabsEn = Tabs{
	Title: "New tab", New: "New tab", NewSymbol: "+",
	Close: "Close", CloseSymbol: "×", CloseOthers: "Close others", Duplicate: "Duplicate",
	DirtySymbol: "●", ConfirmTitle: "Discard unsaved changes?",
	ConfirmDiscard: "This tab has unsaved changes. Closing it will discard them.",
	Discard:        "Discard", Cancel: "Cancel",
}
