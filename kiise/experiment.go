package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
)

func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func main() {

	// make trie
	normTrie := trie.NewEmpty()
	secureTrie := trie.NewEmptySecure()

	sizeCheckEpoch := 100000
	accountsNum := 10100001
	emptyStateDB := &state.StateDB{}
	emptyAccount := state.Account{}
	trieCommitEpoch := 100

	// create trie size log file
	normTrieSizeLog, _ := os.Create("./normTrieSizeLog" + "_" + strconv.Itoa(accountsNum) + "_" + strconv.Itoa(sizeCheckEpoch) + ".txt")
	defer normTrieSizeLog.Close()
	secureTrieSizeLog, _ := os.Create("./secureTrieSizeLog" + "_" + strconv.Itoa(accountsNum) + "_" + strconv.Itoa(sizeCheckEpoch) + ".txt")
	defer secureTrieSizeLog.Close()

	for i := 0; i < accountsNum; i++ {

		if i%trieCommitEpoch == 0 {
			// commit trie changes
			normTrie.Commit(nil)
			secureTrie.Commit(nil)
		}

		if i%sizeCheckEpoch == 0 {
			// write trie storage size
			fmt.Println("# of accounts:", i)
			fmt.Println("trie size:", normTrie.Size(), "/ secure trie size:", secureTrie.Trie().Size())

			sizeLog := strconv.Itoa(i) + "\t" + strconv.FormatInt(normTrie.Size().Int(), 10) + "\n"
			normTrieSizeLog.WriteString(sizeLog)

			sizeLog = strconv.Itoa(i) + "\t" + strconv.FormatInt(secureTrie.Trie().Size().Int(), 10) + "\n"
			secureTrieSizeLog.WriteString(sizeLog)
		}

		// make random address
		//randHex := randomHex(20)
		//fmt.Println("random hex string:", randHex)

		// make incremental hex
		randHex := fmt.Sprintf("%x", i) // make int as hex string
		//fmt.Println("address hex string:", randHex)

		randAddr := common.HexToAddress(randHex)
		//fmt.Println("insert account addr:", randAddr.Hex())

		// encoding value
		emptyStateObject := state.NewObject(emptyStateDB, randAddr, emptyAccount)
		data, _ := rlp.EncodeToBytes(emptyStateObject)

		// insert account into trie
		normTrie.TryUpdate(randAddr[:], data)
		secureTrie.TryUpdate(randAddr[:], data)
	}

}

/*
//
//
//
//
//
//
//
//
//
//
//
// to test storage -> when change value of exist account, is storage increased?
func main() {

	// make trie
	normTrie := trie.NewEmpty()
	emptyStateDB := &state.StateDB{}
	emptyAccount := state.Account{}

	randHex := randomHex(20)
	//fmt.Println("random hex string:", randHex)
	randAddr := common.HexToAddress(randHex)
	//fmt.Println("random addr:", randAddr.Hex())

	emptyStateObject := state.NewObject(emptyStateDB, randAddr, emptyAccount)
	data, _ := rlp.EncodeToBytes(emptyStateObject)

	normTrie.TryUpdate(randAddr[:], data)
	normTrie.Commit(nil)
	fmt.Println("size:", normTrie.Size())

	emptyAccount.Nonce++
	emptyStateObject = state.NewObject(emptyStateDB, randAddr, emptyAccount)
	data, _ = rlp.EncodeToBytes(emptyStateObject)

	normTrie.TryUpdate(randAddr[:], data)
	normTrie.Commit(nil)
	fmt.Println("size:", normTrie.Size())

}
*/
