#!/usr/bin/env bats

# BCrypt hash of "mysecret".
SECRET_HASH='$2a$10$I.NhoT1acD9XkXmXn1IMSOp0qhZDd63iSw1RfHZP7nzyg/ItX5eVa'

# The shared instance every normal test talks to.
MAIN_PORT=8080
# A second instance, for the test that needs a short expiry window. PORT lets it
# run alongside the shared one instead of restarting it.
EXPIRY_PORT=18080

# Starts a server and blocks until it is actually serving.
# $1 is the port (default $MAIN_PORT), $2 is UPLOAD_TIMEOUT_SECS (default 240).
# Output goes to a per-port log so bats doesn't wait on the inherited
# descriptors, and so a failure to start can be shown.
start_server() {
    local port="${1:-$MAIN_PORT}" timeout="${2:-240}"
    PORT="$port" UPLOAD_TIMEOUT_SECS="$timeout" FILEWAY_SECRET_HASHES="$SECRET_HASH" \
        test/fileway > "test/server-$port.log" 2>&1 &
    local pid=$!
    # Through a file, not a variable: setup_file, the tests and teardown_file
    # each run in their own shell.
    echo "$pid" > "test/server-$port.pid"

    local t
    for t in $(seq 40); do
        # A dead pid means it never bound - almost always the port being taken.
        # Catch that here, rather than letting the readiness check pass because
        # something else is answering on that port.
        if ! kill -0 "$pid" 2>/dev/null; then
            echo "server for port $port exited at startup; log follows:" >&2
            cat "test/server-$port.log" >&2
            return 1
        fi
        # Check it is really fileway answering, for the same reason.
        if curl -sf "http://localhost:$port/" 2>/dev/null | grep -q Fileway; then
            return 0
        fi
        sleep 0.25
    done
    echo "server on port $port did not come up within 10s; log follows:" >&2
    cat "test/server-$port.log" >&2
    return 1
}

# Stops a server and blocks until its port stops answering, so a following
# start_server can't attach to the outgoing instance. Kills the recorded pid
# rather than every fileway on the machine, which would take down an instance
# the developer happens to be running.
stop_server() {
    local port="${1:-$MAIN_PORT}"
    if [ -f "test/server-$port.pid" ]; then
        kill "$(cat "test/server-$port.pid")" 2>/dev/null || true
        rm -f "test/server-$port.pid"
    fi
    local t
    for t in $(seq 40); do
        if ! curl -sf -o /dev/null "http://localhost:$port/" 2>/dev/null; then
            return 0
        fi
        sleep 0.25
    done
    echo "server on port $port still answering after 10s" >&2
    return 1
}

# Kills the uploader this test started, and only that one.
#
# The obvious `pkill -f fileway_ul.py` matches on the whole command line, so it
# also kills anything that merely mentions the script: an editor with the file
# open, a grep, a shell whose command happens to contain the name. It killed
# this suite's own runner more than once. Matching on the recorded pid, and on
# children of that pid, cannot hit a bystander.
#
# The zip test wraps the script in `bash -c`, so $! may be the wrapper rather
# than the script itself; pkill -P covers that without depending on whether
# bash chose to exec.
stop_uploader() {
    [ -n "${UPLOADER_PID:-}" ] || return 0

    # By the time we get here the uploader has normally handed over its last
    # chunk and is on its way out, so give it a moment rather than killing it
    # mid-exit. The text test is the one that notices: its payload is delivered
    # the instant the downloader connects, so it is still winding down when the
    # assertion passes.
    local t
    for t in $(seq 20); do
        kill -0 "$UPLOADER_PID" 2>/dev/null || break
        sleep 0.1
    done

    pkill -9 -P "$UPLOADER_PID" 2>/dev/null || true
    kill -9 "$UPLOADER_PID" 2>/dev/null || true

    # Reap the job. Without this bash reports the signal itself, printing a
    # "Killed" line next to a passing test that reads like a failure.
    wait "$UPLOADER_PID" 2>/dev/null || true
    UPLOADER_PID=""
}

setup_file() {
    mkdir -p test/src
    make build-instance
    cp bin/fileway test/
    dd if=/dev/urandom of=test/src/rnd1.bin bs=16749170 count=1
    dd if=/dev/urandom of=test/src/rnd2.bin bs=7536170 count=1
    start_server
}

@test "App is reachable" {
    curl -sf "http://localhost:$MAIN_PORT" > /dev/null
}

# The server bakes its own address into the script it serves, so a script that
# is going to talk to another instance has to be fetched from that instance.
# $1 is the port (default $MAIN_PORT), $2 the destination file.
dld_python_script() {
    local port="${1:-$MAIN_PORT}" out="${2:-test/fileway_ul.py}"
    curl -so "$out" "http://localhost:$port/fileway_ul.py"
    chmod +x "$out"
}

@test "Download the uploader script" {
    dld_python_script
    ls test/fileway_ul.py
}

