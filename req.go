package goreq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dimonrus/gorest"
	"github.com/dimonrus/porterr"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultTimeout Default request timeout
const DefaultTimeout = 30

// Logger Request logger interface
// Implement default logger methods
type Logger interface {
	Print(v ...interface{})
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

// HttpRequest Each request performs via struct bellow
type HttpRequest struct {
	//Host service label. For messages
	Label string
	//Default Http client
	Client *http.Client
	//Service host
	Host string
	//Http method. GET, POST, PUT, etc.
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
	ResponseErrorStrategy func(response *http.Response) error
	//Logger. Implements RequestLogger
	Logger Logger
	//How many body bytes must be logged
	//0 - all body will be logged
	LogBodySize int
}

// Validate request
func (r HttpRequest) validate() error {
	var e porterr.IError
	if r.Method == "" {
		e = porterr.New(porterr.PortErrorValidation, "Request is invalid").HTTP(http.StatusBadRequest)
		e = e.PushDetail(porterr.PortErrorParam, "method", "Method is not defined")
	}
	if r.Url == "" {
		if e == nil {
			e = porterr.New(porterr.PortErrorValidation, "Request is invalid").HTTP(http.StatusBadRequest)
		}
		e = e.PushDetail(porterr.PortErrorParam, "url", "Url is not defined")
	}
	return e
}

// Retry strategy
// if response code belongs to 500, 502, 503, 504 than repeat request
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

// Response error returns
// Default method.
// If you need to override this - please override for ResponseErrorStrategy
func responseError(response *http.Response) error {
	var e porterr.IError
	if response.StatusCode >= http.StatusBadRequest {
		e = porterr.New(
			porterr.PortErrorResponse,
			http.StatusText(response.StatusCode)+": "+response.Request.URL.Path+" Service: "+response.Request.Host,
		).HTTP(response.StatusCode)
	}
	return e
}

// BuildCURL Build curl for logging
func BuildCURL(request HttpRequest) string {
	b := strings.Builder{}
	b.WriteString("curl -X " + request.Method + " '" + request.Host + request.Url + "'")
	//Collect headers
	for k, v := range request.Headers {
		b.WriteString(" -H '" + k + ": " + strings.Join(v, ",") + "' ")
	}
	// log body
	if request.Body != nil {
		if request.LogBodySize == 0 || len(request.Body) < request.LogBodySize {
			b.WriteString("-d '" + string(request.Body) + "'")
		} else {
			b.WriteString("-d '" + string(request.Body[:request.LogBodySize-1]) + "...'")
		}
	}
	return b.String()
}

func initDefault(request *HttpRequest) {
	//Check http client
	if request.Client == nil {
		request.Client = defaultClient
	}
	//Check retry strategy
	if request.RetryStrategy == nil {
		request.RetryStrategy = canContinueRetry
	}
	//Check error response strategy
	if request.ResponseErrorStrategy == nil {
		request.ResponseErrorStrategy = responseError
	}
}

// InitDefaultLogger Init default logger
func (r *HttpRequest) InitDefaultLogger() {
	r.Logger = log.New(os.Stdout, "REQUEST: ", log.Ldate|log.Ltime)
}

// Ensure request
func Ensure(request HttpRequest) (*http.Response, []byte, error) {
	//Validate request
	if err := request.validate(); err != nil {
		return nil, nil, err
	}
	//Set default options
	initDefault(&request)

	//Make new request
	req, err := http.NewRequest(request.Method, request.Host+request.Url, nil)
	if err != nil {
		return nil, nil, porterr.NewF(porterr.PortErrorRequest, "Http Request build error: %s. Service: %s", err, request.Label)
	}

	req.Header = request.Headers

	//Log request as CURL
	var logCurl string
	if request.Logger != nil {
		logCurl = BuildCURL(request)
	}

	// Response
	var response *http.Response

	// Response body
	var bodyBytes []byte

	// Body buffer
	var buffer *bytes.Buffer

	//Calculate request time
	var startTime, endTime, delta int64

	//Loop for retry count
	for i := uint(0); i <= request.RetryCount; i++ {
		//Set body
		buffer = bytes.NewBuffer(request.Body)
		req.Body = io.NopCloser(buffer)
		//Get start time
		startTime = time.Now().UnixNano()
		//Perform request
		response, err = request.Client.Do(req)
		//Get end time
		endTime = time.Now().UnixNano()
		//Calc delta
		delta = (endTime - startTime) / int64(time.Millisecond)
		//If server does not respond
		if err != nil {
			//if no response than log
			if request.Logger != nil {
				request.Logger.Printf("\x1b[31;1m"+logCurl+"\n %s \n FAILED!!!\x1b[0m", err)
			}
			if i >= request.RetryCount {
				return nil, nil, porterr.NewF(porterr.PortErrorSystem, "Http Request (%s) failed. Service: %s, Error: %s", request.Url, request.Label, err)
			}
			if request.RetryTimeout.Nanoseconds() > 0 {
				time.Sleep(request.RetryTimeout)
			}
		} else {
			// Read response
			bodyBytes, err = io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil {
				bodyBytes = []byte{}
				logRequest(&request, response.StatusCode, &bodyBytes, delta, logCurl)
				return nil, nil, porterr.NewF(porterr.PortErrorResponse, "Http Response (%s) read error: %s. Service: %s", request.Host+request.Url, err, request.Label)
			}
			//Log request
			logRequest(&request, response.StatusCode, &bodyBytes, delta, logCurl)

			//Check if you can retry the response
			if request.RetryStrategy(response) {
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

	if response != nil {
		response.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	if request.ResponseErrorStrategy != nil {
		err = request.ResponseErrorStrategy(response)
	}

	return response, bodyBytes, err
}

// Log request
func logRequest(request *HttpRequest, responseStatus int, responseBody *[]byte, delta int64, curl string) {
	// Skip logging if not logger
	if request.Logger == nil {
		return
	}
	//Log response status
	logStatus := fmt.Sprintf("HTTP Status [%v] in: %v ms", responseStatus, delta)

	//Log response body
	var logBody string
	if request.LogBodySize == 0 || len(*responseBody) < request.LogBodySize {
		logBody = fmt.Sprintf("Body: %s", strings.Join(strings.Fields(string(*responseBody)), " "))
	} else {
		logBody = fmt.Sprintf("Body: %s...", strings.Join(strings.Fields(string(*responseBody)[:request.LogBodySize-1]), " "))
	}

	//If response status code more than 300 shows in red
	if responseStatus >= 300 {
		logStatus = "\x1b[31;1m" + logStatus + "\x1b[0m"
		logBody = "\x1b[31;1m" + logBody + "\x1b[0m"
	} else {
		logStatus = "\x1b[32;1m" + logStatus + "\x1b[0m"
		logBody = "\x1b[32;1m" + logBody + "\x1b[0m"
	}
	request.Logger.Print("\n    ", "\x1b[34;1m"+curl+"\x1b[0m", "\n    ", logStatus, "\n    ", logBody)
}

// EnsureJSON ensure JSON request
func (r HttpRequest) EnsureJSON(method string, url string, header http.Header, body interface{}, dto interface{}) (*http.Response, error) {
	// Error interface
	var err error

	// Copy request
	req := r

	// Set method
	req.Method = method

	// Set Url
	req.Url = url

	//Copy headers
	req.Headers = r.Headers.Clone()
	if header != nil {
		for key, value := range header {
			req.Headers.Add(key, strings.Join(value, ","))
		}
	}

	//Reset body
	req.Body = nil

	//Set body
	if body != nil {
		//Marshal body
		req.Body, err = json.Marshal(body)
		if err != nil {
			return nil, porterr.NewF(porterr.PortErrorBody, "Http Request (%s) marshal error: %s. Service: %s", req.Host+req.Url, err.Error(), req.Label)
		}
	}

	// Ensure
	response, data, err := Ensure(req)
	if err != nil {
		return response, err
	}

	// Unmarshal response
	if dto != nil {
		err = json.Unmarshal(data, dto)
		if err != nil {
			return nil, porterr.NewF(porterr.PortErrorBody, "Http Response (%s) marshal error: %s. Service: %s", req.Host+req.Url, err.Error(), req.Label)
		}
	}

	return response, nil
}

// ParallelPaginatorJsonEnsure Execute api call that can have async count of parallel request
func ParallelPaginatorJsonEnsure[F any, R any](form F, hr HttpRequest) (items []R, meta gorest.Meta, e porterr.IError) {
	var _f interface{} = &form
	var _form, ok = _f.(IPaginator)
	if !ok {
		e = porterr.New(porterr.PortErrorRequest, "form must implements IPaginator interface")
		return
	}
	call := func(requestForm F) (data []R, meta gorest.Meta, e porterr.IError) {
		response := gorest.JsonResponse{Data: &data, Meta: &meta}
		_, err := hr.EnsureJSON(hr.Method, hr.Url, nil, requestForm, &response)
		if err != nil {
			e = err.(*porterr.PortError)
		}
		return
	}
	items, meta, e = call(form)
	// return on condition
	// - if error detected
	// - if not parallel operation
	// - if last page were called
	if e != nil || _form.GetParallelCount() == 0 {
		return
	}
	if meta.Page == 0 {
		meta.Page = 1
	}
	if meta.Page*meta.Limit > meta.Total {
		return
	}
	var iterator int
	// count the number of elements that must be fetched
	var total = meta.Total - meta.Page*meta.Limit
	// count the number of total requests
	var respLen = total / meta.Limit
	if meta.Total%meta.Limit > 0 {
		respLen++
	}
	// result for return
	var result = make([]R, total+meta.Limit)
	// data from requests
	var fetch = make(chan PaginatorResponse[R], respLen)
	// max requests in moments
	var request = make(chan struct{}, _form.GetParallelCount())
	// go requests
	go func() {
		for iterator < respLen {
			iterator++
			var p = form
			var fp interface{} = &p
			fp.(IPaginator).SetPage(iterator + meta.Page)
			request <- struct{}{}
			go func(f chan PaginatorResponse[R], p F) {
				items, meta, e := call(p)
				f <- PaginatorResponse[R]{
					Items: items,
					Meta:  meta,
					Error: e,
				}
				<-request
			}(fetch, p)
		}
	}()
	// save data according to order
	copy(result[:meta.Limit], items)
	// process parallel result
	var processed int
	for response := range fetch {
		if response.Error != nil {
			e = response.Error
			return
		}
		copy(result[(response.Meta.Page-meta.Page)*meta.Limit:(response.Meta.Page-meta.Page)*meta.Limit+len(response.Items)], response.Items)
		processed++
		if processed == respLen {
			close(fetch)
			break
		}
	}
	items = result
	return
}
