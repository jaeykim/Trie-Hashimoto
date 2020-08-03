// Copyright 2018 The go-ethereum Authors
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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/allegro/bigcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
)

const GlobalTrieNodeDBLength = 1

var GlobalTrieNodeDB [GlobalTrieNodeDBLength]ethdb.Database

var (
	memcacheCleanHitMeter   = metrics.NewRegisteredMeter("trie/memcache/clean/hit", nil)
	memcacheCleanMissMeter  = metrics.NewRegisteredMeter("trie/memcache/clean/miss", nil)
	memcacheCleanReadMeter  = metrics.NewRegisteredMeter("trie/memcache/clean/read", nil)
	memcacheCleanWriteMeter = metrics.NewRegisteredMeter("trie/memcache/clean/write", nil)

	memcacheFlushTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/flush/time", nil)
	memcacheFlushNodesMeter = metrics.NewRegisteredMeter("trie/memcache/flush/nodes", nil)
	memcacheFlushSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/flush/size", nil)

	memcacheGCTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/gc/time", nil)
	memcacheGCNodesMeter = metrics.NewRegisteredMeter("trie/memcache/gc/nodes", nil)
	memcacheGCSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/gc/size", nil)

	memcacheCommitTimeTimer  = metrics.NewRegisteredResettingTimer("trie/memcache/commit/time", nil)
	memcacheCommitNodesMeter = metrics.NewRegisteredMeter("trie/memcache/commit/nodes", nil)
	memcacheCommitSizeMeter  = metrics.NewRegisteredMeter("trie/memcache/commit/size", nil)
)

// secureKeyPrefix is the database key prefix used to store trie node preimages.
var secureKeyPrefix = []byte("secure-key-")

// secureKeyLength is the length of the above prefix + 32byte hash.
const secureKeyLength = 11 + 32

// Database is an intermediate write layer between the trie data structures and
// the disk database. The aim is to accumulate trie writes in-memory and only
// periodically flush a couple tries to disk, garbage collecting the remainder.
//
// Note, the trie Database is **not** thread safe in its mutations, but it **is**
// thread safe in providing individual, independent node access. The rationale
// behind this split design is to provide read access to RPC handlers and sync
// servers even while the trie is executing expensive garbage collection.
type Database struct {
	diskdb ethdb.KeyValueStore // Persistent storage for matured trie nodes

	cleans  *bigcache.BigCache          // GC friendly memory cache of clean node RLPs
	dirties map[common.Hash]*cachedNode // Data and references relationships of dirty nodes
	oldest  common.Hash                 // Oldest tracked node, flush-list head
	newest  common.Hash                 // Newest tracked node, flush-list tail

	preimages map[common.Hash][]byte // Preimages of nodes from the secure trie
	seckeybuf [secureKeyLength]byte  // Ephemeral buffer for calculating preimage keys

	gctime  time.Duration      // Time spent on garbage collection since last commit
	gcnodes uint64             // Nodes garbage collected since last commit
	gcsize  common.StorageSize // Data storage garbage collected since last commit

	flushtime  time.Duration      // Time spent on data flushing since last commit
	flushnodes uint64             // Nodes flushed since last commit
	flushsize  common.StorageSize // Data storage flushed since last commit

	dirtiesSize   common.StorageSize // Storage size of the dirty node cache (exc. metadata)
	childrenSize  common.StorageSize // Storage size of the external children tracking
	preimagesSize common.StorageSize // Storage size of the preimages cache

	lock sync.RWMutex
}

// rawNode is a simple binary blob used to differentiate between collapsed trie
// nodes and already encoded RLP binary blobs (while at the same time store them
// in the same cache fields).
type rawNode []byte

func (n rawNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawNode) fstring(ind string) string     { panic("this should never end up in a live trie") }
func (n rawNode) infostring(ind string, db *Database) string {
	panic("this should never end up in a live trie")
}                                          // (jmlee)
func (n rawNode) setNonce(newNonce uint64) { panic("this should never end up in a live trie") }
func (n rawNode) getNonce() uint64         { panic("this should never end up in a live trie") }
func (n rawNode) size() common.StorageSize { panic("this should never end up in a live trie") }

// rawFullNode represents only the useful data content of a full node, with the
// caches and flags stripped out to minimize its data storage. This type honors
// the same RLP encoding as the original parent.
type rawFullNode struct {
	Children [17]node
	Nonce    uint64 // nonce field for impt (sjkim)
}

type rawOptFullNode struct { // Optimized full node (sjkim)
	Children [2]node
	Nonce    uint64
}

func (n rawFullNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawFullNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawFullNode) fstring(ind string) string     { panic("this should never end up in a live trie") }
func (n rawFullNode) infostring(ind string, db *Database) string {
	panic("this should never end up in a live trie")
}                                              // (jmlee)
func (n rawFullNode) setNonce(newNonce uint64) { panic("this should never end up in a live trie") }
func (n rawFullNode) getNonce() uint64 { panic("this should never end up in a live trie") }
func (n rawFullNode) size() common.StorageSize { panic("this should never end up in a live trie") }

