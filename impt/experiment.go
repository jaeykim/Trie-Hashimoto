package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	_ "os"
	_ "strconv"

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

	sizeCheckEpoch := 2
	accountsNum := 4
	emptyStateDB := &state.StateDB{}
	emptyAccount := state.Account{}
	trieCommitEpoch := 1

	// create trie size log file
	// normTrieSizeLog, _ := os.Create("./normTrieSizeLog" + "_" + strconv.Itoa(accountsNum) + "_" + strconv.Itoa(sizeCheckEpoch) + ".txt")
	// defer normTrieSizeLog.Close()
	// secureTrieSizeLog, _ := os.Create("./secureTrieSizeLog" + "_" + strconv.Itoa(accountsNum) + "_" + strconv.Itoa(sizeCheckEpoch) + ".txt")
	// defer secureTrieSizeLog.Close()

	for i := 1; i <= accountsNum; i++ {

		// make random address
		//randHex := randomHex(20)
		//fmt.Println("random hex string:", randHex)

		// make incremental hex
		randHex := fmt.Sprintf("%x", i) // make int as hex string
		//fmt.Println("address hex string:", randHex)

		randAddr := common.HexToAddress(randHex)
		//fmt.Println("insert account addr:", randAddr.Hex())

		// encoding value
		emptyAccount.Nonce = uint64(i)
		emptyStateObject := state.NewObject(emptyStateDB, randAddr, emptyAccount)
		data, _ := rlp.EncodeToBytes(emptyStateObject)
		
		// insert account into trie
		normTrie.TryUpdate(randAddr[:], data)
		secureTrie.TryUpdate(randAddr[:], data)

		// commit trie changes
		if i%trieCommitEpoch == 0 {
			fmt.Println("commit trie")
			normTrie.Commit(nil)
			secureTrie.Commit(nil)
		}

		// write trie storage size
		if i%sizeCheckEpoch == 0 {
			fmt.Println("# of accounts:", i)
			fmt.Println("trie size:", normTrie.Size(), "/ secure trie size:", secureTrie.Trie().Size())

			// sizeLog := strconv.Itoa(i) + "\t" + strconv.FormatInt(normTrie.Size().Int(), 10) + "\n"
			// normTrieSizeLog.WriteString(sizeLog)

			// sizeLog = strconv.Itoa(i) + "\t" + strconv.FormatInt(secureTrie.Trie().Size().Int(), 10) + "\n"
			// secureTrieSizeLog.WriteString(sizeLog)
		}

	}

	// print trie nodes
	fmt.Println("\nprint norm trie")
	normTrie.Info()
	fmt.Println("\nprint secure trie")
	secureTrie.Trie().Info()


}
