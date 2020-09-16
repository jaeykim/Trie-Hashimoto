import csv
import matplotlib as mpl
mpl.use('Agg')
import matplotlib.pyplot as plt
import numpy as np
import sys

LOGFILEPATH = 'imptData/impt_data_original_geth_block_process_time/impt_log_files/impt_database_inspect.txt'
GRAPHPATH = 'collectedData/imptGraph/DBInspectResult/'  # impt graph image path

# db inspect epoch (per x blocks)
DBINSPECTEPOCH = 10000

SIZESNUM = 21
# meaning of each log line (21 contents) (delimiter: ',') (unit: KB) (type: float)
# headerSize, bodySize, receiptSize, tdSize, numHashPairing, hashNumPairing, txlookupSize, bloomBitsSize, trieSize, preimageSize,
# cliqueSnapsSize, metadata, ancientHeaders, ancientBodies, ancientReceipts, ancientTds, ancientHashes, chtTrieNodes, bloomTrieNodes, total, unaccounted
graphNames = ["headerSize", "bodySize", "receiptSize", "tdSize", "numHashPairing", "hashNumPairing", "txlookupSize", "bloomBitsSize", "trieSize", "preimageSize",
"cliqueSnapsSize", "metadata", "ancientHeaders", "ancientBodies", "ancientReceipts", "ancientTds", "ancientHashes", "chtTrieNodes", "bloomTrieNodes", "total", "unaccounted"]


# make empty 2d list -> list[b][a]
def TwoD(a, b): 
    # lst = [[0 for col in range(a)] for col in range(b)]
    # return lst
    return np.zeros(a * b, dtype=float).reshape(b, a)

LINENUM = sum(1 for line in open(LOGFILEPATH))
sizes = TwoD(LINENUM, SIZESNUM) # sizes[contents index] = list of its inspected sizes



# read data from log file
f = open(LOGFILEPATH, 'r')
rdr = csv.reader(f)
lineNum = 0
for line in rdr:
    if len(line) == 0:
        continue

    for i in range(len(line)-1):
        sizes[i][lineNum] = float(line[i])

    lineNum = lineNum + 1

f.close()



# draw graphs
print("Drawing graphs...")
blockNums = list(range(1,LINENUM+1))
blockNums = [DBINSPECTEPOCH*x for x in blockNums]
for i in range(SIZESNUM):
    plt.figure()
    plt.title(graphNames[i], pad=10)            # set graph title
    plt.xlabel('block num', labelpad=10)             # set x axis
    plt.ylabel(graphNames[i] + ' (KB)', labelpad=10)      # set y axis
    plt.plot(blockNums, sizes[i]) # draw scatter graph

    # save graph
    graphName = graphNames[i]
    plt.savefig(GRAPHPATH+graphName)

print("Done")
