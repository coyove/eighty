package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"./kkformat"
	"./static"
)

var adminpassword = flag.String("p", "123456", "password")

func serveHeader(w http.ResponseWriter, title string) {
	w.Header().Add("Content-Type", "text/html")
	var templ = `<!DOCTYPE html><html>
	<title>` + title + `</title>
	<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=1.0, minimum-scale=1.0, maximum-scale=1.0">
	<meta charset="utf-8">
	` + static.CSS + `
	<div class=header>
	<a href=/>new snippet</a> <span class=sep>|</span>
	<a href=/list>all snippets</a>
	</div><div id=content-0>`
	w.Write([]byte(templ))
}

func serveFooter(w http.ResponseWriter) {
	w.Write([]byte("</div><div class=footer><span>zzz.gl</span> <span class=sep>|</span> <span>" +
		strconv.FormatUint(bk.TotalSnippets(), 10) + " snippets</span></div>"))
}

func serveError(w http.ResponseWriter, r *http.Request, info string) {
	serveHeader(w, "Error")
	rf := r.Referer()
	if rf == "" {
		rf = "/"
	}

	w.Write([]byte("<div><dl class=err>"))
	write(w, info)
	w.Write([]byte("</dl></div><div><a href='" + rf + "'><dl>"))
	write(w, "back")
	w.Write([]byte("</dl></a></div>"))
	serveFooter(w)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if len(r.RequestURI) > 1 {
		uri := r.RequestURI[1:]
		qm := strings.Index(uri, "?")
		if qm > -1 {
			uri = uri[:qm]
		}

		id, _ := strconv.ParseUint(uri, 16, 64)
		if id == 0 {
			w.WriteHeader(404)
			serveError(w, r, "Invalid Snippet ID")
			return
		}

		s, err := bk.GetSnippet((id))
		if err != nil {
			w.WriteHeader(404)
			serveError(w, r, "Snippet Not Found")
			return
		}

		if r.FormValue("raw") == "1" {
			w.Header().Add("Content-Type", "text/plain; charset=utf8")
			w.Write([]byte(s.Raw))
			return
		}

		serveHeader(w, s.Title)
		w.Write([]byte("<h2>" + s.Title + "</h2>"))
		if kkformat.OwnSnippet(r, s) || isAdmin(r) {
			w.Write([]byte(fmt.Sprintf("<div><a href='/delete?id=%x'><dl><dt>d<dt>e<dt>l<dt>e<dt>t<dt>e</dl></a></div>", s.ID)))
		}
		writeInfo(w, s, 0)
		w.Write([]byte("<hr>"))
		s.WriteTo(w, false)
		bk.IncrSnippetViews(s.ID)
	} else {
		serveHeader(w, "New Snippet")
		w.Write([]byte(static.NewSnippet))
	}
	serveFooter(w)
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
	start := time.Now()
	id, _ := strconv.ParseUint(r.FormValue("id"), 16, 64)
	admin := isAdmin(r)

	if id == 0 {
		goto ADMIN
	}

	s, err = bk.GetSnippet(id)
	if err != nil || s == nil {
		w.WriteHeader(403)
		serveError(w, r, "You don't have the permission to delete this snippet")
		return
	}

	if !kkformat.OwnSnippet(r, s) && !admin {
		w.WriteHeader(403)
		serveError(w, r, "You don't have the permission to delete this snippet")
		return
	}

	bk.DeleteSnippet(s)
	http.Redirect(w, r, "/"+strconv.FormatUint(id, 16), 301)
	log.Println("delete:", time.Now().Sub(start).Nanoseconds()/1e6, "ms")
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
			id, _ := strconv.ParseUint(k[1:], 16, 64)
			if id > 0 {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) > 0 {
		bk.DeleteSnippets(ids...)
	}

	http.Redirect(w, r, r.Referer(), 301)
}

func write(w http.ResponseWriter, in string) {
	for _, r := range in {
		if kkformat.RuneWidth(r) == 1 {
			w.Write([]byte("<dt>"))
		} else {
			w.Write([]byte("<dd>"))
		}
		w.Write([]byte(string(r)))
	}
}

