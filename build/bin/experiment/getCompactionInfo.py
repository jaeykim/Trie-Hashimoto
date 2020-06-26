import csv
import matplotlib as mpl
mpl.use('Agg')
import matplotlib.pyplot as plt
import sys

# LOGFILEPATH = 'imptData/impt_data_original_geth_level_logging/impt_leveldb_compaction_log.txt'
LOGFILEPATH = 'imptData/impt_data_blockNum_indexed_level_logging/impt_leveldb_compaction_log.txt'
GRAPHPATH = 'collectedData/imptGraph/compactionInfo/'  # impt graph image path

DBNUM = 1   # num of db in the log file
MAXLEVEL = 7   # max num of levels in levelDB (ex. 5 means level 0~4 exists)

dbIndex = 0 # db index (0: totaldb, 1: trieDB0, 2: trieDB1, ...)
maxBlockNum = 0 # max block number in the log file

# make empty 3d list -> list[c][b][a]
def ThreeD(a, b, c): 
    lst = [[[0 for col in range(a)] for col in range(b)] for row in range(c)] 
    return lst

# make empty 4d list -> list[d][c][b][a]
def FourD(a, b, c, d): 
    lst = [[[[0 for col in range(a)] for col in range(b)] for row in range(c)] for t in range(d)]
    return lst

def increaseDBIndex():
    # to iterate dbs
    global dbIndex
    dbIndex = dbIndex + 1
    if dbIndex == DBNUM:
        dbIndex = 0

def enum(*sequential, **named):
    enums = dict(zip(sequential, range(len(sequential))), **named)
    return type('Enum', (), enums)

# DBInfos[dbIndex][level][column]
# ex. DBInfos[0][1][2] means in totalDB, in level 1's duration time list for every blocks
LOGCOLUMNS = 5 # num of log columns (tableCount, tableSize, duration, readSize, writeSize)
Columns = enum('TABLE_COUNT','TABLE_SIZE','DURATION', 'READ_SIZE', 'WRITE_SIZE')
DBInfos = FourD(0, LOGCOLUMNS, MAXLEVEL, DBNUM)

# CompactionCounts[dbIndex][column]
# ex. CompactionCounts[0][1] means in totalDB, # of level0Compactions list for every blocks
COMPACTIONCOLUMNS = 5 # memComp, level0Comp, non-level0Comp, seekComp, moveComp
CompactionColumns = enum('MEMCOMP','LEVEL0COMP','NONLEVEL0COMP', 'SEEKCOMP', 'MOVECOMP')
CompactionCounts = ThreeD(0, COMPACTIONCOLUMNS, DBNUM)



# read data from log file
f = open(LOGFILEPATH, 'r')
rdr = csv.reader(f)
cnt = 0

for line in rdr:

    # cnt = cnt + 1
    # if cnt == 100:  # finish at block 11
    #     break

    if len(line) == 0:
        continue

    # print("line:", line)
    # for word in line:
    #     print("word:", word)
    
    if len(line) > 1:
        if len(line) == 6:
            # find compaction count log
            # print("find compaction count info")
            # memComp = int(line[0])
            # level0Comp = int(line[1])
            # nonLevel0Comp = int(line[2])
            # seekComp = int(line[3])
            # moveComp = int(line[4])

            # append compaction count to the list
            for i in range(COMPACTIONCOLUMNS):
                CompactionCounts[dbIndex][i].append(int(line[i]))

            # go to next db
            increaseDBIndex()
        else:
            # find leveldb info log
            # print("find leveldb info log")
            level = int(line[0])
            tables = int(line[1])
            tablesSize = float(line[2])    # MB
            duration = float(line[3])      # sec
            readSize = float(line[4])      # MB
            writeSize = float(line[5])     # MB

            # append infos to the list
            # DBInfos[dbIndex][level][Columns.TABLE_COUNT].append(tables)
            # DBInfos[dbIndex][level][Columns.TABLE_SIZE].append(tablesSize)
            # DBInfos[dbIndex][level][Columns.DURATION].append(duration)
            # DBInfos[dbIndex][level][Columns.READ_SIZE].append(readSize)
            # DBInfos[dbIndex][level][Columns.WRITE_SIZE].append(writeSize)

            DBInfos[dbIndex][level][0].append(tables)
            DBInfos[dbIndex][level][1].append(tablesSize)
            DBInfos[dbIndex][level][2].append(duration)
            DBInfos[dbIndex][level][3].append(readSize)
            DBInfos[dbIndex][level][4].append(writeSize)
    else:
        if line[0] == "leveldbInfo":
            # find "leveldbInfo"
            # print("find leveldbinfo")
            pass
        else:
            # find "inserted block x --------------------"
            # print("find inserted block")
            blockNum = int(line[0].split(" ")[2])
            maxBlockNum = blockNum
            print("now finished block", blockNum)
            print()
    