func (n rawFullNode) EncodeRLP(w io.Writer) error {
	var nodes [18]node

	for i, child := range n.Children {
		if child != nil {
			nodes[i] = child
		} else {
			nodes[i] = nilValueNode
		}
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n.Nonce)
	nodes[17] = valueNode(b)
	return rlp.Encode(w, nodes)
}

func (n rawOptFullNode) canUnload(uint16, uint16) bool {
	panic("this should never end up in a live trie")
}
func (n rawOptFullNode) cache() (hashNode, bool)   { panic("this should never end up in a live trie") }
func (n rawOptFullNode) fstring(ind string) string { panic("this should never end up in a live trie") }
func (n rawOptFullNode) infostring(ind string, db *Database) string {
	panic("this should never end up in a live trie")
}                                                 // (jmlee)
func (n rawOptFullNode) setNonce(newNonce uint64) { panic("this should never end up in a live trie") }
func (n rawOptFullNode) getNonce() uint64         { panic("this should never end up in a live trie") }
func (n rawOptFullNode) size() common.StorageSize { panic("this should never end up in a live trie") }

func (n rawOptFullNode) EncodeRLP(w io.Writer) error {
	var nodes [3]node

	for i, child := range n.Children {
		if child != nil {
			nodes[i] = child
		} else {
			nodes[i] = nilValueNode
		}
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n.Nonce) //
	nodes[2] = valueNode(b)
	return rlp.Encode(w, nodes)
}

// rawShortNode represents only the useful data content of a short node, with the
// caches and flags stripped out to minimize its data storage. This type honors
// the same RLP encoding as the original parent.
type rawShortNode struct {
	Key   []byte
	Val   node
	Nonce uint64 // nonce field for impt (sjkim)
}

func (n rawShortNode) canUnload(uint16, uint16) bool { panic("this should never end up in a live trie") }
func (n rawShortNode) cache() (hashNode, bool)       { panic("this should never end up in a live trie") }
func (n rawShortNode) fstring(ind string) string     { panic("this should never end up in a live trie") }
func (n rawShortNode) infostring(ind string, db *Database) string {
	panic("this should never end up in a live trie")
}                                               // (jmlee)
func (n rawShortNode) setNonce(newNonce uint64) { panic("this should never end up in a live trie") }
func (n rawShortNode) getNonce() uint64         { panic("this should never end up in a live trie") }
func (n rawShortNode) size() common.StorageSize { panic("this should never end up in a live trie") }

func (n rawShortNode) EncodeRLP(w io.Writer) error {
	var nodes [3]node

	nodes[0] = valueNode(n.Key)
	nodes[1] = n.Val
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n.Nonce)
	nodes[2] = valueNode(b)
	return rlp.Encode(w, nodes)
}

// cachedNode is all the information we know about a single cached node in the
// memory database write layer.
type cachedNode struct {
	node node   // Cached collapsed trie node, or raw rlp data
	size uint16 // Byte size of the useful cached data

	parents  uint32                 // Number of live nodes referencing this one
	children map[common.Hash]uint16 // External children referenced by this node

	flushPrev common.Hash // Previous node in the flush-list
	flushNext common.Hash // Next node in the flush-list
}

// cachedNodeSize is the raw size of a cachedNode data structure without any
// node data included. It's an approximate size, but should be a lot better
// than not counting them.
var cachedNodeSize = int(reflect.TypeOf(cachedNode{}).Size())

// cachedNodeChildrenSize is the raw size of an initialized but empty external
// reference map.
const cachedNodeChildrenSize = 48

// rlp returns the raw rlp encoded blob of the cached node, either directly from
// the cache, or by regenerating it from the collapsed node.
func (n *cachedNode) rlp() []byte {
	if node, ok := n.node.(rawNode); ok {
		return node
	}
	blob, err := rlp.EncodeToBytes(n.node)
	if err != nil {
		panic(err)
	}
	return blob
}

// obj returns the decoded and expanded trie node, either directly from the cache,
// or by regenerating it from the rlp encoded blob.
func (n *cachedNode) obj(hash common.Hash) node {
	if node, ok := n.node.(rawNode); ok {
		return mustDecodeNode(hash[:], node)
	}
	return expandNode(hash[:], n.node)
}

// childs returns all the tracked children of this node, both the implicit ones
// from inside the node as well as the explicit ones from outside the node.
func (n *cachedNode) childs() []common.Hash {
	children := make([]common.Hash, 0, 16)
	for child := range n.children {
		children = append(children, child)
	}
	if _, ok := n.node.(rawNode); !ok {
		gatherChildren(n.node, &children)
	}
	return children
}

