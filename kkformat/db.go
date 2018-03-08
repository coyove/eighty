package kkformat

import (
	"bytes"
	"encoding/gob"
	"errors"
	"github.com/boltdb/bolt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

type Snippet struct {
	// should be read-only
	ID    uint64
	Time  int64
	Views int64
	Dead  bool

	// settable
	Short  string
	Title  string
	TTL    int64
	Author string
	Raw    string
	Size   int64
	P40    []byte
	P80    []byte
}

var (
	ErrDupShortName   = errors.New("")
	ErrMissingBucket  = errors.New("")
	ErrInvalidSnippet = errors.New("")

	bSnippets = []byte("snippets")
	bShortid  = []byte("shortid")
)

type viewCount struct {
	Shortcut string
	Count    int64
}

type Backend struct {
	db    *bolt.DB
	views struct {
		counter map[string]*viewCount
		sync.Mutex
	}
}

func itosb(i uint64) []byte {
	return []byte(strconv.FormatUint(i, 16))
}

func (b *Backend) Init(path string) {
	var err error
	b.db, err = bolt.Open(path, 0600, nil)
	if err != nil {
		log.Fatalln(err)
	}

	b.db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists(bSnippets)
		tx.CreateBucketIfNotExists(bShortid)
		return nil
	})

	b.views.counter = make(map[string]*viewCount)
	go func() {
		for range time.Tick(1 * time.Second) {
			b.actualIncrSnippetViews()
		}
	}()
}

func (b *Backend) AddSnippet(s *Snippet) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		sn := tx.Bucket(bSnippets)
		si := tx.Bucket(bShortid)

		shortcut := []byte(s.Short)
		if len(shortcut) > 0 {
			if len(si.Get(shortcut)) > 0 {
				return ErrDupShortName
			}
		}

		s.ID, _ = sn.NextSequence()
		s.Time = time.Now().UnixNano()
		key := itosb(s.ID)

		if len(shortcut) == 0 {
			s.Short = string(key)
			shortcut = []byte(s.Short)
		}

		si.Put(shortcut, key)
		buf := &bytes.Buffer{}
		gob.NewEncoder(buf).Encode(s)
		return sn.Put(key, buf.Bytes())
	})
}

func getSnippetImpl(sn, si *bolt.Bucket, shortcut []byte) (*Snippet, error) {
	key := si.Get(shortcut)
	if len(key) == 0 {
		return nil, ErrInvalidSnippet
	}

	buf := sn.Get(key)
	if len(buf) == 0 {
		return nil, ErrInvalidSnippet
	}

	s := &Snippet{}
	if err := gob.NewDecoder(bytes.NewReader(buf)).Decode(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (b *Backend) GetSnippet(shortcut string) (s *Snippet, err error) {
	dead := false
	err = b.db.View(func(tx *bolt.Tx) error {
		s, err = getSnippetImpl(tx.Bucket(bSnippets), tx.Bucket(bShortid), []byte(shortcut))
		if s != nil {
			if s.TTL > 0 && time.Now().UnixNano()-s.Time > s.TTL*1e9 {
				dead = true
				return ErrInvalidSnippet
			}
		}

		return err
	})

	if dead {
		b.DeleteSnippet(s)
	}

	return
}

func (b *Backend) GetSnippetsLite(start, end uint64) []*Snippet {
	ss := make([]*Snippet, 0, end-start)
	b.db.View(func(tx *bolt.Tx) error {
		sn := tx.Bucket(bSnippets)
		max := sn.Sequence()
		log.Println(max, start, end)
		for i := start; i < end; i++ {
			buf := sn.Get(itosb(max - i))
			s := &Snippet{}

			if len(buf) == 0 {
				s.Dead = true
			} else if err := gob.NewDecoder(bytes.NewReader(buf)).Decode(s); err != nil {
				log.Println("s", err)
				s.Dead = true
			} else if s.TTL > 0 && time.Now().UnixNano()-s.Time > s.TTL*1e9 {
				s.Dead = true
			}

			s.P40 = nil
			s.P80 = nil
			s.Raw = ""
			ss = append(ss, s)
		}

		return nil
	})
	return ss
}

func (b *Backend) DeleteSnippet(s *Snippet) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		sn := tx.Bucket([]byte("snippets"))
		if sn == nil {
			panic("?")
		}

		if bytes.HasPrefix(s.P40, []byte("ex:")) {
			os.RemoveAll(string(s.P40[3:]))
		}

		if bytes.HasPrefix(s.P80, []byte("ex:")) {
			os.RemoveAll(string(s.P80[3:]))
		}

		return sn.Delete(itosb(s.ID))
	})
}

func (s *Snippet) WriteTo(w io.Writer, narrow bool) {
	var b []byte
	if narrow {
		b = s.P40
	} else {
		b = s.P80
	}

	if bytes.HasPrefix(b, []byte("ex:")) {
		fi, err := os.Open(string(b[3:]))
		if err != nil {
			log.Println("WriteTo:", err)
			return
		}

		io.Copy(w, fi)
		fi.Close()
	} else {
		w.Write(b)
	}
}

func (b *Backend) IncrSnippetViews(shortcut string) {
	b.views.Lock()

	var c *viewCount
	if c = b.views.counter[shortcut]; c == nil {
		c = &viewCount{Shortcut: shortcut}
	}

	c.Count++
	b.views.counter[shortcut] = c
	b.views.Unlock()
}

func (b *Backend) actualIncrSnippetViews() {
	b.db.Update(func(tx *bolt.Tx) error {
		b.views.Lock()

		if len(b.views.counter) == 0 {
			b.views.Unlock()
			return nil
		}

		sn, si := tx.Bucket(bSnippets), tx.Bucket(bShortid)

		for _, d := range b.views.counter {
			s, err := getSnippetImpl(sn, si, []byte(d.Shortcut))
			if err != nil {
				continue
			}

			s.Views += d.Count
			buf := &bytes.Buffer{}
			gob.NewEncoder(buf).Encode(s)

			if err := sn.Put([]byte(d.Shortcut), buf.Bytes()); err != nil {
				continue
			}
		}

		b.views.counter = make(map[string]*viewCount)
		b.views.Unlock()
		return nil
	})
}
