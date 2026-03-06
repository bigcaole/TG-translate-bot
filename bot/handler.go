package bot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tg-translate-bot/cache"
	"tg-translate-bot/config"
	"tg-translate-bot/database"
	"tg-translate-bot/quota"
	"tg-translate-bot/translator"
)

// Handler handles Telegram updates and business logic.
type Handler struct {
	api    *tgbotapi.BotAPI
	cfg    *config.Config
	store  *database.Store
	cache  *cache.Client
	quota  *quota.Manager
	trans  *translator.Client
	logger *log.Logger
}

func NewHandler(
	api *tgbotapi.BotAPI,
	cfg *config.Config,
	store *database.Store,
	cacheClient *cache.Client,
	quotaManager *quota.Manager,
	translatorClient *translator.Client,
	logger *log.Logger,
) *Handler {
	return &Handler{
		api:    api,
		cfg:    cfg,
		store:  store,
		cache:  cacheClient,
		quota:  quotaManager,
		trans:  translatorClient,
		logger: logger,
	}
}

func (h *Handler) Run(ctx context.Context) error {
	updateCfg := tgbotapi.NewUpdate(0)
	updateCfg.Timeout = 60
	updates := h.api.GetUpdatesChan(updateCfg)

	for {
		select {
		case <-ctx.Done():
			h.api.StopReceivingUpdates()
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message != nil {
				msg := update.Message
				go h.safeGo(func() {
					h.handleMessage(ctx, msg)
				})
			}
			if update.CallbackQuery != nil {
				cb := update.CallbackQuery
				go h.safeGo(func() {
					h.handleCallback(ctx, cb)
				})
			}
		}
	}
}

func (h *Handler) safeGo(fn func()) {
	defer func() {
		if rec := recover(); rec != nil {
			h.logger.Printf("goroutine panic: %v", rec)
		}
	}()
	fn()
}

