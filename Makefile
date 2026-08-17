.PHONY: test test-unit test-e2e

# Builds an "instance", with reproducibility data somewhat fake ("0" as epoch and v0.0.999 as version)
build-instance:
	- mkdir bin
	cp reproducible_build.sh src/
	cd src && VERSION="v0.0.999" SOURCE_DATE_EPOCH=0 bash reproducible_build.sh
	mv src/fileway bin/
	rm src/reproducible_build.sh

test: test-unit test-e2e

# Unit and handler level, in-process, no network. Run with the race detector
# because several of these cover concurrency invariants.
test-unit:
	cd src && go test ./... -race

# Acceptance level: builds the real binary, runs the server, downloads the
# python uploader and transfers real files through it.
test-e2e:
	bats test/test.sh

cleanup:
	rm -rf bin
