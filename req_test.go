package goreq

import (
	"encoding/json"
	"testing"
	"time"
	"fmt"
	"sync"
)

//https://jsonplaceholder.typicode.com/posts
var Jsonplaceholder = HttpRequest{
	Host:         "https://jsonplaceholder.typicode.com",
	Headers:      map[string][]string{"Content-Type": {"application/json"}},
	Label:        "Jsonplaceholder",
}

var BadJsonplaceholder = HttpRequest{
	Host:         "https://jsdsf.wdsf",
	Headers:      map[string][]string{"Content-Type": {"application/json"}},
	Label:        "BadJsonplaceholder",
	RetryCount:   2,
	RetryTimeout: time.Duration(time.Millisecond*100),
}

type Post struct {
	Id     int    `json:"id"`
	UserId int    `json:"userId"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

func GeBadPosts() (posts []Post, err error) {
	service := BadJsonplaceholder
	service.Method = "GET"
	service.Url = "/posts"
	_, bytes, err := Ensure(service)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &posts)
	if err != nil {
		return nil, nil
	}
	return
}

func GetPosts() (posts []Post, err error) {
	service := Jsonplaceholder
	service.Method = "GET"
	service.Url = "/posts"
	_, bytes, err := Ensure(service)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &posts)
	if err != nil {
		return nil, nil
	}
	return
}

func GetPost(id int) (post *Post, err error) {
	service := Jsonplaceholder
	service.Method = "GET"
	service.Url = fmt.Sprintf("/posts/%v", id)
	_, bytes, err := Ensure(service)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &post)
	if err != nil {
		return nil, nil
	}
	return
}

func TestEnsure(t *testing.T) {
	posts, err := GetPosts()
	if err != nil {
		t.Error(err)
	}
	fmt.Print("Post lenght: ", len(posts), "\n")
	countOfPosts := 10

	postChan := make(chan Post, countOfPosts)
	wg := sync.WaitGroup{}
	wg.Add(countOfPosts)
	for i := 0; i < countOfPosts; i++ {
		go func(index int) {
			post, err := GetPost(index)
			if err != nil {
				return
			}
			postChan <- *post
			wg.Done()
		}(i)
	}
	wg.Wait()
	fmt.Print(len(postChan))

	posts, err = GeBadPosts()
	if err == nil {
		t.Error("Must be an error")
	}
}
