#!/usr/bin/env bash
#
# Execute Docker Scanning, both CLI and Inspector
#
# use coding style from: https://google.github.io/styleguide/shell.xml

set -fb

readonly THISDIR=$(cd "$(dirname "$0")" ; pwd)
readonly BASEDIR=$(cd "$(dirname "$0")/.." ; pwd)
readonly MY_NAME=$(basename "$0")
readonly INSPECTOR_SHELL_SCRIPT="${THISDIR}/hub-docker-inspector.sh"
readonly DOCKER_SCAN_FETCH_URL="https://blackducksoftware.github.io/hub-docker-inspector/scan.docker.sh"
readonly DOCKER_SCAN_SHELL_SCRIPT="${THISDIR}/scan.docker.sh"
readonly EXECUTABLE_SHELL_SCRIPT="${THISDIR}/.scan.docker.sh"
IMAGE_TARFILE="${THISDIR}/scanned.tar"

function usage() {
   >&2 echo "Error: Missing or invalid arguments."
   echo ""
   echo "Example usage: ${0} --image jetty:9.3.11-jre8-alpine --username sysadmin --host my_hub_host.domain.com --port 8443 --scheme https"
   echo "   port defaults to 443, and scheme defaults to https"
   echo ""
   echo "Usage: $0 --image IMAGENAME[:TAG]  [--no-scan] [--no-inspect] [--inspector-version VERSION] scan.cli.sh arguments from 'Options' listed below:"
   "${THISDIR}"/scan.cli.sh --help
   echo ""
   exit 1
}

function using_proxy_instructions() {
   echo "------------------------------------------------------------------------------------"
   echo "You may need set the default proxies for 'wget' or 'curl' to use for http and https."
   echo "To do this, create a ~/.wgetrc file for 'wget' or ~/.curlrc file for 'curl'."
   echo "This will override the values in the environment."
   echo "Add these settings (using your proxy's URL) to the .wgetrc or .curlrc file:"
   echo ""
   echo "https_proxy=http://proxy.yourdomain.com:18023/"
   echo "http_proxy=http://proxy.yourdomain.com:18023/"
   echo ""
   echo "If your proxy requires authentication, add these additional two lines:"
   echo "proxy_user=username"
   echo "proxy_password=password"
   echo "-----------------------------------------------------------------------------------"
   echo ""
}

function execute_remote_request() {
  local command=$2
  local dest_url=$3
  local output_filename=$4
  local my_output my_success
  if [ -z "$output_filename" ]; then
    my_output=$($command 2>&1)
  else 
    my_output=$($command "${TEMP_FILE}" "$dest_url" 2>&1)
  fi
  my_success=$?
  eval "$1='${my_output}'"
  return $my_success
}

function get_remote_file() {
  readonly REQUEST_URL=$1
  readonly TEMP_FILE="${THISDIR}/tmp.file"
  local command_output
  if [ -n "$(which wget)" ]; then
    execute_remote_request command_output "wget -O"  "$REQUEST_URL" "$2"
    if [[ $? -eq 0 ]]; then
      tr -d "\r" <"${TEMP_FILE}" >"$2"
      rm "${TEMP_FILE}"
      chmod 755 "$2"
      return 0
    else
      if [[  "${command_output}" == "${command_output#*Resolving blackducksoftware.github.io}" ]]; then
        using_proxy_instructions
      fi
    fi
    return 1
  else
    if [ -n "$(which curl)" ]; then
      execute_remote_request command_output "curl -s --netrc-optional -I $REQUEST_URL" ""
      status_code=$(echo $command_output | cut -d $'\r' -f1 | cut -d $' ' -f2)
      server_name=$(echo $command_output | cut -d $'\r' -f2 | cut -d $' ' -f3)
      if [ "$status_code" == "200" ]; then
        execute_remote_request command_output "curl --netrc-optional -s -o" "$REQUEST_URL" "$2"
        if [[ $? -eq 0 ]]; then
          tr -d "\r" <"${TEMP_FILE}" >"$2"
          rm "${TEMP_FILE}"
          chmod 755 "$2"
          return $?
        fi
      else 
        if [[ "$server_name" != "GitHub.com" ]]; then
          using_proxy_instructions
        fi
      fi
    fi
    return 2
  fi
}

