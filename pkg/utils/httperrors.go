// The HTTPError type implements the error interface, making it compatible
// with standard Go error handling mechanisms. The package also includes a
// constructor function, NewHTTPError, for creating instances of HTTPError
// with a specified status code and message.

package utils

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}
