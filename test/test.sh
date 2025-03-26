#!/usr/bin/env bats

setup_file() {
    mkdir -p test/src
    make build-instance
    cp bin/fileway test/
    dd if=/dev/urandom of=test/src/rnd1.bin bs=16749170 count=1
    dd if=/dev/urandom of=test/src/rnd2.bin bs=7536170 count=1
    FILEWAY_SECRET_HASHES='$2a$10$I.NhoT1acD9XkXmXn1IMSOp0qhZDd63iSw1RfHZP7nzyg/ItX5eVa' test/fileway &
}

@test "App is reachable" {
    sleep 1
    curl http://localhost:8080 > /dev/null
}

dld_python_script() {
    curl -o test/fileway_ul.py http://localhost:8080/fileway_ul.py
    chmod +x test/fileway_ul.py
}

@test "Download the uploader script" {
    dld_python_script
    ls test/fileway_ul.py
}

@test "Python upload (simple)" {
    dld_python_script
    cd test/src
    FILEWAY_SECRET="mysecret" ../fileway_ul.py rnd1.bin 2>&1 > ../output &
    sleep 1
    cd .. # test/
    URL=$(cat output | grep "a browser" | awk '{print $5}')
    curl -OJ $URL
    HASH1=$(cd src/ && md5sum rnd1.bin)
    HASH2=$(md5sum rnd1.bin)
    [[ "$HASH1" == "$HASH2" ]]
}

wait_for_grep_in_file() {
    local file="$1"
    local pattern="$2"
    local timeout="${3:-15}"  # default timeout of 15 seconds if not specified
    local t=0
    
    until grep "$pattern" "$file" || (( t++ >= timeout )); do
        sync
        sleep 1
    done
    
    # Return success if file exists, failure if timeout
    grep "$pattern" "$file" 2>/dev/null
}

@test "Python upload (zip)" {
    dld_python_script
    cd test/src
    bash -c "FILEWAY_SECRET='mysecret' ../fileway_ul.py --zip rnd1.bin rnd2.bin 2>&1 > ../output" &
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
    FILEWAY_SECRET="mysecret" ../fileway_ul.py --txt Ciαo 2>&1 > ../output &
    sleep 1
    cd .. # test/
    URL=$(cat output | grep "a browser" | awk '{print $5}')
    TEXT=$(curl $URL)
    [[ "$TEXT" == "Ciαo" ]]
}

teardown() {
    killall curl || true
    pkill -9 -f fileway_ul.py || true
    rm -f test/output test/*.bin
}

teardown_file() {
    killall curl || true
    pkill -9 -f fileway_ul.py || true
    killall fileway || true
    rm -rf test/fileway test/output test/fileway_ul.py test/src test/*.zip test/*.bin
    make cleanup
}
