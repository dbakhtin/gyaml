.PHONY: encode encodev2 decode decodev2 cpuprof fuzz build
cmd := example/example

encode: build
	@echo ">>> gyaml encoder <<<"
	@$(cmd)
	@echo ">>> go-yaml encoder <<<"
	@$(cmd) --original

encodev2: build
	@echo ">>> gyaml encoder, more compex data <<<"
	@$(cmd) --v2
	@echo ">>> go-yaml encoder, more complex data <<<"
	@$(cmd) --original --v2

decode: build
	@echo ">>> gyaml decoder <<<"
	@$(cmd) --decode
	@echo ">>> go-yaml decoder <<<"
	@$(cmd) --decode --original

decodev2: build
	@echo ">>> gyaml decoder, more complex data <<<"
	@$(cmd) --decode --v2
	@echo ">>> go-yaml decoder, more complex data <<<"
	@$(cmd) --decode --v2 --original

build:
	@go build -o $(cmd) example/main.go

cpuprof: build
	@echo "showing cpu benchmark stats"
	go tool pprof $(cmd) cpu.prof

fuzz:
	@go test -fuzz=FuzzUnmarshalToMap
