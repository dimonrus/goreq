package goreq

import (
	"net/http"
	"time"
)

// default client for all requests
var defaultClient = &http.Client{Timeout: time.Second * DefaultTimeout}
