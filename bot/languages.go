package bot

var supportedLanguages = []string{"en", "ru", "fr", "de", "it", "ja", "ko", "th", "vi"}

var languageNamesCN = map[string]string{
	"en": "英语",
	"ru": "俄语",
	"fr": "法语",
	"de": "德语",
	"it": "意大利语",
	"ja": "日语",
	"ko": "韩语",
	"th": "泰语",
	"vi": "越南语",
	"zh": "中文",
}

func isSupportedLanguage(lang string) bool {
	for _, item := range supportedLanguages {
		if item == lang {
			return true
		}
	}
	return false
}
