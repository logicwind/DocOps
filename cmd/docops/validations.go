package main

var validKinds = map[string]bool{
	"CTX": true,
	"ADR": true,
	"TP":  true,
}

func validKind(k string) bool {
	return validKinds[k]
}