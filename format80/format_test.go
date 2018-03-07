package format80

import (
	"io/ioutil"
	"testing"
)

type dummyWriter struct{}

func (d *dummyWriter) Write(p []byte) (int, error) {
	return 0, nil
}

func BenchmarkWriteTo(b *testing.B) {
	buf, _ := ioutil.ReadFile("../_raw/rekuiemu.txt")

	for i := 0; i < b.N; i++ {
		fo := &Formatter{LinkTarget: "target='_blank'", Source: buf}
		fo.Columns = 80
		fo.WriteTo(&dummyWriter{})
	}
}
