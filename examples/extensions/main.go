// The example extension files in this directory carry the "ignore" build
// tag: Kit loads them with the Yaegi interpreter at run time, so the Go
// compiler must not build them. That leaves only the _test.go files in
// this package, and "go build ./..." fails on a main package without a
// main function. This stub gives the package a main function so the full
// module builds cleanly. It is not used for anything else.
package main

func main() {}
