#!/bin/sh
pkill myserverproj && echo "Sent kill"
rm -f ./myserverproj && echo "Removed old binary"
echo "Going to build and run..."
go build -o myserverproj simplewhite/main.go && ./myserverproj
