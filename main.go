package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"./kkformat"
)

var adminpassword = flag.String("p", "123456", "password")

func serveHeader(w http.ResponseWriter, title string) {
	w.Header().Add("Content-Type", "text/html")
	var templ = `<!DOCTYPE html><html>
	<title>` + title + `</title>
	<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=1.0, minimum-scale=1.0, maximum-scale=1.0">
	<meta charset="utf-8">
	<style>
	*{box-sizing:border-box;margin:0}
	body{font-size:14px;font-family:Arial,Helvetica,sans-serif}
	#content-0 div{padding:1px 0;min-height:1em;margin:0;max-height:280px;width:100%;line-height:1.5}
	#content-0 dl,
	#content-0 dt,
	#content-0 dd{display:inline-block;zoom:1;*display:inline;white-space:pre;padding:0;margin:0;font-family:consolas,monospace;text-align:center;text-decoration:none}
	#content-0 a{border-bottom: solid 1px;cursor:pointer}
	#content-0 a:hover{background-color:#ffa}
	#content-0 dt.conj{-webkit-touch-callout:none;-webkit-user-select:none;-khtml-user-select:none;-moz-user-select:none;-ms-user-select:none;user-select:none;color:#ccc}
	#content-0 ._image{width:auto;max-width:100%;max-height:280px}
	#content-0 .cls-toc-r-0 *{color:black}
	#content-0 dt{width:8px}
	#content-0 dd{width:16px}
	#content-0 {margin:0 auto;width:641px}
	</style>
	<header>
	<a href=/>home</a>
	<a href=/list>list</a>
	</header>`
	w.Write([]byte(templ))
}

func serve404(w http.ResponseWriter) {
	serveHeader(w, "404")
	w.Write([]byte("Not Found"))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if len(r.RequestURI) > 1 {
		shortcut := r.RequestURI[1:]
		s, err := bk.GetSnippet(shortcut)
		if err != nil {
			serve404(w)
			return
		}

		serveHeader(w, s.Title)
		w.Write([]byte("<div id=content-0><h2>" + s.Title + "</h2>"))
		if kkformat.OwnSnippet(r, s) || isAdmin(r) {
			w.Write([]byte(fmt.Sprintf("<a href='/delete?ui=%s'>del</a>", s.Short)))
		}
		s.WriteTo(w, false)
		w.Write([]byte("</div>"))
		bk.IncrSnippetViews(s.Short)
	} else {
		serveHeader(w, "Index")
		w.Write([]byte(`
		<form method=POST action=/post>
		short:<input name=short>
		title:<input name=title>
		author:<input name=author>
		lifetime:<select name=ttl>
			<option value="60">60s</option>
			<option value="3600">1h</option>
			<option value="86400">1d</option>
			<option value="2592000">1m</option>
			<option value="0" selected>perm</option>
	  	</select>
		<textarea name=content rows=10></textarea>
		<input type=submit value=submit>
		</form>
		`))
	}
}

func isAdmin(r *http.Request) bool {
	if c, err := r.Cookie("admin"); err != nil || c.Value != *adminpassword {
		return false
	}
	return true
}