f.close()



# draw graphs

# range for x-axis to draw
startBlockNum = 1
endBlockNum = maxBlockNum

print("Draw levelDB infos graph")
for i in range(DBNUM):
    for level in range(MAXLEVEL):
        for column in range(LOGCOLUMNS):
            
            if len(DBInfos[i][level][column]) == 0:
                continue

            # draw new graph
            plt.figure()
            
            # set graph title
            graphTitle = ""
            if i == 0:
                graphTitle = graphTitle + "totalDB_"
            else:
                graphTitle = graphTitle + "trieDB" + str(i-1) + "_"
            graphTitle = graphTitle + "atLevel" +str(level) + "_"
            if column == 0:
                graphTitle = graphTitle + "tableCount"
                plt.ylabel('table count', labelpad=10)
            if column == 1:
                graphTitle = graphTitle + "tableSize"
                plt.ylabel('table size (MB)', labelpad=10)
            if column == 2:
                graphTitle = graphTitle + "compactionDuration"
                plt.ylabel('duration (sec)', labelpad=10)
            if column == 3:
                graphTitle = graphTitle + "compactionReadSize"
                plt.ylabel('read size (MB)', labelpad=10)
            if column == 4:
                graphTitle = graphTitle + "compactionWriteSize"
                plt.ylabel('write size (MB)', labelpad=10)
            plt.title(graphTitle, pad=10)
            plt.xlabel('Block Number', labelpad=10)             # set x axis
            blockNums = list(range(startBlockNum,endBlockNum+1))
            plt.scatter(blockNums[-len(DBInfos[i][level][column]):], DBInfos[i][level][column], s=1) # draw scatter graph

            # save graph
            plt.savefig(GRAPHPATH + graphTitle)

print("Draw levelDB compaction graph")
for i in range(DBNUM):
    for column in range(COMPACTIONCOLUMNS):

        if len(CompactionCounts[i][column]) == 0:
            continue

        # draw new graph
        plt.figure()

        # set graph title
        graphTitle = ""
        if i == 0:
            graphTitle = graphTitle + "totalDB_"
        else:
            graphTitle = graphTitle + "trieDB" + str(i-1) + "_"
        
        if column == 0:
                graphTitle = graphTitle + "memoryCompaction"
                plt.ylabel('memory compaction count', labelpad=10)
        if column == 1:
                graphTitle = graphTitle + "level0Compaction"
                plt.ylabel('level0 compaction count', labelpad=10)
        if column == 2:
                graphTitle = graphTitle + "nonLevel0Compaction"
                plt.ylabel('non-level0 compaction count', labelpad=10)
        if column == 3:
                graphTitle = graphTitle + "seekCompaction"
                plt.ylabel('seek compaction count', labelpad=10)
        if column == 4:
                graphTitle = graphTitle + "moveCompaction"
                plt.ylabel('move compaction count', labelpad=10)
        
        plt.title(graphTitle, pad=10)
        plt.xlabel('Block Number', labelpad=10)
        blockNums = list(range(startBlockNum,endBlockNum+1))
        plt.scatter(blockNums, CompactionCounts[i][column], s=1) # draw scatter graph

        # save graph
        plt.savefig(GRAPHPATH + graphTitle)

print("Done!")
