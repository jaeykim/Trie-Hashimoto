// Copyright 2016 The go-ethereum Authors
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
	_"fmt"
	"hash"
	"sync"
	"encoding/binary"
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/impt"
	"golang.org/x/crypto/sha3"
)

type hasher struct {
	tmp    sliceBuffer
	sha    keccakState
	onleaf LeafCallback
}

// keccakState wraps sha3.state. In addition to the usual hash methods, it also supports
// Read to get a variable amount of data from the hash state. Read is faster than Sum
// because it doesn't copy the internal state, but also modifies the internal state.
type keccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

type sliceBuffer []byte

var fakeIMPT bool = true

func (b *sliceBuffer) Write(data []byte) (n int, err error) {
	*b = append(*b, data...)
	return len(data), nil
}

func (b *sliceBuffer) Reset() {
	*b = (*b)[:0]
}

// hashers live in a global db.
var hasherPool = sync.Pool{
	New: func() interface{} {
		return &hasher{
			// tmp: make(sliceBuffer, 0, 550), // cap is as large as a full fullNode.
			tmp: make(sliceBuffer, 0, 600), // cap is as large as a full fullNode. (increase buf size to append nonce as a byte array) (jmlee)
			sha: sha3.NewLegacyKeccak256().(keccakState),
		}
	},
}

func newHasher(onleaf LeafCallback) *hasher {
	h := hasherPool.Get().(*hasher)
	h.onleaf = onleaf
	return h
}

func returnHasherToPool(h *hasher) {
	hasherPool.Put(h)
}

// hash collapses a node down into a hash node, also returning a copy of the
// original node initialized with the computed hash to replace the original one.
func (h *hasher) hash(n node, db *Database, force bool, trieNonces *[]*impt.TrieNonce, isMining bool, blockNum uint64) (node, node, error) {
	// If we're not storing the node, just hashing, use available cached data
	if hash, dirty := n.cache(); hash != nil {
		// Returns cached hash directly only when normal Hash() was called.
		// If HashWithNonce() or HashByNonce() was called, we should update the cached hash,
		// so ignore the short-circuit condition.
		if db == nil && trieNonces == nil {
			// fmt.Println("hasher.hash() finished 1", "/ hash:", hash.fstring(""), "/ dirty:", dirty)
			return hash, n, nil
		}
		if db != nil && !dirty {
			switch n.(type) {
			case *fullNode, *shortNode:
				// hn, b := n.cache()
				// fmt.Println("hasher.hash() finished 2", "/ node info -> hash:", hn.fstring(""), ", dirty:", b, ", nonce:", n.getNonce())
				// If the node is already cached and is not dirty, replace it to hashNode // (sjkim)
				return hash, hash, nil
			default:
				// fmt.Println("hasher.hash() finished 3")
				return hash, n, nil
			}
		}
	}
	// Trie not processed yet or needs storage, walk the children
	collapsed, cached, err := h.hashChildren(n, db, trieNonces, isMining, blockNum)
	if err != nil {
		// fmt.Println("hasher.hash() finished 4")
		return hashNode{}, n, err
	}
	hashed, nonce, err := h.store(collapsed, db, force, trieNonces, isMining, blockNum)
	if err != nil {
		// fmt.Println("hasher.hash() finished 5")
		return hashNode{}, n, err
	}
	// Cache the hash of the node for later reuse and remove
	// the dirty flag in commit mode. It's fine to assign these values directly
	// without copying the node first because hashChildren copies it.
	cachedHash, _ := hashed.(hashNode)
	switch cn := cached.(type) {
	case *shortNode:
		cn.flags.hash = cachedHash
		cn.Nonce = nonce
		// Dirty flag is cleaned after commit is done
		if db != nil {
			cn.flags.dirty = false
		}
	case *fullNode:
		cn.flags.hash = cachedHash
		cn.Nonce = nonce
		// Dirty flag is cleaned after commit is done
		if db != nil {
			cn.flags.dirty = false
		}
	}
	// hn, b = cached.cache()
	// fmt.Println("hasher.hash() finished 6", "/ node info -> hash:", hn.fstring(""), ", dirty:", b, ", nonce:", cached.getNonce())
	return hashed, cached, nil
}

