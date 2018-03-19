package main

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coyove/eighty/drawerpool"
	"github.com/coyove/eighty/kkformat"
	"github.com/coyove/eighty/static"
	"github.com/coyove/goflyway/pkg/lru"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/image/font"
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
	imgH       = 5000
)

var (
	palette         = append(kkformat.GetPalette(), color.RGBA{0xf6, 0xf7, 0xeb, 255})
	drawers         = drawerpool.NewPool(10, imgW, imgH, func(d *font.Drawer) { d.Dst.(*image.Paletted).Palette = palette })
	blackBackground = image.NewPaletted(image.Rect(0, 0, imgW, imgH), palette)
	whiteBackground = image.NewPaletted(image.Rect(0, 0, imgW, imgH), palette)
	s1Background    = image.NewPaletted(image.Rect(0, 0, imgW, imgH), palette)
	smallCache      = lru.NewCache(1024)
	simpleEscaper   = regexp.MustCompile(`([\\\/][nstlhp]|\s)`)

	// Unknown OS, deflate, 0 ts
	gzipHeader = []byte{0x1f, 0x8b, 0x08, 0, 0, 0, 0, 0, 0, 0xff}
	iv         = append([]byte(*adminpassword), make([]byte, 16)...)[:16]
)

func init() {
	draw.Draw(blackBackground, blackBackground.Bounds(), image.Black, image.ZP, draw.Src)
	draw.Draw(s1Background, s1Background.Bounds(), image.NewUniform(palette[len(palette)-1]), image.ZP, draw.Src)
}

func checkReferer(r *http.Request) bool {
	return strings.HasPrefix(r.Referer(), *truereferer)
}

func serveHeader(w http.ResponseWriter, title string) {
	w.Header().Add("Content-Type", "text/html")
	var templ = `<!DOCTYPE html><html><title>` + title + ` - ` + *sitename + `</title>` + static.Header
	w.Write([]byte(templ))
}

func serveFooter(w http.ResponseWriter) {
	w.Write([]byte(fmt.Sprintf(static.Footer, *sitename, float64(ipAccess.size)/1024)))
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
	serveHeader(w, static.NewSnippet)
	w.Write([]byte(fmt.Sprintf(static.NewSnippetForm, "")))
	serveFooter(w)
}

func serveEdit(w http.ResponseWriter, r *http.Request) {
	serveHeader(w, static.NewSnippet)
	w.Write([]byte(fmt.Sprintf(static.NewSnippetForm, unescape(r.RequestURI[len("/edit/"):]))))
	serveFooter(w)
}

func isAdmin(r *http.Request) bool {
	if c, err := r.Cookie("admin"); err != nil || c.Value != *adminpassword {
		return false
	}
	return true
}

func unescape(s string) string {
	t := make([]byte, len(gzipHeader), len(s)+len(gzipHeader))
	copy(t, gzipHeader)

	tu, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		log.Println("unescape:", err)
		return ""
	}

	block, _ := aes.NewCipher(iv)
	str := cipher.NewCTR(block, iv)
	str.XORKeyStream(tu, tu)

	gz, err := gzip.NewReader(bytes.NewReader(append(t, tu...)))
	if err != nil {
		log.Println("unescape:", err)
		return ""
	}

	buf, err := ioutil.ReadAll(gz)
	if err != nil {
		log.Println("unescape:", err)
		return ""
	}

	return string(buf)
}

func escape(s string) string {
	b := &bytes.Buffer{}
	gz := gzip.NewWriter(b)
	if _, err := gz.Write([]byte(s)); err != nil {
		log.Println("escape:", err)
		return ""
	}

	if err := gz.Flush(); err != nil {
		log.Println("escape:", err)
		return ""
	}

	if err := gz.Close(); err != nil {
		log.Println("escape:", err)
		return ""
	}

	buf := b.Bytes()
	xbuf := buf[len(gzipHeader):]
	block, _ := aes.NewCipher(iv)
	str := cipher.NewCTR(block, iv)
	str.XORKeyStream(xbuf, xbuf)
	return base64.URLEncoding.EncodeToString(buf[len(gzipHeader):])
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

	content := r.FormValue("content")
	if len(content) == 0 {
		serveError(w, r, 400, static.EmptyContent)
		return
	} else if len(content) > 64*1024 {
		content = content[:64*1024]
	}

	ty := r.FormValue("theme")
	if ty == "" {
		ty = "r"
	}

	content = escape(content)
	serveHeader(w, content[:4])
	w.Write([]byte(fmt.Sprintf("<img src='/%s/%s'>", ty, content)))
	serveFooter(w)

	log.Println("post:", time.Now().Sub(start).Nanoseconds()/1e6, "ms")
}

