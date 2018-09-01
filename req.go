package goreq

import (
	"time"
	"io/ioutil"
	"net/http"
	"fmt"
	"bytes"
	"strings"
)

//Default request timeout
const DefaultTimeout = 30

//Request logger interface
//Implement default logger methods
type Logger interface {
	Print(v ...interface{})
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

//Each request performs from struct bellow
type HttpRequest struct {
	//Host service label. For messages
	Label string
	//Default Http client
	Client *http.Client
	//Service host
	Host string
	//Http method. GET, POST, PUT etc
	Method string
	//Remote endpoint
	Url string
	//Http headers
	Headers map[string]string
	//Http body
	Body []byte
	//Count of retry attempts
	RetryCount uint
	//Retry timeout. Default 30s
	RetryTimeout time.Duration
	//Retry strategy callback
	RetryStrategy func(response *http.Response) bool
	//Response error
	ResponseErrorStrategy func(response *http.Response, url string, service string) error
	//Logger. Implements RequestLogger
	Logger Logger
}

//Validate request
func (r *HttpRequest) validate() error {
	if r.Method == "" {
		return &Error{Message: "Method is not defined", HttpCode: http.StatusBadRequest}
	}

	if r.Host == "" {
		return &Error{Message: "Host is not defined", HttpCode: http.StatusBadRequest}
	}

	if r.Url == "" {
		return &Error{Message: "Url is not defined", HttpCode: http.StatusBadRequest}
	}
	return nil
}

//Retry strategy
//if response code bellow to 500, 502, 503, 504 than repeat request
func canContinueRetry(response *http.Response) bool {
	switch response.StatusCode {
	case http.StatusInternalServerError:
		return true
	case http.StatusBadGateway:
		return true
	case http.StatusServiceUnavailable:
		return true
	case http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

//Response error returns
func responseError(response *http.Response, route string, service string) (err error) {
	switch response.StatusCode {
	case http.StatusBadRequest:
		err = &Error{Message: fmt.Sprintf("Bad request: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	case http.StatusUnauthorized:
		err = &Error{Message: fmt.Sprintf("Unauthorized: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	case http.StatusForbidden:
		err = &Error{Message: fmt.Sprintf("Forbidden: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	case http.StatusNotFound:
		err = &Error{Message: fmt.Sprintf("Not found: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	case http.StatusConflict:
		err = &Error{Message: fmt.Sprintf("Conflict: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	case http.StatusInternalServerError:
		err = &Error{Message: fmt.Sprintf("Internal server error: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	case http.StatusBadGateway:
		err = &Error{Message: fmt.Sprintf("Bad gateway: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	case http.StatusServiceUnavailable:
		err = &Error{Message: fmt.Sprintf("Service unavailable: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	case http.StatusGatewayTimeout:
		err = &Error{Message: fmt.Sprintf("Gateway timeout: %s. Service: %s", route, service), HttpCode: response.StatusCode}
	}

	return err
}

//Ensure request
func Ensure(request HttpRequest) (*http.Response, []byte, error) {
	//Validate request
	if err := request.validate(); err != nil {
		return nil, nil, err
	}
	//Check http client
	if request.Client == nil {
		request.Client = &http.Client{Timeout: time.Second * DefaultTimeout}
	}
	//Check retry strategy
	if request.RetryStrategy == nil {
		request.RetryStrategy = canContinueRetry
	}
	//Check retry strategy
	if request.ResponseErrorStrategy == nil {
		request.ResponseErrorStrategy = responseError
	}
	//Make new request
	route := request.Host + request.Url
	req, err := http.NewRequest(request.Method, route, bytes.NewBuffer(request.Body))
	if err != nil {
		return nil, nil, &Error{Message: fmt.Sprintf("Http Request build error: %s. Service: %s", err, request.Label), HttpCode: http.StatusInternalServerError}
	}
	//Collect headers
	var headersLog string
	for k, v := range request.Headers {
		req.Header.Add(k, v)
		headersLog += fmt.Sprintf("-H '%s: %s' ", k, v)
	}
	//Calculate request time
	var delta int64
	var response *http.Response

	//Log request as CURL
	logCurl := fmt.Sprintf("curl -X %s '%s' %s -d '%s'", request.Method, route, headersLog, request.Body)

	//Loop for retry count
	for i := uint(0); i <= request.RetryCount; i++ {
		//Get start time
		startTime := time.Now().UnixNano()
		//Perform request
		response, err = request.Client.Do(req)
		//Get end time
		endTime := time.Now().UnixNano()
		//Calc delta
		delta = (endTime - startTime) / int64(time.Millisecond)
		if response == nil || err != nil {
			//If no response than log
			request.Logger.Print(logCurl + "\n FAILED!!!")
			if i >= request.RetryCount {
				return nil, nil, &Error{Message: fmt.Sprintf("Http Request (%s) failed. Service: %s", route, request.Label), HttpCode: http.StatusInternalServerError}
			}
		} else {
			//Check if can retry response
			if canContinueRetry(response) {
				response.Body.Close()
				//Sleep before next round
				if request.RetryTimeout.Nanoseconds() > 0 {
					time.Sleep(request.RetryTimeout)
				}
				continue
			} else {
				break
			}
		}
	}

	//Close body
	defer response.Body.Close()

	//Read response body
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, nil, &Error{Message: fmt.Sprintf("Http Response (%s) read error: %s. Service: %s", route, err.Error(), request.Label), HttpCode: http.StatusInternalServerError}
	}

	//Log response status
	logStatus := fmt.Sprintf("HTTP Status [%v] in: %v ms", response.StatusCode, delta)
	//Log response body
	logBody := fmt.Sprintf("Body: %s", strings.Join(strings.Fields(string(bodyBytes)), " "))
	if response.StatusCode > 300 {
		logStatus = "\x1b[31;1m" + logStatus + "\x1b[0m"
		logBody = "\x1b[31;1m" + logBody + "\x1b[0m"
	} else {
		logStatus = "\x1b[31;1m" + logStatus + "\x1b[0m"
		logBody = "\x1b[32;1m" + logBody + "\x1b[0m"
	}
	request.Logger.Print("\n    ", "\x1b[34;1m"+logCurl+"\x1b[0m", "\n    ", logStatus, "\n    ", logBody)

	return response, bodyBytes, err
}
