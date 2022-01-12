#!/bin/bash
sudo apt-get install apache2 apache2-utils -y
sudo rm -rf htpasswdfile
sudo htpasswd -c  -b htpasswdfile user1 openshift