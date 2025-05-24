package goreq

import (
	"encoding/json"
	"fmt"
	"github.com/dimonrus/gorest"
	"github.com/dimonrus/porterr"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// https://jsonplaceholder.typicode.com/posts
var jsonplaceholder = HttpRequest{
	Host:    "https://jsonplaceholder.typicode.com",
	Logger:  log.New(os.Stdout, "Placeholder: ", log.Lshortfile),
	Headers: map[string][]string{"Content-Type": {"application/json"}},
	Label:   "Jsonplaceholder",
}

var badJsonplaceholder = HttpRequest{
	Host:         "https://jsdsf.wdsf",
	Headers:      map[string][]string{"Content-Type": {"application/json"}},
	Label:        "BadJsonplaceholder",
	RetryCount:   2,
	RetryTimeout: time.Duration(time.Millisecond * 100),
}

type Post struct {
	Id     int    `json:"id,omitempty"`
	UserId int    `json:"userId"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func GeBadPosts() (posts []Post, err error) {
	_, err = badJsonplaceholder.EnsureJSON("GET", "/posts", nil, nil, &posts)
	if err != nil {
		return nil, err
	}
	return posts, nil
}

func GetPosts() (posts []Post, err error) {
	_, err = jsonplaceholder.EnsureJSON("GET", "/posts", nil, nil, &posts)
	if err != nil {
		return nil, err
	}
	return posts, nil
}

func GetPost(id int) (post *Post, err error) {
	p := Post{}
	h := make(http.Header)
	h.Add("x-post", strconv.Itoa(id))
	jsonplaceholder.InitDefaultLogger()
	_, err = jsonplaceholder.EnsureJSON("GET", fmt.Sprintf("/posts/%v", id), h, nil, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func CreatePost(post *Post) (*Post, error) {
	_, err := jsonplaceholder.EnsureJSON("POST", "/posts", nil, post, post)
	if err != nil {
		return nil, err
	}
	return post, nil
}

func TestGetPostAsync(t *testing.T) {
	c := make(chan Post, 2)
	go func() {
		p, err := GetPost(1)
		if err != nil {
			c <- Post{}
			return
		}
		c <- *p
	}()

	go func() {
		p, err := GetPost(2)
		if err != nil {
			c <- Post{}
			return
		}
		c <- *p
	}()
	time.Sleep(time.Second * 2)
}

func TestCreatePostAsync(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		post := &Post{}
		post.Title = "Title of the post"
		post.UserId = 24
		post.Body = "Body of the post"
		CreatePost(post)
		wg.Done()
		if post.Id == 0 {
			t.Error("Post cant be created")
		}
	}()
	go func() {
		post := &Post{}
		post.Title = "Title of the new post"
		post.UserId = 25
		post.Body = "Body of the new post"
		CreatePost(post)
		wg.Done()
		if post.Id == 0 {
			t.Error("Post cant be created")
		}
	}()
	go func() {
		post := &Post{}
		post.Title = "New title of the new post"
		post.UserId = 26
		post.Body = "New body of the new post"
		CreatePost(post)
		wg.Done()
		if post.Id == 0 {
			t.Error("Post cant be created")
		}
	}()
	wg.Wait()
}

func TestCreatePost(t *testing.T) {
	post := &Post{}
	post.Title = "Title of the post"
	post.UserId = 24
	post.Body = "Body of the post"
	CreatePost(post)

	if post.Id == 0 {
		t.Error("Post cant be created")
	}
}

func TestGetPosts(t *testing.T) {
	posts, err := GetPosts()
	if err != nil {
		t.Error(err)
	}
	fmt.Print("Post lenght: ", len(posts), "\n")
}

func TestGetBadPosts(t *testing.T) {
	_, err := GeBadPosts()
	if err == nil {
		t.Error("Must be an error")
	}
}

func TestGroupGetPost(t *testing.T) {
	countOfPosts := 10
	postChan := make(chan Post, countOfPosts)
	wg := sync.WaitGroup{}
	wg.Add(countOfPosts)
	for i := 0; i < countOfPosts; i++ {
		go func(index int) {
			defer wg.Done()
			post, err := GetPost(index)
			if err != nil {
				return
			}
			postChan <- *post
		}(i)
	}
	wg.Wait()
}

func TestGroupClassic(t *testing.T) {
	countOfPosts := 10
	postChan := make(chan Post, countOfPosts)
	wg := sync.WaitGroup{}
	wg.Add(countOfPosts)
	for i := 0; i < countOfPosts; i++ {
		go func(index int) {
			//Make new request
			req, err := http.NewRequest("GET", jsonplaceholder.Host+fmt.Sprintf("/post/%v", index), nil)
			if err != nil {
				postChan <- Post{}
				wg.Done()
				return
			}
			client := &http.Client{Timeout: time.Second * DefaultTimeout}
			response, err := client.Do(req)
			if err != nil {
				log.Print(err)
				postChan <- Post{}
				wg.Done()
				return
			}

			body, err := io.ReadAll(response.Body)

			if err != nil {
				postChan <- Post{}
				wg.Done()
				return
			}
			defer response.Body.Close()

			var post Post
			err = json.Unmarshal(body, &post)
			if err != nil {
				postChan <- Post{}
				wg.Done()
				return
			}
			postChan <- post
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestClient(t *testing.T) {
	w := httptest.NewRecorder()
	data := []byte(`{"message":"hight level message","error":{"message":"Filed with message","code":"FAILED_CODE","name":"Unknown","data":[{"message":"New detail","code":"SOME_CODE","name":"item"},{"message":"New detail 2","code":400,"name":"item second"}]}}`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, err := w.Write(data)
	if err != nil {
		t.Fatal(err)
	}
	resp := w.Result()
	b, err := io.ReadAll(resp.Body)
	fmt.Printf("%s\n", b)
	if err != nil {
		t.Fatal(err)
	}
	response := struct {
		Message string         `json:"message"`
		Error   porterr.IError `json:"error"`
	}{
		Error: &porterr.PortError{},
	}
	err = json.Unmarshal(b, &response)
	if err != nil {
		t.Fatal(err)
	}
	if response.Error.Error() != "Filed with message" {
		t.Fatal("wrong decode")
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatal("wrong status code")
	}
}

func testErrorHandler(w http.ResponseWriter, r *http.Request) {
	e := porterr.New(porterr.PortErrorSearch, "Some failed message").HTTP(http.StatusNotFound)
	e = e.PushDetail(porterr.PortErrorDecoder, "some", "Some error")
	e = e.PushDetail(porterr.PortErrorDecoder, "other", "Some other error")
	res := struct {
		Error porterr.IError `json:"error"`
	}{
		Error: e,
	}
	w.WriteHeader(e.GetHTTP())
	data, _ := json.Marshal(res)
	_, _ = w.Write(data)
}

func testOkHandler(w http.ResponseWriter, r *http.Request) {
	ok := gorest.NewOkJsonResponse("hello", nil, nil)
	data, _ := json.Marshal(ok)
	_, _ = w.Write(data)
}

var localholder = HttpRequest{
	Headers:               map[string][]string{"Content-Type": {"application/json"}},
	Host:                  "",
	Logger:                log.New(os.Stdout, "local: ", log.Llongfile),
	ResponseErrorStrategy: gorest.ResponseErrorStrategy,
	Label:                 "localreq",
}

func TestGoreqError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testErrorHandler))
	_, err := localholder.EnsureJSON(http.MethodGet, s.URL, nil, nil, nil)
	if err == nil {
		t.Fatal("error await")
	}
	e := err.(*porterr.PortError)
	fmt.Println(e.GetDetails())
	fmt.Println(len(e.GetDetails()))
}

func TestGoreqOk(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testOkHandler))
	resp := gorest.JsonResponse{}
	_, err := localholder.EnsureJSON(http.MethodGet, s.URL, nil, nil, &resp)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp.Message)
}

func BenchmarkErrors(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(testErrorHandler))
	//localholder.InitDefaultLogger()
	for i := 0; i < b.N; i++ {
		_, err := localholder.EnsureJSON(http.MethodGet, s.URL, nil, nil, nil)
		if err == nil {
			b.Fatal("error await")
		}
	}
	b.ReportAllocs()
}

func BenchmarkOk(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(testOkHandler))
	//resp := gorest.JsonResponse{}
	for i := 0; i < b.N; i++ {
		_, err := localholder.EnsureJSON(http.MethodGet, s.URL, nil, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
}

func BenchmarkErrorsClassic(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(testErrorHandler))
	for i := 0; i < b.N; i++ {
		_, err := http.Get(s.URL)
		if err != nil {
			b.Fatal("error await")
		}
	}
	b.ReportAllocs()
}

type PaginatorTestItem struct {
	Number int `json:"number"`
}

type PaginatorRequestForm struct {
	Name string `json:"name"`
	Paginator
}

func (p *PaginatorRequestForm) Clone() interface{} {
	var f = *p
	return &f
}

func testPaginatorHandler(w http.ResponseWriter, r *http.Request) {
	var p Paginator
	var t int64
	var total = r.URL.Query()["total"]
	if len(total) > 0 {
		t, _ = strconv.ParseInt(total[0], 10, 64)
	}
	data, _ := io.ReadAll(r.Body)
	err := json.Unmarshal(data, &p)
	if err != nil {
		gorest.NewErrorJsonResponse(porterr.New(porterr.PortErrorRequest, err.Error()))
		return
	}
	if p.Page == 0 {
		p.Page = 1
	}
	meta := gorest.Meta{
		Page:  p.Page,
		Limit: p.Limit,
		Total: int(t),
	}
	var response []PaginatorTestItem
	for i := (p.Page - 1) * p.Limit; i < p.Page*p.Limit; i++ {
		if i >= int(t) {
			continue
		}
		response = append(response, PaginatorTestItem{Number: i})
	}
	ok := gorest.NewOkJsonResponse("paginator", response, meta)
	resp, _ := json.Marshal(ok)
	_, _ = w.Write(resp)
}

func TestParallelPaginatorJsonEnsure(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testPaginatorHandler))
	var total = 125
	var page = 4
	var pCount = 10
	var limit = 13
	s.URL += fmt.Sprintf("/?total=%v", total)
	localholder.Url = s.URL
	localholder.Method = http.MethodPost
	body := PaginatorRequestForm{
		Name: "item",
		Paginator: Paginator{
			Page:          page,
			Limit:         limit,
			ParallelCount: pCount,
		},
	}
	items, meta, e := ParallelPaginatorJsonEnsure[PaginatorRequestForm, PaginatorTestItem](body, localholder)
	if e != nil {
		t.Fatal(e)
	}
	var count int
	var maxPage = total / limit
	if total%limit > 0 {
		maxPage++
	}
	if page < maxPage {
		count = total - page*limit + limit
	} else if page == maxPage {
		count = total - (page-1)*limit
	}
	if len(items) != count {
		t.Fatal("wrong item response")
	}
	for i, item := range items {
		if item.Number != i+(page-1)*limit {
			t.Fatal("wrong order")
		}
	}
	fmt.Println(items, meta)
}

// ensure BenchmarkPaginator-8   	      13323	     85158 ns/op	   42213 B/op	     109 allocs/op
// ensure json BenchmarkPaginator-12    	   18163	     64132 ns/op	   11122 B/op	     127 allocs/op
// http.Post BenchmarkPaginator-8   	      19886	     59323 ns/op	    6255 B/op	      69 allocs/op
func BenchmarkPaginator(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(testPaginatorHandler))
	var total = 125
	var page = 4
	var pCount = 10
	var limit = 13
	s.URL += fmt.Sprintf("/?total=%v", total)
	localholder.Url = s.URL
	localholder.Method = http.MethodPost
	localholder.Logger = nil
	body := PaginatorRequestForm{
		Name: "item",
		Paginator: Paginator{
			Page:          page,
			Limit:         limit,
			ParallelCount: pCount,
		},
	}
	var response []PaginatorTestItem
	for i := 0; i < b.N; i++ {
		localholder.EnsureJSON(http.MethodPost, s.URL, nil, body, &response)
	}
	b.ReportAllocs()
}

func BenchmarkLogRequest(b *testing.B) {
	localholder.Url = fmt.Sprintf("/?total=%v", 2)
	localholder.Method = http.MethodPost
	for i := 0; i < b.N; i++ {
		BuildCURL(localholder)
	}
	b.ReportAllocs()
}

// BenchmarkLogRequest-8   	 5207312	       226.3 ns/op	     136 B/op	       3 allocs/op
func TestBuildCURL(t *testing.T) {
	localholder.Url = fmt.Sprintf("/?total=%v", 2)
	localholder.Method = http.MethodPost
	curl := BuildCURL(localholder)
	t.Log(curl)
}
