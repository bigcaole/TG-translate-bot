package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func mainMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("切换语种", "menu:lang"),
			tgbotapi.NewInlineKeyboardButtonData("自动模式", "auto:toggle"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("本月额度", "quota:view"),
			tgbotapi.NewInlineKeyboardButtonData("个人设置", "settings:view"),
		),
	)
}

func languageKeyboard() tgbotapi.InlineKeyboardMarkup {
	buttons := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("英语", "lang:en"),
		tgbotapi.NewInlineKeyboardButtonData("俄语", "lang:ru"),
		tgbotapi.NewInlineKeyboardButtonData("法语", "lang:fr"),
		tgbotapi.NewInlineKeyboardButtonData("德语", "lang:de"),
		tgbotapi.NewInlineKeyboardButtonData("意大利语", "lang:it"),
		tgbotapi.NewInlineKeyboardButtonData("日语", "lang:ja"),
		tgbotapi.NewInlineKeyboardButtonData("韩语", "lang:ko"),
		tgbotapi.NewInlineKeyboardButtonData("泰语", "lang:th"),
		tgbotapi.NewInlineKeyboardButtonData("越南语", "lang:vi"),
	}

	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 4)
	for i := 0; i < len(buttons); i += 3 {
		end := i + 3
		if end > len(buttons) {
			end = len(buttons)
		}
		rows = append(rows, buttons[i:end])
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "menu:main"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func settingsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("切换机器人开关", "bot:toggle"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回主菜单", "menu:main"),
		),
	)
}
