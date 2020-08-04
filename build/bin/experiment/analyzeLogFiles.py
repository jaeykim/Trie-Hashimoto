import csv
import matplotlib as mpl
mpl.use('Agg')
import matplotlib.pyplot as plt
import numpy as np
import sys
from itertools import zip_longest
from pathlib import Path

IMPTLOGPATH ='imptData/impt_data_blockNum_indexed_block_process_time/impt_log_files/'
IMPTCSVPATH = IMPTLOGPATH + 'csv/'
IMPTGRAPHPATH = IMPTLOGPATH + 'graph/'
ORIGINALLOGPATH = 'imptData/impt_data_original_geth_block_process_time/impt_log_files/'
ORIGINALCSVPATH = ORIGINALLOGPATH + 'csv/'
ORIGINALGRAPHPATH = ORIGINALLOGPATH + 'graph/'

MAXBLOCKNUM = 10

# make empty 2d list -> list[b][a]
def TwoD(a, b, isInt):
    if isInt:
        return np.zeros(a * b, dtype=int).reshape(b, a)
    else:
        return np.zeros(a * b, dtype=float).reshape(b, a)
    
# make the directory if not exist
def makeDir(path):
    Path(path).mkdir(parents=True, exist_ok=True) 



# analyze impt_block_process_time.txt log files
def analyzeBlockProcessTimeLog(isIMPT):

    # MAXBLOCKNUM = 10
    TIMESNUM = 7
    graphNames = ["totalCommitTxTime", "commitTxCount", "bodyWriteTime", "headerWriteTime", "stateFlushTime", "dirtyNodesCount", "blockInsertedTime"]
    
    if isIMPT:
        LOGFILEPATH = IMPTLOGPATH + 'impt_block_process_time.txt'
        GRAPHPATH = IMPTGRAPHPATH + 'blockProcessTime/'
    else:
        LOGFILEPATH = ORIGINALLOGPATH + 'impt_block_process_time.txt'
        GRAPHPATH = ORIGINALGRAPHPATH + 'blockProcessTime/'
    makeDir(GRAPHPATH)

    # times[0][blockNum] = totalCommitTxTime (ns)
    # times[1][blockNum] = commitTxCount
    # times[2][blockNum] = bodyWriteTime (ns)
    # times[3][blockNum] = headerWriteTime (ns)
    # times[4][blockNum] = stateFlushTime (ns)
    # times[5][blockNum] = dirtyNodesCount
    # times[6][blockNum] = blockInsertedTime(Unix time with ns)
    times = TwoD(MAXBLOCKNUM+1, TIMESNUM, True)

    # read data from log file
    f = open(LOGFILEPATH, 'r')
    rdr = csv.reader(f)
    cnt = 0
    blockNum = 1
    for line in rdr:
        if len(line) == 0:
            continue

        line = line[0].split(":")[:-1]

        if line[0][0] == 'c':
            # print("commitTxTime ->", line[1], "ns")
            times[0][blockNum] = times[0][blockNum] + int(line[1])
            times[1][blockNum] = times[1][blockNum] + 1
        elif line[0][0] == 'b':
            # print("blockWriteTime (body, header)->", line[2], "ns /", line[3], "ns")
            times[2][blockNum] = int(line[2])
            times[3][blockNum] = int(line[3])
        elif line[0][0] == 's':
            # print("stateFlushTime ->", line[1], "ns")
            times[4][blockNum] = int(line[1])
        elif line[0][0] == 'd':
            # print("# of dirte nodes ->", line[1])
            times[5][blockNum] = int(line[1])
        elif line[0][0] == 'i':
            # print("insertedblock (blockNum, insertedTime) ->", "block", line[1], "/ at" , line[2], "nano unix time")
            times[6][blockNum] = int(line[2])
            print("at block", line[1], "avg commit time:", int(times[0][blockNum]/times[1][blockNum]))
            if blockNum == MAXBLOCKNUM:
                break
            blockNum = blockNum + 1
            
        else:
            print("unknown error")
            sys.exit()
    f.close()



    # draw graphs
    print("Drawing graphs...")
    blockNums = list(range(1,MAXBLOCKNUM+1))

    # draw average commit tx time
    times[1][0] = 1 # to avoid DIVBYZERO error
    avgCommitTxTime = [int(totalTime/txCount) for totalTime,txCount in zip(times[0], times[1])]



    # make log data as a csv file
    data = [blockNums, times[0][1:], times[1][1:], avgCommitTxTime[1:], times[2][1:], times[3][1:], times[4][1:], times[5][1:], times[6][1:]]
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "block_process_time.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(("block number", "total commit tx time (ns)", "# of commit tx", "avg commit tx time (ns)", "body write time (ns)", "header write time (ns)", "state flush time (ns)", "# of dirty trie nodes", "block inserted unix time (ns)"))
        wr.writerows(export_data)
    myfile.close()



    maxAvgCommitTime = 20000000
    for i in range(len(avgCommitTxTime)):
        if avgCommitTxTime[i] > maxAvgCommitTime:
            avgCommitTxTime[i] = maxAvgCommitTime
    plt.figure()
    plt.title('averageCommitTxTime', pad=10)                # set graph title
    plt.xlabel('block num', labelpad=10)                    # set x axis
    plt.ylabel('average commit tx time (ns)', labelpad=10)  # set y axis
    plt.scatter(blockNums, avgCommitTxTime[1:], s=1)        # draw scatter graph
    graphName = 'averageCommitTxTime'
    plt.savefig(GRAPHPATH+graphName)

    # draw other graphs
    for i in range(2, TIMESNUM):
        plt.figure()
        ylabel = graphNames[i]
        if i != 5:
            ylabel = ylabel + '(ns)'
        plt.title(graphNames[i], pad=10)            # set graph title
        plt.xlabel('block num', labelpad=10)        # set x axis
        plt.ylabel(ylabel, labelpad=10)             # set y axis
        plt.scatter(blockNums, times[i][1:], s=1)   # draw scatter graph
        graphName = graphNames[i]
        plt.savefig(GRAPHPATH+graphName)



