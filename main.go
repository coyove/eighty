package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

var cmdListen = flag.String("listen", "", "dummy HTTP server")
var cmdHost = flag.String("host", "http://127.0.0.1:8080/", "full URL including http:// (or https://) and trailing slash")
var cmdGithub = flag.String("github", "https://github.com/coyove", "your github link")
var cmdFooter = flag.String("footer", "coyove with go80", "footer text, keep it under 80 chars")
var cmdTitle = flag.String("title", "coyove blog", "title text, keep it under 80 chars")
var cmdFontsize = flag.Int("fontsize", 14, "font size in px")

type renderOptions struct {
	title    string
	date     time.Time
	column   int
	content  string
	fontSize int
	css      string
	titleBar string
	github   string
	footer   string
	index    bool
}

func (opt *renderOptions) makeA(text, target, href string) string {
	p, t, s := CalcTag([]rune(text))
	return "<a " + target + " href='" + href + "'>" + p + t + s + "</a>"
}

func (opt *renderOptions) padToCenter(text string) []byte {
	return []byte(appendSpaces(text, opt.column-stringWidth(text), false))
}

func (opt *renderOptions) getTitleBar() string {
	const delim = "<ul><li> <li>|<li> </ul>"
	if opt.titleBar == "" {
		bar := "<div>"
		switch opt.column {
		case 40:
			bar += opt.makeA("home", "", *cmdHost+"index.m.html")
		case 80:
			bar += opt.makeA("home", "", *cmdHost)
		case 120:
			bar += opt.makeA("home", "", *cmdHost+"index.w.html")
		}
		bar += delim + opt.makeA("github", "target='_blank'", opt.github)
		bar += delim + opt.makeA("about", "", *cmdHost+"about.html") + "</div><hr>"
		opt.titleBar = bar
	}

	return opt.titleBar
}

func (opt *renderOptions) getCSS() string {
	if opt.css == "" {
		css := &bytes.Buffer{}
		css.WriteString(fmt.Sprintf("li{width:%dpx}", opt.fontSize/2))
		for i := 1; i <= opt.column; i++ {
			css.WriteString("p[")
			if i <= 26 {
				css.WriteByte(byte(i-1) + 'a')
			} else {
				j := i / 26
				css.WriteByte(byte(j) + 'a')
				css.WriteByte(byte(i-j*26) + 'a')
			}
			css.WriteString("]{width:")
			css.WriteString(fmt.Sprintf("%dpx}", i*opt.fontSize/2))
		}

		opt.css = css.String()
	}
	return opt.css
}

func (opt *renderOptions) getFooter() string {
	return Format80(opt.padToCenter(opt.footer), FormatOptions{width: opt.column})
}

func (opt *renderOptions) getHeader() string {
	titleInContent := Format80(opt.padToCenter(opt.title), FormatOptions{width: opt.column})
	dateInContent := Format80(opt.padToCenter(opt.date.Format(time.RFC3339)), FormatOptions{width: opt.column})
	if opt.index {
		dateInContent = ""
	}

	return titleInContent + dateInContent
}

func renderContent(tmpl *template.Template, path string, opt *renderOptions) error {

	os.Remove(path)
	f, _ := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0755)

	return tmpl.Execute(f, struct {
		Index      bool
		Title      string
		Text       string
		Width      int
		Column     int
		FontHeight int
		CtrlCSS    string
	}{
		opt.index,
		opt.title,
		opt.getTitleBar() + opt.getHeader() + "<div></div>" + opt.content + "<hr>" + opt.getFooter(),
		opt.column*opt.fontSize/2 + 2,
		opt.column,
		opt.fontSize,
		opt.getCSS(),
	})
}

type path_t struct {
	full   string
	mobile string
	wide   string
	title  string
}

