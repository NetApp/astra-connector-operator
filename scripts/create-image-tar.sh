set -ex

if [ "$#" -ne 1 ]; then
    echo 'create-image-tar.sh expects 1 argument: <output filename>'
    echo 'e.g. ./image-tar.sh pca-images.tar'
    exit 1
fi

parentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
outputFilename=$1
crdPath="${parentDir}/../config/samples/astraagent_v1.yaml"

# Parse images from chart. 'yq' parses yaml, and then we sort and create space separated array
images=($(cat ${crdPath} | yq "..|.image? | select(.)" | sort -u | tr "\n" " "))

## List of images we want to exclude. Note: "---" is coming back from the query for some reason. Remove it too
#excludedImages=( "---" "busybox" )
#
## Remove the excludedImages from the list of images to tar
#for del in ${excludedImages[@]}; do
#   images=("${images[@]/$del}")
#done
#
## Pull images in parallel
#echo "${images[*]}" | xargs -P4 -n1 docker pull
#
## Save images
#docker save -o ${outputFilename} ${images[*]}
