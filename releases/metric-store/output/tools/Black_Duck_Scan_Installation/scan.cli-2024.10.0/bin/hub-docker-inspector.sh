#!/bin/bash
#
# This script runs on the host machine, and uses hub-docker-inspector images/containers
# to inspect the given Docker image.
#
# Run this script from the directory that contains the application.properties, configured
# with your Black Duck connection details (hub.url, hub.username, and hub.password),
# and Black Duck Docker connection details (docker.registry.username and docker.registry.password).

# To override the default location of /tmp/hub-docker-inspector, specify
# your own DOCKER_INSPECTOR_JAR_DIR in your environment and
# *that* location will be used.
DOCKER_INSPECTOR_JAR_DIR=${DOCKER_INSPECTOR_JAR_DIR:-/tmp/hub-docker-inspector}

# If you want to pass any additional options to
# curl, specify DOCKER_INSPECTOR_CURL_OPTS in your environment.
# For example, to specify a proxy, you would set
# DOCKER_INSPECTOR_CURL_OPTS=--proxy http://myproxy:3128
DOCKER_INSPECTOR_CURL_OPTS=${DOCKER_INSPECTOR_CURL_OPTS:-}

# DOCKER_INSPECTOR_VERSION should be set in your
# environment if you wish to use a version different
# from LATEST.
jarVersion=${DOCKER_INSPECTOR_VERSION}

#Getting the proxy settings from the environment
PROXY_HOST=${BLACKDUCK_HUB_PROXY_HOST}
PROXY_PORT=${BLACKDUCK_HUB_PROXY_PORT}
PROXY_USERNAME=${BLACKDUCK_HUB_PROXY_USERNAME}
PROXY_PASSWORD=${BLACKDUCK_HUB_PROXY_PASSWORD}

#Getting the proxy settings from the command line switches
for i in "$@"
do
case $i in
    --blackduck.hub.proxy.host=*)
    PROXY_HOST="${i#*=}"
    shift # past argument=value
    ;;
    --blackduck.hub.proxy.port=*)
    PROXY_PORT="${i#*=}"
    shift # past argument=value
    ;;
    --blackduck.hub.proxy.username=*)
    PROXY_USERNAME="${i#*=}"
    shift # past argument=value
    ;;
     --blackduck.hub.proxy.password=*)
    PROXY_PASSWORD="${i#*=}"
    shift # past argument=value
    ;;
    *)
          # ignored option
    ;;
esac
done

#Putting together the Curl proxy options
CURL_PROXY_OPTIONS=""
if [ ! -z "${PROXY_HOST}" ]; then
	CURL_PROXY_OPTIONS="--proxy ${PROXY_HOST}:${PROXY_PORT}"

	if [ ! -z "${PROXY_USERNAME}" ]; then
		CURL_PROXY_OPTIONS="${CURL_PROXY_OPTIONS} --proxy-anyauth --proxy-user ${PROXY_USERNAME}:${PROXY_PASSWORD}"
	fi
fi

##################
# Initialize
##################
latestCommitIdFileUrl="https://blackducksoftware.github.io/hub-docker-inspector/latest-commit-id.txt"
localCommitIdFile="${DOCKER_INSPECTOR_JAR_DIR}/hub-docker-inspector-latest-commit-id.txt"
currentVersionCommitId=""
version="4.4.1"
encodingSetting="-Dfile.encoding=UTF-8"
userSpecifiedJarPath=""
jarPathSpecifiedOnCmdLine=false
latestReleaseVersion=
hubUsernameArgument=""
hubProjectNameArgument=""
hubProjectVersionArgument=""

function printUsage() {
	echo ""
    echo "Usage: $0 [options]"
    echo "options: any property from application.properties can be set by adding an option of the form:"
    echo "  --<property name>=<value>"
    echo ""
    echo "Run this command from the directory that contains the application.properties,"
    echo "configured with your Black Duck connection details (hub.url, hub.username, and hub.password),"
	echo "and Black Duck Docker connection details (docker.registry.username and docker.registry.password)."
	echo ""
	echo "For greater security, the Black Duck password can be set via the environment variable BD_HUB_PASSWORD"
	echo ""
	echo "For example:"
	echo "  export BD_HUB_PASSWORD=mypassword"
	echo "  $0 --hub.url=http://hub.mydomain.com:8080/ --hub.username=myusername ubuntu"
	echo ""
	echo "Documentation: https://sig-product-docs.synopsys.com/category/bd-subtiles"
}

# Write message to stdout
log() {
  echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')]: $@"
}

# Write warning to stdout
warn() {
  echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')]: WARNING: $@"
}

# Write error message to stderr
err() {
  echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')]: ERROR: $@" >&2
}

