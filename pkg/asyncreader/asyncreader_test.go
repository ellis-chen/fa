package asyncreader

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"
)

func BenchmarkCopyNSmall(b *testing.B) {
	bs := bytes.Repeat([]byte{0}, 512+1)
	rd := bytes.NewReader(bs)
	buf := new(bytes.Buffer)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = io.CopyN(buf, rd, 512)
		rd.Reset(bs)
	}
}

func BenchmarkCopyNLarge(b *testing.B) {
	bs := bytes.Repeat([]byte{0}, (32*1024)+1)
	rd := bytes.NewReader(bs)
	buf := new(bytes.Buffer)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = io.CopyN(buf, rd, 32*1024)
		rd.Reset(bs)
	}
}

func BenchmarkCopyNLarge2(b *testing.B) {
	bs := bytes.Repeat([]byte{0}, (32*1024)+1)
	rd := bytes.NewReader(bs)
	buf := new(bytes.Buffer)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = io.Copy(buf, rd)
		rd.Reset(bs)
	}
}

func BenchmarkCopyAsyncReader(b *testing.B) {
	bs := bytes.Repeat([]byte{0}, (32*1024)+1)

	rd := bytes.NewReader(bs)
	r, _ := New(context.Background(), ioutil.NopCloser(rd), 10)
	defer r.Close()

	buf := new(bytes.Buffer)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = io.Copy(buf, rd)
		rd.Reset(bs)
	}
}

func TestAsyncReader(t *testing.T) {
	bs := bytes.Repeat([]byte{0}, (32*1024)+1)

	rd := bytes.NewReader(bs)
	r, _ := New(context.Background(), ioutil.NopCloser(rd), 10)
	defer r.Close()

	buf := new(bytes.Buffer)

	_, _ = io.Copy(buf, r)
	rd.Reset(bs)
}