// gatherChildren traverses the node hierarchy of a collapsed storage node and
// retrieves all the hashnode children.
func gatherChildren(n node, children *[]common.Hash) {
	switch n := n.(type) {
	case *rawShortNode:
		gatherChildren(n.Val, children)

	case rawFullNode:
		for i := 0; i < 16; i++ {
			gatherChildren(n.Children[i], children) // (sjkim)
		}
	case rawOptFullNode:
		for i := 0; i < 2; i++ {
			gatherChildren(n.Children[i], children) // (sjkim)
		}
	case hashNode:
		*children = append(*children, common.BytesToHash(n))

	case valueNode, nil:

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// simplifyNode traverses the hierarchy of an expanded memory node and discards
// all the internal caches, returning a node that only contains the raw data.
func simplifyNode(n node) node {
	switch n := n.(type) {
	case *shortNode:
		// Short nodes discard the flags and cascade
		return &rawShortNode{Key: n.Key, Val: simplifyNode(n.Val), Nonce: n.Nonce} // (sjkim)

	case *fullNode:
		numOfChildren := 0
		for i := 0; i < len(n.Children); i++ {
			if n.Children[i] != nil {
				numOfChildren++
			}
		}

		if numOfChildren > 0 {
			// Full nodes discard the flags and cascade
			node := rawFullNode{Children: n.Children, Nonce: n.Nonce} // (sjkim)
			for i := 0; i < len(node.Children); i++ {
				if node.Children[i] != nil {
					node.Children[i] = simplifyNode(node.Children[i])
				}
			}
			return node
		} else {
			node := rawOptFullNode{Nonce: n.Nonce}
			idx := 0
			for i := 0; i < len(n.Children); i++ {
				if n.Children[i] != nil {
					node.Children[idx] = simplifyNode(n.Children[i])
					idx++
				}
			}
			return node
		}

	case valueNode, hashNode, rawNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// expandNode traverses the node hierarchy of a collapsed storage node and converts
// all fields and keys into expanded memory form.
func expandNode(hash hashNode, n node) node {
	switch n := n.(type) {
	case *rawShortNode:
		// Short nodes need key and child expansion
		return &shortNode{
			Key:   compactToHex(n.Key),
			Val:   expandNode(nil, n.Val),
			Nonce: n.Nonce,
			flags: nodeFlag{
				hash: hash,
			},
		}

	case rawFullNode:
		// Full nodes need child expansion
		node := &fullNode{
			Nonce: n.Nonce,
			flags: nodeFlag{
				hash: hash,
			},
		}
		for i := 0; i < len(node.Children); i++ {
			if n.Children[i] != nil {
				node.Children[i] = expandNode(nil, n.Children[i])
			}
		}
		return node

	case rawOptFullNode:
		node := &fullNode{
			Nonce: n.Nonce,
			flags: nodeFlag{
				hash: hash,
			},
		}

		node.Children[hash[1]/16] = expandNode(nil, n.Children[0]) // fast mining
		node.Children[hash[1]%16] = expandNode(nil, n.Children[1]) // fast mining

		return node

	case valueNode, hashNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

// trienodeHasher is a struct to be used with BigCache, which uses a Hasher to
// determine which shard to place an entry into. It's not a cryptographic hash,
// just to provide a bit of anti-collision (default is FNV64a).
//
// Since trie keys are already hashes, we can just use the key directly to
// map shard id.
type trienodeHasher struct{}

// Sum64 implements the bigcache.Hasher interface.
func (t trienodeHasher) Sum64(key string) uint64 {
	return binary.BigEndian.Uint64([]byte(key))
}

// NewDatabase creates a new trie database to store ephemeral trie content before
// its written out to disk or garbage collected. No read cache is created, so all
// data retrievals will hit the underlying disk database.
func NewDatabase(diskdb ethdb.KeyValueStore) *Database {
	return NewDatabaseWithCache(diskdb, 0)
}

// NewDatabaseWithCache creates a new trie database to store ephemeral trie content
// before its written out to disk or garbage collected. It also acts as a read cache
// for nodes loaded from disk.
func NewDatabaseWithCache(diskdb ethdb.KeyValueStore, cache int) *Database {
	var cleans *bigcache.BigCache
	if cache > 0 {
		cleans, _ = bigcache.NewBigCache(bigcache.Config{
			Shards:             1024,
			LifeWindow:         time.Hour,
			MaxEntriesInWindow: cache * 1024,
			MaxEntrySize:       512,
			HardMaxCacheSize:   cache,
			Hasher:             trienodeHasher{},
		})
	}
	return &Database{
		diskdb: diskdb,
		cleans: cleans,
		dirties: map[common.Hash]*cachedNode{{}: {
			children: make(map[common.Hash]uint16),
		}},
		preimages: make(map[common.Hash][]byte),
	}
}

// DiskDB retrieves the persistent storage backing the trie database.
func (db *Database) DiskDB() ethdb.KeyValueReader {
	return db.diskdb
}

// InsertBlob writes a new reference tracked blob to the memory database if it's
// yet unknown. This method should only be used for non-trie nodes that require
// reference counting, since trie nodes are garbage collected directly through
// their embedded children.
func (db *Database) InsertBlob(hash common.Hash, blob []byte) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.insert(hash, blob, rawNode(blob))
}

// insert inserts a collapsed trie node into the memory database. This method is
// a more generic version of InsertBlob, supporting both raw blob insertions as
// well ex trie node insertions. The blob must always be specified to allow proper
// size tracking.
func (db *Database) insert(hash common.Hash, blob []byte, node node) {
	// If the node's already cached, skip
	if _, ok := db.dirties[hash]; ok {
		return
	}
	// Create the cached entry for this node
	entry := &cachedNode{
		node:      simplifyNode(node),
		size:      uint16(len(blob)),
		flushPrev: db.newest,
	}
	for _, child := range entry.childs() {
		if c := db.dirties[child]; c != nil {
			c.parents++
		}
	}
	db.dirties[hash] = entry

	// Update the flush-list endpoints
	if db.oldest == (common.Hash{}) {
		db.oldest, db.newest = hash, hash
	} else {
		db.dirties[db.newest].flushNext, db.newest = hash, hash
	}
	db.dirtiesSize += common.StorageSize(common.HashLength + entry.size)
}

// insertPreimage writes a new trie node pre-image to the memory database if it's
// yet unknown. The method will make a copy of the slice.
//
// Note, this method assumes that the database's lock is held!
func (db *Database) insertPreimage(hash common.Hash, preimage []byte) {
	if _, ok := db.preimages[hash]; ok {
		return
	}
	db.preimages[hash] = common.CopyBytes(preimage)
	db.preimagesSize += common.StorageSize(common.HashLength + len(preimage))
}

// GetProperDBIndex gets proper index of GlobalTrieNodeDB for indexed trie node (jmlee)
// this can be changed with other db indexing policies
func GetProperDBIndex(hash common.Hash) int {
	index := string(hash.Hex()[2]) // ex. 0xea945... -> index = e
	var dbIndex int64

	// indexing policy 1: 5 DBs - 0 / 1 / 2 / 3 / 4~f
	// if index < strconv.Itoa(GlobalTrieNodeDBLength) {
	// 	dbIndex, _ = strconv.Atoi(index)
	// } else {
	// 	dbIndex = GlobalTrieNodeDBLength-1
	// }

	// indexing policy 2: 6 DBs - 0 / 1 / 2~3 / 4~6 / 7~a / b~f
	// if index == "0" {	// 0
	// 	dbIndex = 0
	// } else if index < "2" {	// 1
	// 	dbIndex = 1
	// } else if index < "4" {	// 2~3
	// 	dbIndex = 2
	// } else if index < "7" {	// 4~6
	// 	dbIndex = 3
	// } else if index < "b" {	// 7~a
	// 	dbIndex = 4
	// } else {	// b~f
	// 	dbIndex = 5
	// }

	// indexing policy 3: 16 DBs - 0/1/2/3/4/5/6/7/8/9/a/b/c/d/e/f
	// dbIndex, _ = strconv.ParseInt(index, 16, 8)

	// indexing policy 4: 1 DB - 0~f
	_ = index
	dbIndex = 0

	return int(dbIndex)
}

// node retrieves a cached trie node from memory, or returns nil if none can be
// found in the memory cache.
func (db *Database) node(hash common.Hash) node {
	// Retrieve the node from the clean cache if available
	if db.cleans != nil {
		if enc, err := db.cleans.Get(string(hash[:])); err == nil && enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return mustDecodeNode(hash[:], enc)
		}
	}
	// Retrieve the node from the dirty cache if available
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		return dirty.obj(hash)
	}
	// Content unavailable in memory, attempt to retrieve from disk
	// enc, err := db.diskdb.Get(hash[:]) // impt: find indexed trie node in proper trie node db (jmlee)
	dbIndex := GetProperDBIndex(hash)
	start1 := time.Now()
	// dont need additional trie db anymore, just set elapsed1 = 0 (jmlee)
	// enc, err := GlobalTrieNodeDB[dbIndex].Get(hash[:])
	elapsed1 := time.Since(start1)
	elapsed1 = 0
	// if enc == nil {
	// 	// not found, just logging it as 0
	// 	elapsed1 = 0
	// }
	start2 := time.Now()
	enc, err := db.diskdb.Get(hash[:])
	elapsed2 := time.Since(start2)
	// fmt.Println("	%% compare DB search time -> triedb:", elapsed1, "vs totaldb:", elapsed2, "-> reduced time:", elapsed2-elapsed1)
	// print trie db index & search time for impt data log
	logData := ""
	logData += strconv.Itoa(dbIndex) + ","
	logData += strconv.Itoa(int(elapsed1.Nanoseconds())) + ","
	logData += strconv.Itoa(int(elapsed2.Nanoseconds())) + ","
	logData += strconv.Itoa(int((elapsed2 - elapsed1).Nanoseconds())) + ","
	// fmt.Println("	logData:", logData)
	// fmt.Println(logData)

	// append or write logData to file
	common.LogToFile("impt_data_log.txt", logData)

	if err != nil || enc == nil {
		return nil
	}
	if db.cleans != nil {
		db.cleans.Set(string(hash[:]), enc)
		memcacheCleanMissMeter.Mark(1)
		memcacheCleanWriteMeter.Mark(int64(len(enc)))
	}
	return mustDecodeNode(hash[:], enc)
}

// Node retrieves an encoded cached trie node from memory. If it cannot be found
// cached, the method queries the persistent database for the content.
func (db *Database) Node(hash common.Hash) ([]byte, error) {
	// It doens't make sense to retrieve the metaroot
	if hash == (common.Hash{}) {
		return nil, errors.New("not found")
	}
	// Retrieve the node from the clean cache if available
	if db.cleans != nil {
		if enc, err := db.cleans.Get(string(hash[:])); err == nil && enc != nil {
			memcacheCleanHitMeter.Mark(1)
			memcacheCleanReadMeter.Mark(int64(len(enc)))
			return enc, nil
		}
	}
	// Retrieve the node from the dirty cache if available
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		return dirty.rlp(), nil
	}
	// Content unavailable in memory, attempt to retrieve from disk
	// enc, err := db.diskdb.Get(hash[:]) // impt: find indexed trie node in proper trie node db (jmlee) (cf. this Node() function is rarely called)
	dbIndex := GetProperDBIndex(hash)
	start1 := time.Now()
	// dont need additional trie db anymore, just set elapsed1 = 0 (jmlee)
	// enc, err := GlobalTrieNodeDB[dbIndex].Get(hash[:])
	elapsed1 := time.Since(start1)
	elapsed1 = 0
	// if enc == nil {
	// 	// not found, just logging it as 0
	// 	elapsed1 = 0
	// }
	start2 := time.Now()
	enc, err := db.diskdb.Get(hash[:])
	elapsed2 := time.Since(start2)
	// fmt.Println("	%%% compare DB search time -> triedb:", elapsed1, "vs totaldb:", elapsed2, "-> reduced time:", elapsed2-elapsed1)

	// print trie db index & search time for impt data log
	logData := ""
	logData += strconv.Itoa(dbIndex) + ","
	logData += strconv.Itoa(int(elapsed1.Nanoseconds())) + ","
	logData += strconv.Itoa(int(elapsed2.Nanoseconds())) + ","
	logData += strconv.Itoa(int((elapsed2 - elapsed1).Nanoseconds())) + ","
	// fmt.Println("	logData:", logData)
	// fmt.Println(logData)

	// append or write logData to file
	common.LogToFile("impt_data_log.txt", logData)

	if err == nil && enc != nil {
		if db.cleans != nil {
			db.cleans.Set(string(hash[:]), enc)
			memcacheCleanMissMeter.Mark(1)
			memcacheCleanWriteMeter.Mark(int64(len(enc)))
		}
	}
	return enc, err
}

// preimage retrieves a cached trie node pre-image from memory. If it cannot be
// found cached, the method queries the persistent database for the content.
func (db *Database) preimage(hash common.Hash) ([]byte, error) {
	// Retrieve the node from cache if available
	db.lock.RLock()
	preimage := db.preimages[hash]
	db.lock.RUnlock()

	if preimage != nil {
		return preimage, nil
	}
	// Content unavailable in memory, attempt to retrieve from disk
	return db.diskdb.Get(db.secureKey(hash[:])) // impt: preimage is not saved in trie node db. just leave it (jmlee)
}

// secureKey returns the database key for the preimage of key, as an ephemeral
// buffer. The caller must not hold onto the return value because it will become
// invalid on the next call.
func (db *Database) secureKey(key []byte) []byte {
	buf := append(db.seckeybuf[:0], secureKeyPrefix...)
	buf = append(buf, key...)
	return buf
}

// Nodes retrieves the hashes of all the nodes cached within the memory database.
// This method is extremely expensive and should only be used to validate internal
// states in test code.
func (db *Database) Nodes() []common.Hash {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var hashes = make([]common.Hash, 0, len(db.dirties))
	for hash := range db.dirties {
		if hash != (common.Hash{}) { // Special case for "root" references/nodes
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

// Reference adds a new reference from a parent node to a child node.
func (db *Database) Reference(child common.Hash, parent common.Hash) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.reference(child, parent)
}

// reference is the private locked version of Reference.
func (db *Database) reference(child common.Hash, parent common.Hash) {
	// If the node does not exist, it's a node pulled from disk, skip
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	// If the reference already exists, only duplicate for roots
	if db.dirties[parent].children == nil {
		db.dirties[parent].children = make(map[common.Hash]uint16)
		db.childrenSize += cachedNodeChildrenSize
	} else if _, ok = db.dirties[parent].children[child]; ok && parent != (common.Hash{}) {
		return
	}
	node.parents++
	db.dirties[parent].children[child]++
	if db.dirties[parent].children[child] == 1 {
		db.childrenSize += common.HashLength + 2 // uint16 counter
	}
}

// Dereference removes an existing reference from a root node.
func (db *Database) Dereference(root common.Hash) {
	// Sanity check to ensure that the meta-root is not removed
	if root == (common.Hash{}) {
		log.Error("Attempted to dereference the trie cache meta root")
		return
	}
	db.lock.Lock()
	defer db.lock.Unlock()

	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	db.dereference(root, common.Hash{})

	db.gcnodes += uint64(nodes - len(db.dirties))
	db.gcsize += storage - db.dirtiesSize
	db.gctime += time.Since(start)

	memcacheGCTimeTimer.Update(time.Since(start))
	memcacheGCSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheGCNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Dereferenced trie from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)
}

// dereference is the private locked version of Dereference.
func (db *Database) dereference(child common.Hash, parent common.Hash) {
	// Dereference the parent-child
	node := db.dirties[parent]

	if node.children != nil && node.children[child] > 0 {
		node.children[child]--
		if node.children[child] == 0 {
			delete(node.children, child)
			db.childrenSize -= (common.HashLength + 2) // uint16 counter
		}
	}
	// If the child does not exist, it's a previously committed node.
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	// If there are no more references to the child, delete it and cascade
	if node.parents > 0 {
		// This is a special cornercase where a node loaded from disk (i.e. not in the
		// memcache any more) gets reinjected as a new node (short node split into full,
		// then reverted into short), causing a cached node to have no parents. That is
		// no problem in itself, but don't make maxint parents out of it.
		node.parents--
	}
	if node.parents == 0 {
		// Remove the node from the flush-list
		switch child {
		case db.oldest:
			db.oldest = node.flushNext
			db.dirties[node.flushNext].flushPrev = common.Hash{}
		case db.newest:
			db.newest = node.flushPrev
			db.dirties[node.flushPrev].flushNext = common.Hash{}
		default:
			db.dirties[node.flushPrev].flushNext = node.flushNext
			db.dirties[node.flushNext].flushPrev = node.flushPrev
		}
		// Dereference all children and delete the node
		for _, hash := range node.childs() {
			db.dereference(hash, child)
		}
		delete(db.dirties, child)
		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
		if node.children != nil {
			db.childrenSize -= cachedNodeChildrenSize
		}
	}
}

// Cap iteratively flushes old but still referenced trie nodes until the total
// memory usage goes below the given threshold.
//
// Note, this method is a non-synchronized mutator. It is unsafe to call this
// concurrently with other mutators.
// TODO: because this function also flushes trie nodes, I have to fix this for impt.
// but this function is not called in archive mode. so just ignore this temporarily (jmlee)
func (db *Database) Cap(limit common.StorageSize) error {
	// Create a database batch to flush persistent data out. It is important that
	// outside code doesn't see an inconsistent state (referenced data removed from
	// memory cache during commit but not yet in persistent storage). This is ensured
	// by only uncaching existing data when the database write finalizes.
	nodes, storage, start := len(db.dirties), db.dirtiesSize, time.Now()
	batch := db.diskdb.NewBatch()

	// db.dirtiesSize only contains the useful data in the cache, but when reporting
	// the total memory consumption, the maintenance metadata is also needed to be
	// counted.
	size := db.dirtiesSize + common.StorageSize((len(db.dirties)-1)*cachedNodeSize)
	size += db.childrenSize - common.StorageSize(len(db.dirties[common.Hash{}].children)*(common.HashLength+2))

	// If the preimage cache got large enough, push to disk. If it's still small
	// leave for later to deduplicate writes.
	flushPreimages := db.preimagesSize > 4*1024*1024
	if flushPreimages {
		for hash, preimage := range db.preimages {
			if err := batch.Put(db.secureKey(hash[:]), preimage); err != nil {
				log.Error("Failed to commit preimage from trie database", "err", err)
				return err
			}
			if batch.ValueSize() > ethdb.IdealBatchSize {
				if err := batch.Write(); err != nil {
					return err
				}
				batch.Reset()
			}
		}
	}
	// Keep committing nodes from the flush-list until we're below allowance
	oldest := db.oldest
	for size > limit && oldest != (common.Hash{}) {
		// Fetch the oldest referenced node and push into the batch
		node := db.dirties[oldest]
		if err := batch.Put(oldest[:], node.rlp()); err != nil {
			return err
		}
		// If we exceeded the ideal batch size, commit and reset
		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				log.Error("Failed to write flush list to disk", "err", err)
				return err
			}
			batch.Reset()
		}
		// Iterate to the next flush item, or abort if the size cap was achieved. Size
		// is the total size, including the useful cached data (hash -> blob), the
		// cache item metadata, as well as external children mappings.
		size -= common.StorageSize(common.HashLength + int(node.size) + cachedNodeSize)
		if node.children != nil {
			size -= common.StorageSize(cachedNodeChildrenSize + len(node.children)*(common.HashLength+2))
		}
		oldest = node.flushNext
	}
	// Flush out any remainder data from the last batch
	if err := batch.Write(); err != nil {
		log.Error("Failed to write flush list to disk", "err", err)
		return err
	}
	// Write successful, clear out the flushed data
	db.lock.Lock()
	defer db.lock.Unlock()

	if flushPreimages {
		db.preimages = make(map[common.Hash][]byte)
		db.preimagesSize = 0
	}
	for db.oldest != oldest {
		node := db.dirties[db.oldest]
		delete(db.dirties, db.oldest)
		db.oldest = node.flushNext

		db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
		if node.children != nil {
			db.childrenSize -= common.StorageSize(cachedNodeChildrenSize + len(node.children)*(common.HashLength+2))
		}
	}
	if db.oldest != (common.Hash{}) {
		db.dirties[db.oldest].flushPrev = common.Hash{}
	}
	db.flushnodes += uint64(nodes - len(db.dirties))
	db.flushsize += storage - db.dirtiesSize
	db.flushtime += time.Since(start)

	memcacheFlushTimeTimer.Update(time.Since(start))
	memcacheFlushSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheFlushNodesMeter.Mark(int64(nodes - len(db.dirties)))

	log.Debug("Persisted nodes from memory database", "nodes", nodes-len(db.dirties), "size", storage-db.dirtiesSize, "time", time.Since(start),
		"flushnodes", db.flushnodes, "flushsize", db.flushsize, "flushtime", db.flushtime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)

	return nil
}

// Commit iterates over all the children of a particular node, writes them out
// to disk, forcefully tearing down all references in both directions. As a side
// effect, all pre-images accumulated up to this point are also written.
//
// Note, this method is a non-synchronized mutator. It is unsafe to call this
// concurrently with other mutators.
func (db *Database) Commit(node common.Hash, report bool) error {
	// Create a database batch to flush persistent data out. It is important that
	// outside code doesn't see an inconsistent state (referenced data removed from
	// memory cache during commit but not yet in persistent storage). This is ensured
	// by only uncaching existing data when the database write finalizes.
	start := time.Now()
	batch := db.diskdb.NewBatch()

	// Move all of the accumulated preimages into a write batch
	for hash, preimage := range db.preimages {
		if err := batch.Put(db.secureKey(hash[:]), preimage); err != nil {
			log.Error("Failed to commit preimage from trie database", "err", err)
			return err
		}
		// If the batch is too large, flush to disk
		if batch.ValueSize() > ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return err
			}
			batch.Reset()
		}
	}
	// Since we're going to replay trie node writes into the clean cache, flush out
	// any batched pre-images before continuing.
	if err := batch.Write(); err != nil {
		return err
	}
	batch.Reset()

	// Move the trie itself into the batch, flushing if enough data is accumulated
	nodes, storage := len(db.dirties), db.dirtiesSize

	uncacher := &cleaner{db}
	if err := db.commit(node, batch, uncacher); err != nil {
		log.Error("Failed to commit trie from trie database", "err", err)
		return err
	}
	// Trie mostly committed to disk, flush any batch leftovers
	if err := batch.Write(); err != nil {
		log.Error("Failed to write trie to disk", "err", err)
		return err
	}
	// Uncache any leftovers in the last batch
	db.lock.Lock()
	defer db.lock.Unlock()

	batch.Replay(uncacher)
	batch.Reset()

	// Reset the storage counters and bumpd metrics
	db.preimages = make(map[common.Hash][]byte)
	db.preimagesSize = 0

	memcacheCommitTimeTimer.Update(time.Since(start))
	memcacheCommitSizeMeter.Mark(int64(storage - db.dirtiesSize))
	memcacheCommitNodesMeter.Mark(int64(nodes - len(db.dirties)))

	logger := log.Info
	if !report {
		logger = log.Debug
	}
	logger("Persisted trie from memory database", "nodes", nodes-len(db.dirties)+int(db.flushnodes), "size", storage-db.dirtiesSize+db.flushsize, "time", time.Since(start)+db.flushtime,
		"gcnodes", db.gcnodes, "gcsize", db.gcsize, "gctime", db.gctime, "livenodes", len(db.dirties), "livesize", db.dirtiesSize)

	// Reset the garbage collection statistics
	db.gcnodes, db.gcsize, db.gctime = 0, 0, 0
	db.flushnodes, db.flushsize, db.flushtime = 0, 0, 0

	return nil
}

