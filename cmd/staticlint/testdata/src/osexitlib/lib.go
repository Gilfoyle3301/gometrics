package osexitlib

import "os"

func Fail() {
	os.Exit(1)
}

func main() {
	os.Exit(1)
}
