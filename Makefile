.PHONY: test

# Builds an "instance", with reproducibility data somewhat fake ("0" as epoch and v0.0.999 as version)
build-instance:
	- mkdir bin
	cp reproducible_build.sh src/
	cd src && VERSION="v0.0.999" SOURCE_DATE_EPOCH=0 bash reproducible_build.sh
	mv src/fileway bin/
	rm src/reproducible_build.sh

test:
	bats test/test.sh

cleanup:
	rm -rf bin
