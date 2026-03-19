.PHONY: encode encodeorig decode decodeorig cpuprof fuzz build

encode: build
	@cmd/cmd

encodev2: build
	@cmd/cmd --v2

encodeorig: build
	@cmd/cmd --original

encodeorigv2: build
	@cmd/cmd --v2 --original

decode: build
	@cmd/cmd --decode

decodev2: build
	@cmd/cmd --v2 --decode

decodeorig: build
	@cmd/cmd --original --decode

decodeorigv2: build
	@cmd/cmd --v2 --original --decode

build:
	@go build -o cmd/cmd cmd/main.go

cpuprof: build
	@echo "showing cpu benchmark stats"
	go tool pprof cmd/cmd cpu.prof

fuzz:
	@go test -fuzz=FuzzUnmarshalToMap
