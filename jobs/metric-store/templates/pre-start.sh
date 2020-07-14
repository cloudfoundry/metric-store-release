#!/bin/bash

mkdir -p /var/vcap/store/metric-store/scrape_configs
chown vcap:vcap /var/vcap/store/metric-store/scrape_configs

rm -rf /var/vcap/store/metric-store/activequeries