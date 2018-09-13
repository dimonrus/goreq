package goreq

import (
	"time"
	"io/ioutil"
	"net/http"
	"fmt"
	"bytes"
	"strings"
	"log"
	"os"
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

//Each request performs via struct bellow
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
	Headers http.Header
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
	//How many body bytes must be logged
	//0 - all body will be logged
	LogBodySize int
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
//if response code belongs to 500, 502, 503, 504 than repeat request
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

// Build curl for logging
func buildCURL(r *http.Request, request *HttpRequest) string {
	//Collect headers
	var headersLog string
	for k, v := range request.Headers {
		headersLog += fmt.Sprintf("-H '%s: %s' ", k, strings.Join(v, ","))
	}
	// log body
	if request.Body != nil {
		if request.LogBodySize == 0 || len(request.Body) < request.LogBodySize {
			headersLog += fmt.Sprintf("-d '%s'", request.Body)
		} else {
			headersLog += fmt.Sprintf("-d '%s...'", request.Body[:request.LogBodySize-1])
		}
	}

	return fmt.Sprintf("curl -X %s '%s' %s", request.Method, request.Host+request.Url, headersLog)
}

func initDefault(request *HttpRequest) {
	//Check http client
	if request.Client == nil {
		request.Client = &http.Client{Timeout: time.Second * DefaultTimeout}
	}
	//Check retry strategy
	if request.RetryStrategy == nil {
		request.RetryStrategy = canContinueRetry
	}
	//Check error response strategy strategy
	if request.ResponseErrorStrategy == nil {
		request.ResponseErrorStrategy = responseError
	}
	//Check logger
	if request.Logger == nil {
		request.Logger = log.New(os.Stdout, "REQUEST: ", log.Ldate|log.Ltime)
	}
}

//Ensure request
func Ensure(request HttpRequest) (*http.Response, []byte, error) {
	//Validate request
	if err := request.validate(); err != nil {
		return nil, nil, err
	}

	//Set default options
	initDefault(&request)

	//Make new request
	req, err := http.NewRequest(request.Method, request.Host+request.Url, bytes.NewBuffer(request.Body))
	if err != nil {
		return nil, nil, &Error{Message: fmt.Sprintf("Http Request build error: %s. Service: %s", err, request.Label), HttpCode: http.StatusInternalServerError}
	}
	req.Header = request.Headers

	//Log request as CURL
	logCurl := buildCURL(req, &request)

	//Calculate request time
	var delta int64

	// Response
	var response *http.Response

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
			//if no response than log
			request.Logger.Print("\x1b[31;1m" + logCurl + "\n FAILED!!!\x1b[0m")
			if i >= request.RetryCount {
				return nil, nil, &Error{Message: fmt.Sprintf("Http Request (%s) failed. Service: %s", request.Url, request.Label), HttpCode: http.StatusInternalServerError}
			}
			if request.RetryTimeout.Nanoseconds() > 0 {
				time.Sleep(request.RetryTimeout)
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
		return nil, nil, &Error{
			Message: fmt.Sprintf(
				"Http Response (%s) read error: %s. Service: %s",
				request.Host+request.Url,
				err.Error(),
				request.Label),
			HttpCode: http.StatusInternalServerError,
		}
	}

	//Log response status
	logStatus := fmt.Sprintf("HTTP Status [%v] in: %v ms", response.StatusCode, delta)

	//Log response body
	var logBody string
	if request.LogBodySize == 0 || len(bodyBytes) < request.LogBodySize {
		logBody = fmt.Sprintf("Body: %s", strings.Join(strings.Fields(string(bodyBytes)), " "))
	} else {
		logBody = fmt.Sprintf("Body: %s...", strings.Join(strings.Fields(string(bodyBytes)), " ")[:request.LogBodySize-1])
	}

	//If response status code more then 300 shows in red
	if response.StatusCode >= 300 {
		logStatus = "\x1b[31;1m" + logStatus + "\x1b[0m"
		logBody = "\x1b[31;1m" + logBody + "\x1b[0m"
	} else {
		logStatus = "\x1b[32;1m" + logStatus + "\x1b[0m"
		logBody = "\x1b[32;1m" + logBody + "\x1b[0m"
	}
	request.Logger.Print("\n    ", "\x1b[34;1m"+logCurl+"\x1b[0m", "\n    ", logStatus, "\n    ", logBody)

	return response, bodyBytes, err
}
