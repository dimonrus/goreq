package goreq

import (
	"github.com/dimonrus/gorest"
	"github.com/dimonrus/porterr"
)

// Ipaginator
type IPaginator interface {
	// GetPage get current page
	GetPage() int
	// SetPage set current page
	SetPage(page int)
	// GetLimit get limit
	GetLimit() int
	// SetLimit set limit
	SetLimit(limit int)
	// GetAsyncCount get parallel request count
	GetAsyncCount() int
	// SetAsyncCount set parallel request count
	SetAsyncCount(count int)
}

// Paginator Base paginator struct
type Paginator struct {
	// Page of pagination
	Page int `json:"page"`
	// Limit for pagination
	Limit int `json:"limit"`
	// Parallel request
	AsyncCount int
}

func (p *Paginator) GetPage() int {
	return p.Page
}

func (p *Paginator) SetPage(page int) {
	p.Page = page
	return
}

func (p *Paginator) GetLimit() int {
	return p.Limit
}
func (p *Paginator) SetLimit(limit int) {
	p.Limit = limit
	return
}
func (p *Paginator) GetAsyncCount() int {
	return p.AsyncCount

}
func (p *Paginator) SetAsyncCount(count int) {
	p.AsyncCount = count
	return
}

// Response from API
type PaginatorResponse[T any] struct {
	// List of elements
	Items []T
	// Meta information
	Meta gorest.Meta
	// Error
	Error porterr.IError
}

// AsyncRequestCall Execute api call that can have async count of parallel request
func AsyncRequestCall[F IPaginator, R any](url, method string, form F, hr HttpRequest) (items []R, meta gorest.Meta, e porterr.IError) {
	call := func(body F) (data []R, meta gorest.Meta, e porterr.IError) {
		response := gorest.JsonResponse{Data: &items, Meta: &meta}
		_, err := hr.EnsureJSON(method, url, nil, body, &response)
		if err != nil {
			e = err.(*porterr.PortError)
		}
		return
	}
	items, meta, e = call(form)
	if e != nil || form.GetAsyncCount() == 0 {
		return
	}
	// Set current page
	var iterator = meta.Page
	if iterator == 0 {
		iterator = 1
	}
	// count number of elements that must be fetched
	var total = meta.Total - iterator*meta.Limit
	// count number of total requests
	var respLen = total / meta.Limit
	if meta.Total%meta.Limit > 0 {
		respLen++
	}
	// Result for return
	var result = make([]R, total+meta.Limit)
	// All parallel requests reuslt
	var resp = make([][]R, respLen+1)
	// Data from requests
	var fetch = make(chan PaginatorResponse[R], form.GetAsyncCount())
	// Max requests in moments
	var request = make(chan struct{}, form.GetAsyncCount())
	// go requests
	go func() {
		for iterator <= respLen {
			iterator++
			r := form
			r.SetPage(iterator)
			request <- struct{}{}
			go func(f chan PaginatorResponse[R], p F) {
				items, meta, e := call(p)
				f <- PaginatorResponse[R]{
					Items: items,
					Meta:  meta,
					Error: e,
				}
				<-request
			}(fetch, r)
		}
	}()
	// process parallel result
	var processed = 1
	for response := range fetch {
		if response.Error != nil {
			e = response.Error
			return
		}
		resp[response.Meta.Page-1] = response.Items
		processed++
		if processed == respLen+1 {
			close(fetch)
			break
		}
	}
	// save acording to order
	copy(result[:meta.Limit], items)
	for i := range resp {
		if resp[i] == nil {
			continue
		}
		copy(result[i*meta.Limit:i*meta.Limit+len(resp[i])], resp[i])
	}
	items = result
	return
}
