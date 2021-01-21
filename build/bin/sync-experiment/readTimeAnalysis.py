import csv
import matplotlib as mpl
mpl.use('Agg')
import matplotlib.pyplot as plt
import numpy as np
import sys
from itertools import zip_longest
from pathlib import Path

# paths
GRAPH_PATH = "./graphs/"

# variables
MAX_BLOCK_NUM = 200000
HASH_PREFIX_LEN = 6

# make empty 2d list -> list[b][a]
def TwoD(a, b, isInt):
    if isInt:
        return np.zeros(a * b, dtype=int).reshape(b, a)
    else:
        return np.zeros(a * b, dtype=float).reshape(b, a)
    


# make the directory if not exist
def makeDir(path):
    Path(path).mkdir(parents=True, exist_ok=True) 



def analysisTrieNodeReadTimeLog():

    print("start analysisTrieNodeReadTimeLog()")
    
    readTimes = []
    for i in range(MAX_BLOCK_NUM+1):
        readTimes.append([])
    
    # read data from log file
    f = open("./trieNodeReadLogs/readTimes.txt", 'r')
    rdr = csv.reader(f)
    cnt = 0
    for line in rdr:

        # if cnt == 1000:
        #     break
        cnt = cnt + 1

        if len(line) == 0:
            continue
        
        nodeHash = line[0]
        readTime = int(line[1])

        blockNumber = int(nodeHash[2:2+HASH_PREFIX_LEN], 16)
        readTimes[blockNumber].append(readTime)

    # calc avg read time
    avgReadTimes = [None] * (MAX_BLOCK_NUM+1)
    for i in range(MAX_BLOCK_NUM+1):
        cnt = len(readTimes[i])
        if cnt == 0:
            avgReadTimes[i] = 0
            continue
        totalTime = sum(readTimes[i])
        avgReadTimes[i] = int(totalTime/cnt)

        # except outliers
        if cnt < 20:
            avgReadTimes[i] = 0
            continue
        percent = 20
        newReadTimes = readTimes[i][int(cnt/percent):cnt-int(cnt/percent)]
        newCnt = len(newReadTimes)
        avgReadTimes[i] = int(sum(newReadTimes)/newCnt)

        # set upper bound (temply)
        if avgReadTimes[i] > 400000:
            avgReadTimes[i] = 0

    # draw graphs
    print("Drawing graphs...")
    blockNums = list(range(0,MAX_BLOCK_NUM+1))
    plt.figure()
    plt.title('trie node read time', pad=10)                # set graph title
    plt.xlabel('block num', labelpad=10)                    # set x axis
    plt.ylabel('average db read time (ns)', labelpad=10)    # set y axis
    plt.scatter(blockNums, avgReadTimes, s=0.3)               # draw scatter graph
    graphName = 'averageTrieNodeReadTime'
    plt.savefig(GRAPH_PATH+graphName)

    # calc read count
    readCounts = [None] * (MAX_BLOCK_NUM+1)
    for i in range(MAX_BLOCK_NUM+1):
        readCounts[i] = len(readTimes[i])

    # draw graphs
    print("Drawing graphs...")
    plt.figure()
    plt.title('trie node read count', pad=10)                   # set graph title
    plt.xlabel('block num', labelpad=10)                        # set x axis
    plt.ylabel('number of trie node read count', labelpad=10)   # set y axis
    plt.scatter(blockNums, readCounts, s=1)                     # draw scatter graph
    graphName = 'trieNodeReadCount'
    plt.savefig(GRAPH_PATH+graphName)



if __name__ == "__main__":

    makeDir(GRAPH_PATH)

    analysisTrieNodeReadTimeLog()
    print("Done")
