#!/bin/bash

mkfs.ext4 -F /dev/nvme0n1
mkdir -p /var/lib/docker
mount /dev/nvme0n1 /mnt/sda
mkdir -p /mnt/sda/docker
mount --rbind /mnt/sda/docker /var/lib/docker