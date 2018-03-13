package kkformat

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/coyove/goflyway/pkg/rand"
)

type Snippet struct {
	// should be read-only
	ID    uint64
	Time  int64
	Last  int64
	Views int64
	Dead  bool
	GUID  [20]byte

	// settable
	Title  string
	TTL    int64
	Author string
	Raw    string
	Size   int64
	P80    []byte
}

var (
	ErrDupShortName   = errors.New("Duplicated shortcut")
	ErrMissingBucket  = errors.New("")
	ErrInvalidSnippet = errors.New("Invalid snippet")

	bSnippets     = []byte("snippets")
	bSOccupy      = []byte("snippetsoccupy")
	LargeP80Magic = []byte("exP80zzz:")
)

type viewCount struct {
	Last  int64
	Count int64
}

type Backend struct {
	Capacity float64
	db       *bolt.DB
	rd       *rand.Rand
	views    struct {
		counter map[uint64]*viewCount
		sync.Mutex
	}
}

func itob(id uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, id)
	return buf
}

func (b *Backend) Init(path string) {
	var err error
	b.db, err = bolt.Open(path, 0600, nil)
	if err != nil {
		log.Fatalln(err)
	}

	b.db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists(bSnippets)
		o, _ := tx.CreateBucketIfNotExists(bSOccupy)
		// o.SetSequence(1)
		if len(o.Get(itob(0))) == 0 {
			var b block
			o.Put(itob(0), b[:])
		}
		return nil
	})

	b.views.counter = make(map[uint64]*viewCount)
	b.rd = rand.New()
	go func() {
		for range time.Tick(5 * time.Second) {
			b.actualIncrSnippetViews()
		}
	}()
}

func OwnSnippet(r *http.Request, s *Snippet) bool {
	name := "s" + strconv.FormatUint(s.ID, 16)
	if c, err := r.Cookie(name); err != nil || c.Value != s.Token() {
		return false
	}
	return true
}

func nextID(sc *bolt.Bucket) uint64 {
	impl := func() uint64 {
		currentO := sc.Sequence()
		key := make([]byte, 8)
		i := 0
		var b block

		for o := currentO; o >= 0; o-- {
			if i++; i > 8 {
				// we have searched 8 blocks and found no free space, stop here
				break
			}

			binary.BigEndian.PutUint64(key, o)
			copy(b[:], sc.Get(key))
			// log.Println(b.getFirstUnmarked())
			if m := b.getFirstUnmarked(); m != -1 {
				b.mark(m)
				sc.Put(key, b[:])
				return uint64(m) + o*4096
			}
		}

		// we can't find a free block, so create a new one
		b.clear()
		nextO, _ := sc.NextSequence()
		m := b.getFirstUnmarked()
		b.mark(m)
		sc.Put(itob(nextO), b[:])
		return uint64(m) + nextO*4096
	}

	x := impl() + 1
	return x
}

func deleteID(sc *bolt.Bucket, id uint64) {
	if id == 0 {
		return
	}

	id--
	kid := id / 4096
	mid := id - kid*4096
	key := itob(kid)

	kblock := sc.Get(key)
	if len(kblock) != 520 {
		return
	}

	var b block
	copy(b[:], kblock)
	b.unmark(int(mid))
	sc.Put(key, b[:])
}

func (b *Backend) AddSnippet(s *Snippet) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		sn := tx.Bucket(bSnippets)
		sc := tx.Bucket(bSOccupy)

		s.ID = nextID(sc)
		maxID := sn.Sequence()
		if s.ID > maxID {
			sn.SetSequence(s.ID)
		}
		s.Time = time.Now().UnixNano()

		sumbuf := make([]byte, 256)
		copy(sumbuf, s.Raw)
		sum := sha1.Sum(sumbuf)
		b.rd.Read(sumbuf[:sha1.Size])
		for i := 0; i < sha1.Size; i++ {
			sum[i] ^= sumbuf[i]
		}
		s.GUID = sha1.Sum(sum[:])

		buf := &bytes.Buffer{}
		gob.NewEncoder(buf).Encode(s)
		return sn.Put(itob(s.ID), buf.Bytes())
	})
}

