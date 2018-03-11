package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/coyove/eighty/kkformat"
	"github.com/coyove/eighty/static"
	"golang.org/x/crypto/acme/autocert"
)

var adminpassword = flag.String("p", "123456", "password")
var sitename = flag.String("n", "zzz.gl", "site name")
var truereferer = flag.String("r", "http://127.0.0.1:8102", "referer")
var listen = flag.String("l", ":8102", "listen address")
var production = flag.Bool("pd", false, "go production")

func checkReferer(r *http.Request) bool {
	return strings.HasPrefix(r.Referer(), *truereferer)
}

func serveHeader(w http.ResponseWriter, title string) {
	w.Header().Add("Content-Type", "text/html")
	var templ = `<!DOCTYPE html><html>
	<title>` + title + ` - ` + *sitename + `</title>
	<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=1.0, minimum-scale=1.0, maximum-scale=1.0">
	<meta charset="utf-8">
	` + static.CSS + `
	<div class=header>
	<a href=/>` + static.NewSnippet + `</a> <span class=sep>|</span>
	<a href=/list>` + static.AllSnippets + `</a>
	</div><div id=content-0>`
	w.Write([]byte(templ))
}

func serveFooter(w http.ResponseWriter) {
	s, b := bk.TotalSnippets()
	w.Write([]byte(fmt.Sprintf(`</div>
		<div class=footer>
		<span>%s</span> <span class=sep>|</span> 
		<span>%d snippets</span> <span class=sep>|</span> 
		<span>%d blocks</span> <span class=sep>|</span> 
		<span>%0.2f%% cap</span>
		</div>`, *sitename, s, b, bk.Capacity*100)))
}

func serveError(w http.ResponseWriter, r *http.Request, code int, info string) {
	w.WriteHeader(code)
	serveHeader(w, static.Error)
	rf := r.Referer()
	if rf == "" {
		rf = "/"
	}

	w.Write([]byte("<div><dl class=err>"))
	write(w, info)
	w.Write([]byte("</dl></div><div><a href='" + rf + "'><dl>"))
	write(w, static.Back)
	w.Write([]byte("</dl></a></div>"))
	serveFooter(w)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) > 1 {
		uri, raw := r.URL.Path[1:], strings.HasSuffix(r.URL.Path, ".txt")
		if raw {
			uri = uri[:len(uri)-4]
		}

		if strings.Contains(r.UserAgent(), "curl") {
			raw = true
		}

		id, _ := strconv.ParseUint(uri, 16, 64)
		if id == 0 {
			if raw {
				w.WriteHeader(400)
			} else {
				serveError(w, r, 400, static.InternalError)
			}
			return
		}

		s, err := bk.GetSnippet((id))
		if err != nil {
			if raw {
				w.WriteHeader(404)
			} else {
				serveError(w, r, 404, static.SnippetNotFound)
			}
			return
		}

		if raw {
			w.Header().Add("Content-Type", "text/plain; charset=utf8")
			w.Write([]byte(s.Raw))
			return
		}

		serveHeader(w, s.Title)
		w.Write([]byte("<h2>" + strings.Replace(s.Title, "<", "&lt;", -1) + "</h2>"))
		if kkformat.OwnSnippet(r, s) || isAdmin(r) {
			w.Write([]byte(fmt.Sprintf("<div><a href='/delete?id=%x&ts=%d'><dl>", s.ID, time.Now().UnixNano())))
			write(w, static.Delete)
			w.Write([]byte("</dl></a><dl>"))
			write(w, " > "+s.Token())
			w.Write([]byte("</dl></div>"))
		}
		writeInfo(w, s, 0)
		w.Write([]byte("<hr>"))
		s.WriteTo(w, false)
		bk.IncrSnippetViews(s.ID)
	} else {
		serveHeader(w, static.NewSnippet)
		w.Write([]byte(static.NewSnippetForm))
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
	if !checkReferer(r) {
		w.WriteHeader(403)
		return
	}

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
		serveError(w, r, 403, static.InternalError)
		return
	}

	if !kkformat.OwnSnippet(r, s) && !admin {
		serveError(w, r, 403, static.NoPermission)
		return
	}

	bk.DeleteSnippet(s)
	http.SetCookie(w, &http.Cookie{Name: fmt.Sprintf("s%x", s.ID), Expires: time.Now()})
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
				log.Println("admin deletes:", id)
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
		w.Write([]byte("<a href='/" + strconv.FormatUint(s.ID, 16) + ".txt'><dl><dt>R<dt>A<dt>W</dl></a><dl><dt> <dt>·<dt> </dl>"))
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

	serveHeader(w, static.AllSnippets)
	w.Write([]byte(`<form method=POST action=/delete>`))
	for _, s := range ss {
		id := strconv.FormatUint(s.ID, 16)
		w.Write([]byte(`<!-- ` + id + ` -->`))

		if s.ID == 0 {
			continue
		}

		w.Write([]byte("<div><input type=checkbox class=del name=s" + id + ">"))
		w.Write([]byte("<dl>"))
		write(w, fmt.Sprintf("%06x ", s.ID))
		w.Write([]byte("</dl><a target=_blank href='/" + id + "'><dl>"))
		write(w, kkformat.Trunc(s.Title, 71))
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

func trunc(in string) string {
	r := []rune(in)
	if len(r) > 32 {
		return string(r[:32])
	}
	return in
}

func servePost(w http.ResponseWriter, r *http.Request) {
	if !checkReferer(r) {
		w.WriteHeader(400)
		return
	}

	start := time.Now()
	s := &kkformat.Snippet{}

	s.Title = trunc(r.FormValue("title"))
	if s.Title == "" {
		s.Title = static.UntitledSnippet
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
		serveError(w, r, 400, static.EmptyContent)
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
	} else {
		output.(*os.File).Close()
	}

	if err := bk.AddSnippet(s); err != nil {
		log.Println(err)
		serveError(w, r, 403, static.InternalError)
		return
	}

	cookie := http.Cookie{
		Name:    fmt.Sprintf("s%x", s.ID),
		Value:   s.Token(),
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

	handlers := map[string]func(http.ResponseWriter, *http.Request){
		"/":       serveIndex,
		"/list":   serveList,
		"/post":   servePost,
		"/delete": serveDelete,
	}

	for p, h := range handlers {
		http.Handle(p, gziphandler.GzipHandler(http.HandlerFunc(h)))
	}

	if !*production {
		log.Println("server started on", *listen)
		log.Fatalln(http.ListenAndServe(*listen, nil))
	} else {
		log.Println("go production")

		*truereferer = "https://" + *sitename
		log.Println("server started on https")
		m := &autocert.Manager{
			Cache:      autocert.DirCache("secret-dir"),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(*sitename),
		}
		go http.ListenAndServe(":http", m.HTTPHandler(nil))
		s := &http.Server{
			Addr:      ":https",
			TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
		}
		log.Fatalln(s.ListenAndServeTLS("", ""))
	}
}
