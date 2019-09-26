import matplotlib as mpl
mpl.use('Agg')
import matplotlib.pyplot as plt

epoch = 100000          # storage size check epoch
accountNum = 10100001   # account number to insert

# read normal trie's storage size
fileName = "normTrieSizeLog" + "_" + str(accountNum) + "_" + str(epoch)
sizelogFile = fileName + ".txt"

# parse storage size file with lines
lines = [line.rstrip('\n') for line in open(sizelogFile)]
accNums = []
sizes = []

for line in lines:
    # parse line into accountNum & size
    strings = str(line).split("\t")
    accNum = int(strings[0])
    size = int(strings[1])

    # insert accountNum & size
    accNums.append(accNum)
    sizes.append(size)

# draw normal trie storage size graph
plt.figure(1)
plt.plot(accNums, sizes, label='normal')



# read secure trie's storage size
fileName = "secureTrieSizeLog" + "_" + str(accountNum) + "_" + str(epoch)
sizelogFile = fileName + ".txt"

# parse storage size file with lines
lines = [line.rstrip('\n') for line in open(sizelogFile)]
accNums = []
sizes = []

for line in lines:
    # parse line into accountNum & size
    strings = str(line).split("\t")
    accNum = int(strings[0])
    size = int(strings[1])

    # insert accountNum & size
    accNums.append(accNum)
    sizes.append(size)

# draw secure trie storage size graph
plt.plot(accNums, sizes, label='secure')



# graph settings
plt.ticklabel_format(axis='both',style = 'sci', scilimits=(6,6))
plt.ylabel('Storage (MB)')
plt.xlabel('Account Number (million)')
plt.legend()

# save graph as png file
graphName = "compare_norm_vs_secure_trie_" + str(accountNum) + "_" + str(epoch)
plt.savefig(graphName)
print("Done!")
