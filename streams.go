package flux

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"
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
		End() error
		String() string
		OnClosed() ActionInterface
		Push()
	}

	//RecordedStreamInterface defines the interface method for time streams
	RecordedStreamInterface interface {
		StreamInterface
		Stamp(int) (time.Time, bool)
		Reply(func(interface{}, time.Time))
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

	//RecordedStream provides a recorded byte streamer that replays when subscribed to
	RecordedStream struct {
		StreamInterface
		buf    *SecureStack
		stamps *SecureMap
	}

	//WrapByteStream provides a ByteStream wrapped around a reader
	WrapByteStream struct {
		StreamInterface
		reader io.ReadCloser
		base   StreamInterface
		doend  *sync.Once
	}

	//TimedStream provides a bytestream that checks for inactivity
	//on reads or writes and if its passed its duration time,it closes
	//the stream
	TimedStream struct {
		StreamInterface
		Idle *TimeWait
	}
)

var (
	//ErrWrongType denotes a wrong-type assertion
	ErrWrongType = errors.New("Wrong Type!")
	//ErrReadMisMatch denotes when the total bytes length is not read
	ErrReadMisMatch = errors.New("Length MisMatch,Data not fully Read")
)

//TimedStreamFrom returns a new TimedStream instance
func TimedStreamFrom(ns StreamInterface, max int, ms time.Duration) *TimedStream {
	ts := &TimedStream{ns, NewTimeWait(max, ms)}

	ts.Idle.Then().WhenOnly(func(_ interface{}) {
		_ = ts.StreamInterface.End()
	})

	return ts
}

//TimedByteStreamWith returns a new TimedStream instance using a bytestream underneaths
func TimedByteStreamWith(tm *TimeWait) *TimedStream {
	ts := &TimedStream{NewByteStream(), tm}

	ts.Idle.Then().WhenOnly(func(_ interface{}) {
		_ = ts.StreamInterface.End()
	})

	return ts
}

//UseTimedByteStream returns a new TimedStream instance using a bytestream underneaths
func UseTimedByteStream(ns StreamInterface, tm *TimeWait) *TimedStream {
	ts := &TimedStream{ns, tm}

	ts.Idle.Then().WhenOnly(func(_ interface{}) {
		_ = ts.StreamInterface.End()
	})

	return ts
}

//TimedByteStream returns a new TimedStream instance using a bytestream underneaths
func TimedByteStream(max int, ms time.Duration) *TimedStream {
	return TimedStreamFrom(NewByteStream(), max, ms)
}

//Close ends the timer and resets the StreamInterface
func (b *TimedStream) Close() error {
	err := b.StreamInterface.Close()
	errx := b.End()
	if err == nil {
		return errx
	}
	return err
}

//End closes the timestream idletimer which closes the inner stream
func (b *TimedStream) End() error {
	b.Idle.Flush()
	return b.StreamInterface.End()
}

//Emit push data into the stream
func (b *TimedStream) Emit(data interface{}) (int, error) {
	n, err := b.StreamInterface.Emit(data)
	b.Idle.Add()
	return n, err
}

//Write push a byte slice into the stream
func (b *TimedStream) Write(data []byte) (int, error) {
	n, err := b.StreamInterface.Write(data)
	b.Idle.Add()
	return n, err
}

//Read is supposed to read data into the supplied byte slice,
//but for a BaseStream this is a no-op and a 0 is returned as read length
func (b *TimedStream) Read(data []byte) (int, error) {
	n, err := b.StreamInterface.Read(data)
	b.Idle.Add()
	return n, err
}

//ReaderModByteStreamBy returns a bytestream that its input will be modded
func ReaderModByteStreamBy(base StreamInterface, fx StreamMod, rd io.ReadCloser) (*WrapByteStream, error) {
	bs, err := DoByteStream(base, fx)
	return &WrapByteStream{bs, rd, base, new(sync.Once)}, err
}

//ReaderModByteStream returns a bytestream that its input will be modded
func ReaderModByteStream(fx StreamMod, rd io.ReadCloser) (*WrapByteStream, error) {
	ns := NewByteStream()
	bs, err := DoByteStream(ns, fx)
	return &WrapByteStream{bs, rd, ns, new(sync.Once)}, err
}

//ReaderByteStream returns a bytestream that wraps a reader closer
func ReaderByteStream(rd io.Reader) *WrapByteStream {
	return &WrapByteStream{NewByteStream(), ioutil.NopCloser(rd), nil, new(sync.Once)}
}

//ReaderStream returns a bytestream that wraps a reader closer
func ReaderStream(ns StreamInterface, rd io.Reader) *WrapByteStream {
	return &WrapByteStream{ns, ioutil.NopCloser(rd), nil, new(sync.Once)}
}

//ReaderCloserStream returns a bytestream that wraps a reader closer
func ReaderCloserStream(ns StreamInterface, rd io.ReadCloser) *WrapByteStream {
	return &WrapByteStream{ns, rd, nil, new(sync.Once)}
}

