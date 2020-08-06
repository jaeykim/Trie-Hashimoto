// how to use in geth console
// > loadScript("PATH_TO_THIS_SCRIPT")
// example: > loadScript("./experiment/getBlockTxCount.js")

// find the blocks which not have 200 txs (for experiment debugging)
var targetTxCount = 200;
var invalidBlockNumbers = [];
var invalidBlocksTxCounts = [];

// iterate all blocks
for (var i = 1; i <= eth.blockNumber; i++) {
    txCount = eth.getBlock(i).transactions.length
    if (txCount != targetTxCount) {
        invalidBlockNumbers.push(i);
        invalidBlocksTxCounts.push(txCount);
    }
    console.log("block", i, "'s tx count:", txCount, "-> invalid block list:", invalidBlockNumbers);
}

// print result
console.log("\nthere are", invalidBlockNumbers.length, "invalid blocks which has weird tx count");
for (var i = 0; i < invalidBlockNumbers.length; i++) {
    console.log("   invalid block number:", invalidBlockNumbers[i], "/ tx count:", invalidBlocksTxCounts[i]);
}
console.log("")
