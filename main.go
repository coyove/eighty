package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/font"

	"github.com/NYTimes/gziphandler"
	"github.com/coyove/eighty/drawerpool"
	"github.com/coyove/eighty/kkformat"
	"github.com/coyove/eighty/static"
	"github.com/coyove/goflyway/pkg/lru"
	"golang.org/x/crypto/acme/autocert"
)

var adminpassword = flag.String("p", "123456", "password")
var sitename = flag.String("n", "png.cat", "site name")
var truereferer = flag.String("r", "http://127.0.0.1:8102", "referer")
var listen = flag.String("l", ":8102", "listen address")
var production = flag.Bool("pd", false, "go production")

const (
	rawmaxsize = 512 * 1024
	cooldown   = 60
	imgW       = 756
)

var (
	s1theme               = color.RGBA{0xf6, 0xf7, 0xeb, 255}
	largeDrawer           *drawerpool.Pool
	smallDrawer           *drawerpool.Pool
	smallPalette, smallBG = kkformat.GetTheme(kkformat.WhiteTheme)
	smallTemplate         = image.NewPaletted(image.Rect(0, 0, imgW, 200), smallPalette)
	smallTemplateS1       = image.NewPaletted(image.Rect(0, 0, imgW, 200), append(smallPalette, s1theme))
	smallCache            = lru.NewCache(1024)
	simpleEscaper         = regexp.MustCompile(`([\\\/][nstlhp]|\s)`)
)

func init() {
	draw.Draw(smallTemplate, smallTemplate.Bounds(), smallBG, image.ZP, draw.Src)
	draw.Draw(smallTemplateS1, smallTemplateS1.Bounds(), image.NewUniform(s1theme), image.ZP, draw.Src)
	largeDrawer = drawerpool.NewPool(1, imgW, 10000, nil)
	smallDrawer = drawerpool.NewPool(10, imgW, 200, func(d *font.Drawer) { d.Dst.(*image.Paletted).Palette = smallTemplateS1.Palette })
}

func checkReferer(r *http.Request) bool {
	return strings.HasPrefix(r.Referer(), *truereferer)
}

func filterHTML(in string) string {
	return strings.Replace(in, "<", "&lt;", -1)
}

func serveHeader(w http.ResponseWriter, title string) {
	w.Header().Add("Content-Type", "text/html")
	var templ = `<!DOCTYPE html><html><title>` + title + ` - ` + *sitename + `</title>` + static.Header
	w.Write([]byte(templ))
}

func serveFooter(w http.ResponseWriter) {
	s, b := bk.TotalSnippets()
	w.Write([]byte(fmt.Sprintf(static.Footer, *sitename, s, b, bk.Capacity*100)))
}

