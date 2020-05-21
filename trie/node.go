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

import (
	"fmt"
	"io"
	"strings"
	"math/big"
	"encoding/binary" // (sjkim)
	
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

var indices = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f", "[17]"}

type node interface {
	fstring(string) string
	infostring(string, *Database) string // print node details in human readable form (jmlee)
	cache() (hashNode, bool)
	setNonce(uint64) // set node's nonce (jmlee)
	getNonce() uint64 // get node's nonce (jmlee)
}

type (
	fullNode struct { // branch node
		Children [17]node // Actual trie node data to encode/decode (needs custom encoder)
		Nonce    uint64 // to change node's hash for impt mining (jmlee)
		flags    nodeFlag
	}
	shortNode struct { // extension node or leaf node
		Key   []byte
		Val   node
		Nonce uint64 // to change node's hash for impt mining (jmlee)
		flags nodeFlag
	}
	hashNode  []byte
	valueNode []byte
)

// nilValueNode is used when collapsing internal trie nodes for hashing, since
// unset children need to serialize correctly.
var nilValueNode = valueNode(nil)

// EncodeRLP encodes a full node into the consensus RLP format.
// This encoding reflects the nonce field
func (n *fullNode) EncodeRLP(w io.Writer) error {
	var nodes [18]node

	for i, child := range &n.Children {
		if child != nil {
			nodes[i] = child
		} else {
			nodes[i] = nilValueNode
		}
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n.getNonce())
	nodes[17] = valueNode(b)
	return rlp.Encode(w, nodes)
}

// EncodeRLP encodes a short node into the consensus RLP format.
// This encoding reflects the nonce field 
func (n *shortNode) EncodeRLP(w io.Writer) error {
	var nodes [3]node

	nodes[0] = valueNode(n.Key)
	nodes[1] = n.Val
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n.getNonce())
	nodes[2] = valueNode(b)
	return rlp.Encode(w, nodes)
}

func (n *fullNode) copy() *fullNode   { copy := *n; return &copy }
func (n *shortNode) copy() *shortNode { copy := *n; return &copy }

// nodeFlag contains caching-related metadata about a node.
type nodeFlag struct {
	hash  hashNode // cached hash of the node (may be nil)
	dirty bool     // whether the node has changes that must be written to the database
}

func (n *fullNode) cache() (hashNode, bool)  { return n.flags.hash, n.flags.dirty }
func (n *shortNode) cache() (hashNode, bool) { return n.flags.hash, n.flags.dirty }
func (n hashNode) cache() (hashNode, bool)   { return nil, true }
func (n valueNode) cache() (hashNode, bool)  { return nil, true }

func (n *fullNode) setNonce(newNonce uint64) { n.Nonce =  newNonce; return } // should be set flags.hash to nil to be rehashed (jmlee)
func (n *shortNode) setNonce(newNonce uint64) { n.Nonce =  newNonce; return }// should be set flags.hash to nil to be rehashed (jmlee)
func (n hashNode) setNonce(newNonce uint64)   { return }
func (n valueNode) setNonce(newNonce uint64)  { return }

func (n *fullNode) getNonce() uint64 { return n.Nonce }
func (n *shortNode) getNonce() uint64 { return n.Nonce }
func (n hashNode) getNonce() uint64 { return 0 } // return meaningless value (jmlee)
func (n valueNode) getNonce() uint64 { return 0 } // return meaningless value (jmlee)

// Pretty printing.
func (n *fullNode) String() string  { return n.fstring("") }
func (n *shortNode) String() string { return n.fstring("") }
func (n hashNode) String() string   { return n.fstring("") }
func (n valueNode) String() string  { return n.fstring("") }

func (n *fullNode) fstring(ind string) string {
	resp := fmt.Sprintf("[\n%s  ", ind)
	for i, node := range &n.Children {
		if node == nil {
			resp += fmt.Sprintf("%s: <nil> ", indices[i])
		} else {
			resp += fmt.Sprintf("%s: %v", indices[i], node.fstring(ind+"  "))
		}
	}
	return resp + fmt.Sprintf("\n%s] ", ind)
}
func (n *shortNode) fstring(ind string) string {
	return fmt.Sprintf("{%x: %v} ", n.Key, n.Val.fstring(ind+"  "))
}
func (n hashNode) fstring(ind string) string {
	return fmt.Sprintf("<%x> ", []byte(n))
}
func (n valueNode) fstring(ind string) string {
	return fmt.Sprintf("%x ", []byte(n))
}

func mustDecodeNode(hash, buf []byte) node {
	n, err := decodeNode(hash, buf)
	if err != nil {
		panic(fmt.Sprintf("node %x: %v", hash, err))
	}
	return n
}

// decodeNode parses the RLP encoding of a trie node.
func decodeNode(hash, buf []byte) (node, error) {
	if len(buf) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	elems, _, err := rlp.SplitList(buf)
	if err != nil {
		return nil, fmt.Errorf("decode error: %v", err)
	}
	switch c, _ := rlp.CountValues(elems); c {
	case 3:
		n, err := decodeShort(hash, elems)
		return n, wrapError(err, "short")
	case 18:
		n, err := decodeFull(hash, elems)
		return n, wrapError(err, "full")
	default:
		return nil, fmt.Errorf("invalid number of list elements: %v", c)
	}
}