// commit is the private locked version of Commit.
func (db *Database) commit(hash common.Hash, batch ethdb.Batch, uncacher *cleaner) error {
	// If the node does not exist, it's a previously committed node
	node, ok := db.dirties[hash]
	if !ok {
		return nil
	}
	for _, child := range node.childs() {
		if err := db.commit(child, batch, uncacher); err != nil {
			return err
		}
	}

	// impt: write indexed trie node to other leveldb (jmlee)
	// fmt.Println("in commit(), hash ", hash.Hex(), "is Put to batch")

	// if GlobalTrieNodeDB[0] != nil{

	// 	// open the batch of proper db for the indexed trie node
	// 	dbIndex := GetProperDBIndex(hash)
	// 	imptBatch := GlobalTrieNodeDB[dbIndex].NewBatch()
	// 	// fmt.Println("in commit(), node", hash.Hex(), "is in db", dbIndex)

	// 	// fmt.Println("imptBatch Put", hash.Hex())
	// 	if err := imptBatch.Put(hash[:], node.rlp()); err != nil {
	// 		fmt.Println("imptBatch Put err")
	// 		return err
	// 	}
	// 	// If we've reached an optimal batch size, commit and start over
	// 	if imptBatch.ValueSize() >= ethdb.IdealBatchSize {
	// 		if err := imptBatch.Write(); err != nil {
	// 			return err
	// 		}
	// 		// maybe i dont need this (jmlee)
	// 		// GlobalTrieNodeDB[dbIndex].lock.Lock()
	// 		// imptBatch.Replay(uncacher)
	// 		// imptBatch.Reset()
	// 		// GlobalTrieNodeDB[dbIndex].lock.Unlock()
	// 	}

	// 	// Trie mostly committed to disk, flush any batch leftovers
	// 	if err := imptBatch.Write(); err != nil {
	// 		log.Error("Failed to write trie to disk", "err", err)
	// 		return err
	// 	}

	// } else {
	// 	fmt.Println("trie node level db not opened yet, just return")
	// }

	// impt: do not commit indexed trie nodes to total leveldb (jmlee)
	// just comment out the code below
	// but if you want to compare db search time, then do not comment out
	if err := batch.Put(hash[:], node.rlp()); err != nil {
		return err
	}
	// If we've reached an optimal batch size, commit and start over
	if batch.ValueSize() >= ethdb.IdealBatchSize {
		if err := batch.Write(); err != nil {
			return err
		}
		db.lock.Lock()
		batch.Replay(uncacher)
		batch.Reset()
		db.lock.Unlock()
	}

	return nil
}

