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
// -31003 (transaction not found) are added by Tasks 5-6 using Error()
// above. -31001 (amount mismatch), -31008 (invalid transaction state), and
// -31050 (invalid account) are added below by Task 4.
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

	// errInternal is not part of the Payme protocol proper (Payme's
	// account/amount/state error codes are all documented above); it is
	// our own catch-all for unexpected DB failures during method
	// handling, so we never mis-report a real server error as an account
	// or amount problem. -32400 mirrors the code range Paycom's own
	// integrations commonly use for "system error" responses.
	errInternal = Error(-32400, map[string]string{
		"ru": "Внутренняя ошибка сервера.",
		"uz": "Server ichki xatosi.",
		"en": "Internal server error.",
	}, "")
)

// Domain errors (Task 4): account/amount/state validation for
// CheckPerformTransaction and CreateTransaction.
var (
	// errAmount: account is valid but params.amount (tiyin) doesn't match
	// payment.amount_uzs*100.
	errAmount = Error(-31001, map[string]string{
		"ru": "Неверная сумма.",
		"uz": "Summa noto'g'ri.",
		"en": "Incorrect amount.",
	}, "")

	// errAccountNotFound: no payment for account.order_id, or the
	// payment's status isn't payable (only 'created' is). Per the spec,
	// account errors carry data naming the offending field.
	errAccountNotFound = Error(-31050, map[string]string{
		"ru": "Заказ не найден.",
		"uz": "Buyurtma topilmadi.",
		"en": "Order not found.",
	}, "order_id")

	// errTransactionState: e.g. CreateTransaction for a new payme_id when
	// the payment already has another active (state 1/2) transaction.
	errTransactionState = Error(-31008, map[string]string{
		"ru": "Невозможно выполнить операцию.",
		"uz": "Amalni bajarib bo'lmaydi.",
		"en": "Unable to perform the operation.",
	}, "")
)