function clean_up() {
   rm -f "${IMAGE_TARFILE}"
}
function self_clean_up() {
   rm -f "${EXECUTABLE_SHELL_SCRIPT}"
}

function update_self_and_invoke() {
  get_remote_file "${DOCKER_SCAN_FETCH_URL}" "${EXECUTABLE_SHELL_SCRIPT}"
  if [ $? -ne 0 ]; then
    echo "Using installed scan.docker.sh"
    cp "${DOCKER_SCAN_SHELL_SCRIPT}" "${EXECUTABLE_SHELL_SCRIPT}"
  fi
  exec "${EXECUTABLE_SHELL_SCRIPT}" "$@"
}

function pull_image_if_necessary() { 
  local image=$1
  local image_version=$2
  local existing_image=$(docker images -q "$image")
  if [ -z "$existing_image" ] || [ "$image_version" == "latest" ]; then
    echo "Existing image not present, pulling from docker repository"
    docker pull "$image"
    existing_image=$(docker images -q "$image")
    if [ -z "${existing_image}" ]; then
      echo "Unable to pull image: $image, exiting"
      exit 5
    fi
  fi
}

# Validate directory is writable and that docker exists and is valid for this User
# additionally, validate 3 parameters: image_name, username, and host
# if dryrun then adjust validations accordingly
#
function perform_validations() {
  if [ ! -w "${THISDIR}" ]; then
    echo "Directory ${THISDIR} is not writable!"
    echo "That directory must be writable by this user in order to execute this script"
    echo "Please fix the directory permissions and try again"
    exit 10
  fi
  if [ -z "${docker_image}" ]; then
    echo "Image name is required, but is not present!"
    usage
  fi
  # If this is not a dry run, then ensure that user and host are provided
  if [ -z "${dry_run}" ]; then
    if [ -z "${hub_user}" ]; then
      echo "Username is required, but is not present"
      usage
    fi
    if [ -z "${hub_host}" ]; then
      echo "Specifying the Black Duck host is required, but is not present"
      usage
    fi
  fi
  if [ -z "$(which docker)" ]; then
    echo "Docker is required to be in your PATH and it cannot be found."
    exit 2
  fi
  # there IS a docker, now validate that this user can execute it
  valid_docker=$(docker info 2>&1)
  valid_docker=${valid_docker%%*permission denied*}
  if [ -z "${valid_docker}" ]; then
    echo "You do not have permission to run Docker. "
    echo "You may need to be a user in the docker group, a root user, or have sudo access."
    exit 3
  fi
}


