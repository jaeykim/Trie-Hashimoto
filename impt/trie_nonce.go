package impt

import (
	_"container/heap"
	_"errors"
	"io"
	_"os"
	_"math/big"
	_"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	_"github.com/ethereum/go-ethereum/common/hexutil"
	_"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// TrieNonce is used for IMPT mining
// before: the hash of each state trie node before mining
// after: the hash of each state trie node after mining
// nonce: the nonce of the state trie node 
type TrieNonce struct {
	before	common.Hash	
	after	common.Hash	
	nonce	uint64		
}

// TrieNonce struct for RLP encoding
type TrieNonceRLP struct {
	Before  common.Hash	`json:"beforeHash"    gencodec:"required"`
	After	common.Hash	`json:"afterHash"    gencodec:"required"`
	Nonce	uint64		`json:"nonce"    gencodec:"required"`
	
}

// NewTrieNonce creates a new TrieNonce Object
func NewTrieNonce(before common.Hash, after common.Hash, nonce uint64) *TrieNonce {
	return &TrieNonce{
		before: before,
		after: after,
		nonce: nonce,
	}
}

// EncodeRLP implements rlp.Encoder
func (tn *TrieNonce) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &TrieNonceRLP{Before:tn.before, After:tn.after, Nonce:tn.nonce})
}

// DecodeRLP implements rlp.Decoder, and loads the consensus fields of a TrieNonceRLP
// from an RLP stream.
func (tn *TrieNonce) DecodeRLP(s *rlp.Stream) error {
	var dec TrieNonceRLP
	if err := s.Decode(&dec); err != nil {
		return err
	}
	tn.before, tn.after, tn.nonce = dec.Before, dec.After, dec.Nonce
	return nil
}

// Before returns the hash of IMPT node before changed by mining
func (tn *TrieNonce) Before() common.Hash {
	return tn.before
}

// After returns the hash of IMPT node after changed by mining
func (tn *TrieNonce) After() common.Hash {
	return tn.after
}

// Nonce returns the nonce value of IMPT node
func (tn *TrieNonce) Nonce() uint64 {
	return tn.nonce
}