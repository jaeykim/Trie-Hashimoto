import csv
import matplotlib as mpl
mpl.use('Agg')
import matplotlib.pyplot as plt


LogFilePath = 'imptData/impt_data_blockNum_indexed_level_logging/impt_which_level.txt' # impt log file path
# LogFilePath = 'imptData/impt_data_original_geth_level_logging/impt_which_level.txt' # impt log file path

GraphPath = 'collectedData/imptGraph/levelCount/'  # impt graph image path

MAXLEVEL = 7    # max num of levels in levelDB (ex. 5 means level 0~4 exists), and MAXLEVEL means memory level (ex. when MAXLEVEL=5, level 4 means memory db)
MAXBLOCKNUM = 800000 + 1    # max num of block (ex. 800000 + 1 means block 0 ~ 800000)

# make empty 2d list -> list[b][a]
def TwoD(a, b): 
    lst = [[0 for col in range(a)] for col in range(b)]
    return lst

# levelCount[level][blockNum] = # of the level searched count at block blockNum
levelCount = TwoD(MAXBLOCKNUM, MAXLEVEL)

# read data from log file
f = open(LogFilePath, 'r')
rdr = csv.reader(f)


cnt = 0

blockNum = 1    # start block number to logging
for line in rdr:
    if len(line) == 0:
        continue

    # print(line)
    words = line[0].split(" ")

    if len(words) == 5:
        levelNum = int(words[-1])
        levelCount[levelNum][blockNum] = levelCount[levelNum][blockNum] + 1
    
    if len(words) == 3:
        blockNum = int(words[-1])
        print("at block", blockNum)
        for i in range(MAXLEVEL):
            if i < MAXLEVEL - 1:
                print(" level", i, "count:", levelCount[i][blockNum])
            else:
                print(" memory db count:", levelCount[i][blockNum])
        blockNum = blockNum + 1
        if blockNum == MAXBLOCKNUM:
            break
        print()

    # cnt = cnt + 1
    # if cnt == 100:
    #     break

f.close()



# draw graphs
print("drawing graphs...")
for level in range(MAXLEVEL):
    plt.figure()                                            # set new graph
    plt.title('search count for level '+str(level), pad=10) # set graph title
    if level == MAXLEVEL-1:
        plt.title('search count for memory db', pad=10)
    plt.xlabel('Block Number', labelpad=10)                 # set x axis
    plt.ylabel('Count', labelpad=10)                        # set y axis

    blockNums = list(range(0, MAXBLOCKNUM))
    # plt.scatter(blockNums, levelCount[level], s=1)          # draw scatter graph
    plt.plot(blockNums, levelCount[level])          # draw scatter graph

    # save graph
    graphName = "searchCount_Level_"+str(level)
    if level == MAXLEVEL-1:
        graphName = "searchCount_MemoryDB"
    plt.savefig(GraphPath+graphName)



print("drawing accumulative graphs...")
for level in range(MAXLEVEL):

    for i in range(len(levelCount[level]) - 1):
        levelCount[level][i+1] = levelCount[level][i] + levelCount[level][i+1]

    plt.figure()                                            # set new graph
    plt.title('accumulative search count for level '+str(level), pad=10) # set graph title
    if level == MAXLEVEL-1:
        plt.title('search count for memory db', pad=10)
    plt.xlabel('Block Number', labelpad=10)                 # set x axis
    plt.ylabel('Count', labelpad=10)                        # set y axis

    blockNums = list(range(0, MAXBLOCKNUM))
    # plt.scatter(blockNums, levelCount[level], s=1)          # draw scatter graph
    plt.plot(blockNums, levelCount[level])          # draw scatter graph

    # save graph
    graphName = "accumulativeSearchCount_Level_"+str(level)
    if level == MAXLEVEL-1:
        graphName = "accumulativeSearchCount_MemoryDB"
    plt.savefig(GraphPath+graphName)


print("drawing total accumulative graphs...")
plt.figure()    # set new graph
plt.title('total accumulative search count', pad=10) # set graph title
plt.xlabel('Block Number', labelpad=10)                 # set x axis
plt.ylabel('Count', labelpad=10)                        # set y axis
blockNums = list(range(0, MAXBLOCKNUM))

colors = ['red', 'black', 'blue', 'brown', 'green', 'cyan', 'magenta']
colors = colors[:MAXLEVEL]
for i in range(len(colors)):
    label = "level " + str(i)
    if i == MAXLEVEL-1:
        label = "memory db"
    plt.plot(blockNums, levelCount[i], color=colors[i], label=label)
plt.legend(loc='best')

# save graph
graphName = "accumulativeSearchCount_total"
plt.savefig(GraphPath+graphName)

print("final result")
totalSearchCount = 0
for i in range(MAXLEVEL):
    totalSearchCount = totalSearchCount + levelCount[i][MAXBLOCKNUM-1]

for i in range(MAXLEVEL):
    if i < MAXLEVEL - 1:
        print(" level", i, "count:", levelCount[i][MAXBLOCKNUM-1], "(", float(levelCount[i][MAXBLOCKNUM-1]*100/totalSearchCount), "% )")
    else:
        print(" memory db count:", levelCount[i][MAXBLOCKNUM-1], "(", float(levelCount[i][MAXBLOCKNUM-1]*100/totalSearchCount), "% )")

print("DONE")
