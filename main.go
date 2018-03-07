package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"text/template"
	"time"

	"./format80"
)

var cmdListen = flag.String("listen", "", "dummy HTTP server")
var cmdGithub = flag.String("github", "https://github.com/coyove", "your github link")
var cmdFooter = flag.String("footer", "coyove with go80", "footer text, keep it under 80 chars")
var cmdTitle = flag.String("title", "coyove blog", "title text, keep it under 80 chars")
var cmdFontsize = flag.Int("fontsize", 14, "font size in px")
var cmdTest = flag.String("f", "", "single file only")

type renderOptions struct {
	title    string
	date     time.Time
	column   int
	content  string
	fontSize int
	titleBar string
	github   string
	footer   string
	pageType string
}

func (opt *renderOptions) makeA(text, target, href string) string {
	t := format80.CalcTag([]rune(text))
	return "<a " + target + " href='" + href + "'><dl>" + t + "</dl></a>"
}

func (opt *renderOptions) padToCenter(text string) []byte {
	return []byte("")
}

func (opt *renderOptions) getTitleBar() string {
	const delim = "<dl><dt> <dt>|<dt> </dl>"

	bar := "<div>"
	pre := "../../"
	switch opt.pageType {
	case "index", "about":
		pre = "./"
	}
	switch opt.column {
	case 40:
		bar += opt.makeA("home", "", pre+"index.m.html")
		bar += delim + opt.makeA("about", "", pre+"about.m.html")
	case 80:
		bar += opt.makeA("home", "", pre)
		bar += delim + opt.makeA("about", "", pre+"about.html")
	case 120:
		bar += opt.makeA("home", "", pre+"index.w.html")
		bar += delim + opt.makeA("about", "", pre+"about.w.html")
	}
	bar += delim + opt.makeA("github", "target='_blank'", opt.github) + "</div><hr>"

	return bar
}

func (opt *renderOptions) getFooter() string {
	return "" //Format80(opt.padToCenter(opt.footer), &FormatOptions{width: opt.column})
}

func (opt *renderOptions) getHeader() string {
	titleInContent := "" //Format80(opt.padToCenter(opt.title), &FormatOptions{width: opt.column})
	dateInContent := ""  // Format80(opt.padToCenter(opt.date.Format(time.RFC3339)), &FormatOptions{width: opt.column})
	if opt.pageType == "index" {
		dateInContent = ""
	}

	return titleInContent + dateInContent
}

func renderContent(tmpl *template.Template, path string, opt *renderOptions) error {

	os.Remove(path)
	f, _ := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0755)
	w := opt.fontSize/2 + 1
	return tmpl.Execute(f, struct {
		Index         bool
		Title         string
		Text          string
		Column        int
		FontHeight    int
		FontWidth     int
		WideFontWidth int
		Width         int
	}{
		opt.pageType == "index",
		opt.title,
		opt.getTitleBar() + opt.getHeader() + "<div></div>" + opt.content + "<hr>" + opt.getFooter(),
		opt.column,
		opt.fontSize,
		w,
		w * 2,
		opt.column*w + 1,
	})
}

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

	buf, _ := ioutil.ReadFile("_raw/unicode3.2_test.txt")
	fo := &format80.Formatter{LinkTarget: "target='_blank'", Source: buf}
	fo.Columns = 80

	now := time.Now().UnixNano()
	fo.ID = now
	f, _ := os.Create("index.html")
	f.WriteString(fmt.Sprintf(CSS, []interface{}{
		now, now, now, now, now, now, now, now, now,
		now, now,
		now, *cmdFontsize/2 + 1,
		now, *cmdFontsize + 2,
		now, (*cmdFontsize/2+1)*fo.Columns + 1, *cmdFontsize,
	}...))
	fo.WriteTo(f)
	f.WriteString("</div>")
}
