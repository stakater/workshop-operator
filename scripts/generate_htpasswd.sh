#!/bin/bash

OUTPUT_HTPASSWD="/tmp/scripts/htpasswdfile.txt"
touch $OUTPUT_HTPASSWD
apt-get install apache2 apache2-utils -y
p=`echo "password" | htpasswd -b -B -i -n  username`
echo $p >> $OUTPUT_HTPASSWD
