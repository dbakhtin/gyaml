gccheck:
	GODEBUG=gctrace=1 go run cmd/main.go

gccheckorig:
	GODEBUG=gctrace=1 go run cmd/main.go --original