function deriveCurrentVersionCommitId() {
	log "Checking ${localCommitIdFile} for commit ID"
	if [ -f "${localCommitIdFile}" ]; then
		log "Existing version commit ID file: ${localCommitIdFile}"
		currentVersionCommitId=$( <"${localCommitIdFile}" )
		log "Current version commit ID: ${currentVersionCommitId}"
	fi
	log "The currently-installed version of the hub-docker-inspector jar file: ${currentVersionCommitId}"
}

function deriveLatestVersionCommitId() {
	log "Downloading ${latestCommitIdFileUrl} to ${localCommitIdFile}"
	curl ${DOCKER_INSPECTOR_CURL_OPTS} ${CURL_PROXY_OPTIONS} -o "${localCommitIdFile}" "${latestCommitIdFileUrl}"
	latestVersionCommitId=$( <"${localCommitIdFile}" )
	log "The latest version of the hub-docker-inspector jar file: ${latestVersionCommitId}"
}

function deriveJarDetails() {
	# If the user specified a version: get that
	if [ -z "${jarVersion}" ]; then
	  deriveLatestReleaseVersion
	  latestReleasedJarUrl="https://test-repo.blackducksoftware.com/artifactory/bds-integrations-release/com/blackducksoftware/integration/hub-docker-inspector/${latestReleaseVersion}/hub-docker-inspector-${latestReleaseVersion}.jar"
	
      selectedJarUrl="${latestReleasedJarUrl}"
      deriveLatestReleasedFilename
      selectedJarFilename="${latestReleasedFilename}"
      downloadedJarPath="${DOCKER_INSPECTOR_JAR_DIR}/${selectedJarFilename}"
    else
      log "Will download hub-docker-inspector-${jarVersion}.jar"
      rm -f "${localCommitIdFile}" # Local commit ID won't apply to this jar
      if [[ $jarVersion == *"SNAPSHOT"* ]]; then
      	selectedRepoKey="bds-integrations-snapshot"
      else
      	selectedRepoKey="bds-integrations-release"
      fi
      selectedJarUrl="https://test-repo.blackducksoftware.com/artifactory/${selectedRepoKey}/com/blackducksoftware/integration/hub-docker-inspector/${jarVersion}/hub-docker-inspector-${jarVersion}.jar"
      downloadedJarPath="${DOCKER_INSPECTOR_JAR_DIR}/hub-docker-inspector-${jarVersion}.jar"
    fi
    log "Selected jar: ${selectedJarUrl}"
    log "  local path: ${downloadedJarPath}"
}

function determineIsJarDownloadRequired() {
	jarDownloadRequired=true
	if [ ! -f "${downloadedJarPath}" ]; then
		log "You don't have a hub-docker-inspector jar file at ${downloadedJarPath}, so it will be downloaded."
	elif [[ ! -z ${jarVersion} ]] && [[ ${jarVersion} != *"SNAPSHOT"* ]]; then
	    log "You specified jar version ${jarVersion}, it's not a snapshot, and it exists at ${downloadedJarPath}; no need to download it."
		jarDownloadRequired=false
	elif [ "${currentVersionCommitId}" != "${latestVersionCommitId}" ] ; then
		log "${downloadedJarPath} needs to be downloaded. (Snapshot versions are downloaded every time; Released versions are not.)"
	else
		log "${downloadedJarPath} is up-to-date."
		jarDownloadRequired=false
	fi
}

function downloadJarIfRequired() {
	if [ ${jarDownloadRequired} == true ]; then
		curl ${DOCKER_INSPECTOR_CURL_OPTS} ${CURL_PROXY_OPTIONS} --fail -L -o "${downloadedJarPath}" "${selectedJarUrl}"
		if [[ $? -ne 0 ]]
		then
			err "Download of ${selectedJarUrl} failed."
			exit -1
		fi
		log "Saved ${selectedJarUrl} to ${downloadedJarPath}"
	fi
}

function prepareLatestJar() {
	deriveCurrentVersionCommitId
	deriveLatestVersionCommitId
	deriveJarDetails
	determineIsJarDownloadRequired
	downloadJarIfRequired
}

