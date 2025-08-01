package protodump

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

const scan = ".proto"
const magicByte = 0xa

func consumeBytes(data []byte, position int) (int, error) {
	start := position
	consumedFieldOne := false
	for {
		number, _, length := protowire.ConsumeField(data[position:])
		if length < 0 {
			err := protowire.ParseError(length)
			// Treat "invalid field number" as end of data, not an error
			if strings.Contains(err.Error(), "invalid field number") {
				return position - start, nil
			}
			// Return other parse errors as actual errors
			return position - start, fmt.Errorf("couldn't consume proto bytes: %w", err)
		}

		// Prevent infinite loop - if we can't consume any bytes, we're done
		if length == 0 {
			return position - start, nil
		}

		// Only consume Field 1 once (to handle the case where protobuf definitions are adjacent
		// in program memory)
		if number == 1 {
			if consumedFieldOne {
				return position - start, nil
			}
			consumedFieldOne = true
		}

		position += length
		
		// Additional safety check - don't read beyond data bounds
		if position-start >= len(data[start:]) {
			return position - start, nil
		}
	}
}

func ScanFile(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't open file: %w", err)
	}
	return Scan(data), nil
}

func Scan(data []byte) [][]byte {
	results := make([][]byte, 0)

	for {
		index := bytes.Index(data, []byte(scan))
		if index == -1 {
			break
		}

		// Look for a 0xa byte: 0 0001 010
		//                      ^ no more bytes for varint
		//                        ^ field 1 (file name)
		//                             ^ type 2 (LEN)
		start := bytes.LastIndexByte(data[:index], magicByte)
		if start == -1 {
			data = data[index+1:]
			continue
		}

		// If the "<file>.proto" filename is 10 characters long then the 0xa byte we found is actually
		// the string length and not the start of the protobuf record, so go back 1 more
		if index-start == (magicByte-len(scan)+1) && start > 0 && data[start-1] == magicByte {
			start -= 1
		}

		length, err := consumeBytes(data, start)
		if err != nil {
			fmt.Printf("%v\n", err)
			if len(data) > index {
				data = data[index+1:]
				continue
			} else {
				break
			}
		}
		results = append(results, data[start:start+length])
		data = data[start+length:]
	}

	return results
}