# analyze impt_database_inspect.txt log files
def analyzeDatabaseInspectLog(isIMPT):

    # db inspect epoch (per x blocks)
    DBINSPECTEPOCH = 10000

    # meaning of each log line (21 contents) (delimiter: ',') (unit: KB) (type: float)
    # headerSize, bodySize, receiptSize, tdSize, numHashPairing, hashNumPairing, txlookupSize, bloomBitsSize, trieSize, preimageSize,
    # cliqueSnapsSize, metadata, ancientHeaders, ancientBodies, ancientReceipts, ancientTds, ancientHashes, chtTrieNodes, bloomTrieNodes, total, unaccounted
    SIZESNUM = 21
    graphNames = ["headerSize", "bodySize", "receiptSize", "tdSize", "numHashPairing", "hashNumPairing", "txlookupSize", "bloomBitsSize", "trieSize", "preimageSize",
    "cliqueSnapsSize", "metadata", "ancientHeaders", "ancientBodies", "ancientReceipts", "ancientTds", "ancientHashes", "chtTrieNodes", "bloomTrieNodes", "total", "unaccounted"]

    if isIMPT:
        LOGFILEPATH = IMPTLOGPATH + 'impt_database_inspect.txt'
        GRAPHPATH = IMPTGRAPHPATH + 'databaseInspect/'
    else:
        LOGFILEPATH = ORIGINALLOGPATH + 'impt_database_inspect.txt'
        GRAPHPATH = ORIGINALGRAPHPATH + 'databaseInspect/'
    makeDir(GRAPHPATH)

    LINENUM = sum(1 for line in open(LOGFILEPATH))
    sizes = TwoD(LINENUM, SIZESNUM, False) # sizes[contents index] = list of its inspected sizes

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

    # make log data as a csv file
    blockNums = list(range(1,LINENUM+1))
    blockNums = [DBINSPECTEPOCH*x for x in blockNums]
    data = [blockNums]
    for i in range(SIZESNUM):
        data.append(sizes[i])
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "database_inspect.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(["all sizes are KB"])
        wr.writerow(("block number", "headerSize", "bodySize", "receiptSize", "tdSize", "numHashPairing", "hashNumPairing", "txlookupSize", "bloomBitsSize", "trieSize", "preimageSize",
    "cliqueSnapsSize", "metadata", "ancientHeaders", "ancientBodies", "ancientReceipts", "ancientTds", "ancientHashes", "chtTrieNodes", "bloomTrieNodes", "total", "unaccounted"))
        wr.writerows(export_data)
    myfile.close()

    # draw graphs
    print("Drawing graphs...")
    for i in range(SIZESNUM):
        plt.figure()
        plt.title(graphNames[i], pad=10)                    # set graph title
        plt.xlabel('block num', labelpad=10)                # set x axis
        plt.ylabel(graphNames[i] + ' (KB)', labelpad=10)    # set y axis
        plt.plot(blockNums, sizes[i])                       # draw scatter graph

        # save graph
        graphName = graphNames[i]
        plt.savefig(GRAPHPATH+graphName)



