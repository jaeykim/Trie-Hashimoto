import csv

# num of leveldb for indexed trie nodes
TrieDBNum = 5

# db search times for each block
trieDBTimes = list()
for i in range(TrieDBNum):
    trieDBTimes.append(list())
totalDBTimes = list()

# db search times for all blocks
allTrieDBTimes = list()
for i in range(TrieDBNum):
    allTrieDBTimes.append(list())
allTotalDBTimes = list()

# read data from log file
f = open('impt_data_log.txt', 'r')
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
        sizes = ""
        for i in range(TrieDBNum+1):
            sizes += str(line[1+i]) + " KB / "
        print("db size at block", blockNum, ":",sizes)

        # print avg search time
        for i in range(TrieDBNum):
            if len(trieDBTimes[i]) != 0:
                avgTime = sum(trieDBTimes[i])/len(trieDBTimes[i])
                print("average search time in trie db", i, ":   ", int(avgTime), "ns", "(", len(trieDBTimes[i]), "queries )")

        if len(totalDBTimes) != 0:
            avgTime = sum(totalDBTimes)/len(totalDBTimes)
            print("average search time in total db:     ", int(avgTime), "ns", "(", len(totalDBTimes), "queries )")
        else:
            print("no db search in this block")
        print()

        # reset lists for the next block
        trieDBTimes = list()
        for i in range(TrieDBNum):
            trieDBTimes.append(list())
        totalDBTimes = list()
        
# print total avg search time
print("\n\nfinal result")
trieTimeSum = 0
trieTimeCnt = 0
for i in range(TrieDBNum):
    if len(allTrieDBTimes[i]) != 0:
        avgTime = sum(allTrieDBTimes[i])/len(allTrieDBTimes[i])
        print("average search time in trie db", i, ":   ", int(avgTime), "ns", "(", len(allTrieDBTimes[i]), "queries )")
        trieTimeSum = trieTimeSum + sum(allTrieDBTimes[i])
        trieTimeCnt = trieTimeCnt + len(allTrieDBTimes[i])

print("\naverage search time in trie db:      ", int(trieTimeSum/trieTimeCnt), "ns", "(", trieTimeCnt, "queries )")

avgTime = sum(allTotalDBTimes)/len(allTotalDBTimes)
print("average search time in total db:     ", int(avgTime), "ns", "(", len(allTotalDBTimes), "queries )\n")

f.close()
