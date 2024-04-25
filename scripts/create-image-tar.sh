# Requires 'yq'. If on mac, install with 'brew install yq'
set -e

if [ "$#" -ne 1 ]; then
    echo 'create-image-tar.sh expects 1 argument: <output filename>'
    echo 'e.g. ./image-tar.sh pca-images.tar'
    exit 1
fi

parentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
outputFilename=$1
crdPath="${parentDir}/../config/samples/astra_v1_astraconnector.yaml"
repo="theotw"

# Parse images from chart. 'yq' parses yaml, and then we sort and create space separated array
images=($(cat ${crdPath} | yq "..|.image? | select(.)" | sort -u | tr "\n" " "))

imagesWithRepo=""
# Add the repo prefix to the image names
for image in "${images[@]}"; do
  # Nats image is 3rd party and already has repo prefixed. Don't add a repo prefix for that
  if [[ "$image" == *"nats:"* ]]; then
    imagesWithRepo="${imagesWithRepo} ${image}"
  else
    imagesWithRepo="${imagesWithRepo} ${repo}/${image}"
  fi
done

# Get operator-image
operatorYamlPath="${parentDir}/../astraconnector_operator.yaml"
pattern="netapp\/astra-connector-operator:([^:\s]+)"
operatorTag=$(sed -nE "s/.*$pattern/\1/p" "$operatorYamlPath")
# Add operator image to list of images
imagesWithRepo+="${imagesWithRepo} netapp/astra-connector-operator:${operatorTag}"

# Pull images in parallel
echo "Pulling images: ${imagesWithRepo}"
echo "${imagesWithRepo}" | xargs -P3 -n1 docker pull

# Save image to tar
echo "Tar images..."
docker save -o ${outputFilename} ${imagesWithRepo}

echo "Tar created: '${outputFilename}'"
