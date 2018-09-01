package goreq

import (
	"encoding/json"
	"testing"
	"time"
	"fmt"
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

func TestEnsure(t *testing.T) {
	posts, err := GetPosts()
	if err != nil {
		t.Error(err)
	}
	fmt.Print("Post lenght: ", len(posts), "\n")
}