func decodeShort(hash, elems []byte) (node, error) {
	kbuf, rest, err := rlp.SplitString(elems)
	if err != nil {
		return nil, err
	}
	flag := nodeFlag{hash: hash}
	key := compactToHex(kbuf)
	if hasTerm(key) {
		// value node
		val, rest, err := rlp.SplitString(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid value node: %v", err)
		}
		nonceVal, _, err := rlp.SplitString(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid value node: %v", err)
		}
		nonce := binary.LittleEndian.Uint64(nonceVal)
		return &shortNode{key, append(valueNode{}, val...), nonce, flag}, nil
	}
	r, rest, err := decodeRef(rest)
	if err != nil {
		return nil, wrapError(err, "val")
	}
	nonceVal, _, err := rlp.SplitString(rest)
	if err != nil {
		return nil, wrapError(err, "val")
	}
	nonce := binary.LittleEndian.Uint64(nonceVal)
	return &shortNode{key, r, nonce, flag}, nil
}

func decodeFull(hash, elems []byte) (*fullNode, error) {
	n := &fullNode{flags: nodeFlag{hash: hash}}
	for i := 0; i < 16; i++ {
		cld, rest, err := decodeRef(elems)
		if err != nil {
			return n, wrapError(err, fmt.Sprintf("[%d]", i))
		}
		n.Children[i], elems = cld, rest
	}
	val, rest, err := rlp.SplitString(elems)
	if err != nil {
		return n, err
	}
	if len(val) > 0 {
		n.Children[16] = append(valueNode{}, val...)
	}
	val, _, err = rlp.SplitString(rest)
	if err != nil {
		return n, err
	}
	n.Nonce = binary.LittleEndian.Uint64(val)

	return n, nil
}

const hashLen = len(common.Hash{})

func decodeRef(buf []byte) (node, []byte, error) {
	kind, val, rest, err := rlp.Split(buf)
	if err != nil {
		return nil, buf, err
	}
	switch {
	case kind == rlp.List:
		// 'embedded' node reference. The encoding must be smaller
		// than a hash in order to be valid.
		if size := len(buf) - len(rest); size > hashLen {
			err := fmt.Errorf("oversized embedded node (size is %d bytes, want size < %d)", size, hashLen)
			return nil, buf, err
		}
		n, err := decodeNode(nil, buf)
		return n, rest, err
	case kind == rlp.String && len(val) == 0:
		// empty node
		return nil, rest, nil
	case kind == rlp.String && len(val) == 32:
		return append(hashNode{}, val...), rest, nil
	default:
		return nil, nil, fmt.Errorf("invalid RLP string size %d (want 0 or 32)", len(val))
	}
}

// wraps a decoding error with information about the path to the
// invalid child node (for debugging encoding issues).
type decodeError struct {
	what  error
	stack []string
}

func wrapError(err error, ctx string) error {
	if err == nil {
		return nil
	}
	if decErr, ok := err.(*decodeError); ok {
		decErr.stack = append(decErr.stack, ctx)
		return decErr
	}
	return &decodeError{err, []string{ctx}}
}

func (err *decodeError) Error() string {
	return fmt.Sprintf("%v (decode path: %s)", err.what, strings.Join(err.stack, "<-"))
}

// print node details in human readable form (jmlee)
func (n *fullNode) infostring(ind string, db *Database) string {
	// print branch node
	hn, _ := n.cache()
	resp := fmt.Sprintf("[\n%s branch node hash: %s (nonce: %d)\n", ind, common.BytesToHash(hn).Hex(), n.getNonce())
	for i, node := range &n.Children {
		if node != nil {
			resp += fmt.Sprintf("%s branch '%s': \n", ind, indices[i])
			resp += fmt.Sprintf("%s	%v\n", ind, node.infostring(ind+"	", db))
		}
	}
	return resp + fmt.Sprintf("\n%s] ", ind)
}
func (n *shortNode) infostring(ind string, db *Database) string {
	// print extension or leaf node
	// if n.Val is branch node, then this node is extension node & n.Key is common prefix
	// if n.Val is account, then this node is leaf node & n.Key is left address of the account (along the path)

	hn, _ := n.cache()
	return fmt.Sprintf("{hash: %s (nonce: %d) -> key: %x - value: %v} ", common.BytesToHash(hn).Hex(), n.getNonce(), n.Key, n.Val.infostring(ind+"  ", db))
}
func (n hashNode) infostring(ind string, db *Database) string {
	// resolve hashNode (get node from db)
	hash := common.BytesToHash([]byte(n))
	if node := db.node(hash); node != nil {
		return node.infostring(ind, db)
	} else {
		// error: should not reach here!
		return fmt.Sprintf("<%x> ", []byte(n))
	}
}

// same struct copied from state_object.go to decode data
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

func (n valueNode) infostring(ind string, db *Database) string {
	// decode data into account & print account
	var acc Account
	rlp.DecodeBytes([]byte(n), &acc)
	return fmt.Sprintf("[ Nonce: %d / Balance: %d ]", acc.Nonce, acc.Balance.Uint64())
}

/*
// rehash gets rehashed node hash value (should be called when its nonce value is changed) (jmlee)
func rehash(n node) node {
	if n == nil {
		hash := hashNode(emptyRoot.Bytes())
		//fmt.Println("rehashed hash:", common.BytesToHash(hash).Hex()) 
		return hash
	}

	h := newHasher(nil)
	defer returnHasherToPool(h)
	hash, _, _ := h.hash(n, nil, true)
	//fmt.Println("rehashed hash:", common.BytesToHash(hash.(hashNode)).Hex()) 
	return hash
}
*/