package goreq

import (
	"github.com/dimonrus/gorest"
	"github.com/dimonrus/porterr"
)

// IPaginator interface
type IPaginator interface {
	// GetPage get current page
	GetPage() int
	// SetPage set current page
	SetPage(page int)
	// GetLimit get limit
	GetLimit() int
	// SetLimit set limit
	SetLimit(limit int)
	// GetParallelCount get parallel request count
	GetParallelCount() int
	// SetParallelCount set parallel request count
	SetParallelCount(count int)
}

// Paginator Base paginator struct
// Inject the struct into your request forms
type Paginator struct {
	// Page of pagination
	Page int `json:"page"`
	// Limit for pagination
	Limit int `json:"limit"`
	// Parallel request
	ParallelCount int
}

// GetPage get current page
func (p *Paginator) GetPage() int {
	return p.Page
}

// SetPage set current page
func (p *Paginator) SetPage(page int) {
	p.Page = page
	return
}

// GetLimit get limit
func (p *Paginator) GetLimit() int {
	return p.Limit
}

// SetLimit set limit
func (p *Paginator) SetLimit(limit int) {
	p.Limit = limit
	return
}

// GetParallelCount get count of max parallel requests
func (p *Paginator) GetParallelCount() int {
	return p.ParallelCount

}

// SetParallelCount set max number of parallel request
func (p *Paginator) SetParallelCount(count int) {
	p.ParallelCount = count
	return
}

// PaginatorResponse Response from API
type PaginatorResponse[T any] struct {
	// List of elements
	Items []T
	// Meta information
	Meta gorest.Meta
	// Error
	Error porterr.IError
}
