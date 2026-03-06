package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// RegisterCommands sets the Telegram command menu shown near the input box.
func RegisterCommands(api *tgbotapi.BotAPI) error {
	commands := []tgbotapi.BotCommand{
		{Command: "menu", Description: "打开主菜单"},
		{Command: "status", Description: "查看个人设置"},
		{Command: "quota", Description: "查看本月额度"},
		{Command: "langs", Description: "切换目标语种"},
		{Command: "auto_on", Description: "开启自动模式"},
		{Command: "auto_off", Description: "关闭自动模式"},
		{Command: "help", Description: "查看帮助"},
	}

	config := tgbotapi.NewSetMyCommandsWithScope(tgbotapi.NewBotCommandScopeAllPrivateChats(), commands...)
	_, err := api.Request(config)
	return err
}
