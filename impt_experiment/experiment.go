package main

import (
	"crypto/rand"
	//"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"os/exec"
	"strings"

	"reflect"
	"os"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	//"github.com/ethereum/go-ethereum/node" // (sjkim)

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	// "github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/core/rawdb"
	// "github.com/ethereum/go-ethereum/ethdb"
	
)

func randomHex() []byte {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return nil
	}
	return bytes
}


func main() {
	sizeCheckEpoch := 5
	accountsNum := 10
	trieCommitEpoch := 1
	trieCountEpoch := 5
	
	file, _ := os.Create("./branch_result_2.csv")
	defer file.Close()
	fmt.Println(reflect.TypeOf(file))


	sizeCmd := exec.Command("du", "-sk", "/home/jmlee/data/impt/db_full/geth/indexedNodes")
	// sizeCmd := exec.Command("ls", "./index")
	sizeCmdOutput, err := sizeCmd.Output()
	output:= strings.Split(string(sizeCmdOutput), "	")
	if err != nil {
		panic(err)
	}
	fmt.Println("> size of db")
	fmt.Println(output)
	for i:=0;i<len(output);i++{
		fmt.Println(i)
		fmt.Println(output[i])
	}
	// fmt.Println(output[1])
	// fmt.Println(output[2])

	return 





	const arrlen = 5
	arrlenstr := strconv.Itoa(arrlen)
	hexstr := "0123456789abcdef"
	for i:=0;i<len(hexstr);i++{
		char := string(hexstr[i])
		if char <arrlenstr{
			fmt.Println("char", char, "is smaller than", arrlenstr)
		} else {
			fmt.Println("char", char, "is bigger than", arrlenstr)
		}
	}


	os.RemoveAll("./index")

	// make trie
	//normTrie := trie.NewEmpty()
	secureTrie := trie.NewEmptySecure()

	emptyStateDB := &state.StateDB{}
	emptyAccount := state.Account{}

	for i := 1; i <= accountsNum; i++ {

		// make random address
		// randHex := randomHex(20)
		// fmt.Println("random hex string:", randHex)

		// make incremental hex
		randHex := fmt.Sprintf("%x", randomHex()) // make random hex string
		//randHex := fmt.Sprintf("%x", i*i) // make int as hex string
		//fmt.Println("address hex string:", randHex)

		randAddr := common.HexToAddress(randHex)
		//fmt.Println("insert account addr:", randAddr.Hex())

		// encoding value
		emptyAccount.Nonce = uint64(i)
		emptyStateObject := state.NewObject(emptyStateDB, randAddr, emptyAccount)
		data, _ := rlp.EncodeToBytes(emptyStateObject)
		
		// insert account into trie
		//normTrie.TryUpdate(randAddr[:], data)
		secureTrie.TryUpdate(randAddr[:], data)

		// write trie storage size
		if i%sizeCheckEpoch == 0 {
			//fmt.Fprintln(file, "# of accounts:", i)
			fmt.Println("# of accounts:", i)
			//fmt.Println("trie size:", normTrie.Size(), "/ secure trie size:", secureTrie.Trie().Size())
			a, b := secureTrie.Trie().Size()
			fmt.Println("secure trie size: ", a, b)

			// sizeLog := strconv.Itoa(i) + "\t" + strconv.FormatInt(normTrie.Size().Int(), 10) + "\n"
			// normTrieSizeLog.WriteString(sizeLog)

			// sizeLog = strconv.Itoa(i) + "\t" + strconv.FormatInt(secureTrie.Trie().Size().Int(), 10) + "\n"
			// secureTrieSizeLog.WriteString(sizeLog)
		}

		
		// commit trie changes
		if i%trieCommitEpoch == 0 {
			//fmt.Println("commit trie\n")
			//normTrie.Commit(nil)
			secureTrie.Commit(nil)
		}
		

		if i%trieCountEpoch == 0 {
			//normTrie.PrintNodeNum(file) // (sjkim)
			secureTrie.Trie().PrintNodeNum(file) // (sjkim)
		}
	}
	


	//var account = [...]uint64{4131, 4135, 4951}
	// create trie size log file
	// normTrieSizeLog, _ := os.Create("./normTrieSizeLog" + "_" + strconv.Itoa(accountsNum) + "_" + strconv.Itoa(sizeCheckEpoch) + ".txt")
	// defer normTrieSizeLog.Close()
	// secureTrieSizeLog, _ := os.Create("./secureTrieSizeLog" + "_" + strconv.Itoa(accountsNum) + "_" + strconv.Itoa(sizeCheckEpoch) + ".txt")
	// defer secureTrieSizeLog.Close()

	

	a, b := secureTrie.Trie().Size()
	fmt.Println("secure trie size: ", a, b)
	secureTrie.Commit(nil)
	fmt.Println("commit trie\n")
	// print trie nodes
	//fmt.Println("\nprint norm trie")
	//normTrie.Print()
	fmt.Println("\nprint secure trie")
	secureTrie.Trie().Print()

	//fmt.Println("norm trie root: ", normTrie.Hash().Hex())
	//fmt.Println("secure trie root: ", secureTrie.Hash().Hex())

	fmt.Println("\n\n\n")

	/*
	// change nonce of norm trie root node
	fmt.Println("set new nonce for norm trie")
	normTrie.SetRootNonce(300)
	normTrie.Print()
	fmt.Println("new norm trie root: ", normTrie.Hash().Hex())

	// change nonce of secure trie root node
	fmt.Println("\n\n\nset new nonce for secure trie")
	secureTrie.Trie().SetRootNonce(300)
	secureTrie.Trie().Print()
	fmt.Println("new secure trie root: ", secureTrie.Hash().Hex())
	*/

	
}

