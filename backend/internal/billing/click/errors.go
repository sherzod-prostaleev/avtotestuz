package click

const (
	errSuccess             = 0
	errSignFailed          = -1
	errAmount              = -2
	errAction              = -3
	errAlreadyPaid         = -4
	errAccountNotFound     = -5
	errTransactionNotFound = -6
	errBadRequest          = -8
	errCancelled           = -9
)

var errorNotes = map[int]string{
	errSuccess:             "Success",
	errSignFailed:          "SIGN CHECK FAILED!",
	errAmount:              "Incorrect parameter amount",
	errAction:              "Action not found",
	errAlreadyPaid:         "Already paid",
	errAccountNotFound:     "User does not exist",
	errTransactionNotFound: "Transaction does not exist",
	errBadRequest:          "Error in request from click",
	errCancelled:           "Transaction cancelled",
}
