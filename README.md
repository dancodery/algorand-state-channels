# algorand-state-channels
Development of State Channels for Algorand


1. python3.11 -m venv venv_algorand_state_channels
2. source venv_algorand_state_channels/bin/activate
3. pip3 install -r requirements.txt


## Run the example usage file
4. python3 example_usage.py


## Rebuild specific container
1. docker-compose stop asc-alice
2. docker-compose rm -f asc-alice
3. docker-compose up -d --build asc-alice \
...
4. docker-compose logs asc-alice

## Open terminal for Alice
docker exec -it asc-alice bash

## Open terminal for Bob
docker exec -it asc-bob bash

## Commands Cheat Sheet

### Docker-Compose Action Commands:
1. **<span style="color: yellow;">docker-compose up [container_name] [--remove-orphans] [-d] [--build]
</span>** ----> Creates and starts all containers defined in docker-compose.yml, [--remove-orphans] removes ophaned containers, [-d] runs in detached mode, [--build] builds images before starting containers
1. **<span style="color: yellow;">docker-compose down</span>** ----> Stops and removes all containers, images and networks defined in docker-compose.yml 
1. **<span style="color: yellow;">docker-compose start</span>** ----> Starts any stopped container defined in docker-compose.yml
1. **<span style="color: yellow;">docker-compose stop</span>** ----> Stops any running container defined in docker-compose.yml
1. **<span style="color: yellow;">docker-compose pause</span>** ----> Pauses any running container defined in docker-compose.yml
1. **<span style="color: yellow;">docker-compose unpause</span>** ----> Unpauses any paused container defined in docker-compose.yml

### Docker-Compose Status Commands:
1. **<span style="color: yellow;">docker-compose ps</span>** ----> Lists all containers that are running     
1. **<span style="color: yellow;">docker-compose logs [-f] [container_name]</span>** ----> Shows the logs of all containers, [-f] follows log output, [container_name] shows logs of specific container


## Acknowledgements
This implementation has taken the following resources for assistance: https://github.com/lightningnetwork/lnd by Olaoluwa Osuntokun at Lightning Labs and https://github.com/lnbook/lnbook by Andreas M. Antonopoulos, Olaoluwa Osuntokun and Rene Pickhardt.


>Intended use for educational purposes only.