@test "Python upload (simple)" {
    dld_python_script
    cd test/src
    # Empty the file here, in the test's own shell, before anything reads it. The
    # uploader's own `>` would do it, but that happens in a background child, so
    # a read can win the race and see the previous test's URL instead of nothing.
    : > ../output
    FILEWAY_SECRET="mysecret" ../fileway_ul.py rnd1.bin 2>&1 > ../output &
    UPLOADER_PID=$!
    sleep 1
    cd .. # test/
    URL=$(cat output | grep "a browser" | awk '{print $5}')
    curl -OJ $URL
    HASH1=$(cd src/ && md5sum rnd1.bin)
    HASH2=$(md5sum rnd1.bin)
    [[ "$HASH1" == "$HASH2" ]]
}

# Blocks until $2 appears in $1, or until $3 seconds have gone by (default 15).
#
# One quiet grep per attempt, and the loop itself decides the return value. An
# earlier version grepped a second time after the loop to produce that value,
# which meant a match found by the first grep could be contradicted by the
# second one if the file changed in between - reporting a failure while printing
# the line it had just matched.
wait_for_grep_in_file() {
    local file="$1"
    local pattern="$2"
    local timeout="${3:-15}"
    local t

    for (( t = 0; t < timeout; t++ )); do
        if grep -q "$pattern" "$file" 2>/dev/null; then
            return 0
        fi
        sleep 1
    done

    echo "'$pattern' did not appear in $file within ${timeout}s; contents follow:" >&2
    cat "$file" >&2 || true
    return 1
}

@test "Python upload (zip)" {
    dld_python_script
    cd test/src
    : > ../output
    bash -c "FILEWAY_SECRET='mysecret' ../fileway_ul.py --zip rnd1.bin rnd2.bin 2>&1 > ../output" &
    UPLOADER_PID=$!
    wait_for_grep_in_file ../output browser 15
    sleep 1
    cd .. # test/
    URL=$(cat output | grep "a browser" | awk '{print $5}')
    curl -OJ $URL
    find . -name "*.zip" -exec unzip -o {} \;
    HASH1=$(cd src/ && md5sum rnd1.bin)
    HASH2=$(md5sum rnd1.bin)
    [[ "$HASH1" == "$HASH2" ]]
    HASH1=$(cd src/ && md5sum rnd2.bin)
    HASH2=$(md5sum rnd2.bin)
    [[ "$HASH1" == "$HASH2" ]]
}


@test "Python upload (text)" {
    dld_python_script
    cd test/src
    : > ../output
    FILEWAY_SECRET="mysecret" ../fileway_ul.py --txt Ciαo 2>&1 > ../output &
    UPLOADER_PID=$!
    sleep 1
    cd .. # test/
    URL=$(cat output | grep "a browser" | awk '{print $5}')
    TEXT=$(curl $URL)
    [[ "$TEXT" == "Ciαo" ]]
}

# When nobody downloads within UPLOAD_TIMEOUT_SECS the server drops the conduit
# and answers 410 from /ping/. The uploader has to report that and exit non-zero
# instead of falling through to a generic error. This is the one path where the
# server, the wire protocol and the python script have to agree, and it is the
# only case in the audit that unit tests cannot reach.
#
# Runs its own instance on $EXPIRY_PORT, so the shared server keeps the
# production timeout and this test cannot disturb the others. Expect ~12s: the
# reaper ticks every 10 seconds regardless of how low the timeout is set.
@test "Python upload (transfer expires)" {
    start_server "$EXPIRY_PORT" 3
    dld_python_script "$EXPIRY_PORT" test/fileway_ul_expiry.py

    cd test/src
    run bash -c "FILEWAY_SECRET='mysecret' timeout 60 ../fileway_ul_expiry.py rnd1.bin 2>&1"
    cd ../.. # repo root

    stop_server "$EXPIRY_PORT"

    [[ "$output" == *"transfer expired"* ]]
    [ "$status" -eq 1 ]
}

teardown() {
    # bats runs teardown in the shell the test body left behind, so the cwd is
    # wherever its last `cd` landed - `test/`, for every uploader test. The paths
    # below are relative to the repo root, so go back there first: otherwise they
    # expand against `test/`, match nothing, and quietly leave `test/output` for
    # the next test to grep.
    cd "$BATS_TEST_DIRNAME/.." || return 1

    # No `killall curl`: every curl in this suite runs in the foreground and is
    # already gone by the time we get here, so killing every curl on the machine
    # could only ever hit somebody else's.
    stop_uploader
    rm -f test/output test/*.bin
}

teardown_file() {
    stop_server "$MAIN_PORT"
    stop_server "$EXPIRY_PORT" # a no-op unless the expiry test left it behind
    rm -rf test/fileway test/output test/src \
           test/fileway_ul.py test/fileway_ul_expiry.py \
           test/server-*.log test/server-*.pid test/*.zip test/*.bin
    make cleanup
}
