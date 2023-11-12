# vim:ft=make:

.PHONY: all build clean update demo

all: build

build: rm-ip-to-cloudprovider
	@go fmt
	@go build

rm-ip-to-cloudprovider:
	@file="./ip-to-cloudprovider"; \
	if [ -f "$$file" ]; then \
		rm ./$$file; \
		echo "Removed old binary"; \
	fi

update: 
	echo "Updateing all cloudprovider ip ranges"; \
	./ip-to-cloudprovider -a	

demo: 
	./ip-to-cloudprovider check-file demo_ips.txt 

clean:
	@rm -f ip-to-cloudprovider