func serveError(w http.ResponseWriter, r *http.Request, code int, info string) {
	w.WriteHeader(code)
	serveHeader(w, static.Error)
	rf := r.Referer()
	if rf == "" {
		rf = "/"
	}

	w.Write([]byte("<div class=err>" + info + "<br><a href='" + rf + "'>" + static.Back + "</a></div>"))
	serveFooter(w)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) > 1 {
		uri, raw, png := r.URL.Path[1:], strings.HasSuffix(r.URL.Path, ".txt"), strings.HasSuffix(r.URL.Path, ".png")

		if raw || png {
			uri = uri[:len(uri)-4]
		}

		id, _ := strconv.ParseUint(uri, 16, 64)
		if id == 0 {
			if raw || png {
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

		bk.IncrSnippetViews(s.ID)

		if raw {
			w.Header().Add("Content-Type", "text/plain; charset=utf8")
			w.Write([]byte(s.Raw))
			return
		} else if png {
			etag := fmt.Sprintf("W/%x", s.GUID[:4])
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(304)
			} else {
				w.Header().Add("Content-Type", "image/png")
				w.Header().Add("ETag", etag)
				s.WriteTo(w, false)
			}
			return
		}

		serveHeader(w, s.Title)
		w.Write([]byte("<div class=snippet><h2>" + filterHTML(s.Title) + "</h2>"))
		if kkformat.OwnSnippet(r, s) || isAdmin(r) {
			w.Write([]byte(fmt.Sprintf("<div><a href='/delete?id=%x&ts=%d'>%s</a> %s</div>",
				s.ID,
				time.Now().UnixNano(),
				static.Delete,
				s.Token(),
			)))
		}
		writeInfo(w, s)
		w.Write([]byte(fmt.Sprintf("</div><img src='/%x.png'>", s.ID)))
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

func writeInfo(w http.ResponseWriter, s *kkformat.Snippet) {
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

	info := fmt.Sprintf(`<div class=info>
@%s · %.2fKB · %s (%02d:%02d:%02d) · %d · <a href='/%x.txt'>RAW</a> · <a href='/%x.png'>SRC</a>
</div>`,
		filterHTML(kkformat.Trunc(s.Author, 10)),
		float64(s.Size)/1024,
		time.Unix(0, s.Time).Format("2006-01-02 15:04:05"),
		h, m, sec,
		s.Views, s.ID, s.ID)

	w.Write([]byte(info))
}

func serveList(w http.ResponseWriter, r *http.Request) {
	startPage, _ := strconv.Atoi(r.FormValue("p"))
	start := startPage * 25
	end := start + 25
	ss := bk.GetSnippetsLite(uint64(start), uint64(end))
	zebra := false

	serveHeader(w, static.AllSnippets)
	w.Write([]byte(`<form method=POST action=/delete>`))
	for _, s := range ss {
		id := strconv.FormatUint(s.ID, 16)
		w.Write([]byte(`<!-- ` + id + ` -->`))

		if s.ID == 0 {
			continue
		}

		zebra = !zebra
		title := fmt.Sprintf(`<div class="title zebra-%v"><div class=upper>
<input type=checkbox class=del name=s%x id=s%x>
<label class=id for=s%x>%x</label>
<a href='/%x'><b>%s</b></a></div>`, zebra, s.ID, s.ID, s.ID, s.ID, s.ID, filterHTML(s.Title))
		w.Write([]byte(title))
		writeInfo(w, s)
		w.Write([]byte("</div>"))
	}

	w.Write([]byte("<div class='paging title'>"))
	for p := startPage - 3; p <= startPage+3; p++ {
		if p < 0 {
			continue
		}

		if p != startPage {
			w.Write([]byte(fmt.Sprintf("<span>[ <a href='?p=%d'>%d</a> ]</span>", p, p)))
		} else {
			w.Write([]byte(fmt.Sprintf("<span>[ <b>%d</b> ]</span>", p)))
		}
	}
	if isAdmin(r) {
		w.Write([]byte("<input type=submit value=delete>"))
	}

	w.Write([]byte("</div></form>"))
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

	ipAccess.Lock()
	defer ipAccess.Unlock()

	if ipAccess.m[r.RemoteAddr] == nil {
		ipAccess.m[r.RemoteAddr] = &ipInfo{r.RemoteAddr, 1, time.Now().UnixNano()}
	} else if !isAdmin(r) {
		serveError(w, r, 502, static.CooldownTime)
		return
	}

	start := time.Now()

	r.Body = http.MaxBytesReader(w, r.Body, rawmaxsize)
	r.ParseForm()

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
	if len(s.Raw) == 0 {
		serveError(w, r, 400, static.EmptyContent)
		return
	} else if len(s.Raw) > 64*1024 {
		s.Raw = s.Raw[:64*1024]
	}

	var th []image.Image
	switch r.FormValue("theme") {
	case "black":
		th = kkformat.BlackTheme
	case "pureblack":
		th = kkformat.PureBlackTheme
	case "purewhite":
		th = kkformat.PureWhiteTheme
	default:
		th = kkformat.WhiteTheme
	}

	pp, bg := kkformat.GetTheme(th)
	drawer := largeDrawer.Get()
	defer drawer.Free()

	drawer.Dst.(*image.Paletted).Palette = pp
	draw.Draw(drawer.Dst, drawer.Dst.Bounds(), bg, image.ZP, draw.Src)

	fo := &kkformat.Formatter{
		Source:     []byte(s.Raw),
		Img:        drawer.Drawer,
		LineHeight: drawerpool.LineHeight,
		Columns:    80,
		Theme:      th,
	}
	img := fo.Render()

	if fo.Rows > 50 {
		nn := time.Now().UnixNano()
		os.MkdirAll(fmt.Sprintf("larges/%d", nn/3600e9), 0777)
		fn := fmt.Sprintf("larges/%d/%d.png", nn/3600e9, nn)

		output, err := os.Create(fn)
		s.P80 = []byte(string(kkformat.LargeP80Magic) + fn)
		if err != nil {
			log.Fatalln(err)
		}

		b := bufio.NewWriter(output)
		if err = png.Encode(b, img); err != nil {
			log.Println(err)
			serveError(w, r, 502, static.InternalError)
			return
		}

		if err = b.Flush(); err != nil {
			log.Println(err)
			serveError(w, r, 502, static.InternalError)
			return
		}

		st, _ := output.Stat()
		s.Size = st.Size()
		output.Close()
	} else {
		b := &bytes.Buffer{}
		if err := png.Encode(b, img); err != nil {
			log.Println(err)
			serveError(w, r, 502, static.InternalError)
			return
		}
		s.P80, s.Size = b.Bytes(), int64(b.Len())
	}

	if err := bk.AddSnippet(s); err != nil {
		log.Println(err)
		serveError(w, r, 502, static.InternalError)
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

func serveSmall(prefix string, raw bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		text, _ := url.QueryUnescape(r.RequestURI[len(prefix):])
		if !raw {
			text = simpleEscaper.ReplaceAllStringFunc(text, func(in string) string {
				if in == " " {
					return "+"
				}
				switch in[1] {
				case 'n':
					return "\n"
				case 's':
					return " "
				case 't':
					return "\t"
				case 'l':
					return "\\"
				case 'h':
					return "#"
				case 'p':
					return "%"
				}
				return in
			})
		}

		if strings.HasSuffix(text, ".png") {
			text = text[:len(text)-4]
		}

		if len(text) == 0 {
			w.WriteHeader(400)
			return
		} else if len(text) > 2048 {
			text = text[:2048]
		}

		start := time.Now()
		drawer := smallDrawer.Get()
		defer drawer.Free()

		w.Header().Add("Content-Type", "image/png")
		w.Header().Add("Cache-control", "public")
		if p, ok := smallCache.Get(prefix + text); ok {
			w.Write(p.([]byte))
			return
		}

		if prefix == "/s1/" {
			copy(drawer.Dst.(*image.Paletted).Pix, smallTemplateS1.Pix)
		} else {
			copy(drawer.Dst.(*image.Paletted).Pix, smallTemplate.Pix)
		}
		fo := &kkformat.Formatter{
			Source:     []byte(text),
			Img:        drawer.Drawer,
			LineHeight: drawerpool.LineHeight,
			Columns:    80,
			Theme:      kkformat.WhiteTheme,
		}
		img := fo.Render()

		if fo.Rows == 1 {
			img = img.(kkformat.IImage).SubImage(image.Rect(fo.Pos.Dx*2, 0, fo.Pos.X+1, fo.LineHeight*3/2))
		}

		b := &bytes.Buffer{}
		if err := png.Encode(b, img); err != nil {
			log.Println(err)
			w.WriteHeader(502)
			return
		}

		w.Write(b.Bytes())
		smallCache.Add(prefix+text, b.Bytes())
		log.Println("small:", time.Now().Sub(start).Nanoseconds()/1e6, "ms, size:", b.Len())
	}
}

type ipInfo struct {
	ip   string
	debt int
	last int64
}

var bk *kkformat.Backend

var ipAccess struct {
	sync.RWMutex
	m map[string]*ipInfo
}

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
		"/s/":     serveSmall("/s/", false),
		"/s1/":    serveSmall("/s1/", false),
		"/r/":     serveSmall("/r/", true),
	}

	for p, h := range handlers {
		http.Handle(p, gziphandler.GzipHandler(http.HandlerFunc(h)))
	}

	ipAccess.m = make(map[string]*ipInfo)
	go func() {
		for range time.Tick(time.Second) {
			ipAccess.Lock()
			now, ctr := time.Now().UnixNano(), 0
			for ip, i := range ipAccess.m {
				if now-i.last > cooldown*1e9 {
					delete(ipAccess.m, ip)
					ctr++
				}
			}
			if ctr > 0 {
				log.Println("free", ctr, "IPs' debts")
			}
			ipAccess.Unlock()
		}
	}()

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
		if _, err := os.Stat("secret-dir"); os.IsNotExist(err) {
			go http.ListenAndServe(":http", m.HTTPHandler(nil))
		}

		s := &http.Server{
			Addr:      ":https",
			TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
		}
		log.Fatalln(s.ListenAndServeTLS("", ""))
	}
}
