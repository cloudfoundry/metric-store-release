#!/usr/bin/env bash

##############################################################################
##
##  scan.cli.wrapper start up script for macOS
##
##############################################################################

# Add default JVM options here. 
# You can also use JAVA_OPTS and SCAN_CLI_OPTS to pass JVM options to this script.
# Finally, the packaged JRE can be overridden by setting BDS_JAVA_HOME
DEFAULT_JVM_OPTS='"-Xms256m" "-Xmx4096m"'

APP_NAME="scan.cli.wrapper"
APP_BASE_NAME=`basename "$0"`

# Use the maximum available, or set MAX_FD != -1 to use that value.
MAX_FD="maximum"

warn ( ) {
    echo "$*"
}

die ( ) {
    echo
    echo "$*"
    echo
    exit 1
}

# OS specific support (must be 'true' or 'false').
cygwin=false
msys=false
darwin=false
case "`uname`" in
  CYGWIN* )
    cygwin=true
    ;;
  Darwin* )
    darwin=true
    ;;
  MINGW* )
    msys=true
    ;;
esac

# Attempt to set APP_HOME
# Resolve links: $0 may be a link
PRG="$0"
# Need this for relative symlinks.
while [ -h "$PRG" ] ; do
    ls=`ls -ld "$PRG"`
    link=`expr "$ls" : '.*-> \(.*\)$'`
    if expr "$link" : '/.*' > /dev/null; then
        PRG="$link"
    else
        PRG=`dirname "$PRG"`"/$link"
    fi
done
SAVED="`pwd`"
cd "`dirname \"$PRG\"`/.." >&-
APP_HOME="`pwd -P`"
cd "$SAVED" >&-

CLASSPATH=$APP_HOME/lib/scan.cli-2024.10.0-standalone.jar

export LANG=en_US.utf8
SCAN_CLI_OPTS="-Done-jar.silent=true -Done-jar.jar.path='$APP_HOME'/lib/cache/scan.cli.impl-standalone.jar $SCAN_CLI_OPTS"
# Determine the Java command to use to start the JVM.
if [ -n "$BDS_JAVA_HOME" ] ; then
    if [ -x "$BDS_JAVA_HOME/jre/sh/java" ] ; then
        # IBM's JDK on AIX uses strange locations for the executables
        JAVACMD="$BDS_JAVA_HOME/jre/sh/java"
    else
        JAVACMD="$BDS_JAVA_HOME/bin/java"
    fi
    if [ ! -x "$JAVACMD" ] ; then
        die "ERROR: BDS_JAVA_HOME is set to an invalid directory: $BDS_JAVA_HOME

Please set the BDS_JAVA_HOME variable in your environment to match the
location of your Java installation."
    fi
else
    JAVA_HOME="$APP_HOME/jre/Contents/Home"
    JAVACMD="$JAVA_HOME/bin/java"
    which "$JAVACMD" >/dev/null 2>&1 || die "ERROR: BDS_JAVA_HOME is not set and the packaged JRE could be found in your PATH.

Please check the location: $APP_HOME/jre in your environment to ensure
the Java installation was not deleted or corrupted."
fi

# Increase the maximum file descriptors if we can.
if [ "$cygwin" = "false" -a "$darwin" = "false" ] ; then
    MAX_FD_LIMIT=`ulimit -H -n`
    if [ $? -eq 0 ] ; then
        if [ "$MAX_FD" = "maximum" -o "$MAX_FD" = "max" ] ; then
            MAX_FD="$MAX_FD_LIMIT"
        fi
        ulimit -n $MAX_FD
        if [ $? -ne 0 ] ; then
            warn "Could not set maximum file descriptor limit: $MAX_FD"
        fi
    else
        warn "Could not query maximum file descriptor limit: $MAX_FD_LIMIT"
    fi
fi

# For Darwin, add options to specify how the application appears in the dock
if $darwin; then
    SCAN_CLI_OPTS="$SCAN_CLI_OPTS -Xdock:name='$APP_NAME' -Xdock:icon='$APP_HOME'/icon/bds-40.icns"
fi

# Split up the JVM_OPTS And SCAN_CLI_OPTS values into an array, following the shell quoting and substitution rules
function splitJvmOpts() {
    JVM_OPTS=("$@")
}
eval splitJvmOpts "$DEFAULT_JVM_OPTS" "$JAVA_OPTS" "$SCAN_CLI_OPTS"

exec "$JAVACMD" "${JVM_OPTS[@]}" -jar "$CLASSPATH"  "$@"