func getSnippetImpl(sn *bolt.Bucket, id uint64) (*Snippet, error) {
	buf := sn.Get(itob(id))
	if len(buf) == 0 {
		return nil, ErrInvalidSnippet
	}

	s := &Snippet{}
	if err := gob.NewDecoder(bytes.NewReader(buf)).Decode(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (b *Backend) GetSnippet(id uint64) (s *Snippet, err error) {
	dead := false
	err = b.db.View(func(tx *bolt.Tx) error {
		s, err = getSnippetImpl(tx.Bucket(bSnippets), id)
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
	if end < start {
		return []*Snippet{}
	}

	ss := make([]*Snippet, 0, end-start)
	b.db.View(func(tx *bolt.Tx) error {
		sn := tx.Bucket(bSnippets)
		max := sn.Sequence()

		for i := start; i < end; i++ {
			buf := sn.Get(itob(max - i))
			s := &Snippet{}

			if len(buf) == 0 {
				s.Dead = true
			} else if err := gob.NewDecoder(bytes.NewReader(buf)).Decode(s); err != nil {
				log.Println("s", err)
				s.Dead = true
			} else if s.TTL > 0 && time.Now().UnixNano()-s.Time > s.TTL*1e9 {
				s.Dead = true
			}

			s.P80 = nil
			s.Raw = ""
			ss = append(ss, s)
		}

		return nil
	})
	return ss
}

func (b *Backend) TotalSnippets() (snippets uint64, blocks uint64) {
	b.db.View(func(tx *bolt.Tx) error {
		blocks = tx.Bucket(bSOccupy).Sequence() + 1
		snippets = tx.Bucket(bSnippets).Sequence()
		return nil
	})
	return
}

func (b *Backend) DeleteSnippet(s *Snippet) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		sn := tx.Bucket(bSnippets)
		sc := tx.Bucket(bSOccupy)

		if bytes.HasPrefix(s.P80, []byte(LargeP80Magic)) {
			os.RemoveAll(string(s.P80[len(LargeP80Magic):]))
		}

		deleteID(sc, s.ID)
		return sn.Delete(itob(s.ID))
	})
}

func (b *Backend) DeleteSnippets(ids ...uint64) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		sn := tx.Bucket(bSnippets)
		sc := tx.Bucket(bSOccupy)

		for _, id := range ids {
			key := itob(id)
			buf := sn.Get(key)

			if bytes.Contains(buf, LargeP80Magic) {
				s := &Snippet{}
				gob.NewDecoder(bytes.NewReader(buf)).Decode(s)
				if bytes.HasPrefix(s.P80, LargeP80Magic) {
					os.RemoveAll(string(s.P80[len(LargeP80Magic):]))
				}
			}

			deleteID(sc, id)
			sn.Delete(key)
		}

		return nil
	})
}

func (s *Snippet) WriteTo(w io.Writer, narrow bool) {
	b := s.P80

	if bytes.HasPrefix(b, []byte(LargeP80Magic)) {
		fi, err := os.Open(string(b[len(LargeP80Magic):]))
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

func (s *Snippet) Token() string {
	return fmt.Sprintf("s%x:%x", s.ID, s.GUID)
}

func (b *Backend) IncrSnippetViews(id uint64) {
	b.views.Lock()

	var c *viewCount
	if c = b.views.counter[id]; c == nil {
		c = &viewCount{}
	}

	c.Count++
	c.Last = time.Now().UnixNano()
	b.views.counter[id] = c
	b.views.Unlock()
}

func (b *Backend) actualIncrSnippetViews() {
	b.db.Update(func(tx *bolt.Tx) error {
		var bk block
		var c float64
		sc := tx.Bucket(bSOccupy)
		total := sc.Sequence() + 1
		key := make([]byte, 8)
		for i := uint64(0); i < total; i++ {
			binary.BigEndian.PutUint64(key, i)
			copy(bk[:], sc.Get(key))
			c += bk.capacity()
		}
		b.Capacity = c / float64(total)

		b.views.Lock()

		if len(b.views.counter) == 0 {
			b.views.Unlock()
			return nil
		}

		sn := tx.Bucket(bSnippets)

		for id, d := range b.views.counter {
			s, err := getSnippetImpl(sn, id)
			if err != nil {
				continue
			}

			s.Views += d.Count
			s.Last = d.Last
			buf := &bytes.Buffer{}
			gob.NewEncoder(buf).Encode(s)

			if err := sn.Put(itob(s.ID), buf.Bytes()); err != nil {
				continue
			}
		}

		b.views.counter = make(map[uint64]*viewCount)
		b.views.Unlock()
		return nil
	})
}
