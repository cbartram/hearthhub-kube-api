FROM ubuntu:22.04

LABEL vendor=Iron\ Gate\ Studio \
      se.irongatestudio.last-change="2022-09-29"

# Install steamcmd
ENV USER=root
ENV HOME=/root

WORKDIR $HOME

# Insert Steam prompt answers
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN echo steam steam/question select "I AGREE" | debconf-set-selections \
 && echo steam steam/license note '' | debconf-set-selections

# Update the repository and install SteamCMD (steamcmd is only 32-bit)
ARG DEBIAN_FRONTEND=noninteractive
RUN dpkg --add-architecture i386 \
 && apt-get update -y \
 && apt-get install -y --no-install-recommends ca-certificates locales steamcmd \
 && rm -rf /var/lib/apt/lists/*

RUN locale-gen en_US.UTF-8
ENV LANG='en_US.UTF-8'
ENV LANGUAGE='en_US:en'

# Create symlink for executable
RUN ln -s /usr/games/steamcmd /usr/bin/steamcmd

# Update SteamCMD and verify latest version
RUN steamcmd +quit

# Fix missing directories and libraries
RUN mkdir -p $HOME/.steam \
 && ln -s $HOME/.local/share/Steam/steamcmd/linux32 $HOME/.steam/sdk32 \
 && ln -s $HOME/.local/share/Steam/steamcmd/linux64 $HOME/.steam/sdk64 \
 && ln -s $HOME/.steam/sdk32/steamclient.so $HOME/.steam/sdk32/steamservice.so \
 && ln -s $HOME/.steam/sdk64/steamclient.so $HOME/.steam/sdk64/steamservice.so

# Install valheim server from steam via steamcmd
WORKDIR /irongate

RUN mkdir /valheim && steamcmd +force_install_dir /valheim +@sSteamCmdForcePlatformType linux +login anonymous +app_update 896660 validate +quit

# Install valheim server dependencies
RUN apt-get update && apt-get install -y \
    curl \
    libatomic1 \
    libpulse-dev \
    libpulse0 \
    && rm -rf /var/lib/apt/lists/*


WORKDIR /valheim
RUN cp ./steamclient.so /usr/lib

# Set default values for arguments & server required env vars
ENV SERVER_NAME="My Server" \
    SERVER_PORT=2456 \
    WORLD_NAME="Dedicated" \
    SERVER_PASS="secret" \
    CROSSPLAY="true" \
    PATH="/irongate:${PATH}" \
    LD_LIBRARY_PATH=/valheim/linux64 \
    SteamAppId=892970

# Make the server executable
RUN chmod +x valheim_server.x86_64

#CMD /bin/bash
CMD ["./valheim_server.x86_64", "-name", "${SERVER_NAME}", "-port", "${SERVER_PORT}", "-world", "${WORLD_NAME}", "-password", "${SERVER_PASS}", "-crossplay"]