# analyze impt_data_log.txt log files
def analyzeDatabaseReadTimeLog(isIMPT):

    # num of leveldb for indexed trie nodes (1 means no additional db for trie nodes)
    TRIEDBNUM = 1

    # MAXBLOCKNUM = 2200
    DATANUM = 3

    if isIMPT:
        LOGFILEPATH = IMPTLOGPATH + 'impt_data_log.txt'
        GRAPHPATH = IMPTGRAPHPATH + 'databaseReadTime/'
    else:
        LOGFILEPATH = ORIGINALLOGPATH + 'impt_data_log.txt'
        GRAPHPATH = ORIGINALGRAPHPATH + 'databaseReadTime/'
    makeDir(GRAPHPATH)

    # logDatas[0][blockNum] = total database read time (ns)
    # logDatas[1][blockNum] = # of database read
    # logDatas[2][blockNum] = database size (KB)
    logDatas = TwoD(MAXBLOCKNUM+1, DATANUM, True)

    # read data from log file
    f = open(LOGFILEPATH, 'r')
    rdr = csv.reader(f)
    blockNum = 1
    for line in rdr:
        if len(line) == 0:
            continue

        lastElement = line[-1]
        line = line[:-1]
        line = list(map(int, line))

        if len(line) != 0 and lastElement != '@':
            # get db search times
            logDatas[0][blockNum] = logDatas[0][blockNum] + int(line[2])
            logDatas[1][blockNum] = logDatas[1][blockNum] + 1
        else:
            # get db size
            logDatas[2][blockNum] = int(line[2])
            if blockNum == MAXBLOCKNUM:
                break
            blockNum = blockNum + 1

    f.close()



    # make log data as a csv file
    blockNums = list(range(1,MAXBLOCKNUM+1))
    data = [blockNums, logDatas[0][1:], logDatas[1][1:], logDatas[2][1:]]
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "database_read_time.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(("block number", "total database read time (ns)", "# of database read", "database size (KB)"))
        wr.writerows(export_data)
    myfile.close()

    #
    # TODO?: drawing graphs
    #