//ReadCloserByteStream returns a bytestream that wraps a reader closer
func ReadCloserByteStream(rd io.ReadCloser) *WrapByteStream {
	return &WrapByteStream{NewByteStream(), rd, nil, new(sync.Once)}
}

//End closes the reader and calls the corresponding end function
func (b *WrapByteStream) End() error {
	var err error
	b.doend.Do(func() {
		err = b.reader.Close()
	})
	errx := b.StreamInterface.End()

	if err == nil {
		return errx
	}

	return err
}

//Close closes the stream and its associated reader
func (b *WrapByteStream) Close() error {
	var err error
	b.doend.Do(func() {
		err = b.reader.Close()
	})
	errx := b.StreamInterface.Close()
	if err == nil {
		return errx
	}

	return err
}

//Read reads the data from internal wrap ReadCloser if it fails,
//it attempts to read from the ByteStream buffer and returns
func (b *WrapByteStream) Read(data []byte) (int, error) {
	nx, err := b.reader.Read(data)

	if err == nil {
		cd := make([]byte, nx)
		copy(cd, data)
		b.Write(cd)
	}

	return nx, err
}

//DeferBaseStream returns a basestream instance
func DeferBaseStream(n int) *BaseStream {
	return &BaseStream{PullSocket(n), NewAction(), nil}
}

//NewBaseStream returns a basestream instance
func NewBaseStream() *BaseStream {
	return &BaseStream{PushSocket(0), NewAction(), nil}
}

//OnClosed returns an action that gets fullfilled when the stream is closed
func (b *BaseStream) OnClosed() ActionInterface {
	return b.closed.Wrap()
}

//Push in the case of an internal pushstream spins up a new go-routine for
//handling incoming data into the blocking channel and for pull stream
//pushes out the data in the channel,only used this when dealing with
//DeferBaseStreams
func (b *BaseStream) Push() {
	b.push.PushStream()
}

//End resets the stream but this is a no-op
func (b *BaseStream) End() error {
	return nil
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

//String returns the content of the buffer
func (b *ByteStream) String() string {
	return b.buf.String()
}

//String returns a string empty value
func (b *BaseStream) String() string {
	return fmt.Sprintf("%+v %+v %+v", b.push, b.sub, b.closed)
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

//NewRecordedStream returns a new Stream instance
func NewRecordedStream() *RecordedStream {
	return &RecordedStream{
		NewBaseStream(),
		NewSecureStack(),
		NewSecureMap(),
	}
}

//NewByteStream returns a new Stream instance
func NewByteStream() *ByteStream {
	return &ByteStream{
		NewBaseStream(),
		new(bytes.Buffer),
	}
}

//Stamp returns the time when a particular data of a particular index was added
func (b *RecordedStream) Stamp(at int) (time.Time, bool) {
	n, ok := b.stamps.Get(at).(time.Time)
	return n, ok
}

//Write reads the data in the byte slice into the buffer while notifying
//listeners
func (b *RecordedStream) Write(data []byte) (int, error) {
	n := b.buf.Add(data)
	b.stamps.Set(n, time.Now())
	_, _ = b.StreamInterface.Emit(data)
	return n, nil
}

//Read reads the last data from the internal buf into the provided slice
func (b *RecordedStream) Read(data []byte) (int, error) {
	total := len(data)

	if total <= 0 {
		total = b.buf.Size()
	}

	last, ok := b.buf.Get(total).([]byte)

	if ok {
		var dup []byte
		sz := len(last)

		if sz >= total {
			dup = last[0:total]
		} else {
			dup = last
		}

		copy(data, dup)
	}

	return total, nil
}

//Replay iterates through the recorded data providing the data and the
//corresponding time it was added
func (b *RecordedStream) Replay(fx func(b interface{}, ms time.Time)) {
	ind := 0
	b.buf.Each(func(data interface{}) {
		ms, _ := b.Stamp(ind)
		fx(b, ms)
		ind++
	})
}

//Stream provides a subscription into the stream and returns a streamer that
//reads the buffer and connects for future data
func (b *RecordedStream) Stream() (StreamInterface, error) {
	sz := b.buf.Size()
	rz := int(sz/2) + sz
	nb := DeferBaseStream(rz)

	b.buf.Each(func(data interface{}) {
		nb.Emit(data)
	})

	nb.sub = b.Subscribe(func(b interface{}, _ *Sub) {
		nb.Emit(b)
		nb.Push()
	})

	return nb, nil
}

//String returns the string of its stack
func (b *RecordedStream) String() string {
	return b.buf.String()
}

//End resets the stream bytebuffer
func (b *RecordedStream) End() error {
	return b.StreamInterface.End()
}

//Close closes the stream
func (b *RecordedStream) Close() error {
	b.StreamInterface.Close()
	b.buf.Clear()
	return nil
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

//End resets the stream bytebuffer
func (b *ByteStream) End() error {
	return b.StreamInterface.End()
}

//Close closes the stream
func (b *ByteStream) Close() error {
	b.StreamInterface.Close()
	b.buf.Reset()
	return nil
}
