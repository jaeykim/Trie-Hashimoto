// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package trie

// Trie keys are dealt with in three distinct encodings:
//
// KEYBYTES encoding contains the actual key and nothing else. This encoding is the
// input to most API functions.
//
// HEX encoding contains one byte for each nibble of the key and an optional trailing
// 'terminator' byte of value 0x10 which indicates whether or not the node at the key
// contains a value. Hex key encoding is used for nodes loaded in memory because it's
// convenient to access.
//
// COMPACT encoding is defined by the Ethereum Yellow Paper (it's called "hex prefix
// encoding" there) and contains the bytes of the key and a flag. The high nibble of the
// first byte contains the flag; the lowest bit encoding the oddness of the length and
// the second-lowest encoding whether the node at the key is a value node. The low nibble
// of the first byte is zero in the case of an even number of nibbles and the first nibble
// in the case of an odd number. All remaining nibbles (now an even number) fit properly
// into the remaining bytes. Compact encoding is used for nodes stored on disk.

func hexToCompact(hex []byte) []byte {
	terminator := byte(0)
	if hasTerm(hex) {
		terminator = 1
		hex = hex[:len(hex)-1]
	}
	buf := make([]byte, len(hex)/2+1)
	buf[0] = terminator << 5 // the flag byte
	if len(hex)&1 == 1 {
		buf[0] |= 1 << 4 // odd flag
		buf[0] |= hex[0] // first nibble is contained in the first byte
		hex = hex[1:]
	}
	decodeNibbles(hex, buf[1:])
	return buf
}

// hexToHashPrefix returns the hash prefix we want to find (sjkim)
func hexToHashPrefix (hex []byte) []byte {
	
	terminator := byte(0)
	if hasTerm(hex) {
		terminator = 1
		//hex = hex[:len(hex)-1]
		hex = hex[:1] // fast mining for leaf node 
	}
	if len(hex) > 3 { hex = hex[:3]; } // restrict hex size to 3
	buf := make([]byte, len(hex)/2+1)
	buf[0] = terminator << 7 // the flag byte; extension node has 1 to 7, leaf node has 8 to f
	buf[0] |= byte(len(hex)) << 4
	if len(hex)&1 == 1 {
		buf[0] |= hex[0] // first nibble is contained in the first byte
		hex = hex[1:]
	}
	decodeNibbles(hex, buf[1:])
	
	//buf := []byte{}
	return buf
}

// compactToHashPrefix returns the hash prefix we want to find (sjkim)
func compactToHashPrefix (compact []byte) []byte {
	//return nil // do not mining (sjkim)
	fixedLength := 2
	terminator := byte(0)
	if compact[0] >> 5 == 1 {
		terminator = byte(1)
		//compact = compact[:len(compact)-1] // fast mining for leaf node 
	}
	if compact[0] & byte(0x10) == 0x10 {
		length := len(compact)
		if length > 4 { length = 4; }
		if len(compact) > fixedLength { compact = compact[:fixedLength]; }
		buf := make([]byte, fixedLength)
		copy(buf, compact)
		buf[0] &= byte(0x0f)
		buf[0] |= byte(2*length-1) << 4
		buf[0] |= terminator << 7
		return buf
	} else if compact[0] & byte(0x10) == 0x00 {
		length := len(compact)
		if length > 4 { length = 4; }
		if len(compact) > fixedLength { compact = compact[:fixedLength]; }
		buf := make([]byte, fixedLength)
		copy(buf, compact)
		buf[0] &= byte(0x0f)
		buf[0] |= byte(2*length-2) << 4
		buf[0] |= terminator << 7
		return buf
	} else {
		panic("compactToHashPresix error")
	}
}

func compactToHex(compact []byte) []byte {
	if len(compact) == 0 {
		return compact
	}
	base := keybytesToHex(compact)
	// delete terminator flag
	if base[0] < 2 {
		base = base[:len(base)-1]
	}
	// apply odd flag
	chop := 2 - base[0]&1
	return base[chop:]
}

func keybytesToHex(str []byte) []byte {
	l := len(str)*2 + 1
	var nibbles = make([]byte, l)
	for i, b := range str {
		nibbles[i*2] = b / 16
		nibbles[i*2+1] = b % 16
	}
	nibbles[l-1] = 16
	return nibbles
}

// hexToKeybytes turns hex nibbles into key bytes.
// This can only be used for keys of even length.
func hexToKeybytes(hex []byte) []byte {
	if hasTerm(hex) {
		hex = hex[:len(hex)-1]
	}
	if len(hex)&1 != 0 {
		panic("can't convert hex key of odd length")
	}
	key := make([]byte, len(hex)/2)
	decodeNibbles(hex, key)
	return key
}

func decodeNibbles(nibbles []byte, bytes []byte) {
	for bi, ni := 0, 0; ni < len(nibbles); bi, ni = bi+1, ni+2 {
		bytes[bi] = nibbles[ni]<<4 | nibbles[ni+1]
	}
}

// prefixLen returns the length of the common prefix of a and b.
func prefixLen(a, b []byte) int {
	var i, length = 0, len(a)
	if len(b) < length {
		length = len(b)
	}
	for ; i < length; i++ {
		if a[i] != b[i] {
			break
		}
	}
	return i
}

// hasTerm returns whether a hex key has the terminator flag.
func hasTerm(s []byte) bool {
	return len(s) > 0 && s[len(s)-1] == 16
}
