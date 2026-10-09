package bot

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// lang is the language of the private-chat promo post and /help. Telegram's
// language_code is only a hint, so anything not Russian gets Uzbek (Latin),
// the site's default locale.
type lang int

const (
	langUz lang = iota
	langRu
)

func langOf(code string) lang {
	if strings.HasPrefix(strings.ToLower(code), "ru") {
		return langRu
	}
	return langUz
}

// appLocale is the site locale segment a language opens in.
func (l lang) appLocale() string {
	if l == langRu {
		return "ru"
	}
	return "uz-Latn"
}

// bannerPath is the promo image the frontend serves from public/bot/, one
// per language since the banner carries copy. The version query lets a
// redesigned banner bypass Telegram's cache of the URL.
func (l lang) bannerPath() string {
	if l == langRu {
		return "/bot/start-banner-ru.jpg?v=1"
	}
	return "/bot/start-banner.jpg?v=1"
}

// defaultSiteURL backs an unset PUBLIC_BASE_URL (config defaults it, so this
// only guards hand-built Bots).
const defaultSiteURL = "https://drivergo.uz"

// startNameMaxRunes keeps a pathological first name from pushing the caption
// past Telegram's 1024-character limit.
const startNameMaxRunes = 40

// The catalog numbers below are the live prod catalog when this copy was
// written (GET /api/v1/variants: 64 tickets, 1277 questions; 285 signs) and
// the official exam rule (20 questions, 25 minutes). "1270+" stays true as
// questions are added; the ticket count needs a touch when a ticket is.
const (
	captionStartUzFmt = "👋 Salom, <b>%s</b>!\n\n" +
		"<b>Driver Go</b> — haydovchilik imtihoniga rasmiy formatda tayyorlanish.\n\n" +
		"🎫 64 bilet, 1270+ rasmiy savol\n" +
		"📝 Imtihon simulyatori: 20 savol, 25 daqiqa\n" +
		"🚦 285 yo'l belgisi — izohlari bilan\n" +
		"🎯 Xatolar ustida ishlash — imtihonda takrorlanmasin\n\n" +
		"🎁 Ro'yxatdan o'ting — <b>24 soat bepul VIP</b>!\n" +
		"👇 «Driver Go'ni ochish» tugmasini bosing."
	captionBackUzFmt = "👋 Xush kelibsiz, <b>%s</b>! Yana qaytganingizdan xursandmiz.\n\n" +
		"Bugun bitta bilet yeching — imtihon kuni o'zingizga rahmat aytasiz 💪\n\n" +
		"🎫 Biletlar — 64 ta, rasmiy savollar\n" +
		"📝 Imtihon — 20 savol, 25 daqiqa\n" +
		"🎯 Xatolar — zaif joylarni mustahkamlang\n" +
		"📊 VIP va streak: /status\n\n" +
		"👇 Davom etish uchun «Driver Go'ni ochish» ni bosing."
	captionStartRuFmt = "👋 Привет, <b>%s</b>!\n\n" +
		"<b>Driver Go</b> — подготовка к экзамену на права в официальном формате.\n\n" +
		"🎫 64 билета, 1270+ официальных вопросов\n" +
		"📝 Симулятор экзамена: 20 вопросов, 25 минут\n" +
		"🚦 285 дорожных знаков с пояснениями\n" +
		"🎯 Работа над ошибками — чтобы не повторить их на экзамене\n\n" +
		"🎁 Зарегистрируйтесь — <b>24 часа VIP бесплатно</b>!\n" +
		"👇 Нажмите «Открыть Driver Go»."
	captionBackRuFmt = "👋 С возвращением, <b>%s</b>! Рады видеть вас снова.\n\n" +
		"Решите сегодня хотя бы один билет — в день экзамена скажете себе спасибо 💪\n\n" +
		"🎫 Билеты — 64, официальные вопросы\n" +
		"📝 Экзамен — 20 вопросов, 25 минут\n" +
		"🎯 Ошибки — подтяните слабые темы\n" +
		"📊 VIP и серия: /status\n\n" +
		"👇 Чтобы продолжить, нажмите «Открыть Driver Go»."

	helpUz = "<b>Driver Go — nima qayerda?</b>\n\n" +
		"🎫 <b>Biletlar</b> — 64 ta rasmiy bilet\n" +
		"📝 <b>Imtihon</b> — 20 savol, 25 daqiqa, haqiqiy qoidalar\n" +
		"🎯 <b>Mashq</b> — mavzular va xatolar ustida ishlash\n" +
		"🚦 <b>Yo'l belgilari</b> — 285 belgi izohlari bilan\n" +
		"👑 <b>VIP</b> — barcha biletlar va imkoniyatlar\n" +
		"💬 <b>Yordam</b> — qo'llab-quvvatlash bilan chat\n\n" +
		"<b>Buyruqlar</b>\n" +
		"/start — bosh menyu\n" +
		"/quiz — shu yerda tezkor test\n" +
		"/status — VIP va streak holati\n" +
		"/eslatma — kunlik «Kun savoli» (19:00) yoqish/o'chirish\n" +
		"/unlink — Telegramni hisobdan uzish\n\n" +
		"👥 Guruhda: botni qo'shing va /quiz yozing."
	helpRu = "<b>Driver Go — что где?</b>\n\n" +
		"🎫 <b>Билеты</b> — 64 официальных билета\n" +
		"📝 <b>Экзамен</b> — 20 вопросов, 25 минут, настоящие правила\n" +
		"🎯 <b>Практика</b> — по темам и работа над ошибками\n" +
		"🚦 <b>Дорожные знаки</b> — 285 знаков с пояснениями\n" +
		"👑 <b>VIP</b> — все билеты и возможности\n" +
		"💬 <b>Поддержка</b> — чат с поддержкой\n\n" +
		"<b>Команды</b>\n" +
		"/start — главное меню\n" +
		"/quiz — быстрый тест прямо здесь\n" +
		"/status — статус VIP и серии\n" +
		"/eslatma — ежедневный «Вопрос дня» (19:00): вкл/выкл\n" +
		"/unlink — отвязать Telegram от аккаунта\n\n" +
		"👥 В группе: добавьте бота и напишите /quiz."

	msgUnknownRu = "Неизвестная команда. Что умеет бот: /help"
)

