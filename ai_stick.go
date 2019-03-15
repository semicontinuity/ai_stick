package main

import (
	. "../ai_stick_comm"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) <= 1 {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: ai_stick <command>, where <command> is one of:")
		_, _ = fmt.Fprintln(os.Stderr, "invoke <coeff file> <image file>")
		_, _ = fmt.Fprintln(os.Stderr, "make-image-message")
		_, _ = fmt.Fprintln(os.Stderr, "make-coefficients-message")
		_, _ = fmt.Fprintln(os.Stderr, "sha256 <N>\t\tCompute SHA-256 for chunks of N bytes")
		_, _ = fmt.Fprintln(os.Stderr, "i2b\t\t\tConvert binary file of integers to binary file of bytes")
		_, _ = fmt.Fprintln(os.Stderr, "agg <N>\t\t\tAggregate sequences of N bytes")
		_, _ = fmt.Fprintln(os.Stderr, "mul <K>\t\t\tMultiply every byte by K")
		_, _ = fmt.Fprintln(os.Stderr, "unpack5\t\t\tUnpack 128-byte sections of 196 5-bit values")
		_, _ = fmt.Fprintln(os.Stderr, "reorder3\t\tReorder 3-dimensional bytes array, arguments: [d1] [d2] [d3] [stride1] [stride2] [stride3]")
		_, _ = fmt.Fprintln(os.Stderr, "extract\t\t\tExtract column, arguments: [stride] [skip] [count]")
		_, _ = fmt.Fprintln(os.Stderr, "compare <f1>..<fn>\tCompare n same-sized files, output bit 0 if a bit is the same in all files, 1 otherwise")
	} else {
		if os.Args[1] == "invoke" {
			device, _ := OpenSgDevice("/dev/sg2")
			coefficients, _ := ioutil.ReadFile(os.Args[2])
			_ = Write(device, coefficients)
			image, _ := ioutil.ReadFile(os.Args[3])
			_ = Write(device, image)
			time.Sleep(12 * time.Millisecond)
			output := MakeOutputBuffer()
			_ = Read(device, output)
			CloseSgDevice(device)

			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(output)
			_ = fo.Close()
		} else if os.Args[1] == "invoke-diff" {
			device, _ := OpenSgDevice("/dev/sg2")
			coefficients, _ := ioutil.ReadFile(os.Args[2])

			offset, _ := strconv.ParseInt(os.Args[5], 10, 32)
			byteOffset, _ := strconv.ParseInt(os.Args[6], 10, 32)
			mask, _ := strconv.ParseInt(os.Args[7], 10, 32)

			i0 := cInt(coefficients, offset, 0)
			i1 := cInt(coefficients, offset, 4)
			i2 := cInt(coefficients, offset, 8)

			coefficients[0x264+((offset+byteOffset)*4)] = coefficients[0x264+((offset+byteOffset)*4)] ^ byte(mask)

			//nano1 := time.Now().UnixNano()
			_ = Write(device, coefficients)
			//nano2 := time.Now().UnixNano()
			//fmt.Println(nano2-nano1)

			image, _ := ioutil.ReadFile(os.Args[3])

			//nano3 := time.Now().UnixNano()
			_ = Write(device, image)
			//nano4 := time.Now().UnixNano()
			//fmt.Println(nano4-nano3)

			time.Sleep(12 * time.Millisecond)
			output := MakeOutputBuffer()
			//nano5 := time.Now().UnixNano()
			_ = Read(device, output)
			//nano6 := time.Now().UnixNano()
			//fmt.Println(nano6-nano5)

			CloseSgDevice(device)

			refOutput, _ := ioutil.ReadFile(os.Args[4])

			diff, diffCount := renderDiff(output, refOutput)
			var detail []int
			if diffCount < 256 {
				detail = diffBlocks(diff)
			}
			fmt.Printf("%06x  %08x %08x %08x\t%02x\t%02x\t%d\t%s\t%v\n", offset, i0, i1, i2, byteOffset, mask, diffCount, renderDiffString(diff), detail)
		} else if os.Args[1] == "make-image-message" {
			byteAt0x14, _ := strconv.ParseInt(os.Args[2], 16, 32)
			byteAt0x970, _ := strconv.ParseInt(os.Args[3], 16, 32)
			byteAt0x974, _ := strconv.ParseInt(os.Args[4], 16, 32)

			args := os.Args[5:]
			headerPayload := make([]byte, len(args))
			for a := 0; a < len(args); a++ {
				v, _ := strconv.ParseInt(args[a], 16, 32)
				headerPayload[a] = byte(v)
			}

			inBuffer, _ := ioutil.ReadFile("/dev/stdin")
			_, _ = fmt.Fprintln(os.Stderr, "Image size:", len(inBuffer))
			outBuffer := MakeImageRequest(byte(byteAt0x14), byte(byteAt0x970), byte(byteAt0x974), inBuffer, headerPayload)
			_, _ = fmt.Fprintln(os.Stderr, "Encoded size:", len(outBuffer))
			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "make-coefficients-message" {
			inBuffer, _ := ioutil.ReadFile("/dev/stdin")
			_, _ = fmt.Fprintln(os.Stderr, "Coefficients size:", len(inBuffer))
			outBuffer := EncodeCoefficientsFileContents(inBuffer)
			_, _ = fmt.Fprintln(os.Stderr, "Encoded size:", len(outBuffer))
			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "sha256" {
			d1, _ := strconv.ParseInt(os.Args[2], 10, 32)
			inBuffer, _ := ioutil.ReadFile("/dev/stdin")
			outBuffer := computeSha256(inBuffer, int(d1));
			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "i2b" {
			inBuffer, _ := ioutil.ReadFile("/dev/stdin")
			outBuffer := lowerBytesOfIntegers(inBuffer);
			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "f2b" {
			floatsToBytes();
		} else if os.Args[1] == "agg" {
			u, _ := strconv.ParseInt(os.Args[2], 10, 32)
			aggregate(int(u));
		} else if os.Args[1] == "mul" {
			k, _ := strconv.ParseInt(os.Args[2], 10, 32)
			multiply(byte(k));
		} else if os.Args[1] == "div" {
			k, _ := strconv.ParseInt(os.Args[2], 10, 32)
			divide(byte(k));
		} else if os.Args[1] == "unpack5" {
			inBuffer, _ := ioutil.ReadFile("/dev/stdin")
			outBuffer := unpack5(inBuffer);
			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "reorder3" {
			d1, _ := strconv.ParseInt(os.Args[2], 10, 32)
			d2, _ := strconv.ParseInt(os.Args[3], 10, 32)
			d3, _ := strconv.ParseInt(os.Args[4], 10, 32)
			s1, _ := strconv.ParseInt(os.Args[5], 10, 32)
			s2, _ := strconv.ParseInt(os.Args[6], 10, 32)
			s3, _ := strconv.ParseInt(os.Args[7], 10, 32)

			inBuffer, _ := ioutil.ReadFile("/dev/stdin")

			outBuffer := reorder3(inBuffer, int(d1), int(d2), int(d3), int(s1), int(s2), int(s3));
			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "reorder5" {
			d1, _ := strconv.ParseInt(os.Args[2], 10, 32)
			d2, _ := strconv.ParseInt(os.Args[3], 10, 32)
			d3, _ := strconv.ParseInt(os.Args[4], 10, 32)
			d4, _ := strconv.ParseInt(os.Args[5], 10, 32)
			d5, _ := strconv.ParseInt(os.Args[6], 10, 32)

			s1, _ := strconv.ParseInt(os.Args[7], 10, 32)
			s2, _ := strconv.ParseInt(os.Args[8], 10, 32)
			s3, _ := strconv.ParseInt(os.Args[9], 10, 32)
			s4, _ := strconv.ParseInt(os.Args[10], 10, 32)
			s5, _ := strconv.ParseInt(os.Args[11], 10, 32)

			inBuffer, _ := ioutil.ReadFile("/dev/stdin")

			outBuffer := reorder5(
				inBuffer,
				int(d1), int(d2), int(d3), int(d4), int(d5),
				int(s1), int(s2), int(s3), int(s4), int(s5))

			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "reorder6" {
			d1, _ := strconv.ParseInt(os.Args[2], 10, 32)
			d2, _ := strconv.ParseInt(os.Args[3], 10, 32)
			d3, _ := strconv.ParseInt(os.Args[4], 10, 32)
			d4, _ := strconv.ParseInt(os.Args[5], 10, 32)
			d5, _ := strconv.ParseInt(os.Args[6], 10, 32)
			d6, _ := strconv.ParseInt(os.Args[7], 10, 32)

			s1, _ := strconv.ParseInt(os.Args[8], 10, 32)
			s2, _ := strconv.ParseInt(os.Args[9], 10, 32)
			s3, _ := strconv.ParseInt(os.Args[10], 10, 32)
			s4, _ := strconv.ParseInt(os.Args[11], 10, 32)
			s5, _ := strconv.ParseInt(os.Args[12], 10, 32)
			s6, _ := strconv.ParseInt(os.Args[13], 10, 32)

			inBuffer, _ := ioutil.ReadFile("/dev/stdin")

			outBuffer := reorder6(
				inBuffer,
				int(d1), int(d2), int(d3), int(d4), int(d5), int(d6),
				int(s1), int(s2), int(s3), int(s4), int(s5), int(s6))

			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "extract" {
			stride, _ := strconv.ParseInt(os.Args[2], 10, 32)
			skip, _ := strconv.ParseInt(os.Args[3], 10, 32)
			count, _ := strconv.ParseInt(os.Args[4], 10, 32)
			inBuffer, _ := ioutil.ReadFile("/dev/stdin")

			outBuffer := extract(inBuffer, int(stride), int(skip), int(count))

			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "dd" {
			skip, _ := strconv.ParseInt(os.Args[2], 10, 32)
			count, _ := strconv.ParseInt(os.Args[3], 10, 32)

			inBuffer, _ := ioutil.ReadFile("/dev/stdin")
			outBuffer := inBuffer[skip : skip+count];
			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		} else if os.Args[1] == "compare" {
			n := len(os.Args) - 2
			files := make([][]byte, n)
			var size int
			for i := 0; i < n; i++ {
				bytes, _ := ioutil.ReadFile(os.Args[2+i])
				if i == 0 {
					size = len(bytes)
				} else if len(bytes) != size {
					_, _ = fmt.Fprintln(os.Stderr, "Sizes must match")
					return
				}
				files[i] = bytes
			}
			outBuffer := compare(files, size)
			fo, _ := os.Create("/dev/stdout")
			_, _ = fo.Write(outBuffer)
			_ = fo.Close()
		}
	}
}

func cInt(coefficients []byte, offset int64, byteOffset int64) uint32 {
	return binary.BigEndian.Uint32([]byte{
		coefficients[cOffset(offset, byteOffset + 0)],
		coefficients[cOffset(offset, byteOffset + 1)],
		coefficients[cOffset(offset, byteOffset + 2)],
		coefficients[cOffset(offset, byteOffset + 3)]})
}

func cOffset(offset int64, byteOffset int64) int64 {
	return 0x264 + ((offset + byteOffset) * 4)
}


func lowerBytesOfIntegers(inBuffer []byte) []byte {
	outBuffer := make([]byte, len(inBuffer)/4)
	for i := 0; i < len(outBuffer); i++ {
		outBuffer[i] = inBuffer[i*4]
	}
	return outBuffer
}

func floatsToBytes() {
	inBuffer, _ := ioutil.ReadFile("/dev/stdin")
	outBuffer := make([]byte, len(inBuffer)/4)
	for i := 0; i < len(outBuffer); i++ {
		b1 := uint32(inBuffer[i*4])
		b2 := uint32(inBuffer[i*4 + 1])
		b3 := uint32(inBuffer[i*4 + 2])
		b4 := uint32(inBuffer[i*4 + 3])
		f := math.Float32frombits((b4 << 24) | (b3 << 16) | (b2 << 8) | b1)
		outBuffer[i] = byte(f)
	}
	fo, _ := os.Create("/dev/stdout")
	_, _ = fo.Write(outBuffer)
	_ = fo.Close()
}

func multiply(k byte) {
	inBuffer, _ := ioutil.ReadFile("/dev/stdin")
	outBuffer := make([]byte, len(inBuffer))
	for i := 0; i < len(outBuffer); i++ {
		outBuffer[i] = inBuffer[i] * k
	}
	fo, _ := os.Create("/dev/stdout")
	_, _ = fo.Write(outBuffer)
	_ = fo.Close()
}

func divide(k byte) {
	inBuffer, _ := ioutil.ReadFile("/dev/stdin")
	outBuffer := make([]byte, len(inBuffer))
	for i := 0; i < len(outBuffer); i++ {
		outBuffer[i] = inBuffer[i] / k
	}
	fo, _ := os.Create("/dev/stdout")
	_, _ = fo.Write(outBuffer)
	_ = fo.Close()
}


func aggregate(blockSize int) {
	inBuffer, _ := ioutil.ReadFile("/dev/stdin")
	sectionCount := (len(inBuffer) + blockSize - 1) / blockSize
	outBuffer := make([]byte, sectionCount)
	offset := 0
	for i := 0; i < len(outBuffer); i++ {
		sum := 0
		for j := 0; j < blockSize; j++ {
			if offset >= len(inBuffer) {
				break
			}
			sum += int(inBuffer[offset])
			offset++
		}
		outBuffer[i] = byte(sum / blockSize)
	}
	fo, _ := os.Create("/dev/stdout")
	_, _ = fo.Write(outBuffer)
	_ = fo.Close()
}


func unpack5(inBuffer []byte) []byte {
	sectionCount := len(inBuffer) / 128
	outBuffer := make([]byte, sectionCount * 196)

	inOffset := 0
	outOffset := 0

	for i := 0; i < sectionCount; i++ {
		unpackRow(inBuffer, inOffset, outBuffer, outOffset, 25)
		inOffset += 16
		outOffset += 25

		unpackRow(inBuffer, inOffset, outBuffer, outOffset, 25)
		inOffset += 16
		outOffset += 25

		unpackRow(inBuffer, inOffset, outBuffer, outOffset, 25)
		inOffset += 16
		outOffset += 25

		unpackRow(inBuffer, inOffset, outBuffer, outOffset, 25)
		inOffset += 16
		outOffset += 25

		unpackRow(inBuffer, inOffset, outBuffer, outOffset, 25)
		inOffset += 16
		outOffset += 25

		unpackRow(inBuffer, inOffset, outBuffer, outOffset, 25)
		inOffset += 16
		outOffset += 25

		unpackRow(inBuffer, inOffset, outBuffer, outOffset, 25)
		inOffset += 16
		outOffset += 25

		unpackRow(inBuffer, inOffset, outBuffer, outOffset, 21)
		inOffset += 16
		outOffset += 21
	}
	return outBuffer
}


func unpackRow(inBuffer []byte, inOffset int, outBuffer []byte, outOffset int, size int) {
	var shift uint32 = 0
	for i := 0; i < size; i++ {
		if shift + 5 > 16 {
			inOffset++
			shift -= 8
		}
		h := uint32(inBuffer[inOffset+1]) << 8
		l := uint32(inBuffer[inOffset])
		v := ((h | l) >> shift) & 0x1F
		outBuffer[outOffset] = byte(v)

		shift += 5
		outOffset++
	}
}


func reorder3(inBuffer []byte, d1 int, d2 int, d3 int, s1 int, s2 int, s3 int) []byte {
	outBuffer := make([]byte, d1*d2*d3)
	offset := 0
	for i1 := 0; i1 < d1; i1++ {
		for i2 := 0; i2 < d2; i2++ {
			for i3 := 0; i3 < d3; i3++ {
				outBuffer[offset] = inBuffer[i1*s1 + i2*s2 + i3*s3]
				offset++
			}
		}
	}

	return outBuffer
}


func reorder5(inBuffer []byte, d1 int, d2 int, d3 int, d4 int, d5 int, s1 int, s2 int, s3 int, s4 int, s5 int) []byte {
	outBuffer := make([]byte, d1*d2*d3*d4*d5)
	offset := 0
	for i1 := 0; i1 < d1; i1++ {
		for i2 := 0; i2 < d2; i2++ {
			for i3 := 0; i3 < d3; i3++ {
				for i4 := 0; i4 < d4; i4++ {
					for i5 := 0; i5 < d5; i5++ {
						outBuffer[offset] = inBuffer[i1*s1 + i2*s2 + i3*s3 + i4*s4 + i5*s5]
						offset++
					}
				}
			}
		}
	}

	return outBuffer
}


func reorder6(inBuffer []byte, d1 int, d2 int, d3 int, d4 int, d5 int, d6 int, s1 int, s2 int, s3 int, s4 int, s5 int, s6 int) []byte {
	outBuffer := make([]byte, d1*d2*d3*d4*d5*d6)
	offset := 0
	for i1 := 0; i1 < d1; i1++ {
		for i2 := 0; i2 < d2; i2++ {
			for i3 := 0; i3 < d3; i3++ {
				for i4 := 0; i4 < d4; i4++ {
					for i5 := 0; i5 < d5; i5++ {
						for i6 := 0; i6 < d6; i6++ {
							i := i1*s1 + i2*s2 + i3*s3 + i4*s4 + i5*s5 + i6*s6
							//if i >= len(inBuffer) {
							//	fmt.Fprintf(os.Stderr, "For %d %d %d %d %d %d", i1, i2, i3, i4, i5, i6)
							//	fmt.Fprintf(os.Stderr, "S: %d %d %d %d %d %d", s1, s2, s3, s4, s5, s6)
							//}
							outBuffer[offset] = inBuffer[i]
							offset++
						}
					}
				}
			}
		}
	}

	return outBuffer
}


func extract(inBuffer []byte, stride int, skip int, count int) []byte {
	height := len(inBuffer) / stride
	outBuffer := make([]byte, height * count)
	offset := 0
	for y := 0; y < height; y++ {
		for x := 0; x < count; x++ {
			outBuffer[offset] = inBuffer[y*stride + skip + x]
			offset++
		}
	}

	return outBuffer
}


func computeSha256(inBuffer []byte, size int) []byte {
	count := len(inBuffer) / size
	outBuffer := make([]byte, count*sha256.Size)
	offset := 0
	for y := 0; y < count; y++ {
		sum256 := sha256.Sum256(inBuffer[y*size : y*size + size])
		for x := 0; x < sha256.Size; x++ {
			outBuffer[offset] = sum256[x]
			offset++
		}
	}

	return outBuffer
}


func compare(inBuffers [][]byte, size int) []byte {
	count := len(inBuffers)
	outBuffer := make([]byte, size)
	for y := 0; y < size; y++ {
		var anded byte = 0xFF
		var ored byte = 0x00
		for f := 0; f < count; f++ {
			anded &= inBuffers[f][y]
			ored |= inBuffers[f][y]
		}
		outBuffer[y] = anded ^ ored
	}

	return outBuffer
}

func renderDiff(data []byte, ref []byte) ([]int, int) {
	diff := make([]int, 270*256)
	offset := 0
	differentBlocks := 0
	for i := 0; i < len(diff); i++ {
		count := 0
		for j := 0; j < 128; j++ {
			if data[offset] != ref[offset] {
				count++
			}
			offset += 4
		}

		diff[i] = count
		if count > 0 {
			differentBlocks++
		}
	}
	return diff, differentBlocks
}

/*
func renderDiff(data []byte, ref []byte) ([]int, int) {
	diff := make([]int, 0)
	offset := 0
	differentBlocks := 0
	for i := 0; i < 270; i++ {
		count := 0
		for j := 0; j < 128*256; j++ {
			if data[offset] != ref[offset] {
				count++
			}
			offset++
		}

		diff = append(diff, count)
		if count > 0 {
			differentBlocks++
		}
	}
	return diff, differentBlocks
}
*/
func renderDiffString(diff []int) string {
	var str strings.Builder

	for i := 0; i < len(diff); {
		count := 0
		for j := 0; j < 256; j++ {
			if diff[i] > 0 {
				count++
			}
			i++
		}

		if count > 0 {
			str.WriteRune('\u2588')
		} else {
			str.WriteRune('.')
		}
	}
	return str.String()
}

func diffBlocks(diff []int) []int {
	diffBlocks := make([]int, 0)
	for i := 0; i < len(diff); i++ {
		if diff[i] > 0 {
			diffBlocks = append(diffBlocks, i)
		}
	}
	return diffBlocks
}
