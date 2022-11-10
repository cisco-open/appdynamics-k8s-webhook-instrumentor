#! /bin/bash

trap ctrl_c INT

function ctrl_c() {
        echo "** Trapped CTRL-C"
        kill $UPSTREAM_PID
}


java -jar upstream-0.0.1-SNAPSHOT.jar --server.port=8181


