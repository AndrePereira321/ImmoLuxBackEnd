package models

type ServerAPIResponse struct {
	Success bool            `json:"success"`
	Data    any             `json:"data"`
	Error   *ServerAPIError `json:"error"`
}

func NewServerAPIResponse(success bool, data any, error *ServerAPIError) *ServerAPIResponse {
	return &ServerAPIResponse{
		Success: success,
		Data:    data,
		Error:   error,
	}
}

type ServerAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewServerAPIError(code, message string) *ServerAPIError {
	return &ServerAPIError{
		Code:    code,
		Message: message,
	}
}
