#!/bin/bash

set -exo pipefail

tree=$(pwd)
tmpdir=$(mktemp --directory)

main() {
  case $1 in
    image)
      image
      ;;
    app)
      app
      ;;
    *)
      echo "unknown command"
      exit 1
      ;;
  esac

  rm --recursive --force "${tmpdir}"
}

image() {
  cp Dockerfile Gemfile* "${tmpdir}"
  docker build --tag flynn/dashboard-builder "${tmpdir}"
}

app() {
  cp --recursive . "${tmpdir}"
  cd "${tmpdir}"

  docker run \
    --volume "${tmpdir}:/build" \
    --workdir /build \
    --user $(id -u) \
    flynn/dashboard-builder \
    bash -c "cp --recursive /app/.bundle . && cp --recursive /app/vendor/bundle vendor/ && bundle exec rake compile"

  tar --create --directory build . > "${tree}/dashboard.tar"
}

main "$@"
