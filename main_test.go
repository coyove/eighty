package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"
)

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

func gen() string {
	ln := r.Intn(4096) + 4096
	buf := make([]byte, ln)
	for i := 0; i < ln; i++ {
		buf[i] = byte(r.Intn(26) + 97)
	}
	return string(buf)
}

func TestNewSnippet(t *testing.T) {
	post := func(wg *sync.WaitGroup) {

		form := url.Values{}
		form.Add("title", time.Now().Format(time.RFC3339))
		form.Add("content", gen())
		req, _ := http.NewRequest("POST", "http://127.0.0.1:8102/post", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Referer", "http://127.0.0.1:8102")
		resp, _ := http.DefaultClient.Do(req)
		if resp != nil && resp.StatusCode != 200 {
			t.Errorf("invalid response: %d", resp.StatusCode)
		}
		resp.Body.Close()
		wg.Done()
	}

	del := func(idx int, wg *sync.WaitGroup) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:8102/delete?id=%x", idx), nil)
		req.Header.Add("Cookie", "admin=123456")
		req.Header.Set("Referer", "http://127.0.0.1:8102")
		resp, _ := http.DefaultClient.Do(req)
		if resp.StatusCode != 200 {
			t.Errorf("invalid response: %d", resp.StatusCode)
		}
		resp.Body.Close()
		wg.Done()
	}

	_ = del
	_ = post

	for c := 0; c < 40; c++ {
		wg := &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			post(wg)

			// wg.Add(1)
			// del(c*100+i, wg)
		}
		wg.Wait()
	}
}
