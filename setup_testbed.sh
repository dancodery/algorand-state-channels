#!/bin/bash

# print out executed commands
# set -x

# stop program in case of error
set -e


# 1. Read node arguments into an array
args=("$@")

# 2. Free hosts
for ((i=0; i<${#args[@]}; i++)); do
    echo "Freeing node ${args[i]}..."
    pos allocations free -k ${args[i]}
done

# 3. Allocate nodes
echo "Allocating nodes ${args[@]}..."
allocate_output=$(pos allocations allocate "${args[@]}")

# 4. Save variables
allocation_id=$(echo "$allocate_output" | awk -F ': ' '/Allocation ID:/ {gsub(/[()]/,"",$2); print $2}')
result_directory=$(echo "$allocate_output" | awk '/Results in/ {print $NF}')
echo "Allocation ID: $allocation_id"
echo "Result Directory: $result_directory"
echo

# 5. Load images for nodes individually
for ((i=0; i<${#args[@]}; i++)); do
    # load image
    echo "Loading image for node ${args[i]}..."
    pos nodes image ${args[i]} debian-bullseye

    # reset node and reboot
    echo "Reset node ${args[i]}..."
    pos nodes reset --non-blocking ${args[i]} 

    echo
done

# 6. Wait for nodes to be ready, so that they can reboot in parallel
echo "Waiting for nodes to be ready..."
sleep 120 # 120 works, 117 not
echo

for ((i=0; i<${#args[@]}; i++)); do
    # 7. Copy files to nodes
    echo "Copying files to node ${args[i]}..."
    pos nodes copy --recursive --dest /root ${args[i]} /home/gockel/algorand-state-channels

    echo
done

# 9. Setup algorand sandbox
sandbox_node=${args[0]}

# Install Docker
echo "Installing Docker on node ${sandbox_node}..."
pos commands launch --infile testbed/install_docker.sh --queued --name docker-setup ${sandbox_node}

echo "Extending file system on node ${sandbox_node}..."
pos commands launch ${sandbox_node} -- mkdir -p /mnt/sda/docker
pos commands launch ${sandbox_node} -- mkfs.ext4 -F /dev/nvme0n1
pos commands launch ${sandbox_node} -- mount /dev/nvme0n1 /mnt/sda
pos commands launch ${sandbox_node} -- mount --rbind /mnt/sda/docker /var/lib/docker

pos commands launch --infile testbed/run_sandbox.sh --queued --name run-sandbox ${sandbox_node}

echo "Sandbox is running on node ${sandbox_node}..."

# 10. Setup for alice and bob
# alice_node=${args[1]}
# bob_node=${args[2]}
# pos commands launch --infile testbed/docker_setup.sh --queued --name docker-setup ${alice_node}
# pos commands launch --infile testbed/docker_setup.sh --queued --name docker-setup ${bob_node}


echo 
