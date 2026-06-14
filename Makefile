# vim:ft=make:

.PHONY: all build clean update demo test

all: test build update

test:
	@NO_COLOR=true go test ./... 

build: clean
	@go fmt ./...
	@go build -o ip-to-cloudprovider .

update:
	./ip-to-cloudprovider -a

demo:
	./ip-to-cloudprovider check-file demo_ips.txt

clean:
	@rm -f ip-to-cloudprovider
