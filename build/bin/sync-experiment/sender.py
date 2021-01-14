# 
# full node who sends sync data to receiver
# $ python3 sender.py
# 

import socket,os
import subprocess
import time

# Port
FULL_PORT           = "8081"
FULL_READY_PORT     = "8091"

# Path (test data path for debugging: "/data/impt_debug/db_full")
GENESIS_PATH    = "../genesis.json"
GETH_DB_PATH    = "/ssd/original_geth/db_full"  # original geth data
TH_DB_PATH      = "/ssd/impt_geth/db_full"      # trie-hashimoto data

# get ip address automatically
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.connect(("8.8.8.8", 80))
FULL_ADDR = str(s.getsockname()[0])
s.close()

# try to open socket
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)  
def openSocket():
    try:
        sock.bind((FULL_ADDR, int(FULL_READY_PORT)))  
        sock.listen(5)
        print("open socket! address -> %s:%s" % (FULL_ADDR, FULL_READY_PORT))
        return True
    
    except Exception as e:
        print("\rtrying to openSocket...", end="")
        return False



def main():
    # trying to open socket (to avoid "already use port" error)
    while not openSocket():
        time.sleep(1)

    # wait for remote command
    while True:  
        print("WAITING for remote commands....")
        connection, address = sock.accept()
        while True:
            # receive message from remote
            print("connected!", "(connection:", connection, ", address:", address, ")")
            data = connection.recv(1024)
            if not data:
                print("no data")
                break

            # decode message to unicode string 
            words = str(data, 'utf8').split(",")
            print("receive command:", words)

            # execute command
            if len(words) == 2 and words[1].isdigit():
                # run full node at background
                dataType = words[0]
                syncBoundary = words[1]
                if dataType == "GETH":
                    DB_PATH = GETH_DB_PATH
                elif dataType == "TH":
                    DB_PATH = TH_DB_PATH
                print("START FULL NODE! [ SyncBoundary : " , syncBoundary, "/ DataType:", dataType, "]")
                Cmd = "../geth --datadir \"" + DB_PATH + "\" --ethash.dagdir \"" + DB_PATH + "_ethash\" --keystore \"../keystore\" --gcmode archive --networkid 12345 --rpc --rpcaddr \"" + FULL_ADDR + "\" --rpcport \"" + FULL_PORT + "\" --rpccorsdomain \"*\" --port 30303 --rpcapi=\"admin,db,eth,debug,miner,net,shh,txpool,personal,web3\" --syncboundary " + syncBoundary + " --allow-insecure-unlock &"
                print("cmd:", Cmd)
                os.system(Cmd)
                print("FULL NODE EXECUTED!")

            elif words[0] == "kill":
                # kill port, terminate full node
                killCmd = "fuser -k " + FULL_PORT + "/tcp"
                os.system(killCmd)
                print("killed")

            else:
                print("unknown command (messages = ", str(data, 'utf8'), ")")
            break



if __name__ == "__main__":
    main()
    print("DONE")