# analyze impt_leveldb_compaction_log.txt log files
def analyzeLevelDBCompaction(isIMPT):

    # MAXBLOCKNUM = 1340
    DBNUM = 1   # num of db in the log file (1 means no additional db for trie nodes)
    MAXLEVEL = 7   # max num of levels in levelDB (ex. 7 means level 0~6 exists)
    COMPACTIONTYPESNUM = 5  # memory, level0, non-level0, seek, move compaction

    if isIMPT:
        LOGFILEPATH = IMPTLOGPATH + 'impt_leveldb_compaction_log.txt'
        GRAPHPATH = IMPTGRAPHPATH + 'leveldbCompaction/'
    else:
        LOGFILEPATH = ORIGINALLOGPATH + 'impt_leveldb_compaction_log.txt'
        GRAPHPATH = ORIGINALGRAPHPATH + 'leveldbCompaction/'
    makeDir(GRAPHPATH)

    # compactions[0][blockNum] = # of memory compaction
    # compactions[1][blockNum] = # of level0 compaction
    # compactions[2][blockNum] = # of non-level0 compaction
    # compactions[3][blockNum] = # of seek compaction
    # compactions[4][blockNum] = # of move compaction
    compactions = TwoD(MAXBLOCKNUM+1, COMPACTIONTYPESNUM, True)

    # tableCounts[level][blockNum] = # of tables at the level
    tableCounts = TwoD(MAXBLOCKNUM+1, MAXLEVEL, True)

    # tableSizes[level][blockNum] = table size at the level (MB)
    tableSizes = TwoD(MAXBLOCKNUM+1, MAXLEVEL, False)

    # durations[level][blockNum] = duration time at the level (sec)
    durations = TwoD(MAXBLOCKNUM+1, MAXLEVEL, False)

    # readSizes[level][blockNum] = read size at the level (MB)
    readSizes = TwoD(MAXBLOCKNUM+1, MAXLEVEL, False)

    # writeSizes[level][blockNum] = write size at the level (MB)
    writeSizes = TwoD(MAXBLOCKNUM+1, MAXLEVEL, False)

    # read data from log file
    f = open(LOGFILEPATH, 'r')
    rdr = csv.reader(f)
    blockNum = 1
    for line in rdr:
        if len(line) == 0:
            continue
        
        if len(line) > 1:
            if len(line) == 6:
                # find compaction count log
                # print("find compaction count info")
                # memComp = int(line[0])
                # level0Comp = int(line[1])
                # nonLevel0Comp = int(line[2])
                # seekComp = int(line[3])
                # moveComp = int(line[4])
                compactions[0][blockNum] = int(line[0])
                compactions[1][blockNum] = int(line[1])
                compactions[2][blockNum] = int(line[2])
                compactions[3][blockNum] = int(line[3])
                compactions[4][blockNum] = int(line[4])

            else:
                # find leveldb info log
                # print("find leveldb info log")
                level = int(line[0])
                # tables = int(line[1])
                # tablesSize = float(line[2])    # MB
                # duration = float(line[3])      # sec
                # readSize = float(line[4])      # MB
                # writeSize = float(line[5])     # MB

                tableCounts[level][blockNum] = int(line[1])
                tableSizes[level][blockNum] = float(line[2])
                durations[level][blockNum] = float(line[3])
                readSizes[level][blockNum] = float(line[4])
                writeSizes[level][blockNum] = float(line[5])

        else:
            if line[0] == "leveldbInfo":
                # find "leveldbInfo"
                # print("find leveldbinfo")
                pass
            else:
                # find "inserted block x --------------------"
                # x = int(line[0].split(" ")[2])
                print("now finished block", blockNum)
                if blockNum == MAXBLOCKNUM:
                    break
                blockNum = blockNum + 1

    f.close()



    # make log data as a csv file
    blockNums = list(range(1,MAXBLOCKNUM+1))
    data = [blockNums, compactions[0][1:], compactions[1][1:], compactions[2][1:], compactions[3][1:], compactions[4][1:]]
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "leveldb_compaction_count.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(("block number", "memory compaction", "level0 compaction", "non-level0 compaction", "seek compaction", "move compaction"))
        wr.writerows(export_data)
    myfile.close()

    data = [blockNums]
    columnTitles = ["block number"]
    for i in range(MAXLEVEL):
        data.append(tableCounts[i][1:])
        columnTitles.append("level " + str(i))
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "leveldb_compaction_info_table_count.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(["table count at each level"])
        wr.writerow(columnTitles)
        wr.writerows(export_data)
    myfile.close()

    data = [blockNums]
    columnTitles = ["block number"]
    for i in range(MAXLEVEL):
        data.append(tableSizes[i][1:])
        columnTitles.append("level " + str(i))
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "leveldb_compaction_info_table_size.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(["table size at each level (MB)"])
        wr.writerow(columnTitles)
        wr.writerows(export_data)
    myfile.close()

    data = [blockNums]
    columnTitles = ["block number"]
    for i in range(MAXLEVEL):
        data.append(durations[i][1:])
        columnTitles.append("level " + str(i))
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "leveldb_compaction_info_duration.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(["duration time at each level (sec)"])
        wr.writerow(columnTitles)
        wr.writerows(export_data)
    myfile.close()

    data = [blockNums]
    columnTitles = ["block number"]
    for i in range(MAXLEVEL):
        data.append(readSizes[i][1:])
        columnTitles.append("level " + str(i))
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "leveldb_compaction_info_read_size.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(["read size at each level (MB)"])
        wr.writerow(columnTitles)
        wr.writerows(export_data)
    myfile.close()

    data = [blockNums]
    columnTitles = ["block number"]
    for i in range(MAXLEVEL):
        data.append(writeSizes[i][1:])
        columnTitles.append("level " + str(i))
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "leveldb_compaction_info_write_size.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(["write size at each level (MB)"])
        wr.writerow(columnTitles)
        wr.writerows(export_data)
    myfile.close()

    #
    # TODO?: drawing graphs
    #



