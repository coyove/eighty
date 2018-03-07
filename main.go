package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"./kkformat"
)

var cmdListen = flag.String("listen", "", "dummy HTTP server")
var cmdGithub = flag.String("github", "https://github.com/coyove", "your github link")
var cmdFooter = flag.String("footer", "coyove with go80", "footer text, keep it under 80 chars")
var cmdTitle = flag.String("title", "coyove blog", "title text, keep it under 80 chars")
var cmdFontsize = flag.Int("fontsize", 14, "font size in px")
var cmdTest = flag.String("f", "", "single file only")

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

func main() {
	flag.Parse()

	buf, _ := ioutil.ReadFile("_raw/rekuiemu.txt")
	fo := &kkformat.Formatter{LinkTarget: "target='_blank'", Source: buf}
	fo.Columns = 80

	now := time.Now().UnixNano()
	fo.ID = now
	f, _ := os.Create("index.html")
	f.WriteString(fmt.Sprintf(CSS, []interface{}{
		now, now, now, now, now, now, now, now, now,
		now, now,
		now, *cmdFontsize/2 + 1,
		now, *cmdFontsize + 2,
		now, (*cmdFontsize/2+1)*int(fo.Columns) + 1, *cmdFontsize,
	}...))
	fo.WriteTo(f)
	f.WriteString("</div>")
	fmt.Println((time.Now().UnixNano() - now) / 1e6)
}
