package i18n

type Toolbox struct {
	Title, Encoding, Crypto, Views, Other                          string
	JSONEscape, Unicode, Hash, AES, Image, Color, Timestamp, Regex string
	Input, Output, Clear, Copy, Copied, CopyError, Working         string
	Encode, Decode, Encrypt, Decrypt, Run, Inspect, Generate       string
	Standard, Component, Form, Format, Compact, Seconds, Millis    string
	HMACKey, AESKey, Pattern, ImageFile                            string
	URLHint, JWTHint, JSONEscapeHint, UnicodeHint, HashHint        string
	AESHint, XMLHint, ImageHint, ColorHint, TimeHint, RegexHint    string
	Invalid, Limit, KeyError, BadCipher, Binary, NoMatch, BadImage string
}

var toolboxZh = Toolbox{
	Title: "工具箱", Encoding: "编解码", Crypto: "加解密", Views: "视图", Other: "其他",
	JSONEscape: "JSON 转义", Unicode: "Unicode", Hash: "哈希", AES: "AES", Image: "图片", Color: "色彩", Timestamp: "时间戳", Regex: "正则表达式",
	Input: "输入", Output: "结果", Clear: "清空", Copy: "复制结果", Copied: "已复制结果", CopyError: "无法访问剪贴板", Working: "处理中…",
	Encode: "编码", Decode: "解码", Encrypt: "加密", Decrypt: "解密", Run: "运行", Inspect: "查看", Generate: "生成",
	Standard: "标准", Component: "URL 组件", Form: "表单参数", Format: "格式化", Compact: "压缩", Seconds: "秒", Millis: "毫秒",
	HMACKey: "文本密钥", AESKey: "十六进制密钥", Pattern: "匹配模式", ImageFile: "打开图片…",
	URLHint: "单个参数名或参数值", JWTHint: "仅解码，未验证签名。", JSONEscapeHint: "JSON 字符串或转义文本", UnicodeHint: `例如 \u4f60\u597d`, HashHint: "MD5 和 SHA-1 仅适用于旧系统兼容。",
	AESHint: "AES-GCM · 16、24 或 32 字节密钥 · Base64（随机 nonce + 密文 + 认证标签）", XMLHint: "UTF-8 XML（不支持 DTD）", ImageHint: "Base64 或图片 data URL · PNG、JPEG、GIF", ColorHint: "#RGB、#RGBA、#RRGGBB 或 #RRGGBBAA", TimeHint: "Unix 时间戳或 RFC3339 时间；留空使用当前时间", RegexHint: "Go / RE2 · UTF-8 字节位置 · 最多 100 个匹配",
	Invalid: "输入格式无效。", Limit: "输入或结果超出大小或复杂度限制。", KeyError: "密钥无效，请检查格式或长度。", BadCipher: "认证失败：密钥错误或密文已损坏。", Binary: "解码结果不是 UTF-8 文本。", NoMatch: "没有匹配项。", BadImage: "无法预览图片：文件不可读、格式不支持或尺寸过大。",
}

var toolboxEn = Toolbox{
	Title: "Toolbox", Encoding: "Encoding", Crypto: "Cryptography", Views: "Viewers", Other: "Other",
	JSONEscape: "JSON Escape", Unicode: "Unicode", Hash: "Hash", AES: "AES", Image: "Image", Color: "Color", Timestamp: "Timestamp", Regex: "Regex",
	Input: "Input", Output: "Result", Clear: "Clear", Copy: "Copy result", Copied: "Result copied", CopyError: "Could not access the clipboard", Working: "Working…",
	Encode: "Encode", Decode: "Decode", Encrypt: "Encrypt", Decrypt: "Decrypt", Run: "Run", Inspect: "Inspect", Generate: "Generate",
	Standard: "Standard", Component: "URL component", Form: "Form parameter", Format: "Format", Compact: "Minify", Seconds: "Seconds", Millis: "Milliseconds",
	HMACKey: "Text key", AESKey: "Hex key", Pattern: "Pattern", ImageFile: "Open image…",
	URLHint: "A single parameter name or value", JWTHint: "Decode only. Signature not verified.", JSONEscapeHint: "JSON string or escaped text", UnicodeHint: `e.g. \u4f60\u597d`, HashHint: "MD5 and SHA-1 are for legacy compatibility only.",
	AESHint: "AES-GCM · 16, 24 or 32-byte key · Base64 (random nonce + ciphertext + tag)", XMLHint: "UTF-8 XML (no DTD)", ImageHint: "Base64 or an image data URL · PNG, JPEG, GIF", ColorHint: "#RGB, #RGBA, #RRGGBB or #RRGGBBAA", TimeHint: "Unix timestamp or RFC3339; leave blank for the current time", RegexHint: "Go / RE2 · UTF-8 byte offsets · Up to 100 matches",
	Invalid: "Invalid input format.", Limit: "Input or result exceeds size or complexity limits.", KeyError: "Invalid key. Check its format and length.", BadCipher: "Authentication failed: wrong key or altered ciphertext.", Binary: "The decoded result is not UTF-8 text.", NoMatch: "No matches.", BadImage: "Cannot preview this image: unreadable, unsupported or too large.",
}
