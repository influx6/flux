package flux

import (
	"bytes"
	"errors"
	"io"
)

type (

	//StreamMod is a function that modifies a data []byte
	StreamMod func(bu []byte) []byte

	//StreamInterface define the interface method rules for streams
	StreamInterface interface {
		Subscribe(func(interface{}, *Sub)) *Sub
		StreamMod(fx func(interface{}) interface{}) (StreamInterface, error)
		Stream() (StreamInterface, error)
		StreamWriter(io.Writer) *Sub
		StreamReader(io.Reader) *Sub
		Write([]byte) (int, error)
		Read([]byte) (int, error)
		Emit(interface{}) (int, error)
		Close() error
		OnClosed() ActionInterface
	}

	//BaseStream defines a basic stream structure
	BaseStream struct {
		push   Pipe
		closed ActionInterface
		sub    *Sub
	}

	//ByteStream provides a buffer back streamer
	ByteStream struct {
		StreamInterface
		buf *bytes.Buffer
	}

	//WrapByteStream provides a ByteStream wrapped around a reader
	WrapByteStream struct {
		StreamInterface
		reader io.ReadCloser
	}
)

var (
	//ErrWrongType denotes a wrong-type assertion
	ErrWrongType = errors.New("Wrong Type!")
	//ErrReadMisMatch denotes when the total bytes length is not read
	ErrReadMisMatch = errors.New("Length MisMatch,Data not fully Read")
)

//WrapByteStreamWith returns a bytestream that wraps a reader closer
func WrapByteStreamWith(rd io.ReadCloser) *WrapByteStream {
	return &WrapByteStream{NewByteStream(), rd}
}

//Read reads the data from internal wrap ReadCloser.
func (b *WrapByteStream) Read(data []byte) (int, error) {
	var nx int
	var errx error
	var err error

	nx, errx = b.reader.Read(data)

	if errx != nil {
		nx, err = b.StreamInterface.Read(data)

		if err == nil {
			errx = nil
		}
	}

	return nx, errx
}

//NewBaseStream returns a basestream instance
func NewBaseStream() *BaseStream {
	return &BaseStream{PushSocket(0), NewAction(), nil}
}

//OnClosed returns an action that gets fullfilled when the stream is closed
func (b *BaseStream) OnClosed() ActionInterface {
	return b.closed.Wrap()
}

//Close closes the stream
func (b *BaseStream) Close() error {
	if b.sub != nil {
		b.sub.Close()
	}
	b.push.Close()
	b.closed.Fullfill(true)
	return nil
}

//Stream creates a new StreamInterface and pipes all current data into that stream
func (b *BaseStream) Stream() (StreamInterface, error) {
	return &BaseStream{PushSocketWith(b.push), NewAction(), nil}, nil
}

//StreamMod provides a new stream that mods the internal details of these streams byte
func (b *BaseStream) StreamMod(fn func(v interface{}) interface{}) (StreamInterface, error) {
	return &BaseStream{DoPushSocket(b.push, func(v interface{}, sock SocketInterface) {
		sock.Emit(fn(v))
	}), NewAction(), nil}, nil
}

//StreamReader provides a subscription into the stream to stream to a reader
func (b *BaseStream) StreamReader(w io.Reader) *Sub {
	return b.Subscribe(func(data interface{}, sub *Sub) {
		buff, ok := data.([]byte)

		if !ok {

			str, ok := data.(string)

			if !ok {
				return
			}

			w.Read([]byte(str))
		}

		_, _ = w.Read(buff)
	})
}

//StreamWriter provides a subscription into the stream into a writer
func (b *BaseStream) StreamWriter(w io.Writer) *Sub {
	return b.Subscribe(func(data interface{}, sub *Sub) {
		buff, ok := data.([]byte)

		if !ok {

			str, ok := data.(string)

			if !ok {
				return
			}

			w.Write([]byte(str))
		}

		_, _ = w.Write(buff)
	})
}

//Subscribe provides a subscription into the stream
func (b *BaseStream) Subscribe(fn func(interface{}, *Sub)) *Sub {
	return b.push.Subscribe(fn)
}

//Emit push data into the stream
func (b *BaseStream) Emit(data interface{}) (int, error) {
	b.push.Emit(data)
	return 1, nil
}

//Write push a byte slice into the stream
func (b *BaseStream) Write(data []byte) (int, error) {
	b.Emit(data)
	return len(data), nil
}

//Read is supposed to read data into the supplied byte slice,
//but for a BaseStream this is a no-op and a 0 is returned as read length
func (b *BaseStream) Read(data []byte) (int, error) {
	return 0, nil
}

//DoByteStream returns a new Stream instance whos data is modified by a function
func DoByteStream(b StreamInterface, fn StreamMod) (*ByteStream, error) {
	modd, err := b.StreamMod(func(v interface{}) interface{} {
		buff, ok := v.([]byte)

		if !ok {

			str, ok := v.(string)

			if !ok {
				return nil
			}

			return fn([]byte(str))
		}

		return fn(buff)

	})

	if err != nil {
		return nil, err
	}

	nb := &ByteStream{modd, new(bytes.Buffer)}

	sub := nb.Subscribe(func(data interface{}, _ *Sub) {
		buff, ok := data.([]byte)

		if !ok {
			return
		}

		_, _ = nb.buf.Write(buff)
	})

	nb.OnClosed().WhenOnly(func(_ interface{}) {
		sub.Close()
	})

	return nb, err
}

//ByteStreamFrom returns a new Stream instance
func ByteStreamFrom(b StreamInterface) *ByteStream {
	nb, _ := DoByteStream(b, func(data []byte) []byte {
		return data
	})
	return nb
}

//NewByteStream returns a new Stream instance
func NewByteStream() *ByteStream {
	return &ByteStream{
		NewBaseStream(),
		new(bytes.Buffer),
	}
}

//Write reads the data in the byte slice into the buffer while notifying
//listeners
func (b *ByteStream) Write(data []byte) (int, error) {
	n, err := b.buf.Write(data)
	_, _ = b.StreamInterface.Emit(data)
	return n, err
}

//Read reads the data from the internal buf into the provided slice
func (b *ByteStream) Read(data []byte) (int, error) {
	n, err := b.buf.Read(data)
	return n, err
}

//Stream provides a subscription into the stream and returns a streamer that
//reads the buffer and connects for future data
func (b *ByteStream) Stream() (StreamInterface, error) {
	data := make([]byte, b.buf.Len())
	_, err := b.buf.Read(data)

	if err != nil {
		return nil, err
	}

	nb := NewByteStream()
	nb.Write(data)

	sub := nb.Subscribe(func(data interface{}, _ *Sub) {
		buff, ok := data.([]byte)

		if !ok {

			str, ok := data.(string)

			if !ok {
				return
			}

			_, _ = nb.Write([]byte(str))
		}

		_, _ = nb.Write(buff)
	})

	nb.OnClosed().WhenOnly(func(_ interface{}) {
		sub.Close()
	})

	return nb, nil
}

//Emit push data into the stream
func (b *ByteStream) Emit(data interface{}) (int, error) {
	buff, ok := data.([]byte)

	if !ok {

		str, ok := data.(string)

		if !ok {
			return 0, ErrWrongType
		}

		return b.StreamInterface.Emit([]byte(str))
	}

	return b.StreamInterface.Emit(buff)
}

//Close closes the stream
func (b *ByteStream) Close() error {
	b.buf.Reset()
	b.StreamInterface.Close()
	return nil
}
