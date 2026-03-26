package protocol

import "fmt"

const (
	CodeOK                     = "OK"
	CodeRPCInvalidRequest      = "RPC_INVALID_REQUEST"
	CodeRPCMissingRequestID    = "RPC_MISSING_REQUEST_ID"
	CodeRPCUnknownMethod       = "RPC_UNKNOWN_METHOD"
	CodeInvalidPayload         = "INVALID_PAYLOAD"
	CodeInvalidSignature       = "INVALID_SIGNATURE"
	CodeInvalidAckSignature    = "INVALID_ACK_SIGNATURE"
	CodeTTLTooLarge            = "TTL_TOO_LARGE"
	CodeMessageTooLarge        = "MESSAGE_TOO_LARGE"
	CodeRecipientNotFound      = "RECIPIENT_NOT_FOUND"
	CodeRecipientQuotaExceeded = "RECIPIENT_QUOTA_EXCEEDED"
	CodeRecipientBytesExceeded = "RECIPIENT_BYTES_EXCEEDED"
	CodeRateLimited            = "RATE_LIMITED"
	CodeUnauthorized           = "UNAUTHORIZED"
	CodeAfterSeqInvalid        = "AFTER_SEQ_INVALID"
	CodeDuplicate              = "DUPLICATE"
	CodeInternalError          = "INTERNAL_ERROR"
)

type AppError struct {
	Code    string
	Message string
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewAppError(code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func ErrorCode(err error) string {
	var appErr *AppError
	if err == nil {
		return ""
	}
	if ok := AsAppError(err, &appErr); ok {
		return appErr.Code
	}
	return CodeInternalError
}

func ErrorMessage(err error) string {
	var appErr *AppError
	if err == nil {
		return ""
	}
	if ok := AsAppError(err, &appErr); ok {
		return appErr.Message
	}
	return err.Error()
}

func AsAppError(err error, target **AppError) bool {
	if err == nil {
		return false
	}
	appErr, ok := err.(*AppError)
	if !ok {
		return false
	}
	*target = appErr
	return true
}
