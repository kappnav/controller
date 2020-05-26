#!/bin/bash

# Copyright 2019, 2020 IBM Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -Eeo pipefail

# Travis builds won't have a peer build dir
. ../build/version.sh

IMAGE=kappnav-controller

echo "Building ${IMAGE} ${VERSION}"
COMMIT=$(git rev-parse HEAD)
docker build --pull --build-arg VERSION=$VERSION --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') --build-arg COMMIT=$COMMIT -t ${IMAGE} .

# If this build is being done on an update to the master branch then tag the image as "dev" and push to docker hub kappnav org
if [ "$TRAVIS" == "true" ] && [ "$TRAVIS_PULL_REQUEST" == "false" ] && [ "$TRAVIS_BRANCH"  == "master" ]; then
    echo $DOCKER_PWD | docker login docker.io -u $DOCKER_USER --password-stdin
    targetImage=docker.io/kappnav/controller:dev
    echo "Pushing Docker image $targetImage"
    docker tag ${IMAGE} ${targetImage}
    docker push ${targetImage}
fi
