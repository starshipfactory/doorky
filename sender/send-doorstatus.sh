#!/bin/sh
#
# Read door status and, if changed, send an HTTP request to a web service
# informing it thusly.

. "$HOME/.doorky/config"

if [ x"$DOORSTATUS_SERVER" = x ]
then
	echo "Please set DOORSTATUS_SERVER in ~/.doorky/config" 1>&2
	exit 1
fi

if [ x"$NTP_SERVER" = x ]
then
	echo "Please set NTP_SERVER in ~/.doorky/config" 1>&2
	exit 1
fi

if [ x"$DOORNAME" = x ]
then
	DOORNAME="door"
fi

if ! test -f "$HOME/.doorky/secret"
then
	echo "Missing ~/.doorky/secret" 1>&2
	exit 1
fi

oldstate=x
ntpdate -s "$NTP_SERVER"

while true
do
	newstate=$(gpioctl 2)

	if [ x"$oldstate" = x"$newstate" ]
	then
		:
	else
		# Hash the new value and the current time stamp
		# and enc
		ts=$(date +%s)
		iv=$(openssl rand -hex 16)
		hash="$(echo -n "$DOORNAME\n$newstate\n$ts" | openssl sha1 -sha256 -binary | openssl aes-256-cbc -iv "$iv" -kfile "$HOME/.doorky/secret" -md sha256 -e -base64 -nosalt -nopad | tr "+/" "-_" | tr -d "\n")"
		ftp -i -o /dev/null "http://$DOORSTATUS_SERVER/api/doorstatus?door=$DOORNAME&val=$newstate&ts=$ts&hash=$hash&iv=$iv" > /dev/null 2>&1
		if [ x"$?" = 0 ]
		then
			oldstate="$newstate"
		fi
	fi

	ntpdate -s "$NTP_SERVER"
	sleep 30
done
