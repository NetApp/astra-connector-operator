#!/bin/bash

user="NetApp"
repo="astra-connector-operator"

latest_release=$(curl --silent "https://api.github.com/repos/$user/$repo/releases/latest" | jq -r .tag_name)

echo "Latest release is: $latest_release"