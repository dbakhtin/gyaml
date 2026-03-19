.PHONY: encode encodev2 decode decodev2 cpuprof fuzz build

encode: build
	@echo ">>> gyaml encoder <<<"
	@cmd/cmd
	@echo ">>> go-yaml encoder <<<"
	@cmd/cmd --original

encodev2: build
	@echo ">>> gyaml encoder, more compex data <<<"
	@cmd/cmd --v2
	@echo ">>> go-yaml encoder, more complex data <<<"
	@cmd/cmd --original --v2

decode: build
	@echo ">>> gyaml decoder <<<"
	@cmd/cmd --decode
	@echo ">>> go-yaml decoder <<<"
	@cmd/cmd --original --decode

decodev2: build
	@echo ">>> gyaml decoder, more complex data <<<"
	@cmd/cmd --decode --v2
	@echo ">>> go-yaml decoder, more complex data <<<"
	@cmd/cmd --original --decode --v2

build:
	@go build -o cmd/cmd cmd/main.go

cpuprof: build
	@echo "showing cpu benchmark stats"
	go tool pprof cmd/cmd cpu.prof

fuzz:
	@go test -fuzz=FuzzUnmarshalToMap
