// Package i18n holds every user-visible string in Reqo, in Chinese (the
// default) and English, as typed struct fields so a missing translation is a
// compile error rather than a blank label.
//
// Each feature owns one section struct in its own file (request.go,
// response.go, ...). To add a string: add a field to the section struct and
// set it in both zh and en values in that same file. Do not edit other
// features' section files.
package i18n

// Lang is a language code as stored in settings.
type Lang string

const (
	Zh Lang = "zh"
	En Lang = "en"
)

// Langs lists the supported languages in display order.
var Langs = []Lang{Zh, En}

// Strings is the complete string table for one language.
type Strings struct {
	Lang        Lang
	Common      Common
	Request     Request
	Response    Response
	History     History
	Collections Collections
	Env         Env
	Import      Import
	Settings    Settings
	Tabs        Tabs
	Scripts     Scripts
	Shell       Shell
	Titlebar    Titlebar
	Search      Search
	WebSocket   WebSocket
}

// For returns the string table for lang, falling back to Chinese.
func For(lang Lang) *Strings {
	if lang == En {
		return &en
	}
	return &zh
}

var zh = Strings{
	Lang:        Zh,
	Common:      commonZh,
	Request:     requestZh,
	Response:    responseZh,
	History:     historyZh,
	Collections: collectionsZh,
	Env:         envZh,
	Import:      importZh,
	Settings:    settingsZh,
	Tabs:        tabsZh,
	Scripts:     scriptsZh,
	Shell:       shellZh,
	Titlebar:    titlebarZh,
	Search:      searchZh,
	WebSocket:   websocketZh,
}

var en = Strings{
	Lang:        En,
	Common:      commonEn,
	Request:     requestEn,
	Response:    responseEn,
	History:     historyEn,
	Collections: collectionsEn,
	Env:         envEn,
	Import:      importEn,
	Settings:    settingsEn,
	Tabs:        tabsEn,
	Scripts:     scriptsEn,
	Shell:       shellEn,
	Titlebar:    titlebarEn,
	Search:      searchEn,
	WebSocket:   websocketEn,
}