func serveSmall(prefix string, raw bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		text := r.RequestURI[len(prefix):]
		if strings.HasSuffix(text, ".png") {
			text = text[:len(text)-4]
		}

		if raw {
			text = unescape(text)
		} else {
			text, _ = url.QueryUnescape(text)
			text = simpleEscaper.ReplaceAllStringFunc(text, func(in string) string {
				if in == " " {
					return "+"
				} else if len(in) > 1 {
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
				}
				return in
			})
		}

		if len(text) == 0 {
			w.WriteHeader(400)
			return
		}

		start := time.Now()
		drawer := drawers.Get()
		defer drawer.Free()

		w.Header().Add("Content-Type", "image/png")
		w.Header().Add("Cache-control", "public")
		if p, ok := smallCache.Get(prefix + text); ok {
			w.Write(p.([]byte))
			return
		}

		fo := &kkformat.Formatter{
			Source:     []byte(text),
			Img:        drawer.Drawer,
			LineHeight: drawerpool.LineHeight,
			Columns:    80,
			Theme:      kkformat.WhiteTheme,
		}

		switch prefix {
		case "/s/", "/r/":
			copy(drawer.Dst.(*image.Paletted).Pix, whiteBackground.Pix)
			fo.Theme = kkformat.WhiteTheme
		case "/sW/", "/rW/":
			copy(drawer.Dst.(*image.Paletted).Pix, whiteBackground.Pix)
			fo.Theme = kkformat.PureWhiteTheme
		case "/sb/", "/rb/":
			copy(drawer.Dst.(*image.Paletted).Pix, blackBackground.Pix)
			fo.Theme = kkformat.BlackTheme
		case "/sB/", "/rB/":
			copy(drawer.Dst.(*image.Paletted).Pix, blackBackground.Pix)
			fo.Theme = kkformat.PureBlackTheme
		case "/s1/":
			copy(drawer.Dst.(*image.Paletted).Pix, s1Background.Pix)
			fo.Theme = kkformat.WhiteTheme
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

		if b.Len() > 256*1024 {
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

var ipAccess struct {
	sync.RWMutex
	size int64
	m    map[string]*ipInfo
}

func main() {
	flag.Parse()

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/edit/", serveEdit)
	http.HandleFunc("/post", servePost)
	http.HandleFunc("/s/", serveSmall("/s/", false))
	http.HandleFunc("/sb/", serveSmall("/sb/", false))
	http.HandleFunc("/sW/", serveSmall("/sW/", false))
	http.HandleFunc("/sB/", serveSmall("/sB/", false))
	http.HandleFunc("/r/", serveSmall("/r/", true))
	http.HandleFunc("/rb/", serveSmall("/rb/", true))
	http.HandleFunc("/rW/", serveSmall("/rW/", true))
	http.HandleFunc("/rB/", serveSmall("/rB/", true))
	http.HandleFunc("/s1/", serveSmall("/s1/", false))
	http.HandleFunc("/rs1/", serveSmall("/rs1/", true))

	ipAccess.m = make(map[string]*ipInfo)
	go func() {
		for range time.Tick(5 * time.Second) {
			ipAccess.Lock()
			now, ctr := time.Now().UnixNano(), 0
			for ip, i := range ipAccess.m {
				if now-i.last > cooldown*1e9 {
					delete(ipAccess.m, ip)
					ctr++
				}
			}
			if ctr > 0 {
				log.Println("clear", ctr, "IPs' debts")
			}
			ipAccess.Unlock()

			ipAccess.size = 0
			smallCache.Info(func(k lru.Key, v interface{}, t int64) {
				ipAccess.size += int64(len(v.([]byte)))
			})
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
