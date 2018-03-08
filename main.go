package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"text/template"
	"time"

	"./kkformat"
)

var helpers = template.FuncMap{
	"size": func(in int64) string {
		return fmt.Sprintf("%.2fKB", float64(in)/1024)
	},

	"date": func(in int64) string {
		return time.Unix(0, in).Format(time.RFC3339)
	},
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if len(r.RequestURI) > 1 {
		shortcut := r.RequestURI[1:]
		s, err := bk.GetSnippet(shortcut)
		if err != nil {
			log.Println(err)
			return
		}

		fontsize := 14
		now := uint64(1e18)
		halfwidth, fullwidth := fontsize/2+1, fontsize+2
		var templ = `<!DOCTYPE html>
		<html>
		<title>kk</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=1.0, minimum-scale=1.0, maximum-scale=1.0">
		<meta charset="utf-8"><div id="content-%d">
		<style>
		*{box-sizing:border-box;margin:0}
		#content-%d div{padding:1px 0;min-height:1em;margin:0;max-height:280px;width:100%%;line-height:1.5}
		#content-%d dl,
		#content-%d dt,
		#content-%d dd{display:inline-block;zoom:1;*display:inline;white-space:pre;padding:0;margin:0;font-family:consolas,monospace;text-align:center;text-decoration:none}
		#content-%d a{border-bottom: solid 1px;cursor:pointer}
		#content-%d a:hover{background-color:#ffa}
		#content-%d dt.conj{-webkit-touch-callout:none;-webkit-user-select:none;-khtml-user-select:none;-moz-user-select:none;-ms-user-select:none;user-select:none;color:#ccc}
		#content-%d ._image{width:auto;max-width:100%%;max-height:280px}
		#content-%d .cls-toc-r-%d *{color:black}
		#content-%d dt{width:%dpx}
		#content-%d dd{width:%dpx}
		#content-%d {margin:0 auto;width:%dpx;font-size:%dpx}
		</style>
		`

		getTempl := func(columns int) string {
			return fmt.Sprintf(templ, []interface{}{
				now, now, now, now, now, now, now, now, now,
				now, now,
				now, halfwidth,
				now, fullwidth,
				now, halfwidth*columns + 1, fontsize,
			}...)
		}

		w.Write([]byte(getTempl(80)))
		s.WriteTo(w, false)
		w.Write([]byte("</div>"))
	} else {
		w.Write([]byte(`<!DOCTYPE HTML>
		<form method=POST action=/post>
		short:<input name=short>
		title:<input name=title>
		author:<input name=author>
		<textarea name=content rows=10></textarea>
		<input type=submit value=submit>
		</form>
		`))
	}
}

func serveList(w http.ResponseWriter, r *http.Request) {
	startPage, _ := strconv.Atoi(r.FormValue("p"))
	start := startPage * 50
	end := start + 50
	ss := bk.GetSnippetsLite(uint64(start), uint64(end))
	templ, _ := template.New("zzz").Funcs(helpers).Parse(`
		<table>
		{{range .Data}}
			<tr>
				{{if .Dead}}<td>-</td>{{else}}
				<td>{{.ID}}</td>
				<td><a href="/{{.Short}}" target=_blank>{{.Title}}</a></td>
				<td>{{size .Size}}</td>
				<td>{{.Author}}</td>
				<td>{{date .Time}}</td>
				{{end}}
			</tr>
		{{end}}
		</table>

		{{range .Pages}}
		<a href="?p={{.}}">{{.}}</a>
		{{end}}`)

	pl := struct {
		Data  []*kkformat.Snippet
		Pages []int
	}{Data: ss, Pages: make([]int, 0, 10)}

	for p := startPage - 5; p <= startPage+5; p++ {
		if p >= 0 {
			pl.Pages = append(pl.Pages, p)
		}
	}

	templ.Execute(w, pl)
}

func servePOST(w http.ResponseWriter, r *http.Request) {
	s := &kkformat.Snippet{}
	s.Short = r.FormValue("short")
	switch s.Short {
	case "post", "about", "help", "list":
		w.Write([]byte("invalid shortcut"))
		return
	}

	s.Title = r.FormValue("title")
	if s.Title == "" {
		s.Title = "Untitled"
	}

	s.Author = r.FormValue("author")
	if s.Author == "" {
		s.Author = "N/A"
	}

	s.Raw = r.FormValue("content")
	if len(s.Raw) > 1024*1024 {
		s.Raw = s.Raw[:1024*1024]
	}

	largeContent := len(s.Raw) > 102400

	fo := &kkformat.Formatter{Source: []byte(s.Raw)}

	// columns, _ := strconv.Atoi(r.FormValue("columns"))
	// if columns < 40 || columns > 200 {
	// 	columns = 80
	// }
	// fo.Columns = uint32(columns)

	fontsize, _ := strconv.Atoi(r.FormValue("fontsize"))
	if fontsize <= 0 {
		fontsize = 14
	}

	// now := time.Now().UnixNano()
	fo.ID = 1e18

	var output40, output80 io.Writer
	if largeContent {
		nn := time.Now().UnixNano()
		os.MkdirAll(fmt.Sprintf("larges/%d", nn/3600e9), 0777)
		fn := fmt.Sprintf("larges/%d/%d", nn/3600e9, nn)

		var err error
		output40, err = os.Create(fn + "-40")
		s.P40 = []byte("ex:" + fn + "-40")
		if err != nil {
			log.Fatalln(err)
		}

		output80, err = os.Create(fn + "-80")
		s.P80 = []byte("ex:" + fn + "-80")
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		output40, output80 = &bytes.Buffer{}, &bytes.Buffer{}
	}

	fo.Columns = 40
	fo.WriteTo(output40)
	fo.Columns = 80
	s.Size, _ = fo.WriteTo(output80)

	if !largeContent {
		s.P40 = output40.(*bytes.Buffer).Bytes()
		s.P80 = output80.(*bytes.Buffer).Bytes()
	}

	if err := bk.AddSnippet(s); err != nil {
		log.Println(err)
		return
	}

	w.Header().Add("Location", "/"+s.Short)
	w.WriteHeader(301)
}

var bk *kkformat.Backend

func main() {
	bk = &kkformat.Backend{}
	bk.Init("main.db")
	os.MkdirAll("larges", 0777)

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/list", serveList)
	http.HandleFunc("/post", servePOST)
	fmt.Println("serve")
	http.ListenAndServe(":8102", nil)
}
