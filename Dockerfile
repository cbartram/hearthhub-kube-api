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
    unzip \
    && rm -rf /var/lib/apt/lists/*


WORKDIR /valheim
RUN cp ./steamclient.so /usr/lib && touch /valheim/output.log
COPY ./scripts/health_check.sh .

# Install BepInEx
RUN curl -X GET -LO https://gcdn.thunderstore.io/live/repository/packages/denikson-BepInExPack_Valheim-5.4.2202.zip && \
    unzip ./denikson-BepInExPack_Valheim-5.4.2202.zip -d /tmp && \
    rm ./denikson-BepInExPack_Valheim-5.4.2202.zip && \
    cp -r /tmp/BepInExPack_Valheim/. .

# Set default values for arguments & server required env vars although these will be overriden by Kubernetes spec.container.args
ENV SERVER_NAME="My Server" \
    SERVER_PORT=2456 \
    WORLD_NAME="Dedicated" \
    SERVER_PASS="secret" \
    CROSSPLAY="true" \
    DOORSTOP_ENABLE="TRUE" \
    DOORSTOP_INVOKE_DLL_PATH=/valheim/BepInEx/core/BepInEx.Preloader.dll \
    DOORSTOP_CORLIB_OVERRIDE_PATH=/valheim/unstripped_corlib \
    PATH="/irongate:${PATH}" \
    LD_LIBRARY_PATH=/valheim/linux64:/valheim/doorstop_libs \
    LD_PRELOAD=/valheim/doorstop_libs/libdoorstop_x64.so \
    SteamAppId=892970

RUN chmod +x valheim_server.x86_64

# This command doesn't actually do anything since it's overwritten by the kubernetes deployment args
CMD ["./valheim_server.x86_64", "-name", "${SERVER_NAME}", "-port", "${SERVER_PORT}", "-world", "${WORLD_NAME}", "-password", "${SERVER_PASS}", "-crossplay"]