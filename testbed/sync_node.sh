#!/bin/bash

interface="eno4"

ip link set dev "$interface" up
ptp4l -i "$interface" --step_threshold=1 -H -f configs/gPTP.cfg &
pmc -u -b 0 -t 1 "SET GRANDMASTER_SETTINGS_NP clockClass 248 \
        clockAccuracy 0xfe offsetScaledLogVariance 0xffff \
        currentUtcOffset 37 leap61 0 leap59 0 currentUtcOffsetValid 1 \
        ptpTimescale 1 timeTraceable 1 frequencyTraceable 0 \
        timeSource 0xa0"
phc2sys -s "$interface" -c CLOCK_REALTIME --step_threshold=1 --transportSpecific=1 -w &