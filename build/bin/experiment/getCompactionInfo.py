import csv
import matplotlib as mpl
mpl.use('Agg')
import matplotlib.pyplot as plt

LogFilePath = 'impt_leveldb_compaction_log.txt'

# read data from log file
f = open(LogFilePath, 'r')
rdr = csv.reader(f)
cnt = 0

for line in rdr:

    cnt = cnt + 1
    if cnt == 30:
        break

    if len(line) == 0:
        continue

    print("line:", line)
    for word in line:
        print("word:", word)
    
    if len(line) > 1:
        if len(line) == 5:
            # find compaction count log
            print("find compaction count info")
            memComp = line[0]
            level0Comp = line[1]
            nonLevel0Comp = line[2]
            seekComp = line[3]
        else:
            # find leveldb info log
            print("find leveldb info log")
            level = line[0]
            tables = line[0]
            tablesSize = line[0]    # MB
            duration = line[0]      # sec
            readSize = line[0]      # MB
            writeSize = line[0]     # MB
    else:
        if line[0] == "leveldbInfo":
            # find "leveldbInfo"
            print("find leveldbinfo")
        else:
            # find "inserted block x --------------------"
            print("find inserted block")
            blockNum = line[0].split(" ")[2]
    print()

f.close()
