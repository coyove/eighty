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
	#content-0 a{border-bottom: solid 1px;cursor:pointer;text-decoration:none}
	#content-0 a:hover{background-color:#ffa}
	#content-0 dt.conj{-webkit-touch-callout:none;-webkit-user-select:none;-khtml-user-select:none;-moz-user-select:none;-ms-user-select:none;user-select:none;color:#ccc}
	#content-0 ._image{width:auto;max-width:100%;max-height:280px}
	#content-0 .cls-toc-r-0 *{color:black}
	#content-0 dt{width:8px}
	#content-0 dd,#content-0 input.del{width:16px}
	#content-0,.header,.footer{margin:0 auto;width:642px}
	#post-form td{padding:2px;}
	#post-form, #post-form .ctrl{resize:vertical;width:100%;max-width:100%;min-width:100%}
	#post-form .title{white-space:no-wrap;width:1px;text-align:right;vertical-align:top}
	.header a,.footer a{color:white;text-decoration:none;display:inline-block;zoom:1;*display:inline;padding:0 2px}
	.header a:hover,.footer a:hover{text-decoration:underline}
	.header,.footer{background:#667;padding:4px;color:white}
	</style>
	<div class=header>
	<a href=/>new</a> |
	<a href=/list>all snippets</a>
	</div><div id=content-0>`
	w.Write([]byte(templ))
}

func serveFooter(w http.ResponseWriter) {
	w.Write([]byte("</div><div class=footer>zzz.gl</div>"))
}

func serve404(w http.ResponseWriter) {
	serveHeader(w, "404")
	w.Write([]byte("Not Found"))
	serveFooter(w)
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
		w.Write([]byte("<h2>" + s.Title + "</h2>"))
		if kkformat.OwnSnippet(r, s) || isAdmin(r) {
			w.Write([]byte(fmt.Sprintf("<a href='/delete?ui=%s'>delete</a>", s.Short)))
		}
		writeInfo(w, s, 0)
		s.WriteTo(w, false)
		bk.IncrSnippetViews(s.Short)
	} else {
		serveHeader(w, "Index")
		w.Write([]byte(`<form method=POST action=/post><table id=post-form>
		<tr>
			<td class=title>Title:</td><td><input class=ctrl name=title></td>
			<td class=title>Author:</td><td><input class=ctrl name=author></td>
		</tr>
		<tr><td colspan=4><textarea class=ctrl name=content rows=10></textarea></td></tr>
		<tr><td colspan=4>
		Link https://host/
		<input name=short width=60 value=` + strconv.FormatUint(bk.TotalSnippets()+1, 16) + `>
		expires in: <select name=ttl>
			<option value="60">60s</option>
			<option value="3600">1h</option>
			<option value="86400">1d</option>
			<option value="2592000">30d</option>
			<option value="0" selected>never</option>
		</select> 
		<input type=submit value=submit style="float:right">
		</td></tr>
		</table>
		</form>
		`))
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
	w.Write([]byte("</dl><dl>"))
	write(w, info)
	w.Write([]byte("</dl></div>"))
}

func serveList(w http.ResponseWriter, r *http.Request) {
	startPage, _ := strconv.Atoi(r.FormValue("p"))
	start := startPage * 25
	end := start + 25
	ss := bk.GetSnippetsLite(uint64(start), uint64(end))

	serveHeader(w, "List")
	// templ.Execute(w, pl)
	w.Write([]byte(`<form method=POST action=/delete>`))
	for _, s := range ss {
		if s.ID == 0 {
			continue
		}

		w.Write([]byte("<div><input type=checkbox class=del name=s" + strconv.FormatUint(s.ID, 10) + ">"))
		title := kkformat.Trunc(s.Title, 71)

		w.Write([]byte("<dl>"))
		write(w, fmt.Sprintf("%06d ", s.ID))
		w.Write([]byte("</dl><a target=_blank href='/" + s.Short + "'><dl>"))
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

var reTrunc = regexp.MustCompile(`([^A-Za-z0-9\-\_]*)`)

func trunc(in string, strict bool) string {
	var buf []rune
	if len(in) > 64 {
		buf = []rune(in)[:64]
		if !strict {
			return string(buf)
		}
	} else {
		if !strict {
			return in
		}
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
	s.Short = trunc(r.FormValue("short"), true)
	switch s.Short {
	case "post", "about", "help", "list", "delete", "edit":
		w.Write([]byte("invalid shortcut"))
		return
	}

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
		s.P80 = []byte(string(kkformat.LargeP80Magic) + fn)
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