// startCaption is the promo post caption. A learner whose Telegram is
// already linked is welcomed back instead of being pitched the signup trial
// (auth.SignupTrialDuration), which they have had.
func startCaption(l lang, firstName string, linked bool) string {
	name := strings.TrimSpace(firstName)
	if name == "" {
		if l == langRu {
			name = "друг"
		} else {
			name = "do'stim"
		}
	}
	name = escapeTelegramHTML(truncateRunes(name, startNameMaxRunes))
	format := captionStartUzFmt
	switch {
	case l == langRu && linked:
		format = captionBackRuFmt
	case l == langRu:
		format = captionStartRuFmt
	case linked:
		format = captionBackUzFmt
	}
	return fmt.Sprintf(format, name)
}

// telegramHTMLEscaper escapes exactly what Telegram's HTML parse mode
// requires. html.EscapeString would also turn the apostrophe that most Uzbek
// names carry (O'g'iloy) into &#39;.
var telegramHTMLEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func escapeTelegramHTML(s string) string { return telegramHTMLEscaper.Replace(s) }

func helpText(l lang) string {
	if l == langRu {
		return helpRu
	}
	return helpUz
}

func unknownText(l lang) string {
	if l == langRu {
		return msgUnknownRu
	}
	return msgUnknown
}

type menuLabels struct {
	open, tickets, exam, practice, signs, vip, support, group string
}

var (
	menuUz = menuLabels{
		open: "📱 Driver Go'ni ochish", tickets: "🎫 Biletlar", exam: "📝 Imtihon",
		practice: "🎯 Mashq", signs: "🚦 Yo'l belgilari", vip: "👑 VIP",
		support: "💬 Yordam", group: "👥 Guruhda quiz o'ynash",
	}
	menuRu = menuLabels{
		open: "📱 Открыть Driver Go", tickets: "🎫 Билеты", exam: "📝 Экзамен",
		practice: "🎯 Практика", signs: "🚦 Дорожные знаки", vip: "👑 VIP",
		support: "💬 Поддержка", group: "👥 Квиз в группе",
	}
)

// menuKeyboard is the private-chat menu under the promo post and /help.
// With the Mini App enabled every button opens it (web_app, private chats
// only), the section buttons through /tg?next=, which routes there after
// sign-in. With TELEGRAM_WEBAPP_URL cleared (the kill switch) they become
// plain links to the same website pages.
func (b *Bot) menuKeyboard(l lang) *InlineKeyboardMarkup {
	labels := menuUz
	if l == langRu {
		labels = menuRu
	}
	var open InlineKeyboardButton
	var section func(text, page string) InlineKeyboardButton
	if entry, _, ok := b.miniAppEntry(l); ok {
		open = InlineKeyboardButton{Text: labels.open, WebApp: &WebAppInfo{URL: entry.String()}}
		section = func(text, page string) InlineKeyboardButton {
			link, _ := sectionURL(b.WebAppURL, b.PublicBaseURL, l, page)
			return InlineKeyboardButton{Text: text, WebApp: &WebAppInfo{URL: link}}
		}
	} else {
		site := b.siteURL() + "/" + l.appLocale()
		open = InlineKeyboardButton{Text: labels.open, URL: site}
		section = func(text, page string) InlineKeyboardButton {
			return InlineKeyboardButton{Text: text, URL: site + "/" + page}
		}
	}
	rows := [][]InlineKeyboardButton{
		{open},
		{section(labels.tickets, "tickets"), section(labels.exam, "exam")},
		{section(labels.practice, "practice"), section(labels.signs, "signs")},
		{section(labels.vip, "premium"), section(labels.support, "support")},
	}
	if username := strings.TrimPrefix(strings.TrimSpace(b.BotUsername), "@"); username != "" {
		rows = append(rows, []InlineKeyboardButton{{
			Text: labels.group,
			URL:  "https://t.me/" + url.PathEscape(username) + "?startgroup=quiz",
		}})
	}
	return &InlineKeyboardMarkup{InlineKeyboard: rows}
}

