#!/bin/bash

export HOSTNAME_IP_ADDRESS=$(hostname -i)

EXT=${QDROUTERD_CONF_TYPE:-conf}
CONFIG_FILE=/tmp/skrouterd.${EXT}

if [ -f $CONFIG_FILE ]; then
    ARGS="-c $CONFIG_FILE"
fi

exec skrouterd $ARGS