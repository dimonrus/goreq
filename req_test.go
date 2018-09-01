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
	Headers:      map[string]string{"Content-Type": "application/json"},
	Label:        "Jsonplaceholder",
	RetryCount:   2,
	RetryTimeout: time.Duration(time.Second),
}

type Post struct {
	Id     int    `json:"id"`
	UserId int    `json:"userId"`
	Title  string `json:"title"`
	Body   string `json:"body"`
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

func GetPost(id int) (post *Post, err error)  {
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

	postChan := make(chan Post, 10)
	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
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
}
