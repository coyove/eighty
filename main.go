package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"./kkformat"
)

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<!DOCTYPE HTML>
		<form method=POST action=/post>
		<textarea name=content rows=10></textarea>
		<input type=submit value=submit>
		</form>
		`))
}

func servePOST(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	if len(content) > 1024*1024 {
		content = content[:1024*1024]
	}

	fo := &kkformat.Formatter{Source: []byte(content)}
	fmt.Println(fo.Source)

	columns, _ := strconv.Atoi(r.FormValue("columns"))
	if columns < 40 || columns > 200 {
		columns = 80
	}
	fo.Columns = uint32(columns)

	fontsize, _ := strconv.Atoi(r.FormValue("fontsize"))
	if fontsize <= 0 {
		fontsize = 14
	}

	now := time.Now().UnixNano()
	fo.ID = now

	const CSS = `<div id="content-%d">
<style>
#content-%d div{padding:1px 0;min-height:1em;margin:0;max-height:280px;width:100%%;}
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

	halfwidth, fullwidth := fontsize/2+1, fontsize+2

	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(fmt.Sprintf(CSS, []interface{}{
		now, now, now, now, now, now, now, now, now,
		now, now,
		now, halfwidth,
		now, fullwidth,
		now, halfwidth*columns + 1, fontsize,
	}...)))
	fo.WriteTo(w)
	w.Write([]byte("</div>"))
}

func main() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/post", servePOST)
	fmt.Println("serve")
	http.ListenAndServe(":8102", nil)
}
