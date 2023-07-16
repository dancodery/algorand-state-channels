#!/bin/bash

interface="eno5"


phc2sys -s "$interface" -c CLOCK_REALTIME --step_threshold=1 --transportSpecific=1 -w &