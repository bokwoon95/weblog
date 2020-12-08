#!/bin/sh
pkill myserverproj && echo "Sent kill"
rm -f ./myserverproj && echo "Removed old binary"
echo "Going to build and run..."
go build -o myserverproj plainsimple/main.go && ./myserverproj