func serveDelete(w http.ResponseWriter, r *http.Request) {
	var s *kkformat.Snippet
	var err error

	short := r.FormValue("ui")
	admin := isAdmin(r)

	if short == "" {
		goto ADMIN
	}

	s, err = bk.GetSnippet(short)
	if err != nil || s == nil {
		w.WriteHeader(403)
		return
	}

	if !kkformat.OwnSnippet(r, s) && !admin {
		w.WriteHeader(403)
		return
	}

	bk.DeleteSnippet(s)
	w.Header().Add("Location", "/"+s.Short)
	w.WriteHeader(301)
	return
ADMIN:
	if !admin {
		w.WriteHeader(403)
		return
	}

	r.ParseForm()
	ids := make([]uint64, 0, 10)
	for k := range r.PostForm {
		if strings.HasPrefix(k, "s") {
			id, _ := strconv.ParseUint(k[1:], 10, 64)
			if id > 0 {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) > 0 {
		bk.DeleteSnippets(ids...)
	}

	w.Header().Add("Location", r.Referer())
	w.WriteHeader(301)
}

func serveList(w http.ResponseWriter, r *http.Request) {
	startPage, _ := strconv.Atoi(r.FormValue("p"))
	start := startPage * 50
	end := start + 50
	ss := bk.GetSnippetsLite(uint64(start), uint64(end))
	templ, _ := template.New("zzz").Funcs(kkformat.Helpers).Parse(`
		<form method=POST action=/delete>
		<table>
		{{range .Data}}
			<tr>
				{{if .Dead}}<td>-</td>{{else}}
				<td><input type=checkbox name=s{{.ID}} del></td>
				<td>{{.ID}}</td>
				<td><a href="/{{.Short}}" target=_blank>{{.Title}}</a></td>
				<td>{{size .Size}}</td>
				<td>{{.Author}}</td>
				<td>{{date .Time}}</td>
				<td>{{expire .Time .TTL}}</td>
				<td>{{.Views}}</td>
				<td></td>
				{{end}}
			</tr>
		{{end}}
		</table>

		{{if .Admin}}
		<input type=submit value=del>
		<a href=# onclick="document.querySelectorAll('input[del]').forEach(function(e){e.checked=!e.checked})">select</a>
		{{end}}
		</form>

		{{range .Pages}}
		{{if eq . $.Page}}
		[{{.}}]
		{{else}}
		<a href="?p={{.}}">{{.}}</a>
		{{end}}
		{{end}}`)

	pl := struct {
		Data  []*kkformat.Snippet
		Pages []int
		Page  int
		Admin bool
	}{
		Data:  ss,
		Pages: make([]int, 0, 10),
		Page:  startPage,
		Admin: isAdmin(r),
	}

	for p := startPage - 5; p <= startPage+5; p++ {
		if p >= 0 {
			pl.Pages = append(pl.Pages, p)
		}
	}

	serveHeader(w, "List")
	templ.Execute(w, pl)
}

var reTrunc = regexp.MustCompile(`([^A-Za-z0-9\-\_]*)`)

func trunc(in string) string {
	var buf []rune
	if len(in) > 64 {
		buf = []rune(in)[:64]
	} else {
		buf = []rune(in)
	}

	for i, r := range buf {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '@' {

		} else {
			buf[i] = '_'
		}
	}

	return string(buf)
}

func servePost(w http.ResponseWriter, r *http.Request) {
	s := &kkformat.Snippet{}
	s.Short = trunc(r.FormValue("short"))
	switch s.Short {
	case "post", "about", "help", "list", "delete", "edit":
		w.Write([]byte("invalid shortcut"))
		return
	}

	s.Title = trunc(r.FormValue("title"))
	if s.Title == "_" {
		s.Title = "Untitled"
	}

	s.Author = trunc(r.FormValue("author"))
	if s.Author == "" {
		s.Author = "N/A"
	}

	if ttl, _ := strconv.Atoi(r.FormValue("ttl")); ttl != 0 {
		s.TTL = int64(ttl)
	}

	s.Raw = r.FormValue("content")
	if len(s.Raw) > 1024*1024 {
		s.Raw = s.Raw[:1024*1024]
	} else if len(s.Raw) == 0 {
		return
	}

	largeContent := len(s.Raw) > 102400

	fo := &kkformat.Formatter{Source: []byte(s.Raw)}

	fontsize, _ := strconv.Atoi(r.FormValue("fontsize"))
	if fontsize <= 0 {
		fontsize = 14
	}

	// now := time.Now().UnixNano()
	fo.ID = 0
	var output io.Writer
	var err error
	if largeContent {
		nn := time.Now().UnixNano()
		os.MkdirAll(fmt.Sprintf("larges/%d", nn/3600e9), 0777)
		fn := fmt.Sprintf("larges/%d/%d", nn/3600e9, nn)
		output, err = os.Create(fn)
		s.P80 = []byte("ex:" + fn)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		output = &bytes.Buffer{}
	}

	fo.Columns = 80
	s.Size, _ = fo.WriteTo(output)

	if !largeContent {
		s.P80 = output.(*bytes.Buffer).Bytes()
	}

	if err := bk.AddSnippet(s); err != nil {
		log.Println(err)
		return
	}

	cookie := http.Cookie{
		Name:    fmt.Sprintf("s%d", s.ID),
		Value:   fmt.Sprintf("%x", s.GUID),
		Expires: time.Now().Add(365 * 24 * time.Hour),
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/"+s.Short, 301)
}

var bk *kkformat.Backend

func main() {
	flag.Parse()

	bk = &kkformat.Backend{}
	bk.Init("main.db")
	os.MkdirAll("larges", 0777)

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/list", serveList)
	http.HandleFunc("/post", servePost)
	http.HandleFunc("/delete", serveDelete)

	fmt.Println("serve")
	http.ListenAndServe(":8102", nil)
}
