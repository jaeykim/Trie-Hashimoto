package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"os/exec"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	// "github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/core/rawdb"
	// "github.com/ethereum/go-ethereum/ethdb"
	
)

func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}


func main() {

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
	// normTrie := trie.NewEmpty()
	dbPath := "index/index_total"	// path to store data
	dbNamespace := "eth/db/nodedata" // maybe meaningless
	ldb, _ := rawdb.NewLevelDBDatabase(dbPath, int(0), int(0), dbNamespace)
	tdb := trie.NewDatabase(ldb)
	// normTrie, _ := trie.New(common.Hash{}, tdb)
	nnormTrie, _ := trie.NewSecure(common.Hash{}, tdb)
	normTrie := nnormTrie.Trie()

	secureTrie := trie.NewEmptySecure()

	sizeCheckEpoch := 1
	accountsNum := 2
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
		// randHex := randomHex(20)
		// fmt.Println("random hex string:", randHex)

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
			nnormTrie.Commit(nil)
			normTrie.Commit(nil) // trie.Commit()을 먼저 해줘야 (flush to memory) trie.DiskDB().Commit()을 할 수 있음 (flush to disk = leveldb)
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

	// remove all prev data path folders
	//os.RemoveAll("./index")

	// // make new leveldbs
	// var levelDBs [5]ethdb.Database
	// // dbPath := "index/index"	// path to store data
	// // dbNamespace := "eth/db/nodedata" // maybe meaningless
	// for i := 0; i < len(levelDBs); i++ {
	// 	var err error
	// 	trie.GlobalTrieNodeDB[i], err = rawdb.NewLevelDBDatabase(dbPath + strconv.Itoa(i), int(0), int(0), dbNamespace + strconv.Itoa(i))
	// 	if err != nil {
	// 		fmt.Println("db open error")
	// 		return
	// 	}
	// }

	// inspect db size
	// rawdb.InspectDatabase(levelDBs[0])
	
	// // save key-value into db
	// levelDBs[0].Put([]byte("abc"),[]byte("def"))
	// levelDBs[0].Put([]byte("abcdg"),[]byte("def1"))
	// levelDBs[0].Put([]byte("abcd"),[]byte("def2"))

	// for i:=0; i<10; i++ {
	// 	str := strconv.Itoa(i)
	// 	levelDBs[0].Put([]byte(str),[]byte(str))
	// }

	// rawdb.InspectDatabase(levelDBs[0])

	// it := levelDBs[0].NewIterator()
	fmt.Println("start print trie db")
	normTrie.DB().Commit(normTrie.Hash(), false)
	it := ldb.NewIterator()
	for it.Next() {
		fmt.Println("node hash: ", common.BytesToHash(it.Key()).Hex())
		fmt.Println("	key: ", it.Key())
		fmt.Println("	value: ", it.Value())
	}
	fmt.Println("end print trie db")

	// // iterate trie nodes
	// it := trie.NewIterator(normTrie.NodeIterator(nil))
	// for it.Next() {
	// 	// nodeHash := it.Hash()


	// 	// key := common.BytesToAddress(it.Key)
	// 	// value := string(it.Value)
	// 	// fmt.Println("key: ", key.Hex(), " / value: ", value)
	// }

	fmt.Println("\nprint norm trie")
	normTrie.Print()
	fmt.Println("\nprint secure trie")
	secureTrie.Trie().Print()




	// ni := normTrie.NodeIterator(nil)
	// for ni.Next(true){
	// 	// var keyBytes []byte
	// 	keyBytes := ni.Hash().Bytes()
	// 	valueBytes, err := normTrie.TryGet(keyBytes)
	// 	if err != nil {
	// 		fmt.Println("there is no value with this key")
	// 	}
	// 	fmt.Println("node hash:", ni.Hash().Hex())
	// 	fmt.Println("node key:", keyBytes)
	// 	fmt.Println("node value:", valueBytes) // -> ??? 왜 못찾지 이거 key 값이 잘못됐나
	// 	fmt.Println("")

	// }





	// close leveldbs
	// for i:=0; i<len(levelDBs);i++{
	// 	closeErr:=levelDBs[i].Close()
	// 	if closeErr != nil{
	// 		fmt.Println("db close err!")
	// 	}
	// }

	// end
	return





	// print trie nodes
	fmt.Println("\nprint norm trie")
	normTrie.Print()
	fmt.Println("\nprint secure trie")
	secureTrie.Trie().Print()

	fmt.Println("norm trie root: ", normTrie.Hash().Hex())
	fmt.Println("secure trie root: ", secureTrie.Hash().Hex())

	fmt.Println("\n\n\n")

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
}

