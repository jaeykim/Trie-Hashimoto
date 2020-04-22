import csv
import matplotlib as mpl
mpl.use('Agg')
import matplotlib.pyplot as plt


TrieDBNum = 6   # num of leveldb for indexed trie nodes
LogFilePath = 'impt_data_log.txt'   # impt log file path
# LogFilePath = 'imptData/impt_data_log_not_indexed_1000.txt' # impt log file path
GraphPath = 'collectedData/imptGraph/'  # impt graph image path

# db search times for each block
trieDBTimes = list()
for i in range(TrieDBNum):
    trieDBTimes.append(list())
totalDBTimes = list()

# db avg search times for all blocks
trieDBAvgTimes = list()
trieDBSizes = list()
for i in range(TrieDBNum):
    trieDBAvgTimes.append(list())
    trieDBSizes.append(list())
totalDBAvgTimes = list()
totalDBSizes = list()

# db search times for all blocks
allTrieDBTimes = list()
for i in range(TrieDBNum):
    allTrieDBTimes.append(list())
allTotalDBTimes = list()



# read data from log file
f = open(LogFilePath, 'r')
# f = open('imptData/impt_data_log_indexed_1000.txt', 'r')
rdr = csv.reader(f)
for line in rdr:
    if len(line) == 0:
        continue

    # print(line)
    # for word in line:
    #     print(word)

    lastElement = line[-1]
    line = line[:-1]
    line = list(map(int, line))

    if len(line) != 0 and lastElement != '@':
        # get db search times
        # print("add", line[1], "to trie db", line[0])
        # print("add", line[2], "to total db")
        trieDBTimes[line[0]].append(line[1])
        allTrieDBTimes[line[0]].append(line[1])
        totalDBTimes.append(line[2])
        allTotalDBTimes.append(line[2])
    else:
        # print block num & db size
        blockNum = line[0]
        sizesLog = ""
        for i in range(TrieDBNum+1):
            sizesLog += str(line[1+i]) + " KB / "
        print("db size at block", blockNum, ":",sizesLog)

        # print avg search time
        for i in range(TrieDBNum):
            if len(trieDBTimes[i]) != 0:
                avgTime = sum(trieDBTimes[i])/len(trieDBTimes[i])
                print("average search time in trie db", i, ":\t", int(avgTime), "ns", "(", len(trieDBTimes[i]), "queries )")
                if line[1+i] != 0:
                    trieDBAvgTimes[i].append(int(avgTime))
                    trieDBSizes[i].append(line[1+i])
                # print("append", int(avgTime), "to trieDBAvgTimes[",i,"] / append ", line[1+i], "to trieDBSizes[",i,"]")

        if len(totalDBTimes) != 0:
            avgTime = sum(totalDBTimes)/len(totalDBTimes)
            print("average search time in total db :\t", int(avgTime), "ns", "(", len(totalDBTimes), "queries )")
            if line[TrieDBNum+1] != 0:
                totalDBAvgTimes.append(int(avgTime))
                totalDBSizes.append(line[TrieDBNum+1])
        else:
            print("no db search in this block")
        print()

        # reset lists for the next block
        trieDBTimes = list()
        for i in range(TrieDBNum):
            trieDBTimes.append(list())
        totalDBTimes = list()

f.close()



# print total avg search time
print("\n\nfinal result\n")
trieTimeSum = 0
trieTimeCnt = 0
for i in range(TrieDBNum):
    if len(allTrieDBTimes[i]) != 0:
        avgTime = sum(allTrieDBTimes[i])/len(allTrieDBTimes[i])
        print("average search time in trie db", i, ":\t", int(avgTime), "ns", "(", len(allTrieDBTimes[i]), "queries )")
        trieTimeSum = trieTimeSum + sum(allTrieDBTimes[i])
        trieTimeCnt = trieTimeCnt + len(allTrieDBTimes[i])

print("\naverage search time in trie db :\t", int(trieTimeSum/trieTimeCnt), "ns", "(", trieTimeCnt, "queries )")

avgTime = sum(allTotalDBTimes)/len(allTotalDBTimes)
print("average search time in total db :\t", int(avgTime), "ns", "(", len(allTotalDBTimes), "queries )\n")



# outlier criteria (ex. outlierCrt: 10  =>  outlier > 10*avg)
outlierCrt = 5
print("\n\nreal final result ( outlier criteria: x", outlierCrt, ")\n")

# print average search time without outliers
realTrieTimeSum = 0
realTrieTimeCnt = 0
trieOutlierCnt = 0
realTrieAvgTimes = [0]*TrieDBNum
for i in range(TrieDBNum):
    if len(allTrieDBTimes[i]) != 0:
        avgTime = int(sum(allTrieDBTimes[i])/len(allTrieDBTimes[i]))
        realList = [time for time in allTrieDBTimes[i] if int(time) < outlierCrt*avgTime]
        outlierList = [time for time in allTrieDBTimes[i] if int(time) >= outlierCrt*avgTime]
        print("real average search time in trie db", i, ":\t", int(sum(realList)/len(realList)), "ns", "(", len(realList), "queries /", len(outlierList), "outliers", ")")
        realTrieTimeSum = realTrieTimeSum + sum(realList)
        realTrieTimeCnt = realTrieTimeCnt + len(realList)
        trieOutlierCnt = trieOutlierCnt + len(outlierList)
        realTrieAvgTimes[i] = int(sum(realList)/len(realList))

print("\nreal average search time in trie db :\t", int(realTrieTimeSum/realTrieTimeCnt), "ns", "(", realTrieTimeCnt, "queries /", trieOutlierCnt, "outliers", ")")

avgTime = sum(allTotalDBTimes)/len(allTotalDBTimes)
realList = [time for time in allTotalDBTimes if int(time) < outlierCrt*avgTime]
outlierList = [time for time in allTotalDBTimes if int(time) >= outlierCrt*avgTime]
print("real average search time in total db :\t", int(sum(realList)/len(realList)), "ns", "(", len(realList), "queries /", len(outlierList), "outliers", ")\n")



# draw graphs
print("drawing graphs...")
for i in range(TrieDBNum):
    if len(trieDBAvgTimes[i]) != 0:
        # delete outliers
        # for j in range(len(trieDBAvgTimes)):
            # if trieDBAvgTimes[i][j] > outlierCrt*realTrieAvgTimes[i]:
            #     del trieDBAvgTimes[i][j]
            #     del trieDBSizes[i][j]

        # draw graph
        plt.figure()                                        # set new graph
        plt.title('trie node DB'+str(i), pad=10)            # set graph title
        plt.xlabel('DB size (KB)', labelpad=10)             # set x axis
        plt.ylabel('DB search time (ns)', labelpad=10)      # set y axis
        plt.scatter(trieDBSizes[i], trieDBAvgTimes[i], s=1) # draw scatter graph

        # save graph
        graphName = "avgSearchTime_DB_"+str(i)
        plt.savefig(GraphPath+graphName)

if len(totalDBAvgTimes) != 0:
    # delete outliers

    # draw graph
    plt.figure()
    plt.title('total DB', pad=10)
    plt.xlabel('DB size (KB)', labelpad=10)
    plt.ylabel('DB search time (ns)', labelpad=10)
    plt.scatter(totalDBSizes, totalDBAvgTimes, s=1)
    graphName = "avgSearchTime_totalDB"
    plt.savefig(GraphPath+graphName)

print("DONE")
