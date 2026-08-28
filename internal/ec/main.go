package ec

var (
	CircularRedirectNotAllowed = &Error{StatusCode: 400, Code: "CircularRedirectNotAllowed", Message: "Circular redirects are not allowed"}
	EmailNotAvailable          = &Error{StatusCode: 409, Code: "EmailNotAvailable", Message: "The specified email is not available"}
	InvalidCredentials         = &Error{StatusCode: 401, Code: "InvalidCredentials", Message: "Invalid Credentials"}
	NoSuchRedirect             = &Error{StatusCode: 404, Code: "NoSuchRedirect", Message: "The specified redirect does not exist"}
	NoSuchUser                 = &Error{StatusCode: 404, Code: "NoSuchUser", Message: "The specified user does not exist"}
	NoSuchAggregatedStats      = &Error{StatusCode: 404, Code: "NoSuchAggregatedStats", Message: "The specified aggregated stats do not exist"}
	SourceDomainNotAvailable   = &Error{StatusCode: 409, Code: "SourceDomainNotAvailable", Message: "The specified source domain is already in use"}
	Unauthorized               = &Error{StatusCode: 401, Code: "Unauthorized", Message: "Unauthorized"}
)

type Error struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *Error) Error() string {
	return e.Message
}

func UnauthorizedMessage(message string) *Error {
	return &Error{
		StatusCode: 401,
		Code:       "Unauthorized",
		Message:    message,
	}
}