// cleaner is a database batch replayer that takes a batch of write operations
// and cleans up the trie database from anything written to disk.
type cleaner struct {
	db *Database
}

// Put reacts to database writes and implements dirty data uncaching. This is the
// post-processing step of a commit operation where the already persisted trie is
// removed from the dirty cache and moved into the clean cache. The reason behind
// the two-phase commit is to ensure ensure data availability while moving from
// memory to disk.
func (c *cleaner) Put(key []byte, rlp []byte) error {
	hash := common.BytesToHash(key)

	// If the node does not exist, we're done on this path
	node, ok := c.db.dirties[hash]
	if !ok {
		return nil
	}
	// Node still exists, remove it from the flush-list
	switch hash {
	case c.db.oldest:
		c.db.oldest = node.flushNext
		c.db.dirties[node.flushNext].flushPrev = common.Hash{}
	case c.db.newest:
		c.db.newest = node.flushPrev
		c.db.dirties[node.flushPrev].flushNext = common.Hash{}
	default:
		c.db.dirties[node.flushPrev].flushNext = node.flushNext
		c.db.dirties[node.flushNext].flushPrev = node.flushPrev
	}
	// Remove the node from the dirty cache
	delete(c.db.dirties, hash)
	c.db.dirtiesSize -= common.StorageSize(common.HashLength + int(node.size))
	if node.children != nil {
		c.db.dirtiesSize -= common.StorageSize(cachedNodeChildrenSize + len(node.children)*(common.HashLength+2))
	}
	// Move the flushed node into the clean cache to prevent insta-reloads
	if c.db.cleans != nil {
		c.db.cleans.Set(string(hash[:]), rlp)
	}
	return nil
}

