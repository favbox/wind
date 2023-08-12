package main

import (
	"fmt"

	"github.com/bytedance/gopkg/lang/mcache"
)

func main() {
	buf := mcache.Malloc(10)
	defer mcache.Free(buf)
	fmt.Println(len(buf), cap(buf))
}
