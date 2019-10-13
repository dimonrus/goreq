package goreq

import (
	"github.com/dimonrus/porterr"
	"net/http/httptest"
	"testing"
	"fmt"
	"sync"
	"time"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"log"
)

//https://jsonplaceholder.typicode.com/posts
var jsonplaceholder = HttpRequest{
	Host:    "https://jsonplaceholder.typicode.com",
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
	_, err = jsonplaceholder.EnsureJSON("GET", fmt.Sprintf("/posts/%v", id), nil, nil, &p)
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
			req, err := http.NewRequest("GET", jsonplaceholder.Host+fmt.Sprintf("/post/%v", i), nil)
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

			body, err := ioutil.ReadAll(response.Body)

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
	b, err := ioutil.ReadAll(resp.Body)
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
