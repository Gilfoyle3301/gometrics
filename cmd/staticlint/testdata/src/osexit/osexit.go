package main

import "os"

type fakeOS struct{}

func (fakeOS) Exit(int) {}

func main() {
	os.Exit(1) // want `direct os.Exit call in func main is forbidden`

	exit := os.Exit
	exit(0)

	local := fakeOS{}
	local.Exit(1)

	helper()
}

func helper() {
	os.Exit(2)
}
