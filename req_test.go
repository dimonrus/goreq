package goreq

import (
	"encoding/json"
	"testing"
	"fmt"
)

//https://jsonplaceholder.typicode.com/posts
var Jsonplaceholder = HttpRequest{
	Host:    "https://jsonplaceholder.typicode.comx",
	Headers: map[string]string{"Content-Type": "application/json"},
	Label: "Jsonplaceholder",
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
	service.Url = "/post"
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
	fmt.Print(posts)
}
