# 
# sync node who receives sync data from sender & send "run geth" command to sender
# $ python3 receiver.py dataType syncType syncNum
# ex. python3 receiver.py GETH fast 10
# --> original geth fast sync, repeat 10 times
# 

from web3 import Web3 # version: web3==5.12.0
import sys
import socket
import os
import time

# Fullnode Settings
FULL_ADDR = "localhost"
FULL_PORT = "8081"
FULL_READY_PORT = "8091"

# Syncnode Settings
SYNC_ADDR = "localhost"
SYNC_PORT = "8084"
SYNC_READY_PORT = "8089"
GENESIS_PATH   = "../genesis.json"
DB_PATH = "/data/trieHashimoto/syncExp/"

# Sync settings for directory names
DATA_TYPE = sys.argv[1] # "GETH" or "TH" (original geth vs trie-hashimoto)
SYNC_TYPE = sys.argv[2] # "fast" or "full"
SYNC_INFO = DATA_TYPE + "_" + SYNC_TYPE # prefix for db directory name. e.g. "GETH_fast", "TH_full"
SYNC_NUMBER = int(sys.argv[3])
SYNC_TIME_LOG_PATH = "./syncTimeLogs/"  # should set this in geth too -> core/blockchain.go: insert() function to logging correctly
DB_LOG_PATH = "./dbInspectResults/"     # should set this in geth too -> core/blockchain.go: insert() function to logging correctly

# sync options
MAX_WAIT_TIME = 2*24*60*60 # seconds (how long do you wait for syncing)
MAX_WAIT_TIME = 5*60*60 # seconds (how long do you wait for syncing)

# Boundaries and Path
sync_boundaries = [100000]
# sync_boundaries = [50000, 100000, 150000, 200000, 250000, 300000, 350000, 400000, 450000, 500000]

# Providers
fullnode = Web3(Web3.HTTPProvider("http://" + FULL_ADDR + ":" + FULL_PORT))
syncnode = Web3(Web3.HTTPProvider("http://" + SYNC_ADDR + ":" + SYNC_PORT))

# Functions
def main():

    # make directory to save sync result log files
    Cmd = "mkdir " + SYNC_TIME_LOG_PATH
    os.system(Cmd)
    Cmd = "mkdir " + DB_LOG_PATH
    os.system(Cmd)

    for i in range(len(sync_boundaries)):

        print("sync_boundary:", sync_boundaries[i])

        # run full node
        print("trying to start full node...")
        while not runFullNode(DATA_TYPE, sync_boundaries[i]):
            time.sleep(5)
        
        # get full node's nodeinfo_enode
        print("try to get enode from full node")
        enode = fullnode.geth.admin.nodeInfo()['enode']
        # enode = enode.replace("127.0.0.1", FULL_ADDR) # is it needed? (maybe not...)
        
        # Create log directory
        dir_name = SYNC_INFO + "_" + str(sync_boundaries[i])
        print("Make directory [", DB_PATH + dir_name + "_log", "]")
        Cmd = "mkdir -p " + DB_PATH + dir_name + "_log"
        os.system(Cmd)

        # do sync for SYNC_NUMBER times
        for j in range(SYNC_NUMBER):
            # try syncing until it success
            while not runSyncNode(enode, dir_name, sync_boundaries[i], j):
                time.sleep(5)

        # kill full node
        while not killFullNode("kill"):
            time.sleep(1)



def killFullNode(message):
    print("Send kill signal to full node")
    try:
        # send kill message to full node
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.connect((FULL_ADDR, int(FULL_READY_PORT)))
        while fullnode.isConnected():
        s.send(bytes(str(message), 'utf8'))
            time.sleep(1)
        print("full node definitely turned off")
        return True

    except Exception as e:
        print(e)
        return False



def runFullNode(dataType, syncBoundary):
    print("Start Full Node For SyncBoundary : " + str(syncBoundary))
    try:
        # connecting to the full node ready server
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM) 
        s.connect((FULL_ADDR, int(FULL_READY_PORT)))
        cmd = dataType + "," + str(syncBoundary)
        s.send(bytes(cmd, 'utf8'))

        # check fullnode provider connection
        while not fullnode.isConnected():
            pass
        
        # successfully connected to full node
        print("connected to full node!")
        print("current block number:", fullnode.eth.blockNumber)
        return True

    except Exception as e:
        print(e)
        return False