dry_run=""
hub_host=""
docker_image=""
hub_user=""
function main() {
  cp "${EXECUTABLE_SHELL_SCRIPT}" "${DOCKER_SCAN_SHELL_SCRIPT}"
  local hub_port="443"
  local hub_scheme="https"
  local hub_project=""
  local hub_version=""
  local name_arg=""
  local do_scan=1
  local do_inspect=1
  local pull_from_remote=1
  local inspector_version=
  local trust_cert=0
  while [[ $# -ge 1 ]]; do
    opt="$1"
    case $opt in
      --image)
        docker_image="$2"
        shift
        ;;
      --host)
        hub_host="$2"
        shift
        ;;
      --username)
        hub_user="$2"
        shift
	    ;;
      --port)
        hub_port="$2"
        shift
        ;;
      --scheme)
        hub_scheme="$2"
        shift
        ;;
      --inspector-version)
        inspector_version="$2"
        shift
        ;;
      --name)
        name_arg="$2"
        shift
        ;;
      --dryRunWriteDir)
        dry_run="--dryRunWriteDir $2"
        shift
        ;;
      --project)
        hub_project="$2"
        shift
        ;;
      --release)
        hub_version="$2"
        shift
        ;;
      --use-local)
        pull_from_remote=0
        ;;
      --no-scan)
        do_scan=0
        ;;
      --no-inspect)
        do_inspect=0
        ;;
      --insecure)
      	trust_cert=1
      	;;
      *)
	    optional_args="${optional_args} $1"
        ;;
    esac
    shift
  done

  perform_validations

  readonly HUB_URL="${hub_scheme}://${hub_host}:${hub_port}/"
  if [ -z "$hub_project" ]; then
    hub_project="${docker_image%:*}"
  fi
  if [ -z "$hub_version" ]; then
    hub_version="${docker_image##*:}"
    if [ "$hub_version" == "$docker_image" ]; then 
      hub_version="latest"
    fi
  fi
  raw_image_name="${docker_image%:*}"
  clean_image_name=$(echo "$raw_image_name" | sed  -e "s/\//_/g")  # convert slash to underscore
  clean_image_name=$(echo "$clean_image_name" | sed -e "s/\:/_/g") # convert colon to underscore
  IMAGE_TARFILE="${THISDIR}/${clean_image_name%:*}-${hub_version}.tar"

  # if the --name scan.cli option was not provided
  if [ -z "${name_arg}" ]; then
    name_arg="scanner_${hub_project}_${hub_version}"
  else
    name_arg="${name_arg}"
  fi

  # update_docker_inspector
  inspector_script_name="hub-docker-inspector"
  if [ -n "$inspector_version" ]; then
    inspector_script_name="hub-docker-inspector-${inspector_version}"
  fi
  readonly INSPECTOR_FETCH_URL="https://blackducksoftware.github.io/hub-docker-inspector/${inspector_script_name}.sh"
  get_remote_file "${INSPECTOR_FETCH_URL}" "${INSPECTOR_SHELL_SCRIPT}"
  if [ $? -ne 0 ]; then
    echo "Using default hub-docker-inspector.sh"
  fi

  if [ $pull_from_remote -eq 1 ]; then
    pull_image_if_necessary $docker_image $hub_version
  fi

  local cli_status=0
  docker save -o "${IMAGE_TARFILE}" "$docker_image" 2>/dev/null
  if [ $? -eq 0 ]; then
    if [ $do_scan -eq 1 ]; then
      echo "Perform scan:"
      local hub_parameters=""
      if [ -n "${hub_host}" ]; then
        hub_parameters="--host ${hub_host} --port $hub_port --scheme $hub_scheme"
      fi
      local scan_insecure_parameters=""
      if [ $trust_cert -eq 1 ]; then
      	echo "WARNING: Ignoring certificate errors"
      	scan_insecure_parameters="--insecure"
      fi
      "${THISDIR}"/scan.cli.sh $hub_parameters --username "$hub_user" $dry_run --project "$hub_project" --release "$hub_version" --name "$name_arg" $scan_insecure_parameters $optional_args "${IMAGE_TARFILE}"
      cmd_status=$?
      if [ $cmd_status -ne 0 ]; then
        exit $cmd_status
      fi
    fi
    if [ $do_inspect -eq 1 ]; then
      echo "Conduct  Inspection:"
      local inspector_insecure_parameters=""
      if [ $trust_cert -eq 1 ]; then
      	echo "WARNING: Ignoring certificate errors"
      	inspector_insecure_parameters="--hub.always.trust.cert=true"
      fi
	  if [ -z "${dry_run}" ]; then
        "${INSPECTOR_SHELL_SCRIPT}" --hub.username="$hub_user" --hub.url=$HUB_URL --hub.project.name="$hub_project" --hub.project.version="$hub_version" $inspector_insecure_parameters "${IMAGE_TARFILE}"
      else
        "${INSPECTOR_SHELL_SCRIPT}" --hub.project.name="$hub_project" --hub.project.version="$hub_version" $inspector_insecure_parameters "${IMAGE_TARFILE}"
      fi
      cmd_status=$?
      if [ $cmd_status -ne 0 ]; then
        exit $cmd_status
      fi   
    fi
  else
    echo "Unable to save: ${IMAGE_TARFILE} to a tar file."
    echo "Please verify the image name:tag and try again."
  fi
}

# the 'real' version will be executed as .scan.docker.sh
if [[ $MY_NAME = \.* ]]; then
   # invoke real main program
   trap "clean_up; self_clean_up" EXIT
   main "$@"
else
   # update myself and invoke updated version
   trap clean_up EXIT
   update_self_and_invoke "$@"
fi

