#!/bin/bash

OUTPUT_HTPASSWD="generate_htpasswd/htpasswdfile.txt"
touch $OUTPUT_HTPASSWD
sudo apt-get install apache2 apache2-utils -y
p=`echo "password" | htpasswd -b -B -i -n  username`
echo $p
echo $p >> $OUTPUT_HTPASSWD
cat $OUTPUT_HTPASSWD