// miniAppEntry is the configured Mini App URL in the learner's language and
// the locale its path carries, which /tg's ?next= must share (safeNextPath).
// TELEGRAM_WEBAPP_URL names one locale (…/uz-Latn/tg); Russian swaps that
// segment. A URL without a locale segment is used as is with uz-Latn.
func (b *Bot) miniAppEntry(l lang) (*url.URL, string, bool) {
	return miniAppEntryFor(b.WebAppURL, l)
}

// miniAppEntryFor is miniAppEntry for callers without a Bot (the daily
// reminder runs outside the update path).
func miniAppEntryFor(webAppURL string, l lang) (*url.URL, string, bool) {
	if strings.TrimSpace(webAppURL) == "" {
		return nil, "", false
	}
	u, err := url.Parse(webAppURL)
	if err != nil || u.Host == "" {
		return nil, "", false
	}
	segs := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if !isAppLocale(segs[0]) {
		return u, "uz-Latn", true
	}
	if l == langRu {
		segs[0] = "ru"
		u.Path = "/" + strings.Join(segs, "/")
		u.RawPath = ""
	}
	return u, segs[0], true
}

func isAppLocale(s string) bool {
	switch s {
	case "uz-Latn", "uz-Cyrl", "ru":
		return true
	}
	return false
}

func (b *Bot) siteURL() string { return siteBaseURL(b.PublicBaseURL) }

func siteBaseURL(publicBaseURL string) string {
	if base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/"); base != "" {
		return base
	}
	return defaultSiteURL
}

// sectionURL is a Mini App link that opens on page (via /tg?next=), or the
// same page on the website when the Mini App is switched off; webApp says
// which, since Telegram needs a web_app button for the first and a url
// button for the second.
func sectionURL(webAppURL, publicBaseURL string, l lang, page string) (link string, webApp bool) {
	if entry, loc, ok := miniAppEntryFor(webAppURL, l); ok {
		u := *entry
		q := u.Query()
		q.Set("next", "/"+loc+"/"+page)
		u.RawQuery = q.Encode()
		return u.String(), true
	}
	return siteBaseURL(publicBaseURL) + "/" + l.appLocale() + "/" + page, false
}

func (b *Bot) isLinked(ctx context.Context, tgUserID int64) (bool, error) {
	_, err := b.Link.Q.GetTelegramAccountByTgUserID(ctx, tgUserID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

// sendStartPost answers a plain private /start with the banner, the caption
// and the menu. Telegram fetches the banner from our site; when it cannot
// (a permanent 4xx such as a failed image fetch) the same caption and menu
// go out as text, so the learner still gets a working menu. Retryable
// failures are returned for a redelivery, as everywhere else.
func (b *Bot) sendStartPost(ctx context.Context, chatID int64, from *User) error {
	l := langOf(from.LanguageCode)
	linked, err := b.isLinked(ctx, from.ID)
	if err != nil {
		b.logger().Error("bot: start lookup failed", zap.Error(err), zap.Int64("tg_user_id", from.ID))
		return errors.Join(err, b.replyErr(b.TG.SendMessage(ctx, chatID, msgLinkInternal)))
	}
	caption := startCaption(l, from.FirstName, linked)
	markup := b.menuKeyboard(l)
	_, err = b.TG.SendHTMLPhoto(ctx, chatID, b.siteURL()+l.bannerPath(), caption, markup)
	var api *APIError
	if err == nil || !errors.As(err, &api) || !api.Permanent() {
		return err
	}
	b.logger().Warn("bot: start banner not delivered, sending text",
		zap.Int("code", api.Code), zap.String("description", api.Description))
	_, err = b.TG.SendHTMLText(ctx, chatID, caption, markup)
	return b.replyErr(err)
}

// sendHelp is the private /help: where each section lives, the commands,
// and the same menu as /start.
func (b *Bot) sendHelp(ctx context.Context, chatID int64, from *User) error {
	l := langOf(from.LanguageCode)
	_, err := b.TG.SendHTMLText(ctx, chatID, helpText(l), b.menuKeyboard(l))
	return b.replyErr(err)
}
