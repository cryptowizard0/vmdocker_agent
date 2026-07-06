#!/bin/sh
# Default claude start.sh (platform template).
#
# The claude runtime has no long-running engine to start: the claude CLI is
# provided by the base image and /vmm/health becomes ready once it is on PATH.
# Add module-specific initialization below and return.
#
# Copy and edit this file in your module to customize startup.
set -eu

# Add module-specific initialization / workspace seeding below this line.
exit 0
