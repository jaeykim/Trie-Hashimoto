from web3 import Web3
import sys
#import socket
import random
#import json
#import rlp
import time
#import binascii
#import numpy as np
import os,binascii
from datetime import datetime
from multiprocessing import Pool

# Settings
FULL_PORT = "8081"
PASSWORD = "1234"

# multiprocessing to send transactions
THREAD_COUNT = 1

# tx arguments option
INCREMENTAL_RECEIVER_ADDRESS = False # set tx receiver: incremental vs random
MAX_ADDRESS = 100000000              # set max address to set the receiver address upper bound (0 means there is no bound)
INCREMENTAL_SEND_AMOUNT = True       # set send amount: incremental vs same (1 wei)

# providers
fullnode = Web3(Web3.HTTPProvider("http://localhost:" + FULL_PORT))

# functions
def main(accountNum, txPerBlock, miningThreadNum):
    
    ACCOUNT_NUM = accountNum
    TX_PER_BLOCK = txPerBlock
    MINING_THREAD_NUM = miningThreadNum # Geth's mining option

    if ACCOUNT_NUM < TX_PER_BLOCK:
        print("too less accounts. at least", TX_PER_BLOCK, "accounts are needed")
        return

    print("Insert ", ACCOUNT_NUM, " accounts")
    print("tx per block:", TX_PER_BLOCK)
    print("geth mining thread:", MINING_THREAD_NUM, "\n")

    # unlock coinbase
    fullnode.geth.personal.unlockAccount(fullnode.eth.coinbase, PASSWORD, 0)

    # get current block
    currentBlock = fullnode.eth.blockNumber

    # main loop for send txs
    print("start sending transactions")
    startTime = datetime.now()
    offset = 1
    cnt = 0
    txNums = [int(TX_PER_BLOCK/THREAD_COUNT)]*THREAD_COUNT
    txNums[0] += TX_PER_BLOCK%THREAD_COUNT
    for i in range(int(ACCOUNT_NUM / TX_PER_BLOCK)):
        # set arguments for multithreading function
        arguments = []
        for j in range(THREAD_COUNT):
            arguments.append((txNums[j], offset))
            offset += txNums[j]
        
        # send transactions
        sendPool.starmap(sendTransactions, arguments)
        cnt = cnt + TX_PER_BLOCK
        if cnt % 10000 == 0:
            elapsed = datetime.now() - startTime
            print("inserted ", (i+1)*TX_PER_BLOCK, "accounts / elapsed time:", elapsed)

        # mining
        fullnode.geth.miner.start(MINING_THREAD_NUM)  # start mining with multiple threads
        while (fullnode.eth.blockNumber == currentBlock):
            pass # just wait for mining
        fullnode.geth.miner.stop()  # stop mining
        currentBlock = fullnode.eth.blockNumber



def sendTransaction(to):
    #print("start try to send tx to full node")
    #print("to: ", to, "/ from: ", fullnode.eth.coinbase)
    while True:
        try:
            fullnode.eth.sendTransaction(
                {'to': to, 'from': fullnode.eth.coinbase, 'value': '1', 'gas': '21000', 'data': ""})
            break
        except:
            continue



def sendTransactions(num, offset):
    for i in range(int(num)):
        # set receiver
        if INCREMENTAL_RECEIVER_ADDRESS:
            to = intToAddr(int(offset+i))
        else:
            # if the upper bound is set, select receiver within the bound
            if MAX_ADDRESS != 0:
                to = intToAddr(random.randint(1, MAX_ADDRESS))
            # just any random address
            else:
                to = makeRandHex()

        # to = "0xe4f853b9d237b220f0ECcdf55d224c54a30032Df"
        
        # set send amount
        if INCREMENTAL_SEND_AMOUNT:
            amount = int(offset+i)
        else:
            amount = int(1)

        # print("to: ", to, "/ from: ", fullnode.eth.coinbase, "/ amount:", amount)

        while True:
            try:
                fullnode.eth.sendTransaction(
                    {'to': to, 'from': fullnode.eth.coinbase, 'value': hex(amount), 'gas': '21000', 'data': ""})
                break
            except:
                time.sleep(1)
                continue



def makeRandHex():
	randHex = binascii.b2a_hex(os.urandom(20))
	return Web3.toChecksumAddress("0x" + randHex.decode('utf-8'))



def intToAddr(num):
    intToHex = f'{num:0>40x}'
    return Web3.toChecksumAddress("0x" + intToHex)



if __name__ == "__main__":

    startTime = datetime.now()
    sendPool = Pool(THREAD_COUNT) # -> important: this should be in this "__main__" function

    isTH = True
    threadNums = [96, 64, 60, 56, 52, 48, 44, 40, 32, 24, 16, 8, 4, 2, 1]

    if not isTH:
        # for ethash: sending 'totalTxNum' txs and mining blocks including 'txPerBlock' txs with 'threadNum' threads
        totalTxNum = 1000
        txPerBlock = 1

        for threadNum in threadNums:
            main(totalTxNum-txPerBlock, txPerBlock, threadNum)
            # main(1, 1, threadNum) # for test
            elapsed = datetime.now() - startTime
            print("elapsed time:", elapsed)
            print("")
        
        for threadNum in threadNums:
            main(txPerBlock, txPerBlock, threadNum)
    else:
        # for TH: sending 'totalTxNum' txs and mining blocks including 'txPerBlock' txs with 'threadNum' threads
        totalTxNum = 200
        txPerBlock = 200

        for threadNum in threadNums:
            main(totalTxNum, txPerBlock, threadNum)
            # main(1, 1, threadNum) # for test
            elapsed = datetime.now() - startTime
            print("elapsed time:", elapsed)
            print("")

    elapsed = datetime.now() - startTime
    print("total elapsed time:", elapsed)
    print("DONE")
