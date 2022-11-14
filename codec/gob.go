package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	coon io.ReadWriteCloser // 链接实例
	buf  *bufio.Writer
	dec  *gob.Decoder
	enc  *gob.Encoder
}

// 实现Codec检查
var _ Codec = (*GobCodec)(nil)

func (c *GobCodec) ReadHeader(h *Header) error {
	// 解码
	return c.dec.Decode(h)
}

func (c *GobCodec) ReadBody(body interface{}) error {
	// 解码
	return c.dec.Decode(body)
}

func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	// 编码
	if err = c.enc.Encode(h); err != nil {
		log.Println("rpc codec: gob error encoding header:", err)
		return err
	}
	if err = c.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (c *GobCodec) Close() error {
	return c.coon.Close()
}

func NewGobCodec(coon io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(coon)
	return &GobCodec{
		coon: coon,
		buf:  buf,
		dec:  gob.NewDecoder(coon),
		enc:  gob.NewEncoder(buf),
	}
}