#
function deriveLatestReleaseVersion() {
	if [[ -z "${latestReleaseVersion}" ]]; then
		latestReleaseVersion=$(curl ${DOCKER_INSPECTOR_CURL_OPTS} ${CURL_PROXY_OPTIONS} https://test-repo.blackducksoftware.com/artifactory/bds-integrations-release/com/blackducksoftware/integration/hub-docker-inspector/maven-metadata.xml | grep latest | sed -e 's@<latest>@@' -e 's@</latest>@@' -e 's/^[ \t]*//')
	fi
	echo "Latest release version: ${latestReleaseVersion}"
}

#
function deriveLatestReleasedFilename() {
	log "Deriving name of latest released jar file"
	deriveLatestReleaseVersion
    latestReleasedFilename=hub-docker-inspector-${latestReleaseVersion}.jar
    log "Latest released jar filename: ${latestReleasedFilename}"
}

# Expand tilde
function expandPath() {
	echo "${@/#~/$HOME}"
}

# escape spaces
function escapeSpaces() {
	echo "${@// /%20}"
}

# Look through args for ones this script needs to act on
function preProcessOptions() {
	cmdlineargindex=0
	for cmdlinearg in "$@"
	do
		if [[ "${cmdlinearg}" == --jar.path=* ]]
		then
			userSpecifiedJarPath=$(echo "${cmdlinearg}" | cut -d '=' -f 2)
			userSpecifiedJarPath=$(expandPath "${userSpecifiedJarPath}")
			jarPathSpecifiedOnCmdLine=true
		elif [[ "$cmdlinearg" == --hub.username=* ]]
                then
                        hubUsername=$(echo "$cmdlinearg" | cut -d '=' -f 2)
                        hubUsernameEscaped=$(escapeSpaces "${hubUsername}")
                        hubUsernameArgument="--hub.username=${hubUsernameEscaped}"
                elif [[ "$cmdlinearg" == --hub.project.name=* ]]
                then
                        hubProjectName=$(echo "$cmdlinearg" | cut -d '=' -f 2)
                        hubProjectNameEscaped=$(escapeSpaces "${hubProjectName}")
                        hubProjectNameArgument="--hub.project.name=${hubProjectNameEscaped}"
                elif [[ "$cmdlinearg" == --hub.project.version=* ]]
                then
                        hubProjectVersion=$(echo "$cmdlinearg" | cut -d '=' -f 2)
                        hubProjectVersionEscaped=$(escapeSpaces "${hubProjectVersion}")
                        hubProjectVersionArgument="--hub.project.version=${hubProjectVersionEscaped}"
		elif [[ "${cmdlinearg}" == --spring.config.location=* ]]
		then
			# Once IDETECT-339 is done/released, this clause can go away
			springConfigLocation=$(echo "${cmdlinearg}" | cut -d '=' -f 2)
			if ! [[ "${springConfigLocation}" == file:* ]]
			then
				springConfigLocation="file:${springConfigLocation}"
			fi
			if ! [[ "${springConfigLocation}" == */application.properties ]]
			then
				if [[ "${springConfigLocation}" == */ ]]
				then
					springConfigLocation="${springConfigLocation}application.properties"
				else
					springConfigLocation="${springConfigLocation}/application.properties"
				fi
			fi
			options[${cmdlineargindex}]="--spring.config.location=${springConfigLocation}"
		else
			if [[ "${cmdlineargindex}" -eq $(( $# - 1)) ]]
			then
				if [[ "${cmdlinearg}" =~ ^-.* ]]
				then
					options[${cmdlineargindex}]="${cmdlinearg}"
				else
					image="${cmdlinearg}"
					if [[ "${image}" == *.tar ]]
					then
						warn "This command line format is deprecated. Please replace the final argument ${image} with --docker.tar=${image}"
						options[${cmdlineargindex}]="--docker.tar=${cmdlinearg}"
					else
						warn "This command line format is deprecated. Please replace the final argument ${image} with --docker.image=${image}"
						options[${cmdlineargindex}]="--docker.image=${cmdlinearg}"
					fi
				fi
			else
				options[${cmdlineargindex}]="${cmdlinearg}"
			fi
		fi
		(( cmdlineargindex += 1 ))
	done
}

##################
# Start script
##################

if [ $# -lt 1 ]
then
    printUsage
    exit -1
fi

if [ \( "$1" = -v \) -o \( "$1" = --version \) ]
then
	echo "$(basename $0) ${version}"
	exit 0
fi

if [ \( "$1" = -j \) -o \( "$1" = --pulljar \) ]
then
	deriveLatestReleasedFilename
    deriveJarDetails
	curl ${DOCKER_INSPECTOR_CURL_OPTS} ${CURL_PROXY_OPTIONS} --fail -L -o "${latestReleasedFilename}" "${latestReleasedJarUrl}"
	if [[ $? -ne 0 ]]
	then
		err "Download of ${latestReleasedJarUrl} failed."
		exit -1
	fi
	log "Saved ${latestReleasedJarUrl} to $(pwd)"
	exit 0
fi

preProcessOptions "$@"

log "Jar dir: ${DOCKER_INSPECTOR_JAR_DIR}"
mkdir -p "${DOCKER_INSPECTOR_JAR_DIR}"

if [[ ${jarPathSpecifiedOnCmdLine} == true ]]
then
	jarPath="${userSpecifiedJarPath}"
else
	prepareLatestJar
	jarPath="${downloadedJarPath}"
fi

log "jarPath: ${jarPath}"
log "Options: ${options[*]}"
log "Jar dir: ${DOCKER_INSPECTOR_JAR_DIR}"
java "${encodingSetting}" ${DOCKER_INSPECTOR_JAVA_OPTS} -jar "${jarPath}" ${options[*]} ${hubUsernameArgument} ${hubProjectNameArgument} ${hubProjectVersionArgument}
status=$?
log "Return code: ${status}"
exit ${status}
