#!/usr/bin/bash

while [[ "$1" ]]; do
  case "$1"
    -h | --help | help)
      sh-gen help $(readlink -f "$0") <<EOF
      Example script acts as a template to demonstrate how shell generation can
      enable quick implementation of bash completion and help functionality.
      EOF
      shift
      continue
      ;;
    #@completion command push-config Push config to the config endpoint
    #@completion command-arg push-config --config Specify the config string for push-config
    push-config)
      shift
      local PUSH_CONFIG_DATA='{"some":"config"}'
      if [[ "$1" == "--config" && "$2" ]]; then
        PUSH_CONFIG_DATA="$2"
        shift; shift
      elif [[ "$1" == "--config" && ! "$2" ]]; then
        echo "No argument provided for 'push-data --config'"
        exit 1
      fi
      curl -X PUT -d $PUSH_CONFIG_DATA https://localhost:8080/config
      continue
      ;;
  esac
done