#!/bin/bash

interface="eno4"

git clone http://git.code.sf.net/p/linuxptp/code linuxptp
cd linuxptp/
make
make install
echo '
[global]
gmCapable               1
priority1               248
priority2               248
logAnnounceInterval     0
logSyncInterval         -3
syncReceiptTimeout      3
neighborPropDelayThresh 800
min_neighbor_prop_delay -20000000
transportSpecific       0x1
ptp_dst_mac             01:80:C2:00:00:0E
network_transport       L2
delay_mechanism         P2P
' | tee configs/gPTP.cfg
ip link set dev "$interface" up