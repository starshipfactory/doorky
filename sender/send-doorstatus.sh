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
		hash="$(echo -n "$newstate\n$ts" | openssl sha1 -sha256 -binary | openssl aes-256-cbc -kfile "$HOME/.doorky/secret" -md sha256 -e -base64 | tr -d "\n")"
		ftp -i -o /dev/null "http://$DOORSTATUS_SERVER/api/doorstatus?val=$newstate&ts=$ts&hash=$hash" > /dev/null 2>&1
		if [ x"$?" == 0 ]
		then
			oldstate="$newstate"
		fi
	fi

	ntpdate -s "$NTP_SERVER"
	sleep 30
done