func writeInfo(w http.ResponseWriter, s *kkformat.Snippet, leftPadding uint32) {
	var h, m, sec int64
	if s.TTL > 0 {
		rem := s.Time + s.TTL*1e9 - time.Now().UnixNano()
		if rem < 0 {
			h = 0
			m = 0
			sec = 0
		} else {
			rem = rem / 1e9
			h = rem / 3600
			m = rem/60 - h*60
			sec = rem - h*3600 - m*60
		}
	} else {
		h = 99
		m = 99
		sec = 99
	}

	info := fmt.Sprintf("%.2fKB · %s (%02d:%02d:%02d) · %d",
		float64(s.Size)/1024,
		time.Unix(0, s.Time).Format("2006-01-02 15:04:05"),
		h, m, sec,
		s.Views)

	author := "@" + kkformat.Trunc(s.Author, 10)
	gap := 80 - kkformat.StringWidth(info) - kkformat.StringWidth(author) - leftPadding
	rawButton := leftPadding == 0
	if rawButton {
		gap -= 6
	}

	if gap > 80 {
		gap = 0
	}

	w.Write([]byte("<div><dl>"))
	for i := uint32(0); i < leftPadding; i++ {
		w.Write([]byte("<dt> "))
	}
	w.Write([]byte("</dl><dl>"))
	write(w, author)
	w.Write([]byte("</dl><dl>"))
	for i := uint32(0); i < gap; i++ {
		w.Write([]byte("<dt> "))
	}
	w.Write([]byte("</dl>"))
	if rawButton {
		w.Write([]byte("<a href='?raw=1'><dl><dt>R<dt>A<dt>W</dl></a><dl><dt> <dt>·<dt> </dl>"))
	}
	w.Write([]byte("<dl>"))
	write(w, info)
	w.Write([]byte("</dl></div>"))
}

func serveList(w http.ResponseWriter, r *http.Request) {
	startPage, _ := strconv.Atoi(r.FormValue("p"))
	start := startPage * 25
	end := start + 25
	ss := bk.GetSnippetsLite(uint64(start), uint64(end))

	serveHeader(w, "All Snippets")
	w.Write([]byte(`<form method=POST action=/delete>`))
	for _, s := range ss {
		id := strconv.FormatUint(s.ID, 16)
		w.Write([]byte(`<!-- ` + id + ` -->`))

		if s.ID == 0 {
			continue
		}

		w.Write([]byte("<div><input type=checkbox class=del name=s" + id + ">"))
		title := kkformat.Trunc(s.Title, 71)

		w.Write([]byte("<dl>"))
		write(w, fmt.Sprintf("%06x ", s.ID))
		w.Write([]byte("</dl><a target=_blank href='/" + id + "'><dl>"))
		write(w, title)
		w.Write([]byte("</dl></a>"))
		w.Write([]byte("</div>"))

		writeInfo(w, s, 9)
	}

	w.Write([]byte("<div><dl>"))
	for p := startPage - 3; p <= startPage+3; p++ {
		if p < 0 {
			continue
		}

		if p != startPage {
			w.Write([]byte(fmt.Sprintf("</dl><a href='?p=%d'><dl>", p)))
		}

		write(w, fmt.Sprintf("[ %d ]", p))

		if p != startPage {
			w.Write([]byte("</dl></a><dl>"))
		}
	}
	w.Write([]byte("</dl></div>"))

	if isAdmin(r) {
		w.Write([]byte("<input type=submit value=delete>"))
	}

	w.Write([]byte("</form>"))
	serveFooter(w)
}

func trunc(in string, strict bool) string {
	if len(in) > 32 {
		return string([]rune(in)[:32])
	}
	return in
}

func servePost(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s := &kkformat.Snippet{}

	s.Title = trunc(r.FormValue("title"), false)
	if s.Title == "" {
		s.Title = "Untitled"
	}

	s.Author = trunc(r.FormValue("author"), false)
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
		w.WriteHeader(400)
		serveError(w, r, "Empty Content")
		return
	}

	largeContent := len(s.Raw) > 102400

	fo := &kkformat.Formatter{Source: []byte(s.Raw), ID: 0, Columns: 80}

	fontsize, _ := strconv.Atoi(r.FormValue("fontsize"))
	if fontsize <= 0 {
		fontsize = 14
	}

	var output io.Writer
	var err error
	if largeContent {
		nn := time.Now().UnixNano()
		os.MkdirAll(fmt.Sprintf("larges/%d", nn/3600e9), 0777)
		fn := fmt.Sprintf("larges/%d/%d", nn/3600e9, nn)
		output, err = os.Create(fn)
		s.P80 = []byte(string(kkformat.LargeP80Magic) + fn)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		output = &bytes.Buffer{}
	}

	s.Size, _ = fo.WriteTo(output)
	if !largeContent {
		s.P80 = output.(*bytes.Buffer).Bytes()
	}

	if err := bk.AddSnippet(s); err != nil {
		log.Println(err)
		w.WriteHeader(400)
		serveError(w, r, "Internal Error")
		return
	}

	cookie := http.Cookie{
		Name:    fmt.Sprintf("s%x", s.ID),
		Value:   fmt.Sprintf("%x", s.GUID),
		Expires: time.Now().Add(365 * 24 * time.Hour),
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/"+strconv.FormatUint(s.ID, 16), 301)
	log.Println("post:", time.Now().Sub(start).Nanoseconds()/1e6, "ms")
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
