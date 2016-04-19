package g

import (
	"fmt"
	"os"
	"runtime"
)

const (
	VERSION = "0.2.0"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	//log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

// display version info.
func HandleVersion(displayVersion bool) {
	if displayVersion {
		fmt.Println(VERSION)
		os.Exit(0)
	}
}
