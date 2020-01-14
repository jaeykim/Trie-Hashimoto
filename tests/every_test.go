package tests

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

func ExampleTest() {
	i := 255
	h := fmt.Sprintf("%x", i)
	//fmt.Printf("Hex conv of '%d' is '%s'\n", i, h)
	h = fmt.Sprintf("%X", i)
	//fmt.Printf("HEX conv of '%d' is '%s'\n", i, h)

	for i = 0; i < 32; i++ {
		h = fmt.Sprintf("%x", i)
		fmt.Printf("Hex conv of '%d' is '%s'\n", i, h)
		randAddr := common.HexToAddress(h)
		fmt.Println("random addr:", randAddr.Hex())
	}

	// output: 1
}
