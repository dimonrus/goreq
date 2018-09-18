package goreq

//Request error
type Error struct {
	//Error message
	Message string `json:"message"`
	//Http status code
	HttpCode int `json:"-"`
	//Error code. Detail user code
	ErrorCode *string `json:"code,omitempty"`
	//Errors can be different
	Errors interface{} `json:"errors,omitempty"`
}

//Error implements error interface method
func (r Error) Error() string {
	return r.Message
}