func (h *Handler) handleMessage(parent context.Context, msg *tgbotapi.Message) {
	if msg == nil || msg.From == nil {
		return
	}

	if !h.isAllowed(msg.From.ID) {
		h.sendText(msg.Chat.ID, "你没有权限使用该机器人。", mainMenuKeyboard())
		return
	}

	if msg.IsCommand() {
		h.handleCommand(parent, msg)
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	h.handleTranslation(parent, msg.Chat.ID, msg.From.ID, text)
}

func (h *Handler) handleCommand(parent context.Context, msg *tgbotapi.Message) {
	cmd := strings.ToLower(msg.Command())
	args := strings.TrimSpace(msg.CommandArguments())

	switch cmd {
	case "start", "menu":
		h.sendText(msg.Chat.ID, "欢迎使用翻译机器人。请通过下方菜单设置语种和模式。", mainMenuKeyboard())
	case "help":
		help := "可用命令：\n" +
			"/start - 打开主菜单\n" +
			"/menu - 打开主菜单\n" +
			"/set <语种代码> - 设定模式目标语种（如 /set ja）\n" +
			"/auto on|off - 开启/关闭自动模式\n" +
			"/status - 查看个人设置与额度"
		h.sendText(msg.Chat.ID, help, mainMenuKeyboard())
	case "set":
		h.handleSetLanguage(parent, msg.Chat.ID, msg.From.ID, args)
	case "auto":
		h.handleAutoCommand(parent, msg.Chat.ID, msg.From.ID, args)
	case "status":
		h.sendStatus(parent, msg.Chat.ID, msg.From.ID)
	default:
		h.sendText(msg.Chat.ID, "未识别的命令。请使用 /help 查看说明。", mainMenuKeyboard())
	}
}

func (h *Handler) handleSetLanguage(parent context.Context, chatID int64, userID int64, args string) {
	lang := normalizeLang(args)
	if !isSupportedLanguage(lang) {
		h.sendText(chatID, "语种不支持。支持：en, ru, fr, de, it, ja, ko, th, vi", languageKeyboard())
		return
	}

	ctx, cancel := context.WithTimeout(parent, h.cfg.RequestTimeout)
	defer cancel()

	if err := h.store.UpdateTargetLanguage(ctx, userID, lang); err != nil {
		h.logger.Printf("update target language failed: %v", err)
		h.sendText(chatID, "设置失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	h.sendText(chatID, fmt.Sprintf("已切换目标语种为%s，并关闭自动模式。", languageNameCN(lang)), mainMenuKeyboard())
}

func (h *Handler) handleAutoCommand(parent context.Context, chatID int64, userID int64, args string) {
	arg := strings.ToLower(strings.TrimSpace(args))
	if arg != "on" && arg != "off" {
		h.sendText(chatID, "用法：/auto on 或 /auto off", mainMenuKeyboard())
		return
	}
	enabled := arg == "on"

	ctx, cancel := context.WithTimeout(parent, h.cfg.RequestTimeout)
	defer cancel()

	if err := h.store.SetAutoMode(ctx, userID, enabled, h.cfg.DefaultTargetLanguage); err != nil {
		h.logger.Printf("set auto mode failed: %v", err)
		h.sendText(chatID, "设置失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	if enabled {
		h.sendText(chatID, "自动模式已开启。非中文消息将自动翻译为简体中文。", mainMenuKeyboard())
		return
	}
	h.sendText(chatID, "自动模式已关闭。将按设定模式翻译。", mainMenuKeyboard())
}

func (h *Handler) sendStatus(parent context.Context, chatID int64, userID int64) {
	ctx, cancel := context.WithTimeout(parent, h.cfg.RequestTimeout)
	defer cancel()

	settings, err := h.store.GetOrCreateUserSettings(ctx, userID, h.cfg.DefaultTargetLanguage)
	if err != nil {
		h.logger.Printf("get settings failed: %v", err)
		h.sendText(chatID, "读取设置失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	usage, err := h.quota.Usage(ctx, time.Now())
	if err != nil {
		h.logger.Printf("get usage failed: %v", err)
		usage = 0
	}

	status := fmt.Sprintf(
		"个人设置\n模式：%s\n目标语种：%s\n机器人状态：%s\n\n本月已用：%d / %d 字符",
		modeName(settings.AutoMode),
		languageNameCN(settings.TargetLanguage),
		onOffName(settings.BotEnabled),
		usage,
		quota.FreeQuota,
	)
	h.sendText(chatID, status, settingsKeyboard())
}

func (h *Handler) handleCallback(parent context.Context, cb *tgbotapi.CallbackQuery) {
	if cb == nil || cb.From == nil {
		return
	}
	if !h.isAllowed(cb.From.ID) {
		h.answerCallback(cb.ID, "无权限")
		if cb.Message != nil {
			h.sendText(cb.Message.Chat.ID, "你没有权限使用该机器人。", mainMenuKeyboard())
		}
		return
	}
	if cb.Message == nil {
		h.answerCallback(cb.ID, "无效请求")
		return
	}

	chatID := cb.Message.Chat.ID
	userID := cb.From.ID
	data := cb.Data

	switch {
	case data == "menu:main":
		h.answerCallback(cb.ID, "已返回主菜单")
		h.sendText(chatID, "主菜单", mainMenuKeyboard())
	case data == "menu:lang":
		h.answerCallback(cb.ID, "请选择语种")
		h.sendText(chatID, "请选择设定模式下的目标语种：", languageKeyboard())
	case strings.HasPrefix(data, "lang:"):
		lang := normalizeLang(strings.TrimPrefix(data, "lang:"))
		h.answerCallback(cb.ID, "语种已更新")
		h.handleSetLanguage(parent, chatID, userID, lang)
	case data == "auto:toggle":
		h.answerCallback(cb.ID, "自动模式已切换")
		h.toggleAutoMode(parent, chatID, userID)
	case data == "quota:view":
		h.answerCallback(cb.ID, "正在读取额度")
		h.sendQuota(parent, chatID)
	case data == "settings:view":
		h.answerCallback(cb.ID, "正在读取设置")
		h.sendStatus(parent, chatID, userID)
	case data == "bot:toggle":
		h.answerCallback(cb.ID, "机器人开关已切换")
		h.toggleBotEnabled(parent, chatID, userID)
	default:
		h.answerCallback(cb.ID, "未知操作")
	}
}

func (h *Handler) toggleAutoMode(parent context.Context, chatID int64, userID int64) {
	ctx, cancel := context.WithTimeout(parent, h.cfg.RequestTimeout)
	defer cancel()

	settings, err := h.store.GetOrCreateUserSettings(ctx, userID, h.cfg.DefaultTargetLanguage)
	if err != nil {
		h.logger.Printf("get settings for toggle auto failed: %v", err)
		h.sendText(chatID, "读取设置失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	next := !settings.AutoMode
	if err := h.store.SetAutoMode(ctx, userID, next, h.cfg.DefaultTargetLanguage); err != nil {
		h.logger.Printf("toggle auto mode failed: %v", err)
		h.sendText(chatID, "切换失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	if next {
		h.sendText(chatID, "自动模式已开启。", mainMenuKeyboard())
		return
	}
	h.sendText(chatID, "自动模式已关闭。", mainMenuKeyboard())
}

func (h *Handler) toggleBotEnabled(parent context.Context, chatID int64, userID int64) {
	ctx, cancel := context.WithTimeout(parent, h.cfg.RequestTimeout)
	defer cancel()

	settings, err := h.store.GetOrCreateUserSettings(ctx, userID, h.cfg.DefaultTargetLanguage)
	if err != nil {
		h.logger.Printf("get settings for toggle bot failed: %v", err)
		h.sendText(chatID, "读取设置失败，请稍后重试。", settingsKeyboard())
		return
	}

	next := !settings.BotEnabled
	if err := h.store.SetBotEnabled(ctx, userID, next, h.cfg.DefaultTargetLanguage); err != nil {
		h.logger.Printf("toggle bot enabled failed: %v", err)
		h.sendText(chatID, "切换失败，请稍后重试。", settingsKeyboard())
		return
	}

	h.sendText(chatID, fmt.Sprintf("机器人状态已切换为：%s", onOffName(next)), settingsKeyboard())
}

func (h *Handler) sendQuota(parent context.Context, chatID int64) {
	ctx, cancel := context.WithTimeout(parent, h.cfg.RequestTimeout)
	defer cancel()

	usage, err := h.quota.Usage(ctx, time.Now())
	if err != nil {
		h.logger.Printf("quota usage failed: %v", err)
		h.sendText(chatID, "读取额度失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	ratio := float64(usage) / float64(quota.FreeQuota) * 100
	text := fmt.Sprintf("本月已用额度：%d / %d 字符（%.2f%%）", usage, quota.FreeQuota, ratio)
	h.sendText(chatID, text, mainMenuKeyboard())
}

func (h *Handler) handleTranslation(parent context.Context, chatID int64, userID int64, text string) {
	ctx, cancel := context.WithTimeout(parent, h.cfg.RequestTimeout)
	defer cancel()

	settings, err := h.store.GetOrCreateUserSettings(ctx, userID, h.cfg.DefaultTargetLanguage)
	if err != nil {
		h.logger.Printf("load settings failed: %v", err)
		h.sendText(chatID, "读取用户设置失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	if !settings.BotEnabled {
		h.sendText(chatID, "机器人已关闭，请在“个人设置”中开启。", settingsKeyboard())
		return
	}

	if open, err := h.quota.IsCircuitOpen(ctx, time.Now()); err == nil && open {
		h.sendText(chatID, "本月额度已耗尽，翻译服务已暂停，将于下月自动恢复。", mainMenuKeyboard())
		return
	}

	detected, err := h.trans.DetectLanguage(ctx, text)
	if err != nil {
		h.logger.Printf("detect language failed: %v", err)
		h.sendText(chatID, "语种识别失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	sourceLang, targetLang, needTranslate, hint := chooseDirection(settings, detected)
	if !needTranslate {
		h.sendText(chatID, hint, mainMenuKeyboard())
		return
	}

	cacheKey := buildCacheKey(sourceLang, targetLang, text)
	cachedValue, hit, err := h.cache.GetTranslation(ctx, cacheKey)
	if err != nil {
		h.logger.Printf("get cache failed: %v", err)
	}
	if hit {
		h.sendText(chatID, formatTranslation(sourceLang, targetLang, cachedValue, true), mainMenuKeyboard())
		return
	}

	chars := int64(len([]rune(text)))
	usage, warned, cutoffTriggered, circuitOpen, err := h.quota.Consume(ctx, chars, time.Now())
	if err != nil {
		h.logger.Printf("quota consume failed: %v", err)
		h.sendText(chatID, "额度计数失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	if warned {
		h.notifyAdmins(fmt.Sprintf("【额度预警】本月已使用 %d/%d 字符（达到80%%）。", usage, quota.FreeQuota))
	}
	if cutoffTriggered {
		h.notifyAdmins(fmt.Sprintf("【额度熔断】本月已使用 %d/%d 字符，翻译服务已自动暂停。", usage, quota.FreeQuota))
	}
	if circuitOpen {
		h.sendText(chatID, "本月额度已耗尽，翻译服务已暂停，将于下月自动恢复。", mainMenuKeyboard())
		return
	}

	translated, err := h.trans.TranslateText(ctx, text, sourceLang, targetLang)
	if err != nil {
		h.logger.Printf("translate text failed: %v", err)
		h.sendText(chatID, "翻译失败，请稍后重试。", mainMenuKeyboard())
		return
	}

	if err := h.cache.SetTranslation(ctx, cacheKey, translated, h.cfg.CacheTTL); err != nil {
		h.logger.Printf("set cache failed: %v", err)
	}

	h.sendText(chatID, formatTranslation(sourceLang, targetLang, translated, false), mainMenuKeyboard())
}

func (h *Handler) isAllowed(userID int64) bool {
	_, ok := h.cfg.AllowedUsers[userID]
	return ok
}

func (h *Handler) notifyAdmins(message string) {
	for _, adminID := range h.cfg.AdminUsers {
		h.sendText(adminID, message, mainMenuKeyboard())
	}
}

func (h *Handler) sendText(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	if _, err := h.api.Send(msg); err != nil {
		h.logger.Printf("send message failed: %v", err)
	}
}

func (h *Handler) answerCallback(callbackID, text string) {
	cb := tgbotapi.NewCallback(callbackID, text)
	if _, err := h.api.Request(cb); err != nil {
		h.logger.Printf("answer callback failed: %v", err)
	}
}

func chooseDirection(settings *database.UserSettings, detected string) (sourceLang string, targetLang string, needTranslate bool, hint string) {
	detectedNorm := normalizeLang(detected)
	if settings.AutoMode {
		if isChinese(detectedNorm) {
			return "", "", false, "自动模式下仅翻译非中文内容。"
		}
		return detectedNorm, "zh-CN", true, ""
	}

	target := normalizeLang(settings.TargetLanguage)
	if isChinese(detectedNorm) {
		return "zh-CN", target, true, ""
	}
	if sameBaseLang(detectedNorm, target) {
		return detectedNorm, "zh-CN", true, ""
	}
	return detectedNorm, "zh-CN", true, ""
}

func buildCacheKey(sourceLang, targetLang, text string) string {
	raw := sourceLang + "|" + targetLang + "|" + text
	sum := sha1.Sum([]byte(raw))
	return "tr:" + hex.EncodeToString(sum[:])
}

func formatTranslation(sourceLang, targetLang, translated string, fromCache bool) string {
	result := fmt.Sprintf("翻译结果（%s -> %s）\n%s", languageNameCN(sourceLang), languageNameCN(targetLang), translated)
	if fromCache {
		result += "\n\n（命中缓存）"
	}
	return result
}

func modeName(auto bool) string {
	if auto {
		return "自动模式"
	}
	return "设定模式"
}

func onOffName(enabled bool) string {
	if enabled {
		return "开启"
	}
	return "关闭"
}

func languageNameCN(code string) string {
	normalized := normalizeLang(code)
	if isChinese(normalized) {
		return "中文"
	}
	if name, ok := languageNamesCN[normalized]; ok {
		return name
	}
	if name, ok := languageNamesCN[strings.ToLower(code)]; ok {
		return name
	}
	return strings.ToUpper(code)
}

func normalizeLang(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	lang = strings.ReplaceAll(lang, "_", "-")
	if lang == "" {
		return ""
	}
	parts := strings.Split(lang, "-")
	return parts[0]
}

func sameBaseLang(a, b string) bool {
	return normalizeLang(a) == normalizeLang(b)
}

func isChinese(lang string) bool {
	base := normalizeLang(lang)
	return base == "zh"
}