// hashChildren replaces the children of a node with their hashes if the encoded
// size of the child is larger than a hash, returning the collapsed node as well
// as a replacement for the original node with the child hashes cached in.
func (h *hasher) hashChildren(original node, db *Database, trieNonces *[]*impt.TrieNonce, isMining bool, blockNum uint64) (node, node, error) {
	var err error

	switch n := original.(type) {
	case *shortNode:
		// Hash the short node's child, caching the newly hashed subtree
		collapsed, cached := n.copy(), n.copy()
		collapsed.Key = hexToCompact(n.Key)
		cached.Key = common.CopyBytes(n.Key)

		if _, ok := n.Val.(valueNode); !ok {
			collapsed.Val, cached.Val, err = h.hash(n.Val, db, false, trieNonces, isMining, blockNum)
			if err != nil {
				return original, original, err
			}
		}
		return collapsed, cached, nil

	case *fullNode:
		// Hash the full node's children, caching the newly hashed subtrees
		collapsed, cached := n.copy(), n.copy()

		for i := 0; i < 16; i++ {
			if n.Children[i] != nil {
				collapsed.Children[i], cached.Children[i], err = h.hash(n.Children[i], db, false, trieNonces, isMining, blockNum)
				if err != nil {
					return original, original, err
				}
			}
		}
		cached.Children[16] = n.Children[16]
		return collapsed, cached, nil

	default:
		// Value and hash nodes don't have children so they're left as were
		return n, original, nil
	}
}

// store hashes the node n and if we have a storage layer specified, it writes
// the key/value pair to it and tracks any node->child references as well as any
// node->external trie references.
func (h *hasher) store(n node, db *Database, force bool, trieNonces *[]*impt.TrieNonce, isMining bool, blockNum uint64) (node, uint64, error) {
	// Don't store hashes or empty nodes.
	if _, isHash := n.(hashNode); n == nil || isHash {
		//fmt.Println("hasher.store End 1")
		return n, 0, nil	
	}
	// Generate the RLP encoding of the node
	h.tmp.Reset()
	if err := rlp.Encode(&h.tmp, n); err != nil {
		panic("encode error: " + err.Error())
	}
	if len(h.tmp) < 32 && !force {
		return n, 0, nil // Nodes smaller than 32 bytes are stored inside their parent
	}
	// Larger nodes are replaced by their hash and stored in the database.
	hash, _ := n.cache()

	/*
	switch n.(type) {
		case *fullNode:
			// 강제로 nonce 값이 들어가도록 만들었음 
			// TODO: bytesNonce 에서 0이 아닌 애들만 붙여줘도 괜찮을듯 (작은 최적화)
			byteNonce := make([]byte, binary.MaxVarintLen64)
			binary.PutUvarint(byteNonce, n.getNonce())
			h.tmp = append(h.tmp, byteNonce...)
			// fmt.Println("appended byte:", h.tmp)
	}
	*/
	// if n.getNonce() != 0 {
	// 	fmt.Println("INFOOO")
	// 	byteNonce := make([]byte, binary.MaxVarintLen64)
	// 	binary.PutUvarint(byteNonce, n.getNonce())
	// 	fmt.Println("n.nonce:", n.getNonce(), "/ bytes:", byteNonce)
	// 	fmt.Println("n.cache.hash:", common.BytesToHash(hash).Hex())
	// 	fmt.Println("h.tmp:", h.tmp)
	
	// 	// 예상이 맞았음
	// 	// n이 branch node이면 nonce가 바뀌어도 h.tmp 가 바뀌지 않음

	// 	switch nn := n.(type) {
	// 		case *fullNode:
	// 			fmt.Println("print branch node!!!")
	// 			for i := 0; i < 16; i++ {
	// 				if nn.Children[i] != nil {
	// 					fmt.Println("at branch", i, ": ", nn.Children[i])
	// 				}
	// 			}
	// 			// 강제로 nonce 값이 들어가도록 만들었음 
	// 			// TODO: bytesNonce 에서 0이 아닌 애들만 붙여줘도 괜찮을듯 (작은 최적화)
	// 			h.tmp = append(h.tmp, byteNonce...)
	// 			fmt.Println("appended byte:", h.tmp)
	// 		// default:
	// 		// 	fmt.Println("HEHEHE")
	// 	}
	// }
	// fmt.Println("h.tmp:", h.tmp)

	nonce := uint64(0)

	switch n := n.(type) {
	case *shortNode, *fullNode:
		// For IMPT; HashWithNonce()
		if db == nil {
			if trieNonces == nil {
				// Used for normal Hash()
				if hash == nil { hash = h.makeHashNode(h.tmp); }
			} else if isMining {
				// Used for HashWithNonce()
				// Make new hashNode even if hashNode info already exists in cache
				hash = h.makeHashNode(h.tmp)
				oldHash := common.BytesToHash(hash)
				var newHash common.Hash 
				if fakeIMPT {
					hash = modifyHash(n, hash, blockNum)
				} else {
					for ; ; nonce++ {
						h.tmp.Reset()
						n.setNonce(nonce)
						if err := rlp.Encode(&h.tmp, n); err != nil {
							panic("encode error: " + err.Error())
						}
						hash = h.makeHashNode(h.tmp)
						// If the hash has the valid hash prefix, store the result
						if validHashNode(hash, blockNum) {
							break
						}
					}
				}
				newHash = common.BytesToHash(hash)
				nonce = n.getNonce()
				*trieNonces = append(*trieNonces, impt.NewTrieNonce(oldHash, newHash, nonce))
			} else if !isMining {
				// Used for HashByNonce()
				// Make new hashNode even if hashNode info already exists in cache
				hash = h.makeHashNode(h.tmp)
				okay := false
				oldHash := common.BytesToHash(hash)
				for _, trieNonce := range *trieNonces {
					// Update normal MPT node to indexed MPT node
					// Set nonce and generate new hashNode
					if oldHash == trieNonce.Before() {
						if fakeIMPT {
							hash = modifyHash(n, hash, blockNum)				
						} else {
							h.tmp.Reset()
							nonce = trieNonce.Nonce()
							n.setNonce(nonce)
							if err := rlp.Encode(&h.tmp, n); err != nil {
								panic("encode error: " + err.Error())
							}
							hash = h.makeHashNode(h.tmp)
						}
						// Panic if it generates different hashNode or the hash is not valid
						if common.BytesToHash(hash) != trieNonce.After() || !validHashNode(hash, blockNum) {
							panic("HashByNonce error")
						}
						okay = true
						break
					}
				}
				// Panic if there are no any matched info for current node
				if !okay {
					panic("HashByNonce error")
				}
			}
		} 
	case valueNode:
		if hash == nil {
			hash = h.makeHashNode(h.tmp)
		}
	default:
		panic("makeHashNode error")
	}

	if db != nil {
		// We are pooling the trie nodes into an intermediate memory cache
		hash := common.BytesToHash(hash)

		db.lock.Lock()
		db.insert(hash, h.tmp, n)
		db.lock.Unlock()

		// Track external references from account->storage trie
		if h.onleaf != nil {
			switch n := n.(type) {
			case *shortNode:
				if child, ok := n.Val.(valueNode); ok {
					h.onleaf(child, hash)
				}
			case *fullNode:
				for i := 0; i < 16; i++ {
					if child, ok := n.Children[i].(valueNode); ok {
						h.onleaf(child, hash)
					}
				}
			}
		}
	}
	return hash, nonce, nil
}