# analyze impt_which_level.txt log files
def analyzeLevelDBReadLevel(isIMPT):

    # max num of levels in levelDB (ex. 8 means level 0~6 & memory level (= level 7) exist)
    MAXLEVEL = 8

    # MAXBLOCKNUM = 1340

    if isIMPT:
        LOGFILEPATH = IMPTLOGPATH + 'impt_which_level.txt'
        GRAPHPATH = IMPTGRAPHPATH + 'leveldbReadLevel/'
    else:
        LOGFILEPATH = ORIGINALLOGPATH + 'impt_which_level.txt'
        GRAPHPATH = ORIGINALGRAPHPATH + 'leveldbReadLevel/'
    makeDir(GRAPHPATH)

    # levelCount[level][blockNum] = # of the level searched count at the block
    levelCount = TwoD(MAXBLOCKNUM+1, MAXLEVEL, True)

    # read data from log file
    f = open(LOGFILEPATH, 'r')
    rdr = csv.reader(f)
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
            if blockNum == MAXBLOCKNUM:
                break
            blockNum = blockNum + 1
            print()

    f.close()


    # make log data as a csv file
    blockNums = list(range(1,MAXBLOCKNUM+1))
    data = [blockNums]
    columnTitles = ["block number"]
    for i in range(MAXLEVEL):
        data.append(levelCount[i][1:])
        columnTitles.append("level " + str(i))
    columnTitles[-1] = "memory level"
    export_data = zip_longest(*data, fillvalue = '')
    if isIMPT:
        csvFilePath = IMPTCSVPATH
    else:
        csvFilePath = ORIGINALCSVPATH
    with open(csvFilePath + "leveldb_read_level.csv", 'w', encoding="ISO-8859-1", newline='') as myfile:
        wr = csv.writer(myfile)
        wr.writerow(["how many elements are searched at each level"])
        wr.writerow(columnTitles)
        wr.writerows(export_data)
    myfile.close()

    #
    # TODO?: drawing graphs
    #



if __name__ == "__main__":

    # make directory to save files/graphs
    makeDir(IMPTCSVPATH)
    makeDir(IMPTGRAPHPATH)
    makeDir(ORIGINALCSVPATH)
    makeDir(ORIGINALGRAPHPATH)

    # analyze impt_block_process_time.txt log files
    analyzeBlockProcessTimeLog(True)
    analyzeBlockProcessTimeLog(False)

    # analyze impt_database_inspect.txt log files
    analyzeDatabaseInspectLog(True)
    analyzeDatabaseInspectLog(False)

    # analyze impt_data_log.txt log files
    analyzeDatabaseReadTimeLog(True)
    analyzeDatabaseReadTimeLog(False)

    # analyze impt_leveldb_compaction_log.txt log files
    analyzeLevelDBCompaction(True)
    analyzeLevelDBCompaction(False)

    # analyze impt_which_level.txt log files
    analyzeLevelDBReadLevel(True)
    analyzeLevelDBReadLevel(False)

    print("Done")
