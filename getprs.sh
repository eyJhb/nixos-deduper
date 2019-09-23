#!/bin/sh

PAGE=1
PERPAGE=100
URL="https://api.github.com/repos/NixOS/NixPkgs/pulls?state=open"

while true; do
    echo "Page: ${PAGE}"
    OUTPUT=$(curl --silent "${URL}&per_page=${PERPAGE}&page=${PAGE}")
    if [ $(echo $OUTPUT | wc -c) -lt 100 ]; then
        echo $OUTPUT
        echo "No more pages"
        break
    fi

    echo $OUTPUT > "outputs/res-${PAGE}.json"

    # fix this 
    TEST=1
    PAGE=$(($PAGE + $TEST))
done


# comebine stuff - does not work... uses outputs/ in go anyways
#echo combine with the following command
#echo "jq -s '.[]' outputs/res-* > prs.json"