func (h *hasher) makeHashNode(data []byte) hashNode {
	n := make(hashNode, h.sha.Size())
	h.sha.Reset()
	h.sha.Write(data)
	h.sha.Read(n)
	return n
}

// validHashNode returns whether the node hash has a valid prefix or not.
// It returns true if the hash prefix is equal to the block number.
func validHashNode(hash []byte, blockNum uint64) bool {
	bs := make([]byte, 8)
    binary.BigEndian.PutUint64(bs, blockNum)
	return bytes.Equal(hash[:3], bs[5:])
}

// modifyHash returns a new hashNode without finding proper nonce
// Just overlap the hash prefix with what we want
func modifyHash(n node, hash hashNode, blockNum uint64) hashNode {
	
	// var blockNum = uint64(100)

	bs := make([]byte, 8)
    binary.BigEndian.PutUint64(bs, blockNum)

	newHash := hashNode(hash)
	copy(newHash, hash)
	switch n.(type) {
	case *shortNode, *fullNode:
		copy(newHash[:3], bs[5:])
		return newHash
	default:
		return nil
	}
}

func _modifyHash(n node, hash hashNode) hashNode {
	
	newHash := hashNode(hash)
	copy(newHash, hash)
	switch n := n.(type) {
	case *shortNode:
		
		keyHashPrefix := compactToHashPrefix(n.Key)
		copy(newHash[:len(keyHashPrefix)], keyHashPrefix)
		return newHash
		
	case *fullNode:
		numOfChildren := 0
		var tmp [4]int
		idx := 0
		for i := 0; i < len(n.Children); i++ {
			if n.Children[i] != nil {
				if numOfChildren < 4 {
					tmp[idx] = i
					idx++
				}
				numOfChildren++
			}
		}
		newHash[0] = byte(numOfChildren%16)
		newHash[1] = byte(tmp[0]) << 4
		newHash[1] |= byte(tmp[1])
		newHash[2] = byte(tmp[2]) << 4
		newHash[2] |= byte(tmp[3])
		return newHash 
	default:
		return nil
	}
}