func (c *cleaner) Delete(key []byte) error {
	panic("Not implemented")
}

// Size returns the current storage size of the memory cache in front of the
// persistent database layer.
func (db *Database) Size() (common.StorageSize, common.StorageSize) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	// db.dirtiesSize only contains the useful data in the cache, but when reporting
	// the total memory consumption, the maintenance metadata is also needed to be
	// counted.
	var metadataSize = common.StorageSize((len(db.dirties) - 1) * cachedNodeSize)
	var metarootRefs = common.StorageSize(len(db.dirties[common.Hash{}].children) * (common.HashLength + 2))
	return db.dirtiesSize + db.childrenSize + metadataSize - metarootRefs, db.preimagesSize
}

// verifyIntegrity is a debug method to iterate over the entire trie stored in
// memory and check whether every node is reachable from the meta root. The goal
// is to find any errors that might cause memory leaks and or trie nodes to go
// missing.
//
// This method is extremely CPU and memory intensive, only use when must.
func (db *Database) verifyIntegrity() {
	// Iterate over all the cached nodes and accumulate them into a set
	reachable := map[common.Hash]struct{}{{}: {}}

	for child := range db.dirties[common.Hash{}].children {
		db.accumulate(child, reachable)
	}
	// Find any unreachable but cached nodes
	var unreachable []string
	for hash, node := range db.dirties {
		if _, ok := reachable[hash]; !ok {
			unreachable = append(unreachable, fmt.Sprintf("%x: {Node: %v, Parents: %d, Prev: %x, Next: %x}",
				hash, node.node, node.parents, node.flushPrev, node.flushNext))
		}
	}
	if len(unreachable) != 0 {
		panic(fmt.Sprintf("trie cache memory leak: %v", unreachable))
	}
}

// accumulate iterates over the trie defined by hash and accumulates all the
// cached children found in memory.
func (db *Database) accumulate(hash common.Hash, reachable map[common.Hash]struct{}) {
	// Mark the node reachable if present in the memory cache
	node, ok := db.dirties[hash]
	if !ok {
		return
	}
	reachable[hash] = struct{}{}

	// Iterate over all the children and accumulate them too
	for _, child := range node.childs() {
		db.accumulate(child, reachable)
	}
}
