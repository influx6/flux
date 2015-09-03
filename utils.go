package flux

import (
	"encoding/binary"
	"math"
	"regexp"
	"strconv"
	"time"
)

var elapso = regexp.MustCompile(`(\d+)(\w+)`)

//MakeDuration allows you to make create a duration from a string
func MakeDuration(target string, def int) time.Duration {
	if !elapso.MatchString(target) {
		return time.Duration(def)
	}

	matchs := elapso.FindAllStringSubmatch(target, -1)

	if len(matchs) <= 0 {
		return time.Duration(def)
	}

	match := matchs[0]

	if len(match) < 3 {
		return time.Duration(def)
	}

	dur := time.Duration(ConvertToInt(match[1], def))

	mtype := match[2]

	switch mtype {
	case "s":
		return dur * time.Second
	case "mcs":
		return dur * time.Microsecond
	case "ns":
		return dur * time.Nanosecond
	case "ms":
		return dur * time.Millisecond
	case "m":
		return dur * time.Minute
	case "h":
		return dur * time.Hour
	default:
		return time.Duration(dur) * time.Second
	}
}

//ConvertToInt wraps the internal int coverter
func ConvertToInt(target string, def int) int {
	fo, err := strconv.Atoi(target)
	if err != nil {
		return def
	}
	return fo
}

func uInt16ToByteArray(value uint16, bufferSize int) []byte {
	toWriteLen := make([]byte, bufferSize)
	binary.LittleEndian.PutUint16(toWriteLen, value)
	return toWriteLen
}

// Formula for taking size in bytes and calculating # of bits to express that size
// http://www.exploringbinary.com/number-of-bits-in-a-decimal-integer/
func messageSizeToBitLength(messageSize int) int {
	bytes := float64(messageSize)
	header := math.Ceil(math.Floor(math.Log2(bytes)+1) / 8.0)
	return int(header)
}