func main() {
	flag.Parse()

	os.RemoveAll("./blog")
	os.Mkdir("./blog", 0755)

	filesMap := make(map[int][]path_t)
	filesSort := make([]int, 0)

	tmpl, _ := template.ParseFiles("temp.html")
	wg := &sync.WaitGroup{}
	throt := 0
	sp := time.Now()
	filepath.Walk("./_raw", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && strings.HasSuffix(path, ".txt") {
			if throt == 10 {
				wg.Wait()
				throt = 0
			}

			wg.Add(1)
			throt++
			go func() {
				now := time.Now()
				buf, _ := ioutil.ReadFile(path)
				fn := filepath.Base(path)
				fn = fn[:len(fn)-4]

				o := &renderOptions{}
				o.github = *cmdGithub
				o.footer = *cmdFooter
				o.fontSize = *cmdFontsize
				o.title = "<Untitled>"
				o.date = info.ModTime()

				fo := FormatOptions{linkTarget: "target='_blank'", pc: []PrefixCallback{
					PrefixCallback{
						prefix: "####", callback: func(in string) bool {
							o.title = in[4:]
							return false
						},
					},

					PrefixCallback{
						prefix: "****", callback: func(in string) bool {
							if sdate := in[4:]; sdate != "" {
								date, err := time.Parse(time.RFC3339, sdate)
								if err != nil {
									ts, err := strconv.Atoi(sdate)
									if err != nil {
										date = info.ModTime()
									} else {
										date = time.Unix(0, int64(ts))
									}
								}
								o.date = date
							}
							return false
						},
					},
				}}

				o.column = 80
				fo.width = 80
				o.content = Format80(buf, fo)

				y := o.date.Year()
				m := int(o.date.Month())

				dir := fmt.Sprintf("blog/%d/%d/", y, m)
				if fn == "about" {
					dir = "blog/"
				} else {
					os.MkdirAll(dir, 0755)
				}

				d := y*100 + m
				if filesMap[d] == nil {
					filesMap[d] = make([]path_t, 0)
					filesSort = append(filesSort, d)
				}

				p := path_t{dir + fn + ".html", dir + fn + ".m.html", dir + fn + ".w.html", o.title}
				if fn != "about" {
					filesMap[d] = append(filesMap[d], p)
				}
				renderContent(tmpl, p.full, o)
				len1 := len(o.content)

				o.column = 40
				fo.width = 40
				o.content = Format80(buf, fo)
				renderContent(tmpl, p.mobile, o)
				len2 := len(o.content)

				o.column = 120
				fo.width = 120
				o.content = Format80(buf, fo)
				renderContent(tmpl, p.wide, o)
				len3 := len(o.content)

				log.Printf("[%.3fs] %s, title: %s, write (normal) %d kb / (narrow) %d kb / (wide) %d kb",
					time.Now().Sub(now).Seconds(), path, o.title, len1/1024, len2/1024, len3/1024)
				wg.Done()
			}()
		}

		return nil
	})

	wg.Wait()
	sort.Ints(filesSort)
	index, indexm, indexw := bytes.Buffer{}, bytes.Buffer{}, bytes.Buffer{}
	filecount := 0

	for i := len(filesSort) - 1; i >= 0; i-- {
		d := filesSort[i]
		y := d / 100
		m := d - y*100

		date := fmt.Sprintf("-- %d/%d\n", y, m)

		index.WriteString(date)
		indexm.WriteString(date)
		indexw.WriteString(date)

		for _, p := range filesMap[d] {
			filecount++
			index.WriteString("  " + p.title + "\n    " + *cmdHost + p.full[5:] + " \n")
			indexm.WriteString("  " + p.title + "\n    " + *cmdHost + p.mobile[5:] + " \n")
			indexw.WriteString("  " + p.title + "\n    " + *cmdHost + p.wide[5:] + " \n")
		}
	}

	o := &renderOptions{
		title:    *cmdTitle,
		github:   *cmdGithub,
		footer:   *cmdFooter,
		index:    true,
		content:  Format80(index.Bytes(), FormatOptions{width: 80}),
		column:   80,
		fontSize: *cmdFontsize,
	}
	renderContent(tmpl, "blog/index.html", o)

	o.column = 40
	o.content = Format80(indexm.Bytes(), FormatOptions{width: 40})
	renderContent(tmpl, "blog/index.m.html", o)

	o.column = 120
	o.content = Format80(indexw.Bytes(), FormatOptions{width: 120})
	renderContent(tmpl, "blog/index.w.html", o)

	log.Println("finished generating", filecount, "files in", time.Now().Sub(sp).Seconds(), "sec")

	if *cmdListen != "" {
		http.Handle("/", http.FileServer(http.Dir("blog")))
		log.Println("listening on", *cmdListen)
		http.ListenAndServe(*cmdListen, nil)
	}
}
