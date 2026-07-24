package payme

// rpcError is a JSON-RPC 2.0 error object. Payme's account-related errors
// (added in later tasks: -31001/-31003/-31008/-31050) require Message with
// ru/uz/en keys and Data naming the offending field — the shape is built
// here so those tasks can construct them with Error() below. Transport
// errors (this task) use the same shape for consistency, though Payme only
// mandates the 3-language message + data for account errors.
type rpcError struct {
	Code    int               `json:"code"`
	Message map[string]string `json:"message"`
	Data    string            `json:"data,omitempty"`
}

// Error builds a JSON-RPC error object. msg should carry "ru"/"uz"/"en"
// keys; data names the offending field/param (empty when not applicable).
func Error(code int, msg map[string]string, data string) *rpcError {
	return &rpcError{Code: code, Message: msg, Data: data}
}

// Transport-level errors (Task 3). Domain errors such as
// -31001 (amount mismatch), -31003 (transaction not found),
// -31008 (invalid transaction state), and -31050 (invalid account) are
// added by Tasks 4-6 using Error() above.
var (
	errNotPost = Error(-32300, map[string]string{
		"ru": "Метод не поддерживается. Используйте POST.",
		"uz": "Metod qo'llab-quvvatlanmaydi. POST so'rovidan foydalaning.",
		"en": "Method not supported. Use POST.",
	}, "")

	errParse = Error(-32700, map[string]string{
		"ru": "Ошибка при разборе JSON.",
		"uz": "JSON'ni tahlil qilishda xato.",
		"en": "Error parsing JSON.",
	}, "")

	errUnknownMethod = Error(-32601, map[string]string{
		"ru": "Метод не найден.",
		"uz": "Metod topilmadi.",
		"en": "Method not found.",
	}, "")

	errAuth = Error(-32504, map[string]string{
		"ru": "Недостаточно прав для выполнения операции.",
		"uz": "Amalni bajarish uchun ruxsat yetarli emas.",
		"en": "Insufficient privilege to perform this operation.",
	}, "")
)
