#!/bin/bash
cd $(dirname $(readlink -f $0))
chmod +x pingmon-*-*
sudo setcap cap_net_raw=+ep pingmon-*-*