def runSyncNode(enode, dirName, syncBoundary, n):
    print("Start "+ str(n) + "th" + SYNC_TYPE + "sync")
    file_name = SYNC_INFO + "_" + str(syncBoundary) + "_" + str(n)
    try:
        # set dirs for sync node
        dirName = dirName + "_data"
        fileName = SYNC_INFO + "_" + str(syncBoundary) + "_" + str(n)
        syncBoundary = syncBoundary
        print("Start", SYNC_TYPE, "sync! [" , file_name, "]")
        Cmd = "rm -rf " + DB_PATH + dirName
        os.system(Cmd)
        Cmd = "rm -rf " + DB_PATH + dirName + "_ethash"
        os.system(Cmd)

        # init sync node
        Cmd = "../geth --datadir=\"" + DB_PATH + dirName + "\" init " + GENESIS_PATH
        os.system(Cmd)
        print("sync node init finished")

        # run sync node at background
        if SYNC_TYPE == "full":
            # full archival sync
            Cmd = "../geth --datadir \"" + DB_PATH + dirName + "\" --ethash.dagdir \"" + DB_PATH + dirName + "_ethash\" --keystore \"../keystore\" --gcmode archive --syncmode \"" + SYNC_TYPE + "\" --networkid 12345 --rpc --rpcaddr \"" + SYNC_ADDR + "\" --rpcport \"" + SYNC_PORT + "\" --rpccorsdomain \"*\" --port 30304 --rpcapi=\"admin,db,eth,debug,miner,net,shh,txpool,personal,web3\" --syncboundary " + str(syncBoundary) + " --allow-insecure-unlock &" 
        else:
            # fast non-archival sync
            Cmd = "../geth --datadir \"" + DB_PATH + dirName + "\" --ethash.dagdir \"" + DB_PATH + dirName + "_ethash\" --keystore \"../keystore\" --syncmode \"" + SYNC_TYPE + "\" --networkid 12345 --rpc --rpcaddr \"" + SYNC_ADDR + "\" --rpcport \"" + SYNC_PORT + "\" --rpccorsdomain \"*\" --port 30304 --rpcapi=\"admin,db,eth,debug,miner,net,shh,txpool,personal,web3\" --syncboundary " + str(syncBoundary) + " --allow-insecure-unlock &" 
        print("run sync node cmd:", Cmd)
        os.system(Cmd)

        # check syncnode provider connection
        while not syncnode.isConnected():
            print("trying to connect to syncnode...")
            time.sleep(1)
        print("syncnode provider connected!")

        # wait until sync start
        waitTime = 0
        while syncnode.eth.syncing is False:
            if waitTime % 10 == 0:
                syncnode.geth.admin.addPeer(enode)
            print("\rtrying to addPeer... tried %d times" % (waitTime/10+1), end="")
            waitTime += 1
            time.sleep(1)
        print("sync is started")
        syncStartTime = time.time()

        # wait until whole sync done and terminate
        while syncnode.isConnected():
            tempSyncInfo = syncnode.eth.syncing
            if tempSyncInfo is not False:
                syncInfo = tempSyncInfo
            time.sleep(1)
            if time.time() - syncStartTime >= MAX_WAIT_TIME:
                raise Exception("waited too long, maybe something goes wrong")
        syncEndTime = time.time()
        print("sync node disconnected")
        
        # save result
        print("sync is finished: takes %d seconds" % (syncEndTime-syncStartTime))
        print("sync info:", syncInfo)
        logFile = open(SYNC_TIME_LOG_PATH + DATA_TYPE + "_" + SYNC_TYPE + "_" + str(syncBoundary) + ".txt", "a+")
        logFile.write(str(syncEndTime-syncStartTime))
        logFile.write("," + str(syncInfo))
        logFile.write("\n")
        logFile.close()

        # clear data
        Cmd = "rm -rf " + DB_PATH + dirName
        os.system(Cmd)
        Cmd = "rm -rf " + DB_PATH + dirName + "_ethash"
        os.system(Cmd)

        return True

    except Exception as e:
        # error occured, kill sync node
        print("ERROR: error occured while syncing:", e)
        print("kill sync node, and try to sync again")
        killCmd = "fuser -k " + SYNC_PORT + "/tcp"
        os.system(killCmd)
        return False



if __name__ == "__main__":
    main()
    print("DONE")
